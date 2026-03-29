package runner

import (
	"fmt"
	"strings"
	"sync"

	"workflow/internal/config"
	"workflow/internal/output"
)

// Options controls runner behaviour.
type Options struct {
	Limit    string // comma-separated host/group filter
	Tags     string // comma-separated tag filter
	DryRun   bool
	Parallel bool
	Verbose  bool
}

// Runner orchestrates playbook execution.
type Runner struct {
	inv  *config.Inventory
	opts Options
}

func New(inv *config.Inventory, opts Options) *Runner {
	return &Runner{inv: inv, opts: opts}
}

// Run executes all plays in the playbook.
func (r *Runner) Run(pb config.Playbook) error {
	summaries := map[string]*output.HostSummary{}
	var summaryMu sync.Mutex

	for _, play := range pb {
		target := play.Hosts
		if target == "" {
			target = "all"
		}

		hosts, err := r.inv.Resolve(target)
		if err != nil {
			return fmt.Errorf("play %q: %w", play.Name, err)
		}

		// Apply --limit
		if r.opts.Limit != "" {
			limited, err := r.inv.Resolve(r.opts.Limit)
			if err != nil {
				return fmt.Errorf("--limit: %w", err)
			}
			hosts = intersect(hosts, limited)
		}

		if len(hosts) == 0 {
			fmt.Printf("play %q: no matching hosts, skipping\n", play.Name)
			continue
		}

		output.PlayBanner(play.Name, target)

		// Init summaries.
		for _, h := range hosts {
			summaryMu.Lock()
			if _, ok := summaries[h.Name]; !ok {
				summaries[h.Name] = &output.HostSummary{}
			}
			summaryMu.Unlock()
		}

		// Build tag set.
		tagFilter := parseCSV(r.opts.Tags)

		for _, task := range play.Tasks {
			// Tag filter.
			if len(tagFilter) > 0 && !hasAnyTag(task.Tags, tagFilter) {
				continue
			}

			output.TaskBanner(taskLabel(task))

			exec := func(h *config.HostDef) {
				e := newExecutor(h, play.Vars, r.opts)
				result := e.Run(task)

				summaryMu.Lock()
				s := summaries[h.Name]
				switch result {
				case resultOK:
					s.OK++
				case resultChanged:
					s.Changed++
				case resultFailed:
					s.Failed++
				}
				summaryMu.Unlock()
			}

			if r.opts.Parallel {
				var wg sync.WaitGroup
				for _, h := range hosts {
					wg.Add(1)
					go func(h *config.HostDef) {
						defer wg.Done()
						exec(h)
					}(h)
				}
				wg.Wait()
			} else {
				for _, h := range hosts {
					exec(h)
				}
			}
		}
	}

	output.Summary(summaries)
	return nil
}

func intersect(a, b []*config.HostDef) []*config.HostDef {
	bSet := make(map[string]bool, len(b))
	for _, h := range b {
		bSet[h.Name] = true
	}
	var result []*config.HostDef
	for _, h := range a {
		if bSet[h.Name] {
			result = append(result, h)
		}
	}
	return result
}

func parseCSV(s string) map[string]bool {
	m := map[string]bool{}
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			m[v] = true
		}
	}
	return m
}

func hasAnyTag(taskTags []string, filter map[string]bool) bool {
	for _, t := range taskTags {
		if filter[t] {
			return true
		}
	}
	return false
}

func taskLabel(t *config.Task) string {
	if t.Name != "" {
		return t.Name
	}
	if t.Shell != "" {
		return "shell: " + t.Shell
	}
	if t.Copy != nil {
		return "copy: " + t.Copy.Src + " → " + t.Copy.Dest
	}
	if t.Fetch != nil {
		return "fetch: " + t.Fetch.Src
	}
	if t.Template != nil {
		return "template: " + t.Template.Src
	}
	if t.HTTP != nil {
		method := t.HTTP.Method
		if method == "" {
			method = "GET"
		}
		return method + " " + t.HTTP.URL
	}
	return "unknown task"
}
