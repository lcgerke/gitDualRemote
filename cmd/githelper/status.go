package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/scenarios"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	statusNoFetch      bool
	statusQuick        bool
	statusShowFixes    bool
	statusCoreRemote   string
	statusGitHubRemote string
)

var statusCmd = &cobra.Command{
	Use:   "status [path]",
	Short: "Check repository sync status",
	Long: `Quickly check the sync status of a repository.

Shows:
- Existence state (E1-E8)
- Sync state (S1-S13)
- Working tree state (W1-W5)
- Corruption state (C1-C8)
- Suggested fixes

Use --quick to skip corruption checks.
Use --no-fetch to use cached remote data (faster but may be stale).
Use --show-fixes to display suggested fixes.`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false, "Skip fetching from remotes")
	statusCmd.Flags().BoolVar(&statusQuick, "quick", false, "Skip corruption checks")
	statusCmd.Flags().BoolVar(&statusShowFixes, "show-fixes", false, "Show suggested fixes")
	statusCmd.Flags().StringVar(&statusCoreRemote, "core-remote", "origin", "Name of Core remote")
	statusCmd.Flags().StringVar(&statusGitHubRemote, "github-remote", "github", "Name of GitHub remote")
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	// Get repository path
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	// Create git client
	gitClient := git.NewClient(repoPath)

	// Validate git version
	if err := gitClient.ValidateGitVersion(); err != nil {
		return fmt.Errorf("git version check failed: %w", err)
	}

	// Check if it's a git repository
	if !gitClient.IsRepository() {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	// Configure detection options
	options := scenarios.DefaultDetectionOptions()
	options.SkipFetch = statusNoFetch
	options.SkipCorruption = statusQuick

	// Create classifier
	classifier := scenarios.NewClassifier(gitClient, statusCoreRemote, statusGitHubRemote, options)

	// Detect state
	if !out.IsJSON() {
		fmt.Println("🔍 Analyzing repository state...")
		fmt.Println()
	}

	state, err := classifier.Detect()
	if err != nil {
		return fmt.Errorf("state detection failed: %w", err)
	}

	// Output results
	if out.IsJSON() {
		// JSON output
		jsonBytes, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal state: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		// Human-readable output
		printStatusReport(out, state, statusShowFixes)
	}

	return nil
}

