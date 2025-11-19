package main

import (
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/constants"
	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	updateState bool
)

var githubStatusCmd = &cobra.Command{
	Use:   "status <repo-name>",
	Short: "Show GitHub sync status for a repository",
	Long:  "Displays the current GitHub integration and sync status for a repository.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGitHubStatus,
}

func init() {
	githubStatusCmd.Flags().BoolVar(&updateState, "update-state", false, "Update state file with current status")
}

func runGitHubStatus(cmd *cobra.Command, args []string) error {
	repoName := args[0]

	// Set up output
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	// Load state
	stateMgr, err := state.NewManager("")
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	repo, err := stateMgr.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	if repo.GitHub == nil || !repo.GitHub.Enabled {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status":  "not_configured",
				"message": "GitHub integration not configured",
			})
		} else {
			out.Error("GitHub integration not configured for this repository")
			out.Info("Run: githelper github setup " + repoName)
		}
		return nil
	}

	// Check git remote configuration
	gitClient := git.NewClient(repo.Path)
	pushURLs, err := gitClient.GetPushURLs(constants.DefaultCoreRemote)
	if err != nil {
		pushURLs = []string{}
	}

	// If --update-state, check actual divergence and update
	if updateState {
		// Try to check real divergence
		// Use main branch by default
		status, err := gitClient.CheckDivergence(constants.DefaultCoreRemote, constants.DefaultCoreRemote, constants.DefaultBranch)
		if err == nil {
			// Update state based on actual status
			if status.InSync {
				repo.GitHub.SyncStatus = "synced"
				repo.GitHub.NeedsRetry = false
				repo.GitHub.LastError = ""
			} else if status.GitHubAhead > 0 {
				repo.GitHub.SyncStatus = "diverged"
			} else if status.BareAhead > 0 {
				repo.GitHub.SyncStatus = "behind"
				repo.GitHub.NeedsRetry = true
			}

			// Save updated state
			if err := stateMgr.AddRepository(repoName, repo); err != nil {
				if !quiet {
					out.Warning(fmt.Sprintf("Failed to update state: %v", err))
				}
			}
		}
	}

	if out.IsJSON() {
		out.JSON(map[string]interface{}{
			"repository":   repoName,
			"github_user":  repo.GitHub.User,
			"github_repo":  repo.GitHub.Repo,
			"sync_status":  repo.GitHub.SyncStatus,
			"last_sync":    repo.GitHub.LastSync,
			"needs_retry":  repo.GitHub.NeedsRetry,
			"last_error":   repo.GitHub.LastError,
			"push_urls":    pushURLs,
		})
	} else {
		out.Header(fmt.Sprintf("GitHub Status: %s", repoName))
		fmt.Println()

		fmt.Printf("GitHub: %s/%s\n", repo.GitHub.User, repo.GitHub.Repo)
		fmt.Printf("Status: %s\n", repo.GitHub.SyncStatus)

		if !repo.GitHub.LastSync.IsZero() {
			fmt.Printf("Last Sync: %s\n", repo.GitHub.LastSync.Format("2006-01-02 15:04:05"))
		}

		if repo.GitHub.NeedsRetry {
			out.Warning("Needs retry")
		}

		if repo.GitHub.LastError != "" {
			out.Errorf("Last error: %s", repo.GitHub.LastError)
		}

		fmt.Println("\nPush URLs:")
		for i, url := range pushURLs {
			fmt.Printf("  %d. %s\n", i+1, url)
		}

		out.Separator()
	}

	return nil
}
