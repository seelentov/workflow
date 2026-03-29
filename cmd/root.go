package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow — task automation tool (Ansible-like)",
	Long: `Workflow lets you define tasks in YAML playbooks
and execute them on remote servers via SSH.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
