package runner

import (
	"testing"

	"workflow/internal/config"
)

func TestNewExecutor_BuiltinVars(t *testing.T) {
	host := &config.HostDef{
		Name: "web1",
		Host: "192.168.1.1",
		User: "ubuntu",
	}
	e := newExecutor(host, nil, Options{})

	if e.vars["host"] != "web1" {
		t.Errorf("want host=web1, got %v", e.vars["host"])
	}
	if e.vars["hostname"] != "192.168.1.1" {
		t.Errorf("want hostname=192.168.1.1, got %v", e.vars["hostname"])
	}
	if e.vars["user"] != "ubuntu" {
		t.Errorf("want user=ubuntu, got %v", e.vars["user"])
	}
}

func TestNewExecutor_PlayVarsMerged(t *testing.T) {
	host := &config.HostDef{Name: "web1", Host: "1.1.1.1", User: "root"}
	playVars := map[string]any{
		"play_var": "play_val",
		"shared":   "from_play",
	}
	e := newExecutor(host, playVars, Options{})

	if e.vars["play_var"] != "play_val" {
		t.Errorf("play var not set: %v", e.vars["play_var"])
	}
}

func TestNewExecutor_HostVarsOverridePlayVars(t *testing.T) {
	host := &config.HostDef{
		Name: "web1",
		Host: "1.1.1.1",
		User: "root",
		Vars: map[string]any{
			"shared":   "from_host",
			"host_var": "host_val",
		},
	}
	playVars := map[string]any{"shared": "from_play"}
	e := newExecutor(host, playVars, Options{})

	if e.vars["shared"] != "from_host" {
		t.Errorf("host vars should override play vars, got %v", e.vars["shared"])
	}
	if e.vars["host_var"] != "host_val" {
		t.Errorf("host-specific var not set: %v", e.vars["host_var"])
	}
}

func TestRenderString_Substitution(t *testing.T) {
	e := &executor{
		vars: map[string]any{
			"name": "world",
			"port": 8080,
		},
	}
	tests := []struct {
		input string
		want  string
	}{
		{"hello {{ .name }}", "hello world"},
		{"port={{ .port }}", "port=8080"},
		{"no template here", "no template here"},
		{"", ""},
	}
	for _, tt := range tests {
		got, err := e.renderString(tt.input)
		if err != nil {
			t.Errorf("renderString(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("renderString(%q): want %q, got %q", tt.input, tt.want, got)
		}
	}
}

func TestRenderString_MissingKeyZero(t *testing.T) {
	e := &executor{vars: map[string]any{}}
	// missingkey=zero: missing map key renders as <no value>
	got, err := e.renderString("val={{ .missing }}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "val=<no value>" {
		t.Errorf("want 'val=<no value>', got %q", got)
	}
}

func TestRenderString_InvalidTemplate(t *testing.T) {
	e := &executor{vars: map[string]any{}}
	_, err := e.renderString("{{ .foo")
	if err == nil {
		t.Error("expected error for unclosed template action")
	}
}

func TestRenderBytes(t *testing.T) {
	e := &executor{
		vars: map[string]any{"greeting": "hello"},
	}
	out, err := e.renderBytes([]byte("{{ .greeting }} world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "hello world" {
		t.Errorf("want 'hello world', got %q", string(out))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"", 5, ""},
		{"exact", 5, "exact"},
		{"toolong", 3, "too…"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d): want %q, got %q", tt.input, tt.n, tt.want, got)
		}
	}
}

func TestExecutorRun_DryRun(t *testing.T) {
	host := &config.HostDef{Name: "web1", Host: "1.1.1.1", User: "root"}
	e := newExecutor(host, nil, Options{DryRun: true})
	task := &config.Task{Shell: "echo hello"}
	result := e.Run(task)
	if result != resultOK {
		t.Errorf("dry-run should return resultOK, got %d", result)
	}
}

func TestExecutorRun_VarsTask(t *testing.T) {
	host := &config.HostDef{Name: "web1", Host: "1.1.1.1", User: "root"}
	e := newExecutor(host, nil, Options{})
	task := &config.Task{
		Vars: map[string]any{
			"new_var": "new_value",
			"another": 42,
		},
	}
	result := e.Run(task)
	if result != resultOK {
		t.Errorf("vars task should return resultOK, got %d", result)
	}
	if e.vars["new_var"] != "new_value" {
		t.Errorf("var not set: %v", e.vars["new_var"])
	}
	if e.vars["another"] != 42 {
		t.Errorf("var not set: %v", e.vars["another"])
	}
}
