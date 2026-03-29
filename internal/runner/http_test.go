package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workflow/internal/config"
)

func TestCheckStatus_DefaultRange(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{301, false},
		{400, false},
		{404, false},
		{500, false},
	}
	for _, tt := range tests {
		got := checkStatus(tt.code, nil)
		if got != tt.want {
			t.Errorf("checkStatus(%d, nil): want %v, got %v", tt.code, tt.want, got)
		}
	}
}

func TestCheckStatus_ExplicitList(t *testing.T) {
	tests := []struct {
		code     int
		expected []int
		want     bool
	}{
		{200, []int{200, 201}, true},
		{201, []int{200, 201}, true},
		{204, []int{200, 201}, false},
		{404, []int{200, 201}, false},
		{404, []int{404}, true},
		{500, []int{500, 503}, true},
	}
	for _, tt := range tests {
		got := checkStatus(tt.code, tt.expected)
		if got != tt.want {
			t.Errorf("checkStatus(%d, %v): want %v, got %v", tt.code, tt.expected, tt.want, got)
		}
	}
}

func TestBuildResponseMap_JSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	body := []byte(`{"id": 123, "name": "test"}`)
	m := buildResponseMap(resp, body, true)

	if m["status"] != 200 {
		t.Errorf("want status=200, got %v", m["status"])
	}
	if m["ok"] != true {
		t.Errorf("want ok=true, got %v", m["ok"])
	}
	if m["body"] != string(body) {
		t.Errorf("want body=%s, got %v", body, m["body"])
	}
	// Top-level JSON fields merged
	if m["id"] != float64(123) {
		t.Errorf("want id=123, got %v", m["id"])
	}
	if m["name"] != "test" {
		t.Errorf("want name=test, got %v", m["name"])
	}
	if m["json"] == nil {
		t.Error("want json field populated")
	}
	if m["headers"] == nil {
		t.Error("want headers field populated")
	}
}

func TestBuildResponseMap_NonJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
	}
	body := []byte("plain text body")
	m := buildResponseMap(resp, body, true)

	if m["json"] != nil {
		t.Errorf("want no json field for plain text, got %v", m["json"])
	}
	if m["body"] != "plain text body" {
		t.Errorf("want body='plain text body', got %v", m["body"])
	}
}

func TestBuildResponseMap_NotOK(t *testing.T) {
	resp := &http.Response{
		StatusCode: 404,
		Header:     http.Header{},
	}
	m := buildResponseMap(resp, []byte("not found"), false)
	if m["ok"] != false {
		t.Errorf("want ok=false, got %v", m["ok"])
	}
	if m["status"] != 404 {
		t.Errorf("want status=404, got %v", m["status"])
	}
}

func TestBuildResponseMap_MultiValueHeader(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"X-Multi": []string{"a", "b"}},
	}
	m := buildResponseMap(resp, []byte{}, true)
	headers := m["headers"].(map[string]any)
	val := headers["X-Multi"]
	if _, ok := val.([]string); !ok {
		t.Errorf("multi-value header should be []string, got %T", val)
	}
}

func TestBuildResponseMap_JSONArray(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	body := []byte(`[1, 2, 3]`)
	m := buildResponseMap(resp, body, true)

	// JSON array is not merged as top-level keys
	if m["json"] == nil {
		t.Error("want json field populated")
	}
	// No extra numeric keys should be present
	if _, exists := m["0"]; exists {
		t.Error("array elements should not be merged as top-level keys")
	}
}

func TestRenderAny_String(t *testing.T) {
	e := &executor{vars: map[string]any{"name": "world"}}
	result, err := renderAny("hello {{ .name }}", e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("want 'hello world', got %v", result)
	}
}

