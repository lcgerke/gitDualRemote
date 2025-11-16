package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCheckDivergence tests the divergence detection functionality
func TestCheckDivergence(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "githelper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize test repo
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	client := NewClient(repoPath)

	// Initialize git repo
	if err := client.Init(false); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Configure git user (required for commits)
	client.ConfigSet("user.name", "Test User")
	client.ConfigSet("user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if err := client.Add("."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	if err := client.Commit("Initial commit"); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Create a bare repo to act as remote
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	if err := InitBareRepo(bareRepoPath); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Add bare repo as remote
	if err := client.AddRemote("origin", bareRepoPath); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}

	// Push to bare repo
	if err := client.PushSetUpstream("origin", "master"); err != nil {
		// Try main if master fails
		if err := client.PushSetUpstream("origin", "main"); err != nil {
			t.Fatalf("Failed to push: %v", err)
		}
	}

	// Get current branch
	branch, err := client.GetCurrentBranch()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}

	t.Run("InSync", func(t *testing.T) {
		// Both remotes at same commit - should be in sync
		status, err := client.CheckDivergence("origin", "origin", branch)
		if err != nil {
			t.Fatalf("CheckDivergence failed: %v", err)
		}

		if !status.InSync {
			t.Errorf("Expected InSync=true, got false")
		}

		if status.BareAhead != 0 {
			t.Errorf("Expected BareAhead=0, got %d", status.BareAhead)
		}

		if status.GitHubAhead != 0 {
			t.Errorf("Expected GitHubAhead=0, got %d", status.GitHubAhead)
		}
	})

	t.Run("LocalAhead", func(t *testing.T) {
		// Create a new commit locally
		if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		if err := client.Add("."); err != nil {
			t.Fatalf("Failed to add files: %v", err)
		}

		if err := client.Commit("Second commit"); err != nil {
			t.Fatalf("Failed to create commit: %v", err)
		}

		// Now local is 1 commit ahead of origin
		// Fetch to update remote tracking
		client.Fetch("origin")

		// Check divergence - local should be ahead
		// Note: We're comparing origin/branch (remote) to HEAD (local)
		// For this test, we'll use rev-list directly
		commits, err := client.GetRevList(fmt.Sprintf("origin/%s", branch), "HEAD")
		if err != nil {
			t.Fatalf("Failed to get rev list: %v", err)
		}

		if len(commits) != 1 {
			t.Errorf("Expected 1 commit ahead, got %d", len(commits))
		}
	})
}

// TestGetCommit tests getting commit SHA for a ref
func TestGetCommit(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "githelper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize test repo
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	client := NewClient(repoPath)

	// Initialize git repo
	if err := client.Init(false); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Configure git user
	client.ConfigSet("user.name", "Test User")
	client.ConfigSet("user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if err := client.Add("."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	if err := client.Commit("Test commit"); err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	// Get commit for HEAD
	commit, err := client.GetCommit("HEAD")
	if err != nil {
		t.Fatalf("Failed to get commit: %v", err)
	}

	// Should be a valid SHA (40 hex characters)
	if len(commit) != 40 {
		t.Errorf("Expected 40 character SHA, got %d: %s", len(commit), commit)
	}
}

// TestFetch tests fetching from a remote
func TestFetch(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "githelper-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create bare repo
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	if err := InitBareRepo(bareRepoPath); err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create working repo
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	client := NewClient(repoPath)

	// Initialize and setup
	if err := client.Init(false); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	client.ConfigSet("user.name", "Test User")
	client.ConfigSet("user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	if err := client.Add("."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	if err := client.Commit("Initial commit"); err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	// Add remote
	if err := client.AddRemote("origin", bareRepoPath); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}

	// Push
	if err := client.Push("origin", ""); err != nil {
		t.Fatalf("Failed to push: %v", err)
	}

	// Fetch should work
	if err := client.Fetch("origin"); err != nil {
		t.Fatalf("Failed to fetch: %v", err)
	}
}

// Helper function to execute git command directly (for test setup)
func runGitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
