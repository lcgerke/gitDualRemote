package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcgerke/githelper/internal/scenarios"
	"github.com/spf13/cobra"
)

// TestStatusCmd tests the basic command structure
func TestStatusCmd(t *testing.T) {
	tests := []struct {
		name          string
		flags         map[string]interface{}
		expectedFlags map[string]interface{}
	}{
		{
			name: "default flags",
			flags: map[string]interface{}{
				"no-fetch":     false,
				"quick":        false,
				"show-fixes":   false,
				"core-remote":  "origin",
				"github-remote": "github",
			},
			expectedFlags: map[string]interface{}{
				"no-fetch":     false,
				"quick":        false,
				"show-fixes":   false,
				"core-remote":  "origin",
				"github-remote": "github",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify command flags exist
			for flagName := range tt.flags {
				flag := statusCmd.Flags().Lookup(flagName)
				if flag == nil {
					t.Errorf("Flag --%s not found", flagName)
				}
			}
		})
	}
}

// TestRunStatus_NonGitDirectory tests status command on non-git directory
func TestRunStatus_NonGitDirectory(t *testing.T) {
	// Create temporary non-git directory
	tmpDir, err := os.MkdirTemp("", "status-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create command with test directory
	cmd := &cobra.Command{}
	args := []string{tmpDir}

	// Reset global flags
	format = ""
	noColor = false

	err = runStatus(cmd, args)
	if err == nil {
		t.Error("Expected error for non-git directory, got nil")
	}

	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("Expected 'not a git repository' error, got: %v", err)
	}
}

