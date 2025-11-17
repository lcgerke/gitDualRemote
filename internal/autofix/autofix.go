package autofix

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/hooks"
	"github.com/lcgerke/githelper/internal/state"
)

// Issue represents a fixable issue
type Issue struct {
	Type        string // "missing_hooks", "invalid_state", "missing_remote"
	Description string
	RepoName    string
	RepoPath    string
	Severity    string // "low", "medium", "high"
}

// Fixer handles automatic fixing of common issues
type Fixer struct {
	stateMgr *state.Manager
	dryRun   bool
}

// NewFixer creates a new auto-fixer
func NewFixer(stateMgr *state.Manager, dryRun bool) *Fixer {
	return &Fixer{
		stateMgr: stateMgr,
		dryRun:   dryRun,
	}
}

// DetectIssues scans for fixable issues
func (f *Fixer) DetectIssues() ([]*Issue, error) {
	issues := []*Issue{}

	st, err := f.stateMgr.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	for name, repo := range st.Repositories {
		// Check if directory exists
		if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
			issues = append(issues, &Issue{
				Type:        "missing_directory",
				Description: fmt.Sprintf("Repository directory not found: %s", repo.Path),
				RepoName:    name,
				RepoPath:    repo.Path,
				Severity:    "high",
			})
			continue
		}

		// Check if it's a git repository
		gitClient := git.NewClient(repo.Path)
		if !gitClient.IsRepository() {
			issues = append(issues, &Issue{
				Type:        "not_git_repo",
				Description: "Directory exists but is not a git repository",
				RepoName:    name,
				RepoPath:    repo.Path,
				Severity:    "high",
			})
			continue
		}

		// Check hooks
		hookIssues := f.detectHookIssues(name, repo.Path)
		issues = append(issues, hookIssues...)

		// Check GitHub integration
		if repo.GitHub != nil && repo.GitHub.Enabled {
			if repo.GitHub.NeedsRetry {
				issues = append(issues, &Issue{
					Type:        "needs_sync",
					Description: "GitHub remote needs sync",
					RepoName:    name,
					RepoPath:    repo.Path,
					Severity:    "medium",
				})
			}
		}
	}

	return issues, nil
}

func (f *Fixer) detectHookIssues(repoName, repoPath string) []*Issue {
	issues := []*Issue{}
	hookDir := filepath.Join(repoPath, ".git", "hooks")

	hookFiles := []string{"pre-push", "post-push"}
	for _, hookName := range hookFiles {
		hookPath := filepath.Join(hookDir, hookName)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			issues = append(issues, &Issue{
				Type:        "missing_hook",
				Description: fmt.Sprintf("Hook not installed: %s", hookName),
				RepoName:    repoName,
				RepoPath:    repoPath,
				Severity:    "low",
			})
		}
	}

	return issues
}

// FixIssue attempts to fix a single issue
func (f *Fixer) FixIssue(issue *Issue) error {
	if f.dryRun {
		return nil // Dry run, don't actually fix
	}

	switch issue.Type {
	case "missing_hook":
		return f.fixMissingHooks(issue.RepoPath)
	case "needs_sync":
		// This requires user intervention (network operation)
		return fmt.Errorf("sync requires manual intervention: run 'githelper github sync %s'", issue.RepoName)
	case "missing_directory", "not_git_repo":
		// These are severe and require manual intervention
		return fmt.Errorf("critical issue requires manual resolution")
	default:
		return fmt.Errorf("unknown issue type: %s", issue.Type)
	}
}

func (f *Fixer) fixMissingHooks(repoPath string) error {
	manager := hooks.NewManager(repoPath)

	// Install both hooks (pre-push and post-push)
	if err := manager.Install(); err != nil {
		return fmt.Errorf("failed to install hooks: %w", err)
	}

	return nil
}

// FixAll attempts to fix all detected issues
func (f *Fixer) FixAll(issues []*Issue) (int, int, error) {
	fixed := 0
	failed := 0

	for _, issue := range issues {
		if err := f.FixIssue(issue); err != nil {
			failed++
		} else {
			fixed++
		}
	}

	return fixed, failed, nil
}
