package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcgerke/githelper/internal/config"
	"github.com/lcgerke/githelper/internal/git"
	ghclient "github.com/lcgerke/githelper/internal/github"
	remoteclient "github.com/lcgerke/githelper/internal/remote/github"
	"github.com/lcgerke/githelper/internal/hooks"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/lcgerke/githelper/internal/vault"
	"github.com/spf13/cobra"
)

var (
	githubUser       string
	githubRepo       string
	createRepo       bool
	privateRepo      bool
	skipHooks        bool
)

var githubSetupCmd = &cobra.Command{
	Use:   "setup <repo-name>",
	Short: "Set up GitHub dual-remote for a repository",
	Long: `Configures dual-push for a repository to sync with GitHub.

This command:
1. Retrieves SSH key from Vault
2. Configures repository-local SSH
3. Creates GitHub repository (if requested)
4. Sets up dual-push remotes (bare repo + GitHub)
5. Installs hooks (with backup)
6. Verifies configuration`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubSetup,
}

func init() {
	githubSetupCmd.Flags().StringVar(&githubUser, "user", "", "GitHub username (default: from config)")
	githubSetupCmd.Flags().StringVar(&githubRepo, "repo", "", "GitHub repository name (default: same as local)")
	githubSetupCmd.Flags().BoolVar(&createRepo, "create", false, "Create GitHub repository if it doesn't exist")
	githubSetupCmd.Flags().BoolVar(&privateRepo, "private", true, "Create private repository (used with --create)")
	githubSetupCmd.Flags().BoolVar(&skipHooks, "skip-hooks", false, "Skip hook installation")
}

