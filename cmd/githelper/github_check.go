package main

import (
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/config"
	ghclient "github.com/lcgerke/githelper/internal/github"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var githubCheckCmd = &cobra.Command{
	Use:   "test <repo-name>",
	Short: "Test GitHub connectivity for a repository",
	Long:  "Tests the GitHub API connection and SSH connectivity for a repository.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGitHubCheck,
}

func runGitHubCheck(cmd *cobra.Command, args []string) error {
	repoName := args[0]
	ctx := cmd.Context()

	// Set up output
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
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

	// Get PAT from Vault
	cfgMgr, err := config.NewManager(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	pat, err := cfgMgr.GetPAT(repoName)
	if err != nil {
		return fmt.Errorf("failed to get PAT from Vault: %w", err)
	}

	// Test GitHub API connection
	if !quiet {
		out.Info("Testing GitHub API connection...")
	}

	ghClient := ghclient.NewClient(ctx, pat)
	if err := ghClient.TestConnection(); err != nil {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status": "error",
				"api_connection": "failed",
				"error": err.Error(),
			})
		} else {
			out.Error(fmt.Sprintf("GitHub API connection failed: %v", err))
		}
		return err
	}

	// Check if repository exists
	exists, err := ghClient.RepositoryExists(repo.GitHub.User, repo.GitHub.Repo)
	if err != nil {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status": "error",
				"api_connection": "success",
				"repository_check": "failed",
				"error": err.Error(),
			})
		} else {
			out.Error(fmt.Sprintf("Failed to check repository: %v", err))
		}
		return err
	}

	if !exists {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"status": "error",
				"api_connection": "success",
				"repository_exists": false,
				"error": "repository not found on GitHub",
			})
		} else {
			out.Error(fmt.Sprintf("Repository %s/%s not found on GitHub", repo.GitHub.User, repo.GitHub.Repo))
		}
		return fmt.Errorf("repository not found")
	}

	// All tests passed
	if out.IsJSON() {
		out.JSON(map[string]interface{}{
			"status": "success",
			"api_connection": "success",
			"repository_exists": true,
		})
	} else {
		if !quiet {
			out.Success("GitHub API connection verified")
			out.Success(fmt.Sprintf("Repository %s/%s is accessible", repo.GitHub.User, repo.GitHub.Repo))
		}
	}

	return nil
}
