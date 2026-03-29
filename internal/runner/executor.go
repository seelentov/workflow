package runner

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"

	"workflow/internal/config"
	"workflow/internal/output"
	sshclient "workflow/internal/ssh"
)

type taskResult int

const (
	resultOK      taskResult = iota
	resultChanged taskResult = iota
	resultFailed  taskResult = iota
)

// executor runs tasks on a single host.
type executor struct {
	host *config.HostDef
	vars map[string]any
	opts Options
}

func newExecutor(host *config.HostDef, playVars map[string]any, opts Options) *executor {
	// Merge play vars + host vars.
	merged := make(map[string]any)
	for k, v := range playVars {
		merged[k] = v
	}
	for k, v := range host.Vars {
		merged[k] = v
	}
	// Built-in variables.
	merged["host"] = host.Name
	merged["hostname"] = host.Host
	merged["user"] = host.User

	return &executor{host: host, vars: merged, opts: opts}
}

// Run executes a single task and returns its outcome.
func (e *executor) Run(task *config.Task) taskResult {
	if e.opts.DryRun {
		output.DryRun(e.host.Name, taskLabel(task))
		return resultOK
	}

	// vars — no SSH needed.
	if task.Vars != nil {
		for k, v := range task.Vars {
			e.vars[k] = v
		}
		output.OK(e.host.Name, "vars set", e.opts.Verbose, "", "")
		return resultOK
	}

	// http — runs on the control machine, no SSH needed.
	if task.HTTP != nil {
		return e.runHTTP(task)
	}

	// All remaining task types require an SSH connection.
	return e.runSSH(task)
}

// runSSH connects to the host and dispatches SSH-based tasks.
func (e *executor) runSSH(task *config.Task) taskResult {
	client, err := sshclient.Connect(sshclient.Config{
		Host:     e.host.Host,
		Port:     e.host.Port,
		User:     e.host.User,
		Password: e.host.Password,
		KeyFile:  e.host.KeyFile,
	})
	if err != nil {
		output.Failed(e.host.Name, err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "connection error ignored")
			return resultChanged
		}
		return resultFailed
	}
	defer client.Close()

	// Check `when` condition via SSH.
	if task.When != "" {
		cond, err := e.renderString(task.When)
		if err != nil {
			output.Failed(e.host.Name, "render when: "+err.Error(), "", "")
			return resultFailed
		}
		res, err := client.Run(cond)
		if err != nil || res.ExitCode != 0 {
			output.Skipped(e.host.Name, "when condition not met")
			return resultOK
		}
	}

	switch {
	case task.Shell != "":
		return e.runShell(client, task)
	case task.Copy != nil:
		return e.runCopy(client, task)
	case task.Fetch != nil:
		return e.runFetch(client, task)
	case task.Template != nil:
		return e.runTemplate(client, task)
	default:
		output.Failed(e.host.Name, "no action defined in task", "", "")
		return resultFailed
	}
}

func (e *executor) runShell(client *sshclient.Client, task *config.Task) taskResult {
	cmd, err := e.renderString(task.Shell)
	if err != nil {
		output.Failed(e.host.Name, "render shell: "+err.Error(), "", "")
		return resultFailed
	}

	res, err := client.Run(cmd)
	if err != nil {
		output.Failed(e.host.Name, err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}

	if res.ExitCode != 0 {
		msg := fmt.Sprintf("exit code %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
		output.Failed(e.host.Name, msg, res.Stdout, res.Stderr)
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}

	summary := truncate(cmd, 60)
	if strings.TrimSpace(res.Stdout) != "" {
		output.Changed(e.host.Name, summary, e.opts.Verbose, res.Stdout, res.Stderr)
		return resultChanged
	}
	output.OK(e.host.Name, summary, e.opts.Verbose, res.Stdout, res.Stderr)
	return resultOK
}

func (e *executor) runCopy(client *sshclient.Client, task *config.Task) taskResult {
	src, err := e.renderString(task.Copy.Src)
	if err != nil {
		output.Failed(e.host.Name, "render src: "+err.Error(), "", "")
		return resultFailed
	}
	dest, err := e.renderString(task.Copy.Dest)
	if err != nil {
		output.Failed(e.host.Name, "render dest: "+err.Error(), "", "")
		return resultFailed
	}

	mode := os.FileMode(0o644)
	if task.Copy.Mode != "" {
		n, err := strconv.ParseUint(task.Copy.Mode, 8, 32)
		if err != nil {
			output.Failed(e.host.Name, "invalid mode: "+task.Copy.Mode, "", "")
			return resultFailed
		}
		mode = os.FileMode(n)
	}

	if err := client.Upload(src, dest, mode); err != nil {
		output.Failed(e.host.Name, "copy: "+err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}

	output.Changed(e.host.Name, src+" → "+dest, e.opts.Verbose, "", "")
	return resultChanged
}

func (e *executor) runFetch(client *sshclient.Client, task *config.Task) taskResult {
	src, err := e.renderString(task.Fetch.Src)
	if err != nil {
		output.Failed(e.host.Name, "render src: "+err.Error(), "", "")
		return resultFailed
	}
	dest, err := e.renderString(task.Fetch.Dest)
	if err != nil {
		output.Failed(e.host.Name, "render dest: "+err.Error(), "", "")
		return resultFailed
	}
	// Replace {{ .host }} in dest path.
	dest = strings.ReplaceAll(dest, "{{ .host }}", e.host.Name)

	if err := client.Download(src, dest); err != nil {
		output.Failed(e.host.Name, "fetch: "+err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}

	output.Changed(e.host.Name, src+" → "+dest, e.opts.Verbose, "", "")
	return resultChanged
}

func (e *executor) runTemplate(client *sshclient.Client, task *config.Task) taskResult {
	src, err := e.renderString(task.Template.Src)
	if err != nil {
		output.Failed(e.host.Name, "render src: "+err.Error(), "", "")
		return resultFailed
	}
	dest, err := e.renderString(task.Template.Dest)
	if err != nil {
		output.Failed(e.host.Name, "render dest: "+err.Error(), "", "")
		return resultFailed
	}

	tmplData, err := os.ReadFile(src)
	if err != nil {
		output.Failed(e.host.Name, "read template: "+err.Error(), "", "")
		return resultFailed
	}

	rendered, err := e.renderBytes(tmplData)
	if err != nil {
		output.Failed(e.host.Name, "render template: "+err.Error(), "", "")
		return resultFailed
	}

	mode := os.FileMode(0o644)
	if task.Template.Mode != "" {
		n, err := strconv.ParseUint(task.Template.Mode, 8, 32)
		if err != nil {
			output.Failed(e.host.Name, "invalid mode: "+task.Template.Mode, "", "")
			return resultFailed
		}
		mode = os.FileMode(n)
	}

	if err := client.UploadBytes(rendered, dest, mode); err != nil {
		output.Failed(e.host.Name, "template upload: "+err.Error(), "", "")
		if task.IgnoreErrors {
			output.Ignored(e.host.Name, "")
			return resultChanged
		}
		return resultFailed
	}

	output.Changed(e.host.Name, src+" → "+dest, e.opts.Verbose, "", "")
	return resultChanged
}

func (e *executor) renderString(s string) (string, error) {
	rendered, err := e.renderBytes([]byte(s))
	return string(rendered), err
}

func (e *executor) renderBytes(data []byte) ([]byte, error) {
	tmpl, err := template.New("").Option("missingkey=zero").Parse(string(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.vars); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
