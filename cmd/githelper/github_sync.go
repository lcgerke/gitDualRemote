package main

import (
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	retryGitHub bool
	branch      string
)

var githubSyncCmd = &cobra.Command{
	Use:   "sync <repo-name>",
	Short: "Sync GitHub remote with bare repository",
	Long: `Synchronizes the GitHub remote with the bare repository.

This command:
1. Fetches from both remotes
2. Detects divergence between bare and GitHub
3. Pushes missing commits from bare to GitHub
4. Updates sync status in state file

Use --retry-github to force sync even after partial push failures.`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubSync,
}

func init() {
	githubSyncCmd.Flags().BoolVar(&retryGitHub, "retry-github", false, "Retry syncing to GitHub after partial failure")
	githubSyncCmd.Flags().StringVar(&branch, "branch", "main", "Branch to sync (default: main)")
}

func runGitHubSync(cmd *cobra.Command, args []string) error {
	repoName := args[0]

	// Set up output
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	if !out.IsJSON() {
		out.Header(fmt.Sprintf("🔄 Syncing GitHub for: %s", repoName))
		out.Separator()
	}

	// Load repository state
	stateMgr, err := state.NewManager("")
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	repo, err := stateMgr.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	if repo.GitHub == nil || !repo.GitHub.Enabled {
		return fmt.Errorf("GitHub integration not configured. Run: githelper github setup %s", repoName)
	}

	// Initialize git client
	gitClient := git.NewClient(repo.Path)

	// Get remote names from git config
	// For dual-push setup, we use "origin" as the remote with two push URLs
	// We need to determine the actual remote names for bare and GitHub
	bareRemote := "origin"      // This is the fetch URL (bare repo)
	githubRemoteName := "github" // We'll check if this exists

	// Check if we have a separate github remote or if it's configured as a push URL
	remotes, err := gitClient.ListRemotes()
	if err != nil {
		return fmt.Errorf("failed to list remotes: %w", err)
	}

	// Find the GitHub remote
	hasGitHubRemote := false
	for _, remote := range remotes {
		if remote == "github" {
			hasGitHubRemote = true
			break
		}
	}

	// If there's no separate github remote, we're using dual-push on origin
	// In this case, we need a different approach
	if !hasGitHubRemote {
		out.Info("Using dual-push configuration (origin remote with multiple push URLs)")

		// In dual-push mode, we can't easily compare remotes
		// We need to add a temporary GitHub remote for comparison
		githubURL := fmt.Sprintf("git@github.com:%s/%s.git", repo.GitHub.User, repo.GitHub.Repo)

		out.Info("Adding temporary GitHub remote for sync check...")
		if err := gitClient.AddRemote("github-temp", githubURL); err != nil {
			// Remote might already exist, try to continue
			out.Warning("Could not add temporary remote (may already exist)")
		}
		defer func() {
			// Clean up temporary remote
			gitClient.RemoveRemote("github-temp")
		}()

		githubRemoteName = "github-temp"
	}

	// Check divergence
	out.Info(fmt.Sprintf("Checking divergence between bare and GitHub (branch: %s)...", branch))

	status, err := gitClient.CheckDivergence(bareRemote, githubRemoteName, branch)
	if err != nil {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
			})
		} else {
			out.Error(fmt.Sprintf("Failed to check divergence: %v", err))
		}
		return err
	}

	// Display status
	if status.InSync {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status":       "success",
				"in_sync":      true,
				"bare_commit":  status.BareRef,
				"github_commit": status.GitHubRef,
			})
		} else {
			out.Success("Remotes are in sync!")
			out.Infof("  Bare commit:   %s", status.BareRef[:8])
			out.Infof("  GitHub commit: %s", status.GitHubRef[:8])
		}

		// Update state
		repo.GitHub.SyncStatus = "synced"
		repo.GitHub.NeedsRetry = false
		repo.GitHub.LastError = ""
		if err := stateMgr.AddRepository(repoName, repo); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}

		return nil
	}

	// Display divergence
	if !out.IsJSON() {
		out.Warning("Divergence detected:")
		if status.BareAhead > 0 {
			out.Infof("  Bare is ahead by %d commit(s)", status.BareAhead)
		}
		if status.GitHubAhead > 0 {
			out.Infof("  GitHub is ahead by %d commit(s)", status.GitHubAhead)
		}
	}

	// Check for conflicting situation
	if status.GitHubAhead > 0 {
		errMsg := "GitHub has commits that bare repository doesn't have - manual resolution required"
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status":        "error",
				"in_sync":       false,
				"bare_ahead":    status.BareAhead,
				"github_ahead":  status.GitHubAhead,
				"error":         errMsg,
			})
		} else {
			out.Error(errMsg)
			out.Info("This typically means commits were pushed directly to GitHub.")
			out.Info("Resolve manually by:")
			out.Info("  1. git fetch github")
			out.Info("  2. git merge github/main (or rebase)")
			out.Info("  3. git push")
		}

		// Update state
		repo.GitHub.SyncStatus = "diverged"
		repo.GitHub.NeedsRetry = false
		repo.GitHub.LastError = errMsg
		stateMgr.AddRepository(repoName, repo)

		return fmt.Errorf("%s", errMsg)
	}

	// GitHub is behind - we can sync
	if status.BareAhead > 0 {
		out.Infof("Syncing %d commit(s) to GitHub...", status.BareAhead)

		if err := gitClient.SyncToGitHub(bareRemote, githubRemoteName, branch); err != nil {
			if out.IsJSON() {
				out.JSON(map[string]interface{}{
					"status":      "error",
					"bare_ahead":  status.BareAhead,
					"error":       err.Error(),
				})
			} else {
				out.Error(fmt.Sprintf("Failed to sync to GitHub: %v", err))
			}

			// Update state
			repo.GitHub.SyncStatus = "behind"
			repo.GitHub.NeedsRetry = true
			repo.GitHub.LastError = err.Error()
			stateMgr.AddRepository(repoName, repo)

			return err
		}

		// Verify sync
		out.Info("Verifying sync...")
		verifyStatus, err := gitClient.CheckDivergence(bareRemote, githubRemoteName, branch)
		if err != nil {
			return fmt.Errorf("failed to verify sync: %w", err)
		}

		if !verifyStatus.InSync {
			return fmt.Errorf("sync verification failed - remotes still diverged")
		}

		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status":         "success",
				"synced":         true,
				"commits_synced": status.BareAhead,
				"bare_commit":    verifyStatus.BareRef,
				"github_commit":  verifyStatus.GitHubRef,
			})
		} else {
			out.Success(fmt.Sprintf("Synced %d commit(s) to GitHub", status.BareAhead))
			out.Infof("  Commit: %s", verifyStatus.BareRef[:8])
		}

		// Update state
		repo.GitHub.SyncStatus = "synced"
		repo.GitHub.NeedsRetry = false
		repo.GitHub.LastError = ""
		if err := stateMgr.AddRepository(repoName, repo); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}
	}

	if !out.IsJSON() {
		out.Separator()
		out.Success("Sync complete!")
	}

	return nil
}
