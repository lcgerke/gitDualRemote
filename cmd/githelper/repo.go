package main

import (
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
	Long:  "Create, list, and manage bare repositories and their local clones.",
}

func init() {
	// Add repo subcommands
	repoCmd.AddCommand(repoCreateCmd)
	repoCmd.AddCommand(repoListCmd)
}
