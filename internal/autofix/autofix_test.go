package autofix

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/state"
)

func TestDetectIssues(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "githelper-autofix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create fixer
	fixer := NewFixer(stateMgr, false)

	t.Run("EmptyState", func(t *testing.T) {
		issues, err := fixer.DetectIssues()
		if err != nil {
			t.Fatalf("DetectIssues failed: %v", err)
		}

		if len(issues) != 0 {
			t.Errorf("Expected 0 issues for empty state, got %d", len(issues))
		}
	})

	t.Run("MissingDirectory", func(t *testing.T) {
		// Add a repository with non-existent path
		repo := &state.Repository{
			Path:    "/nonexistent/path",
			Remote:  "origin",
			Created: testTime(),
		}

		if err := stateMgr.AddRepository("test-repo", repo); err != nil {
			t.Fatalf("Failed to add repository: %v", err)
		}

		issues, err := fixer.DetectIssues()
		if err != nil {
			t.Fatalf("DetectIssues failed: %v", err)
		}

		if len(issues) == 0 {
			t.Error("Expected issues for missing directory")
		}

		// Check that we got missing_directory issue
		found := false
		for _, issue := range issues {
			if issue.Type == "missing_directory" {
				found = true
				if issue.Severity != "high" {
					t.Errorf("Expected severity=high, got %s", issue.Severity)
				}
			}
		}

		if !found {
			t.Error("Expected missing_directory issue")
		}
	})

	t.Run("MissingHooks", func(t *testing.T) {
		// Create a temporary git repository
		repoDir := filepath.Join(tmpDir, "test-git-repo")
		if err := os.MkdirAll(filepath.Join(repoDir, ".git", "hooks"), 0755); err != nil {
			t.Fatalf("Failed to create git repo: %v", err)
		}

		// Add repository to state
		repo := &state.Repository{
			Path:    repoDir,
			Remote:  "origin",
			Created: testTime(),
		}

		if err := stateMgr.AddRepository("git-repo", repo); err != nil {
			t.Fatalf("Failed to add repository: %v", err)
		}

		issues, err := fixer.DetectIssues()
		if err != nil {
			t.Fatalf("DetectIssues failed: %v", err)
		}

		// Should detect missing hooks
		hookIssues := 0
		for _, issue := range issues {
			if issue.Type == "missing_hook" {
				hookIssues++
				if issue.Severity != "low" {
					t.Errorf("Expected severity=low for hooks, got %s", issue.Severity)
				}
			}
		}

		// Should have 2 hook issues (pre-push and post-push)
		if hookIssues != 2 {
			t.Errorf("Expected 2 hook issues, got %d", hookIssues)
		}
	})
}

func TestFixIssue(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "githelper-autofix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	fixer := NewFixer(stateMgr, false)

	t.Run("FixMissingHooks", func(t *testing.T) {
		// Create a git repository
		repoDir := filepath.Join(tmpDir, "fix-hooks-repo")
		if err := os.MkdirAll(filepath.Join(repoDir, ".git", "hooks"), 0755); err != nil {
			t.Fatalf("Failed to create git repo: %v", err)
		}

		issue := &Issue{
			Type:        "missing_hook",
			Description: "Hook not installed: pre-push",
			RepoName:    "test",
			RepoPath:    repoDir,
			Severity:    "low",
		}

		// Fix should succeed
		if err := fixer.FixIssue(issue); err != nil {
			t.Errorf("FixIssue failed: %v", err)
		}

		// Verify hooks were installed
		prePushPath := filepath.Join(repoDir, ".git", "hooks", "pre-push")
		postPushPath := filepath.Join(repoDir, ".git", "hooks", "post-push")

		if _, err := os.Stat(prePushPath); os.IsNotExist(err) {
			t.Error("pre-push hook was not installed")
		}

		if _, err := os.Stat(postPushPath); os.IsNotExist(err) {
			t.Error("post-push hook was not installed")
		}
	})

	t.Run("CannotFixCritical", func(t *testing.T) {
		issue := &Issue{
			Type:        "missing_directory",
			Description: "Directory missing",
			RepoName:    "test",
			RepoPath:    "/nonexistent",
			Severity:    "high",
		}

		// Should return error for critical issues
		if err := fixer.FixIssue(issue); err == nil {
			t.Error("Expected error for critical issue, got nil")
		}
	})
}

func TestDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "githelper-autofix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create fixer with dry-run enabled
	fixer := NewFixer(stateMgr, true)

	// Create a git repository
	repoDir := filepath.Join(tmpDir, "dryrun-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git", "hooks"), 0755); err != nil {
		t.Fatalf("Failed to create git repo: %v", err)
	}

	issue := &Issue{
		Type:        "missing_hook",
		Description: "Hook not installed: pre-push",
		RepoName:    "test",
		RepoPath:    repoDir,
		Severity:    "low",
	}

	// Dry run should not return error
	if err := fixer.FixIssue(issue); err != nil {
		t.Errorf("Dry run should not fail: %v", err)
	}

	// But hooks should NOT be installed
	prePushPath := filepath.Join(repoDir, ".git", "hooks", "pre-push")
	if _, err := os.Stat(prePushPath); !os.IsNotExist(err) {
		t.Error("Dry run should not install hooks")
	}
}

func TestFixAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "githelper-autofix-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	fixer := NewFixer(stateMgr, false)

	// Create a git repository
	repoDir := filepath.Join(tmpDir, "fixall-repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git", "hooks"), 0755); err != nil {
		t.Fatalf("Failed to create git repo: %v", err)
	}

	issues := []*Issue{
		{
			Type:        "missing_hook",
			Description: "Hook not installed: pre-push",
			RepoName:    "test",
			RepoPath:    repoDir,
			Severity:    "low",
		},
		{
			Type:        "missing_directory",
			Description: "Directory missing",
			RepoName:    "bad",
			RepoPath:    "/nonexistent",
			Severity:    "high",
		},
	}

	fixed, failed, err := fixer.FixAll(issues)
	if err != nil {
		t.Fatalf("FixAll failed: %v", err)
	}

	if fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d", fixed)
	}

	if failed != 1 {
		t.Errorf("Expected 1 failed, got %d", failed)
	}
}

// Helper function to create test time
func testTime() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}
