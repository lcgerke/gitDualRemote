package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	format   string
	noColor  bool
	quiet    bool
	verbose  bool

	// Root command
	rootCmd = &cobra.Command{
		Use:   "githelper",
		Short: "Unified Git management tool for bare repos and GitHub dual-remote sync",
		Long: `GitHelper manages both bare repository workflows and GitHub dual-remote
synchronization. It combines repository lifecycle management with
seamless GitHub backup integration.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Verify git is installed
			if err := git.CheckGitVersion(); err != nil {
				return fmt.Errorf("git check failed: %w", err)
			}
			return nil
		},
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "Output format (human|json)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add subcommands
	rootCmd.AddCommand(repoCmd)
	rootCmd.AddCommand(githubCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	ctx := context.Background()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