func printStatusReport(out *ui.Output, state *scenarios.RepositoryState, showFixes bool) {
	fmt.Printf("Repository: %s\n", state.RepoPath)
	fmt.Printf("Detection Time: %dms\n", state.DetectionTime.Milliseconds())
	fmt.Println()

	// Existence
	fmt.Println("📦 Existence:")
	fmt.Printf("  %s - %s\n", state.Existence.ID, state.Existence.Description)
	if state.Existence.LocalExists {
		fmt.Printf("  Local: ✓ %s\n", state.Existence.LocalPath)
	} else {
		fmt.Printf("  Local: ✗ missing\n")
	}
	if state.Existence.CoreExists {
		fmt.Printf("  Core (%s): ✓ %s\n", state.CoreRemote, state.Existence.CoreURL)
	} else {
		fmt.Printf("  Core (%s): ✗ not configured\n", state.CoreRemote)
	}
	if state.Existence.GitHubExists {
		fmt.Printf("  GitHub (%s): ✓ %s\n", state.GitHubRemote, state.Existence.GitHubURL)
	} else {
		fmt.Printf("  GitHub (%s): ✗ not configured\n", state.GitHubRemote)
	}
	fmt.Println()

	// Sync - now supports partial sync (E2/E3) and full sync (E1)
	if state.Existence.LocalExists && (state.Existence.CoreExists || state.Existence.GitHubExists) {
		// Display sync status
		if state.Sync.Error != "" {
			fmt.Println("🔄 Sync Status:")
			fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
			out.Warning(fmt.Sprintf("  ⚠️  Error: %s", state.Sync.Error))
		} else if state.Sync.PartialSync {
			fmt.Println("🔄 Sync Status (partial):")
			fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
			if state.Sync.Branch != "" {
				fmt.Printf("  Branch: %s\n", state.Sync.Branch)
			}

			// Show available remote stats
			if state.Sync.AvailableRemote != "" {
				if state.Sync.LocalAheadOfCore > 0 || state.Sync.LocalBehindCore > 0 {
					fmt.Printf("  Local vs %s: ", state.Sync.AvailableRemote)
					if state.Sync.LocalAheadOfCore > 0 {
						fmt.Printf("%d ahead", state.Sync.LocalAheadOfCore)
					}
					if state.Sync.LocalBehindCore > 0 {
						if state.Sync.LocalAheadOfCore > 0 {
							fmt.Printf(", ")
						}
						fmt.Printf("%d behind", state.Sync.LocalBehindCore)
					}
					fmt.Printf("\n")
				} else if state.Sync.LocalAheadOfGitHub > 0 || state.Sync.LocalBehindGitHub > 0 {
					fmt.Printf("  Local vs %s: ", state.Sync.AvailableRemote)
					if state.Sync.LocalAheadOfGitHub > 0 {
						fmt.Printf("%d ahead", state.Sync.LocalAheadOfGitHub)
					}
					if state.Sync.LocalBehindGitHub > 0 {
						if state.Sync.LocalAheadOfGitHub > 0 {
							fmt.Printf(", ")
						}
						fmt.Printf("%d behind", state.Sync.LocalBehindGitHub)
					}
					fmt.Printf("\n")
				}
			}
			if state.Sync.Diverged {
				out.Warning("  ⚠️  DIVERGED - manual merge required")
			}
		} else if state.Existence.CoreExists && state.Existence.GitHubExists {
			// Full three-way sync
			fmt.Println("🔄 Sync Status:")
			fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
			if state.Sync.Branch != "" {
				fmt.Printf("  Branch: %s\n", state.Sync.Branch)
			}
			if state.Sync.LocalAheadOfCore > 0 {
				fmt.Printf("  Local ahead of Core: %d commits\n", state.Sync.LocalAheadOfCore)
			}
			if state.Sync.LocalBehindCore > 0 {
				fmt.Printf("  Local behind Core: %d commits\n", state.Sync.LocalBehindCore)
			}
			if state.Sync.LocalAheadOfGitHub > 0 {
				fmt.Printf("  Local ahead of GitHub: %d commits\n", state.Sync.LocalAheadOfGitHub)
			}
			if state.Sync.LocalBehindGitHub > 0 {
				fmt.Printf("  Local behind GitHub: %d commits\n", state.Sync.LocalBehindGitHub)
			}
			if state.Sync.Diverged {
				out.Warning("  ⚠️  DIVERGED - manual merge required")
			}
		}
		fmt.Println()
	}

	// Working Tree
	if state.Existence.LocalExists {
		fmt.Println("📝 Working Tree:")
		fmt.Printf("  %s - %s\n", state.WorkingTree.ID, state.WorkingTree.Description)
		if len(state.WorkingTree.StagedFiles) > 0 {
			fmt.Printf("  Staged files: %d\n", len(state.WorkingTree.StagedFiles))
		}
		if len(state.WorkingTree.UnstagedFiles) > 0 {
			fmt.Printf("  Unstaged files: %d\n", len(state.WorkingTree.UnstagedFiles))
		}
		if len(state.WorkingTree.UntrackedFiles) > 0 {
			fmt.Printf("  Untracked files: %d\n", len(state.WorkingTree.UntrackedFiles))
		}
		if len(state.WorkingTree.ConflictFiles) > 0 {
			out.Error(fmt.Sprintf("  Conflicts: %d files", len(state.WorkingTree.ConflictFiles)))
		}
		if len(state.WorkingTree.OrphanedSubmodules) > 0 {
			out.Warning(fmt.Sprintf("  ⚠️  Orphaned submodules: %d (in index but not in .gitmodules)", len(state.WorkingTree.OrphanedSubmodules)))
			for _, sub := range state.WorkingTree.OrphanedSubmodules {
				out.Warning(fmt.Sprintf("    - %s", sub))
			}
		}
		fmt.Println()
	}

	// Corruption/Health
	if !state.Corruption.Healthy {
		fmt.Println("⚠️  Repository Health:")
		fmt.Printf("  %s - %s\n", state.Corruption.ID, state.Corruption.Description)
		if len(state.Corruption.LargeBinaries) > 0 {
			fmt.Printf("  Large binaries: %d files\n", len(state.Corruption.LargeBinaries))
		}
		fmt.Println()
	}

	// Warnings
	if len(state.Warnings) > 0 {
		fmt.Println("⚠️  Warnings:")
		for _, warning := range state.Warnings {
			fmt.Printf("  %s: %s\n", warning.Code, warning.Message)
			if warning.Hint != "" {
				fmt.Printf("    Hint: %s\n", warning.Hint)
			}
		}
		fmt.Println()
	}

	// Show fixes if requested
	if showFixes {
		fixes := scenarios.SuggestFixes(state)
		if len(fixes) > 0 {
			fmt.Println("🔧 Suggested Fixes:")
			for i, fix := range fixes {
				autofix := ""
				if fix.AutoFixable {
					autofix = " [auto-fixable]"
				}
				fmt.Printf("  %d. [%s] %s%s\n", i+1, fix.ScenarioID, fix.Description, autofix)
				fmt.Printf("     Command: %s\n", fix.Command)
			}
			fmt.Println()
		}
	}

	// Summary
	if state.Sync.ID == "S1" && state.WorkingTree.Clean && state.Corruption.Healthy {
		out.Success("✅ Repository is healthy and in sync")
	} else {
		if showFixes {
			fmt.Println("Run 'githelper repair --auto' to apply auto-fixable changes")
		} else {
			fmt.Println("Run 'githelper status --show-fixes' to see suggested fixes")
		}
	}
}