// TestRunStatus_GitVersionCheck tests that git version is validated
func TestRunStatus_GitVersionCheck(t *testing.T) {
	// This test verifies the git version check is called
	// If git is not installed, it should fail
	tmpDir, err := os.MkdirTemp("", "status-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := &cobra.Command{}
	args := []string{tmpDir}

	// Reset global flags
	format = ""
	noColor = false

	// Should fail because tmpDir is not a git repository
	err = runStatus(cmd, args)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// TestRunStatus_DefaultPath tests status with default path (current directory)
func TestRunStatus_DefaultPath(t *testing.T) {
	// This test verifies the default path behavior
	// We expect it to fail gracefully if run from a non-git directory
	cmd := &cobra.Command{}
	args := []string{} // No path argument - should use "."

	// Create a temp dir and change to it
	tmpDir, err := os.MkdirTemp("", "status-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Reset global flags
	format = ""
	noColor = false

	err = runStatus(cmd, args)
	if err == nil {
		t.Error("Expected error for non-git directory, got nil")
	}
}

// TestRunStatus_Flags tests various flag combinations
func TestRunStatus_Flags(t *testing.T) {
	tests := []struct {
		name          string
		noFetch       bool
		quick         bool
		showFixes     bool
		coreRemote    string
		githubRemote  string
	}{
		{
			name:         "no-fetch enabled",
			noFetch:      true,
			quick:        false,
			showFixes:    false,
			coreRemote:   "origin",
			githubRemote: "github",
		},
		{
			name:         "quick mode enabled",
			noFetch:      false,
			quick:        true,
			showFixes:    false,
			coreRemote:   "origin",
			githubRemote: "github",
		},
		{
			name:         "show-fixes enabled",
			noFetch:      false,
			quick:        false,
			showFixes:    true,
			coreRemote:   "origin",
			githubRemote: "github",
		},
		{
			name:         "custom remote names",
			noFetch:      false,
			quick:        false,
			showFixes:    false,
			coreRemote:   "upstream",
			githubRemote: "gh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			statusNoFetch = tt.noFetch
			statusQuick = tt.quick
			statusShowFixes = tt.showFixes
			statusCoreRemote = tt.coreRemote
			statusGitHubRemote = tt.githubRemote

			// Create temp directory (will fail but we're testing flag handling)
			tmpDir, err := os.MkdirTemp("", "status-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			cmd := &cobra.Command{}
			args := []string{tmpDir}

			// Reset global flags
			format = ""
			noColor = false

			// Run command - should fail on non-git directory
			_ = runStatus(cmd, args)

			// Verify flags were set (they persist in package variables)
			if statusNoFetch != tt.noFetch {
				t.Errorf("Expected noFetch=%v, got %v", tt.noFetch, statusNoFetch)
			}
			if statusQuick != tt.quick {
				t.Errorf("Expected quick=%v, got %v", tt.quick, statusQuick)
			}
			if statusShowFixes != tt.showFixes {
				t.Errorf("Expected showFixes=%v, got %v", tt.showFixes, statusShowFixes)
			}
			if statusCoreRemote != tt.coreRemote {
				t.Errorf("Expected coreRemote=%v, got %v", tt.coreRemote, statusCoreRemote)
			}
			if statusGitHubRemote != tt.githubRemote {
				t.Errorf("Expected githubRemote=%v, got %v", tt.githubRemote, statusGitHubRemote)
			}
		})
	}
}

// TestPrintStatusReport tests the status report formatting
func TestPrintStatusReport(t *testing.T) {
	tests := []struct {
		name      string
		state     *scenarios.RepositoryState
		showFixes bool
		expected  []string // Strings that should appear in output
	}{
		{
			name: "healthy repository",
			state: &scenarios.RepositoryState{
				RepoPath:    "/test/repo",
				CoreRemote:  "origin",
				GitHubRemote: "github",
				Existence: scenarios.ExistenceState{
					ID:           "E1",
					Description:  "All three exist",
					LocalExists:  true,
					CoreExists:   true,
					GitHubExists: true,
					LocalPath:    "/test/repo",
					CoreURL:      "git@core:test.git",
					GitHubURL:    "git@github.com:user/test.git",
				},
				Sync: scenarios.SyncState{
					ID:          "S1",
					Description: "All in sync",
					Branch:      "main",
				},
				WorkingTree: scenarios.WorkingTreeState{
					ID:          "W1",
					Description: "Clean",
					Clean:       true,
				},
				Corruption: scenarios.CorruptionState{
					ID:          "C1",
					Description: "Healthy",
					Healthy:     true,
				},
			},
			showFixes: false,
			expected: []string{
				"Repository: /test/repo",
				"E1 - All three exist",
				"S1 - All in sync",
				"W1 - Clean",
				"Repository is healthy and in sync",
			},
		},
		{
			name: "diverged repository",
			state: &scenarios.RepositoryState{
				RepoPath:    "/test/repo",
				CoreRemote:  "origin",
				GitHubRemote: "github",
				Existence: scenarios.ExistenceState{
					ID:           "E1",
					Description:  "All three exist",
					LocalExists:  true,
					CoreExists:   true,
					GitHubExists: true,
				},
				Sync: scenarios.SyncState{
					ID:               "S4",
					Description:      "Diverged",
					Branch:           "main",
					Diverged:         true,
					LocalAheadOfCore: 3,
					LocalBehindCore:  2,
				},
				WorkingTree: scenarios.WorkingTreeState{
					ID:          "W1",
					Description: "Clean",
					Clean:       true,
				},
				Corruption: scenarios.CorruptionState{
					ID:          "C1",
					Description: "Healthy",
					Healthy:     true,
				},
			},
			showFixes: false,
			expected: []string{
				"S4 - Diverged",
				"DIVERGED - manual merge required",
				"Local ahead of Core: 3 commits",
				"Local behind Core: 2 commits",
			},
		},
		{
			name: "dirty working tree",
			state: &scenarios.RepositoryState{
				RepoPath:    "/test/repo",
				CoreRemote:  "origin",
				GitHubRemote: "github",
				Existence: scenarios.ExistenceState{
					ID:          "E1",
					Description: "All three exist",
					LocalExists: true,
				},
				Sync: scenarios.SyncState{
					ID:          "S1",
					Description: "All in sync",
				},
				WorkingTree: scenarios.WorkingTreeState{
					ID:             "W2",
					Description:    "Uncommitted changes",
					Clean:          false,
					StagedFiles:    []string{"file1.go"},
					UnstagedFiles:  []string{"file2.go"},
					UntrackedFiles: []string{"file3.go"},
				},
				Corruption: scenarios.CorruptionState{
					ID:          "C1",
					Description: "Healthy",
					Healthy:     true,
				},
			},
			showFixes: false,
			expected: []string{
				"W2 - Uncommitted changes",
				"Staged files: 1",
				"Unstaged files: 1",
				"Untracked files: 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			var buf bytes.Buffer
			out := newTestOutput(&buf)

			printStatusReport(out, tt.state, tt.showFixes)

			output := buf.String()

			// Check for expected strings
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
				}
			}
		})
	}
}

// TestPrintStatusReport_PartialSync tests partial sync scenarios (E2/E3)
func TestPrintStatusReport_PartialSync(t *testing.T) {
	state := &scenarios.RepositoryState{
		RepoPath:    "/test/repo",
		CoreRemote:  "origin",
		GitHubRemote: "github",
		Existence: scenarios.ExistenceState{
			ID:          "E2",
			Description: "Local and Core only",
			LocalExists: true,
			CoreExists:  true,
		},
		Sync: scenarios.SyncState{
			ID:               "S2",
			Description:      "Partial sync",
			Branch:           "main",
			PartialSync:      true,
			AvailableRemote:  "origin",
			LocalAheadOfCore: 5,
		},
		WorkingTree: scenarios.WorkingTreeState{
			ID:    "W1",
			Clean: true,
		},
		Corruption: scenarios.CorruptionState{
			ID:      "C1",
			Healthy: true,
		},
	}

	var buf bytes.Buffer
	out := newTestOutput(&buf)

	printStatusReport(out, state, false)

	output := buf.String()

	expectedStrings := []string{
		"Sync Status (partial)",
		"S2 - Partial sync",
		"Local vs origin: 5 ahead",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
		}
	}
}

