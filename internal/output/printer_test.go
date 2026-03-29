package output

import (
	"testing"
)

func TestHostSummary(t *testing.T) {
	s := HostSummary{OK: 3, Changed: 2, Failed: 1}
	if s.OK != 3 || s.Changed != 2 || s.Failed != 1 {
		t.Errorf("unexpected HostSummary values: %+v", s)
	}
}

func TestHostSummary_Zero(t *testing.T) {
	var s HostSummary
	if s.OK != 0 || s.Changed != 0 || s.Failed != 0 {
		t.Error("zero-value HostSummary should have all zero counts")
	}
}

// The following tests verify that output functions do not panic.
// They write to stdout, which is acceptable in test output.

func TestPlayBanner(t *testing.T) {
	PlayBanner("Deploy app", "web")
	PlayBanner("", "")
}

func TestTaskBanner(t *testing.T) {
	TaskBanner("Run migrations")
	TaskBanner("")
}

func TestOK(t *testing.T) {
	OK("web1", "echo hello", false, "", "")
	OK("web1", "echo hello", true, "stdout line", "stderr line")
	OK("web1", "echo hello", true, "", "")
}

func TestChanged(t *testing.T) {
	Changed("web1", "uploaded file", false, "", "")
	Changed("web1", "uploaded file", true, "some output", "")
	Changed("web1", "uploaded file", true, "out", "err")
}

func TestFailed(t *testing.T) {
	Failed("web1", "exit code 1", "", "")
	Failed("web1", "connection refused", "stdout", "stderr")
	Failed("web1", "error", "multiline\nstdout", "multiline\nstderr")
}

func TestIgnored(t *testing.T) {
	Ignored("web1", "connection error ignored")
	Ignored("web1", "")
}

func TestSkipped(t *testing.T) {
	Skipped("web1", "when condition not met")
	Skipped("web1", "")
}

func TestDryRun(t *testing.T) {
	DryRun("web1", "echo hello")
	DryRun("web1", "")
}

func TestSummary_WithResults(t *testing.T) {
	Summary(map[string]*HostSummary{
		"web1": {OK: 5, Changed: 2, Failed: 0},
		"web2": {OK: 1, Changed: 0, Failed: 3},
	})
}

func TestSummary_Empty(t *testing.T) {
	Summary(map[string]*HostSummary{})
}

func TestSummary_AllFailed(t *testing.T) {
	Summary(map[string]*HostSummary{
		"db1": {OK: 0, Changed: 0, Failed: 5},
	})
}
