package output

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
)

var mu sync.Mutex

var (
	cyan   = color.New(color.FgCyan, color.Bold).SprintFunc()
	green  = color.New(color.FgGreen, color.Bold).SprintFunc()
	yellow = color.New(color.FgYellow, color.Bold).SprintFunc()
	red    = color.New(color.FgRed, color.Bold).SprintFunc()
	faint  = color.New(color.Faint).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

// PlayBanner prints the play header.
func PlayBanner(name, hosts string) {
	mu.Lock()
	defer mu.Unlock()
	line := strings.Repeat("*", 60)
	fmt.Println()
	fmt.Println(cyan(line))
	fmt.Printf("%s PLAY: %s  [hosts: %s]\n", cyan("*"), bold(name), hosts)
	fmt.Println(cyan(line))
}

// TaskBanner prints the task header.
func TaskBanner(name string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\n%s TASK: %s\n", yellow("---"), bold(name))
	fmt.Println(faint(strings.Repeat("-", 60)))
}

// OK prints a successful result.
func OK(host, summary string, verbose bool, stdout, stderr string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", green("ok"), cyan(host), faint(summary))
	if verbose {
		printOutput(stdout, stderr)
	}
}

// Changed prints a changed result (command produced output).
func Changed(host, summary string, verbose bool, stdout, stderr string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", yellow("changed"), cyan(host), faint(summary))
	if verbose {
		printOutput(stdout, stderr)
	}
}

// Failed prints a failure.
func Failed(host, summary string, stdout, stderr string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", red("FAILED"), cyan(host), summary)
	printOutput(stdout, stderr)
}

// Ignored prints an ignored failure.
func Ignored(host, summary string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", yellow("ignored"), cyan(host), faint(summary))
}

// Skipped prints a skipped task.
func Skipped(host, reason string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", faint("skipped"), cyan(host), faint(reason))
}

// DryRun prints a dry-run task.
func DryRun(host, action string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("%s [%s] %s\n", faint("dry-run"), cyan(host), faint(action))
}

// Summary prints the final run summary.
func Summary(results map[string]*HostSummary) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Println()
	fmt.Println(bold(strings.Repeat("=", 60)))
	fmt.Println(bold("SUMMARY"))
	fmt.Println(bold(strings.Repeat("=", 60)))
	for host, s := range results {
		status := green("ok=" + fmt.Sprint(s.OK))
		changed := yellow(" changed=" + fmt.Sprint(s.Changed))
		failed := ""
		if s.Failed > 0 {
			failed = " " + red("failed="+fmt.Sprint(s.Failed))
		}
		fmt.Printf("  %-30s : %s%s%s\n", cyan(host), status, changed, failed)
	}
	fmt.Println()
}

func printOutput(stdout, stderr string) {
	if stdout = strings.TrimSpace(stdout); stdout != "" {
		for _, line := range strings.Split(stdout, "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		for _, line := range strings.Split(stderr, "\n") {
			fmt.Printf("    %s %s\n", red("ERR"), line)
		}
	}
}

// HostSummary tracks per-host task outcomes.
type HostSummary struct {
	OK      int
	Changed int
	Failed  int
}
