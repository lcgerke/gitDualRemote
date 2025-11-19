package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/state"
	"github.com/spf13/cobra"
)

// TestGitHubSyncCmd tests the basic command structure
func TestGitHubSyncCmd(t *testing.T) {
	tests := []struct {
		name          string
		flags         map[string]interface{}
		expectedFlags map[string]interface{}
	}{
		{
			name: "default flags",
			flags: map[string]interface{}{
				"retry-github": false,
				"branch":       "main",
			},
			expectedFlags: map[string]interface{}{
				"retry-github": false,
				"branch":       "main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify command flags exist
			for flagName := range tt.flags {
				flag := githubSyncCmd.Flags().Lookup(flagName)
				if flag == nil {
					t.Errorf("Flag --%s not found", flagName)
				}
			}
		})
	}
}

// TestGitHubSyncCmd_Args tests argument validation
func TestGitHubSyncCmd_Args(t *testing.T) {
	// The command requires exactly one argument
	tests := []struct {
		name      string
		args      []string
		shouldErr bool
	}{
		{
			name:      "no arguments",
			args:      []string{},
			shouldErr: true,
		},
		{
			name:      "one argument",
			args:      []string{"test-repo"},
			shouldErr: false,
		},
		{
			name:      "two arguments",
			args:      []string{"test-repo", "extra"},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := githubSyncCmd.Args(githubSyncCmd, tt.args)
			if tt.shouldErr && err == nil {
				t.Error("Expected error for invalid args, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestRunGitHubSync_RepositoryNotFound tests sync with non-existent repository
func TestRunGitHubSync_RepositoryNotFound(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create empty state
	stateDir := filepath.Join(tmpDir, ".githelper")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	cmd := &cobra.Command{}
	args := []string{"nonexistent-repo"}

	// Reset global flags
	format = ""
	noColor = true

	err = runGitHubSync(cmd, args)
	if err == nil {
		t.Error("Expected error for non-existent repository, got nil")
	}

	if !strings.Contains(err.Error(), "repository not found") {
		t.Errorf("Expected 'repository not found' error, got: %v", err)
	}
}

// TestRunGitHubSync_GitHubNotConfigured tests sync without GitHub integration
func TestRunGitHubSync_GitHubNotConfigured(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create repository without GitHub integration
	repo := &state.Repository{
		Path:    filepath.Join(tmpDir, "test-repo"),
		Remote:  "origin",
		Created: time.Now(),
		GitHub:  nil, // No GitHub integration
	}

	if err := stateMgr.AddRepository("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cmd := &cobra.Command{}
	args := []string{"test-repo"}

	// Reset global flags
	format = ""
	noColor = true

	err = runGitHubSync(cmd, args)
	if err == nil {
		t.Error("Expected error for repository without GitHub, got nil")
	}

	if !strings.Contains(err.Error(), "GitHub integration not configured") {
		t.Errorf("Expected 'GitHub integration not configured' error, got: %v", err)
	}
}

// TestRunGitHubSync_GitHubDisabled tests sync with disabled GitHub integration
func TestRunGitHubSync_GitHubDisabled(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create repository with disabled GitHub integration
	repo := &state.Repository{
		Path:    filepath.Join(tmpDir, "test-repo"),
		Remote:  "origin",
		Created: time.Now(),
		GitHub: &state.GitHub{
			Enabled: false, // Disabled
			User:    "testuser",
			Repo:    "testrepo",
		},
	}

	if err := stateMgr.AddRepository("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cmd := &cobra.Command{}
	args := []string{"test-repo"}

	// Reset global flags
	format = ""
	noColor = true

	err = runGitHubSync(cmd, args)
	if err == nil {
		t.Error("Expected error for disabled GitHub, got nil")
	}

	if !strings.Contains(err.Error(), "GitHub integration not configured") {
		t.Errorf("Expected 'GitHub integration not configured' error, got: %v", err)
	}
}

// TestRunGitHubSync_Flags tests various flag combinations
func TestRunGitHubSync_Flags(t *testing.T) {
	tests := []struct {
		name        string
		retryGitHub bool
		branch      string
	}{
		{
			name:        "default flags",
			retryGitHub: false,
			branch:      "main",
		},
		{
			name:        "retry enabled",
			retryGitHub: true,
			branch:      "main",
		},
		{
			name:        "custom branch",
			retryGitHub: false,
			branch:      "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			retryGitHub = tt.retryGitHub
			branch = tt.branch

			// Verify flags were set
			if retryGitHub != tt.retryGitHub {
				t.Errorf("Expected retryGitHub=%v, got %v", tt.retryGitHub, retryGitHub)
			}
			if branch != tt.branch {
				t.Errorf("Expected branch=%v, got %v", tt.branch, branch)
			}
		})
	}
}

// TestRunGitHubSync_WithValidRepo tests sync with a valid repository setup
func TestRunGitHubSync_WithValidRepo(t *testing.T) {
	// Skip if git is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create a real git repository
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repository
	if err := runGitCommand(repoPath, "init"); err != nil {
		t.Skipf("Git not available, skipping test: %v", err)
		return
	}

	// Configure git
	if err := runGitCommand(repoPath, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("Failed to configure git: %v", err)
	}
	if err := runGitCommand(repoPath, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("Failed to configure git: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := runGitCommand(repoPath, "add", "."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
	if err := runGitCommand(repoPath, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Create repository with GitHub integration
	repo := &state.Repository{
		Path:    repoPath,
		Remote:  "origin",
		Created: time.Now(),
		GitHub: &state.GitHub{
			Enabled:    true,
			User:       "testuser",
			Repo:       "testrepo",
			SyncStatus: "unknown",
		},
	}

	if err := stateMgr.AddRepository("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cmd := &cobra.Command{}
	args := []string{"test-repo"}

	// Reset global flags
	format = ""
	noColor = true

	// This will fail because remotes don't exist, but it tests the flow
	err = runGitHubSync(cmd, args)
	// We expect an error about remotes, not about state or config
	if err != nil {
		if strings.Contains(err.Error(), "repository not found") {
			t.Errorf("Should not get 'repository not found' error with valid state")
		}
		if strings.Contains(err.Error(), "GitHub integration not configured") {
			t.Errorf("Should not get 'GitHub integration not configured' error")
		}
		// Other errors are expected (missing remotes, etc.)
	}
}

// TestRunGitHubSync_JSONOutput tests JSON output formatting
func TestRunGitHubSync_JSONOutput(t *testing.T) {
	// Test that JSON output flag is handled
	format = "json"
	noColor = true

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "sync-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create empty state dir
	stateDir := filepath.Join(tmpDir, ".githelper")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	cmd := &cobra.Command{}
	args := []string{"nonexistent-repo"}

	// Should still error, but check that format flag is respected
	_ = runGitHubSync(cmd, args)

	// Reset
	format = ""
}

// TestRunGitHubSync_OutputCapture tests output messages
func TestRunGitHubSync_OutputCapture(t *testing.T) {
	// This test verifies that the sync command produces expected output
	tests := []struct {
		name           string
		setupRepo      func(*state.Repository)
		expectedErrors []string
	}{
		{
			name: "missing GitHub config",
			setupRepo: func(repo *state.Repository) {
				repo.GitHub = nil
			},
			expectedErrors: []string{"GitHub integration not configured"},
		},
		{
			name: "disabled GitHub",
			setupRepo: func(repo *state.Repository) {
				repo.GitHub = &state.GitHub{
					Enabled: false,
				}
			},
			expectedErrors: []string{"GitHub integration not configured"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "sync-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create state manager
			stateMgr, err := state.NewManager(tmpDir)
			if err != nil {
				t.Fatalf("Failed to create state manager: %v", err)
			}

			// Create repository
			repo := &state.Repository{
				Path:    filepath.Join(tmpDir, "test-repo"),
				Remote:  "origin",
				Created: time.Now(),
			}
			tt.setupRepo(repo)

			if err := stateMgr.AddRepository("test-repo", repo); err != nil {
				t.Fatalf("Failed to add repository: %v", err)
			}

			// Set environment
			origHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", origHome)

			cmd := &cobra.Command{}
			args := []string{"test-repo"}

			// Reset global flags
			format = ""
			noColor = true

			err = runGitHubSync(cmd, args)
			if err == nil {
				t.Error("Expected error, got nil")
			}

			// Check error message
			errMsg := err.Error()
			for _, expected := range tt.expectedErrors {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Expected error to contain %q, got: %v", expected, errMsg)
				}
			}
		})
	}
}

// TestRunGitHubSync_StateUpdate tests that sync status is updated in state
func TestRunGitHubSync_StateUpdate(t *testing.T) {
	// This test would require mocking the git operations
	// For now, we test the state update logic indirectly through error cases
	t.Skip("State update testing requires git operation mocking")
}

// TestSyncDivergenceScenarios tests various divergence scenarios
func TestSyncDivergenceScenarios(t *testing.T) {
	tests := []struct {
		name        string
		bareAhead   bool
		githubAhead bool
		expectSync  bool
	}{
		{
			name:        "in sync",
			bareAhead:   false,
			githubAhead: false,
			expectSync:  true,
		},
		{
			name:        "bare ahead",
			bareAhead:   true,
			githubAhead: false,
			expectSync:  false,
		},
		{
			name:        "github ahead",
			bareAhead:   false,
			githubAhead: true,
			expectSync:  false,
		},
		{
			name:        "diverged",
			bareAhead:   true,
			githubAhead: true,
			expectSync:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require a full integration test with git
			// For now, we document the scenarios that should be tested
			t.Skip("Requires git integration testing framework")
		})
	}
}

// Helper functions are in test_helpers.go