func TestRenderAny_Map(t *testing.T) {
	e := &executor{vars: map[string]any{"env": "prod"}}
	input := map[string]any{
		"env":    "{{ .env }}",
		"static": "value",
	}
	result, err := renderAny(input, e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["env"] != "prod" {
		t.Errorf("want env=prod, got %v", m["env"])
	}
	if m["static"] != "value" {
		t.Errorf("want static=value, got %v", m["static"])
	}
}

func TestRenderAny_Slice(t *testing.T) {
	e := &executor{vars: map[string]any{"item": "rendered"}}
	input := []any{"{{ .item }}", "static", 42}
	result, err := renderAny(input, e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := result.([]any)
	if s[0] != "rendered" {
		t.Errorf("want s[0]=rendered, got %v", s[0])
	}
	if s[1] != "static" {
		t.Errorf("want s[1]=static, got %v", s[1])
	}
	if s[2] != 42 {
		t.Errorf("want s[2]=42, got %v", s[2])
	}
}

func TestRenderAny_Scalar(t *testing.T) {
	e := &executor{vars: map[string]any{}}
	for _, v := range []any{42, 3.14, true, nil} {
		result, err := renderAny(v, e)
		if err != nil {
			t.Errorf("renderAny(%v): unexpected error: %v", v, err)
		}
		if result != v {
			t.Errorf("renderAny(%v): want identity, got %v", v, result)
		}
	}
}

func TestRenderAny_NestedMap(t *testing.T) {
	e := &executor{vars: map[string]any{"val": "deep"}}
	input := map[string]any{
		"outer": map[string]any{
			"inner": "{{ .val }}",
		},
	}
	result, err := renderAny(input, e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	outer := result.(map[string]any)["outer"].(map[string]any)
	if outer["inner"] != "deep" {
		t.Errorf("want inner=deep, got %v", outer["inner"])
	}
}

func TestRunHTTP_GetSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			Method: "GET",
			URL:    server.URL,
		},
	}
	result := e.Run(task)
	if result != resultChanged {
		t.Errorf("want resultChanged for successful HTTP, got %d", result)
	}
}

func TestRunHTTP_DefaultMethodGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{URL: server.URL}, // no Method set
	}
	e.Run(task)
}

func TestRunHTTP_UnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL:          server.URL,
			ExpectStatus: []int{200},
		},
	}
	result := e.Run(task)
	if result != resultFailed {
		t.Errorf("want resultFailed for unexpected status, got %d", result)
	}
}

func TestRunHTTP_IgnoreErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		IgnoreErrors: true,
		HTTP: &config.HTTPArgs{
			URL:          server.URL,
			ExpectStatus: []int{200},
		},
	}
	result := e.Run(task)
	if result != resultChanged {
		t.Errorf("want resultChanged when ignore_errors=true, got %d", result)
	}
}

func TestRunHTTP_Register(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"id": 42, "name": "created"}`))
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL:      server.URL,
			Register: "resp",
		},
	}
	e.Run(task)

	if e.vars["resp"] == nil {
		t.Fatal("expected response registered in vars")
	}
	resp := e.vars["resp"].(map[string]any)
	if resp["status"] != 201 {
		t.Errorf("want status=201, got %v", resp["status"])
	}
	if resp["id"] != float64(42) {
		t.Errorf("want id=42 merged from JSON, got %v", resp["id"])
	}
	if resp["name"] != "created" {
		t.Errorf("want name=created, got %v", resp["name"])
	}
}

func TestRunHTTP_BearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mytoken" {
			t.Errorf("want 'Bearer mytoken', got %q", auth)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL:    server.URL,
			Bearer: "mytoken",
		},
	}
	e.Run(task)
}

func TestRunHTTP_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("basic auth mismatch: user=%s pass=%s ok=%v", user, pass, ok)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL: server.URL,
			BasicAuth: &config.BasicAuthArgs{
				Username: "admin",
				Password: "secret",
			},
		},
	}
	e.Run(task)
}

func TestRunHTTP_JSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("want Content-Type=application/json, got %s", ct)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["key"] != "value" {
			t.Errorf("want key=value, got %v", body["key"])
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			Method: "POST",
			URL:    server.URL,
			JSON:   map[string]any{"key": "value"},
		},
	}
	e.Run(task)
}

func TestRunHTTP_FormBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("want form Content-Type, got %s", ct)
		}
		r.ParseForm()
		if r.FormValue("field") != "val" {
			t.Errorf("want field=val, got %s", r.FormValue("field"))
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			Method: "POST",
			URL:    server.URL,
			Form:   map[string]string{"field": "val"},
		},
	}
	e.Run(task)
}

func TestRunHTTP_TemplateURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	host := &config.HostDef{Name: "myhost", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL: server.URL + "/hosts/{{ .host }}",
		},
	}
	result := e.Run(task)
	if result != resultChanged {
		t.Errorf("want resultChanged, got %d", result)
	}
}

func TestRunHTTP_InvalidURL(t *testing.T) {
	host := &config.HostDef{Name: "local", Host: "localhost", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		HTTP: &config.HTTPArgs{
			URL: "://invalid",
		},
	}
	result := e.Run(task)
	if result != resultFailed {
		t.Errorf("want resultFailed for invalid URL, got %d", result)
	}
}