func runGitHubSetup(cmd *cobra.Command, args []string) error {
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

	if !out.IsJSON() {
		out.Header(fmt.Sprintf("🔧 Setting up GitHub integration for: %s", repoName))
		out.Separator()
	}

	// Initialize config manager
	cfgMgr, err := config.NewManager(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Get configuration
	cfg, fromCache, err := cfgMgr.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Show config status
	if !out.IsJSON() {
		if fromCache {
			age := cfgMgr.GetCacheAge()
			out.Warningf("Configuration: Vault (cached %s ago) ⚡", formatDuration(age))
		} else {
			out.Success("Configuration: Vault (live) ✓")
		}
		fmt.Println()
	}

	// Get repository from state
	stateMgr, err := state.NewManager("")
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	repo, err := stateMgr.GetRepository(repoName)
	if err != nil {
		return fmt.Errorf("repository not found in state: %w. Create it first with 'githelper repo create'", err)
	}

	// Use config GitHub username if not specified
	if githubUser == "" {
		githubUser = cfg.GitHubUsername
	}
	if githubUser == "" {
		return fmt.Errorf("GitHub username not specified (use --user or configure in Vault)")
	}

	// Use repo name if GitHub repo name not specified
	if githubRepo == "" {
		githubRepo = repoName
	}

	// Download SSH key from Vault
	out.Info("Retrieving SSH key from Vault...")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Download key to disk
	vaultClient, err := vault.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}

	privateKeyPath, err := vaultClient.DownloadSSHKey(repoName, sshDir)
	if err != nil {
		return fmt.Errorf("failed to download SSH key: %w", err)
	}
	out.Success(fmt.Sprintf("SSH key downloaded to %s", privateKeyPath))

	// Configure repository-local SSH
	out.Info("Configuring repository-local SSH...")
	gitClient := git.NewClient(repo.Path)
	if err := gitClient.ConfigureSSH(privateKeyPath); err != nil {
		return fmt.Errorf("failed to configure SSH: %w", err)
	}
	out.Success("Configured repository-local SSH")

	// Get PAT from Vault and set as environment variable for new client
	out.Info("Retrieving GitHub PAT from Vault...")
	pat, err := cfgMgr.GetPAT(repoName)
	if err != nil {
		return fmt.Errorf("failed to get PAT from Vault: %w", err)
	}

	// Set token as environment variable for new remote client
	os.Setenv("GITHUB_TOKEN", pat)
	defer os.Unsetenv("GITHUB_TOKEN")

	// Create new GitHub client using remote package
	ghRepoURL := fmt.Sprintf("git@github.com:%s/%s.git", githubUser, githubRepo)
	ghClient, err := remoteclient.NewClient(ghRepoURL)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Test GitHub connection
	out.Info("Testing GitHub connection...")
	if err := ghClient.TestConnection(); err != nil {
		return fmt.Errorf("GitHub connection test failed: %w", err)
	}
	out.Success("GitHub connection verified")

	// Check if GitHub repository exists
	exists, err := ghClient.RepositoryExists()
	if err != nil {
		return fmt.Errorf("failed to check if repository exists: %w", err)
	}

	if !exists {
		if createRepo {
			out.Infof("Creating GitHub repository: %s/%s...", githubUser, githubRepo)

			// Prefer gh CLI if available (doesn't require PAT from Vault)
			if ghclient.CheckGHCLIAvailable() && ghclient.CheckGHAuthenticated() {
				out.Info("Using gh CLI for repository creation")
				// Use old client for gh CLI support (backward compatibility)
				oldClient := ghclient.NewClient(context.Background(), pat)
				err := oldClient.CreateRepositoryViaGH(githubRepo, fmt.Sprintf("%s repository", repoName), privateRepo)
				if err != nil {
					return fmt.Errorf("failed to create GitHub repository via gh CLI: %w", err)
				}
			} else {
				// Use new API method
				out.Info("Using GitHub API for repository creation")
				_, err := ghClient.CreateRepository(githubRepo, fmt.Sprintf("%s repository", repoName), privateRepo)
				if err != nil {
					return fmt.Errorf("failed to create GitHub repository: %w", err)
				}
			}
			out.Success(fmt.Sprintf("Created GitHub repository: https://github.com/%s/%s", githubUser, githubRepo))
		} else {
			return fmt.Errorf("GitHub repository %s/%s does not exist. Use --create to create it", githubUser, githubRepo)
		}
	} else {
		out.Success(fmt.Sprintf("GitHub repository exists: %s/%s", githubUser, githubRepo))
	}

	// Configure dual-push
	out.Info("Configuring dual-push remotes...")
	bareRepoURL := repo.Remote

	// Setup dual-push: one git push → pushes to both bare and GitHub
	if err := gitClient.SetupDualPush("origin", bareRepoURL, bareRepoURL, ghRepoURL); err != nil {
		return fmt.Errorf("failed to setup dual-push: %w", err)
	}

	// Verify dual-push configuration
	verified, err := gitClient.VerifyDualPush("origin", bareRepoURL, ghRepoURL)
	if err != nil {
		return fmt.Errorf("failed to verify dual-push: %w", err)
	}
	if !verified {
		return fmt.Errorf("dual-push verification failed")
	}

	out.Success("Configured dual-push remotes")
	out.Infof("  Push URL 1: %s", bareRepoURL)
	out.Infof("  Push URL 2: %s", ghRepoURL)

	// Install hooks (unless skipped)
	if !skipHooks {
		out.Info("Installing hooks...")
		hooksMgr := hooks.NewManager(repo.Path)

		if err := hooksMgr.Install(); err != nil {
			return fmt.Errorf("failed to install hooks: %w", err)
		}

		// Check if backups were created
		if hooksMgr.HasBackup("pre-push") {
			out.Success("Backed up existing pre-push hook to pre-push.githelper-backup")
		}
		if hooksMgr.HasBackup("post-push") {
			out.Success("Backed up existing post-push hook to post-push.githelper-backup")
		}

		out.Success("Installed pre-push and post-push hooks")
	} else {
		out.Info("Skipped hook installation (--skip-hooks)")
	}

	// Update state
	if repo.GitHub == nil {
		repo.GitHub = &state.GitHub{}
	}
	repo.GitHub.Enabled = true
	repo.GitHub.User = githubUser
	repo.GitHub.Repo = githubRepo
	repo.GitHub.SyncStatus = "synced"

	if err := stateMgr.AddRepository(repoName, repo); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	if !out.IsJSON() {
		fmt.Println()
		out.Separator()
		out.Success("GitHub integration complete!")
		out.Info("Test with: git push")
		out.Info("This will push to both remotes automatically")
	} else {
		out.JSON(map[string]interface{}{
			"status":       "success",
			"repository":   repoName,
			"github_user":  githubUser,
			"github_repo":  githubRepo,
			"github_url":   ghRepoURL,
			"bare_url":     bareRepoURL,
			"hooks_installed": !skipHooks,
		})
	}

	return nil
}
