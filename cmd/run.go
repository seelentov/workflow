package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"workflow/internal/config"
	"workflow/internal/runner"
)

var (
	inventoryFile string
	limitHosts    string
	filterTags    string
	dryRun        bool
	parallel      bool
	verbose       bool
)

var runCmd = &cobra.Command{
	Use:   "run [playbook.yaml]",
	Short: "Run a playbook against inventory",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlaybook,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "inventory.yaml", "path to inventory file")
	runCmd.Flags().StringVarP(&limitHosts, "limit", "l", "", "limit execution to specific hosts (comma-separated)")
	runCmd.Flags().StringVar(&filterTags, "tags", "", "only run tasks with these tags (comma-separated)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show tasks without executing")
	runCmd.Flags().BoolVarP(&parallel, "parallel", "p", true, "execute on hosts in parallel")
	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show full command output")
}

func runPlaybook(cmd *cobra.Command, args []string) error {
	inv, err := config.LoadInventory(inventoryFile)
	if err != nil {
		return fmt.Errorf("loading inventory %q: %w", inventoryFile, err)
	}

	pb, err := config.LoadPlaybook(args[0])
	if err != nil {
		return fmt.Errorf("loading playbook %q: %w", args[0], err)
	}

	opts := runner.Options{
		Limit:    limitHosts,
		Tags:     filterTags,
		DryRun:   dryRun,
		Parallel: parallel,
		Verbose:  verbose,
	}

	r := runner.New(inv, opts)
	return r.Run(pb)
}
