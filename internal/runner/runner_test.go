package runner

import (
	"testing"

	"workflow/internal/config"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]bool
	}{
		{"", map[string]bool{}},
		{"a", map[string]bool{"a": true}},
		{"a,b,c", map[string]bool{"a": true, "b": true, "c": true}},
		{"a, b , c", map[string]bool{"a": true, "b": true, "c": true}},
		{"a,,b", map[string]bool{"a": true, "b": true}},
		{" , , ", map[string]bool{}},
	}
	for _, tt := range tests {
		got := parseCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseCSV(%q): want len=%d got len=%d (%v)", tt.input, len(tt.want), len(got), got)
			continue
		}
		for k := range tt.want {
			if !got[k] {
				t.Errorf("parseCSV(%q): missing key %q in result %v", tt.input, k, got)
			}
		}
	}
}

func TestHasAnyTag(t *testing.T) {
	filter := map[string]bool{"deploy": true, "restart": true}

	if !hasAnyTag([]string{"deploy"}, filter) {
		t.Error("expected true for exact matching tag")
	}
	if !hasAnyTag([]string{"other", "restart"}, filter) {
		t.Error("expected true when one of many tags matches")
	}
	if hasAnyTag([]string{"other", "noop"}, filter) {
		t.Error("expected false when no tags match")
	}
	if hasAnyTag([]string{}, filter) {
		t.Error("expected false for empty task tags")
	}
	if hasAnyTag(nil, filter) {
		t.Error("expected false for nil task tags")
	}
	// empty filter always returns false (no tags required)
	if hasAnyTag([]string{"deploy"}, map[string]bool{}) {
		t.Error("expected false for empty filter")
	}
}

func TestIntersect(t *testing.T) {
	h1 := &config.HostDef{Name: "web1"}
	h2 := &config.HostDef{Name: "web2"}
	h3 := &config.HostDef{Name: "db1"}

	// Partial overlap
	result := intersect([]*config.HostDef{h1, h2}, []*config.HostDef{h2, h3})
	if len(result) != 1 || result[0].Name != "web2" {
		t.Errorf("partial overlap: want [web2], got %v", hostNames(result))
	}

	// No overlap
	result = intersect([]*config.HostDef{h1, h2}, []*config.HostDef{h3})
	if len(result) != 0 {
		t.Errorf("no overlap: want empty, got %v", hostNames(result))
	}

	// Full overlap
	result = intersect([]*config.HostDef{h1, h2}, []*config.HostDef{h1, h2})
	if len(result) != 2 {
		t.Errorf("full overlap: want 2, got %d", len(result))
	}

	// Empty first list
	result = intersect([]*config.HostDef{}, []*config.HostDef{h1})
	if len(result) != 0 {
		t.Errorf("empty first list: want empty, got %v", hostNames(result))
	}

	// Empty second list
	result = intersect([]*config.HostDef{h1}, []*config.HostDef{})
	if len(result) != 0 {
		t.Errorf("empty second list: want empty, got %v", hostNames(result))
	}
}

func TestTaskLabel(t *testing.T) {
	tests := []struct {
		task *config.Task
		want string
	}{
		{&config.Task{Name: "My task"}, "My task"},
		{&config.Task{Shell: "echo hi"}, "shell: echo hi"},
		{&config.Task{Copy: &config.CopyArgs{Src: "a", Dest: "b"}}, "copy: a → b"},
		{&config.Task{Fetch: &config.FetchArgs{Src: "/remote/path"}}, "fetch: /remote/path"},
		{&config.Task{Template: &config.TemplateArgs{Src: "tmpl.j2"}}, "template: tmpl.j2"},
		{&config.Task{HTTP: &config.HTTPArgs{Method: "POST", URL: "http://x"}}, "POST http://x"},
		{&config.Task{HTTP: &config.HTTPArgs{URL: "http://x"}}, "GET http://x"},
		{&config.Task{}, "unknown task"},
	}
	for _, tt := range tests {
		got := taskLabel(tt.task)
		if got != tt.want {
			t.Errorf("taskLabel: want %q, got %q", tt.want, got)
		}
	}
}

// TaskLabel: Name takes priority over all action fields
func TestTaskLabel_NamePriority(t *testing.T) {
	task := &config.Task{
		Name:  "named task",
		Shell: "echo hi",
	}
	if got := taskLabel(task); got != "named task" {
		t.Errorf("want 'named task', got %q", got)
	}
}

// helper
func hostNames(hosts []*config.HostDef) []string {
	names := make([]string, len(hosts))
	for i, h := range hosts {
		names[i] = h.Name
	}
	return names
}