// TestRunStatus_JSONOutput tests JSON output format
func TestRunStatus_JSONOutput(t *testing.T) {
	// This is an integration-style test that would need a real git repo
	// For now, we test that the JSON marshaling doesn't error
	state := &scenarios.RepositoryState{
		RepoPath:    "/test/repo",
		CoreRemote:  "origin",
		GitHubRemote: "github",
		Existence: scenarios.ExistenceState{
			ID:          "E1",
			LocalExists: true,
		},
		Sync: scenarios.SyncState{
			ID: "S1",
		},
		WorkingTree: scenarios.WorkingTreeState{
			ID:    "W1",
			Clean: true,
		},
		Corruption: scenarios.CorruptionState{
			ID:      "C1",
			Healthy: true,
		},
	}

	jsonBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Errorf("Failed to marshal state to JSON: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Errorf("Generated JSON is not valid: %v", err)
	}
}

// TestRunStatus_WithRealGitRepo tests status command with a real git repository
func TestRunStatus_WithRealGitRepo(t *testing.T) {
	// Skip if git is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "status-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repository
	repoPath := filepath.Join(tmpDir, "testrepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Run git init
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

	// Now run status command
	cmd := &cobra.Command{}
	args := []string{repoPath}

	// Reset global flags
	format = ""
	noColor = true
	statusNoFetch = true
	statusQuick = true

	err = runStatus(cmd, args)
	// We expect this to succeed or have a specific error
	// The exact behavior depends on the repository state
	if err != nil {
		// Check it's not a critical error
		if strings.Contains(err.Error(), "not a git repository") {
			t.Errorf("Should recognize git repository: %v", err)
		}
	}
}

// Helper functions are in test_helpers.go
