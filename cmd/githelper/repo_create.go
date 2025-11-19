package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcgerke/githelper/internal/config"
	"github.com/lcgerke/githelper/internal/constants"
	"github.com/lcgerke/githelper/internal/errors"
	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	repoType   string
	cloneDir   string
)

var repoCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new bare repository and local clone",
	Long: `Creates a bare repository on the remote server and clones it locally.

The bare repository is created according to the pattern configured in Vault
(e.g., gitmanager@lcgasgit:/srv/git/{repo}.git) and then cloned to a local
working directory.`,
	Args: cobra.ExactArgs(1),
	RunE: runRepoCreate,
}

func init() {
	repoCreateCmd.Flags().StringVar(&repoType, "type", "", "Repository type (go, python, etc.)")
	repoCreateCmd.Flags().StringVar(&cloneDir, "clone-dir", "", "Directory to clone into (default: ~/repos/<name>)")
}

func runRepoCreate(cmd *cobra.Command, args []string) error {
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
		out.Header(fmt.Sprintf("🚀 Creating Repository: %s", repoName))
		out.Separator()
	}

	// Initialize config manager
	cfgMgr, err := config.NewManager(ctx, "")
	if err != nil {
		return errors.Wrap(errors.ErrorTypeConfig, "failed to initialize config manager", err)
	}

	// Get configuration
	cfg, fromCache, err := cfgMgr.GetConfig()
	if err != nil {
		return errors.Wrap(errors.ErrorTypeConfig, "failed to get configuration from Vault", err)
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

	// Construct bare repo URL from pattern
	bareRepoURL := strings.ReplaceAll(cfg.BareRepoPattern, "{repo}", repoName)
	if !strings.HasSuffix(bareRepoURL, ".git") {
		bareRepoURL += ".git"
	}

	// For Phase 1, we'll assume the bare repo is on a remote server via SSH
	// In a real implementation, we'd handle different URL schemes
	// For now, let's simulate creating it locally for testing

	// Determine clone directory
	if cloneDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return errors.Wrap(errors.ErrorTypeFileSystem, "failed to get home directory", err)
		}
		cloneDir = filepath.Join(home, "repos", repoName)
	}

	// Check if clone directory already exists
	if _, err := os.Stat(cloneDir); err == nil {
		return errors.WithHint(
			errors.New(errors.ErrorTypeFileSystem, fmt.Sprintf("directory already exists: %s", cloneDir)),
			"Choose a different name or use --clone-dir to specify a different location",
		)
	}

	// Parse bare repo URL to determine if it's local or remote
	// For Phase 1 demo, we'll handle file:// and local paths
	var bareRepoPath string

	if strings.HasPrefix(bareRepoURL, "file://") {
		bareRepoPath = strings.TrimPrefix(bareRepoURL, "file://")
	} else if !strings.Contains(bareRepoURL, "@") && !strings.Contains(bareRepoURL, "://") {
		// Local path
		bareRepoPath = bareRepoURL
	} else {
		// Remote SSH URL - for Phase 1, we'll error out
		// In Phase 2+, we'd handle this via SSH
		return errors.WithHint(
			errors.New(errors.ErrorTypeValidation, "remote bare repository creation not yet implemented"),
			"Use a local file path or file:// URL for now. Remote SSH support coming in Phase 2",
		)
	}

	// Create bare repository
	out.Infof("Creating bare repository at %s...", bareRepoURL)
	if err := git.InitBareRepo(bareRepoPath); err != nil {
		return errors.Wrap(errors.ErrorTypeGit, "failed to create bare repository", err)
	}
	out.Success(fmt.Sprintf("Created bare repository: %s", bareRepoURL))

	// Clone the bare repository
	out.Infof("Cloning to %s...", cloneDir)
	if err := git.Clone(bareRepoPath, cloneDir); err != nil {
		return errors.Wrap(errors.ErrorTypeGit, "failed to clone repository", err)
	}
	out.Success(fmt.Sprintf("Cloned to: %s", cloneDir))

	// Initialize repository with appropriate files based on type
	if repoType != "" {
		switch repoType {
		case "go":
			if err := initGoRepo(cloneDir, repoName, out); err != nil {
				return errors.Wrap(errors.ErrorTypeFileSystem, "failed to initialize Go repository", err)
			}
		default:
			out.Warningf("Unknown repository type '%s', skipping initialization", repoType)
		}
	}

	// Create initial commit if repository is empty
	client := git.NewClient(cloneDir)

	// Check if we need an initial commit (if no commits exist yet)
	currentBranch, err := client.GetCurrentBranch()
	if err != nil || currentBranch == "" {
		// No commits yet, create initial commit
		readmePath := filepath.Join(cloneDir, "README.md")
		if _, err := os.Stat(readmePath); os.IsNotExist(err) {
			// Create README if it doesn't exist
			content := fmt.Sprintf("# %s\n\nCreated by githelper on %s\n",
				repoName, time.Now().Format("2006-01-02"))
			if err := os.WriteFile(readmePath, []byte(content), 0644); err != nil {
				return errors.Wrap(errors.ErrorTypeFileSystem, "failed to create README file", err)
			}
		}

		if err := client.Add("."); err != nil {
			return errors.Wrap(errors.ErrorTypeGit, "failed to stage files", err)
		}

		if err := client.Commit("Initial commit"); err != nil {
			return errors.Wrap(errors.ErrorTypeGit, "failed to create initial commit", err)
		}

		out.Success("Created initial commit")

		// Push to bare repo
		if err := client.PushSetUpstream(constants.DefaultCoreRemote, constants.MasterBranch); err != nil {
			// Try main branch if master fails
			if err := client.PushSetUpstream(constants.DefaultCoreRemote, constants.DefaultBranch); err != nil {
				return errors.Wrap(errors.ErrorTypeGit, "failed to push initial commit", err)
			}
		}
		out.Success("Pushed initial commit to bare repository")
	}

	// Save to state file
	stateMgr, err := state.NewManager("")
	if err != nil {
		return errors.Wrap(errors.ErrorTypeState, "failed to initialize state manager", err)
	}

	repo := &state.Repository{
		Path:    cloneDir,
		Remote:  bareRepoURL,
		Created: time.Now(),
		Type:    repoType,
	}

	if err := stateMgr.AddRepository(repoName, repo); err != nil {
		return errors.Wrap(errors.ErrorTypeState, "failed to save repository state", err)
	}

	if !out.IsJSON() {
		fmt.Println()
		out.Separator()
		out.Success(fmt.Sprintf("Repository ready! cd %s", cloneDir))
	} else {
		out.JSON(map[string]interface{}{
			"status":        "success",
			"repository":    repoName,
			"bare_url":      bareRepoURL,
			"clone_dir":     cloneDir,
			"type":          repoType,
		})
	}

	return nil
}

func initGoRepo(workdir, repoName string, out *ui.Output) error {
	out.Info("Initializing Go module...")

	// Create go.mod
	goModContent := fmt.Sprintf("module github.com/lcgerke/%s\n\ngo 1.21\n", repoName)
	goModPath := filepath.Join(workdir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return err
	}

	// Create basic .gitignore
	gitignoreContent := `# Binaries
*.exe
*.dll
*.so
*.dylib

# Test binary
*.test

# Output
*.out

# Vendor
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~
`
	gitignorePath := filepath.Join(workdir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return err
	}

	out.Success("Initialized Go repository")
	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
