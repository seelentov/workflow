package runner

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"workflow/internal/config"
	"workflow/internal/output"
)

func (e *executor) runHTTP(task *config.Task) taskResult {
	args := task.HTTP

	rawURL, err := e.renderString(args.URL)
	if err != nil {
		output.Failed(e.host.Name, "render url: "+err.Error(), "", "")
		return resultFailed
	}

	method := strings.ToUpper(args.Method)
	if method == "" {
		method = "GET"
	}

	// Build request body.
	var bodyReader io.Reader
	autoContentType := ""

	switch {
	case args.JSON != nil:
		rendered, err := renderAny(args.JSON, e)
		if err != nil {
			output.Failed(e.host.Name, "render json: "+err.Error(), "", "")
			return resultFailed
		}
		encoded, err := json.Marshal(rendered)
		if err != nil {
			output.Failed(e.host.Name, "marshal json: "+err.Error(), "", "")
			return resultFailed
		}
		bodyReader = bytes.NewReader(encoded)
		autoContentType = "application/json"

	case args.Body != "":
		rendered, err := e.renderString(args.Body)
		if err != nil {
			output.Failed(e.host.Name, "render body: "+err.Error(), "", "")
			return resultFailed
		}
		bodyReader = strings.NewReader(rendered)

	case len(args.Form) > 0:
		form := url.Values{}
		for k, v := range args.Form {
			rv, err := e.renderString(v)
			if err != nil {
				output.Failed(e.host.Name, "render form field "+k+": "+err.Error(), "", "")
				return resultFailed
			}
			form.Set(k, rv)
		}
		bodyReader = strings.NewReader(form.Encode())
		autoContentType = "application/x-www-form-urlencoded"
	}

	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		output.Failed(e.host.Name, "build request: "+err.Error(), "", "")
		return resultFailed
	}

	// Auto content-type (can be overridden by explicit header).
	if autoContentType != "" {
		req.Header.Set("Content-Type", autoContentType)
	}

	// User-defined headers (may override Content-Type).
	for k, v := range args.Headers {
		rv, err := e.renderString(v)
		if err != nil {
			output.Failed(e.host.Name, "render header "+k+": "+err.Error(), "", "")
			return resultFailed
		}
		req.Header.Set(k, rv)
	}

	// Bearer token (shorthand; overrides Authorization header if also set).
	if args.Bearer != "" {
		token, err := e.renderString(args.Bearer)
		if err != nil {
			output.Failed(e.host.Name, "render bearer: "+err.Error(), "", "")
			return resultFailed
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Basic auth.
	if args.BasicAuth != nil {
		user, _ := e.renderString(args.BasicAuth.Username)
		pass, _ := e.renderString(args.BasicAuth.Password)
		req.SetBasicAuth(user, pass)
	}

	// Build HTTP client.
	timeout := 30 * time.Second
	if args.Timeout != "" {
		if d, err := time.ParseDuration(args.Timeout); err == nil {
			timeout = d
		}
	}

	transport := &http.Transport{}
	if args.VerifySSL != nil && !*args.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	followRedirects := true
	if args.FollowRedirects != nil {
		followRedirects = *args.FollowRedirects
	}

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if !followRedirects {
		httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		output.Failed(e.host.Name, "request: "+err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		output.Failed(e.host.Name, "read response body: "+err.Error(), "", "")
		return resultFailed
	}

	// Determine if status is acceptable.
	statusOK := checkStatus(resp.StatusCode, args.ExpectStatus)

	// Register response as a variable.
	if args.Register != "" {
		e.vars[args.Register] = buildResponseMap(resp, respBody, statusOK)
	}

	summary := fmt.Sprintf("%s %s → %d", method, truncate(rawURL, 45), resp.StatusCode)

	if !statusOK {
		msg := fmt.Sprintf("unexpected status %d", resp.StatusCode)
		output.Failed(e.host.Name, msg, string(respBody), "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, summary)
			return resultChanged
		}
		return resultFailed
	}

	output.Changed(e.host.Name, summary, e.opts.Verbose, string(respBody), "")
	return resultChanged
}

// checkStatus returns true if code is acceptable.
func checkStatus(code int, expected []int) bool {
	if len(expected) == 0 {
		return code >= 200 && code < 300
	}
	for _, c := range expected {
		if code == c {
			return true
		}
	}
	return false
}

// buildResponseMap builds the map stored in register variable.
// Top-level JSON fields are merged in for convenient access: {{ .reg.id }}.
func buildResponseMap(resp *http.Response, body []byte, ok bool) map[string]any {
	m := map[string]any{
		"status": resp.StatusCode,
		"body":   string(body),
		"ok":     ok,
	}

	// Flatten response headers.
	headers := make(map[string]any, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) == 1 {
			headers[k] = v[0]
		} else {
			headers[k] = v
		}
	}
	m["headers"] = headers

	// Parse JSON body if applicable.
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") || json.Valid(body) {
		var parsed any
		if err := json.Unmarshal(body, &parsed); err == nil {
			m["json"] = parsed
			// Merge top-level map keys for convenient {{ .reg.field }} access.
			if obj, ok := parsed.(map[string]any); ok {
				for k, v := range obj {
					m[k] = v
				}
			}
		}
	}

	return m
}

// renderAny recursively renders template strings inside arbitrary values.
func renderAny(v any, e *executor) (any, error) {
	switch val := v.(type) {
	case string:
		return e.renderString(val)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			r, err := renderAny(v2, e)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(val))
		for i, v2 := range val {
			r, err := renderAny(v2, e)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	default:
		return v, nil
	}
}
