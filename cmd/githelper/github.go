package main

import (
	"github.com/spf13/cobra"
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "Manage GitHub integration",
	Long:  "Set up and manage GitHub dual-remote synchronization for repositories.",
}

func init() {
	// Add github subcommands
	githubCmd.AddCommand(githubSetupCmd)
	githubCmd.AddCommand(githubStatusCmd)
	githubCmd.AddCommand(githubCheckCmd)
}
