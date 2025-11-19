package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/errors"
	"github.com/lcgerke/githelper/internal/state"
)

// TestNetworkFailure_UnreachableRemote tests handling of unreachable remote repositories
func TestNetworkFailure_UnreachableRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	// Create a local git repository
	workDir := filepath.Join(tmpDir, "work")
	setupLocalGitRepo(t, workDir)

	// Add a remote that points to a non-existent/unreachable location
	unreachableURL := "ssh://git@nonexistent.invalid.domain:9999/repo.git"
	runGitCommand(t, workDir, "remote", "add", "origin", unreachableURL)

	// Attempt to fetch from unreachable remote
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	// Verify error occurs
	if err == nil {
		t.Fatal("Expected fetch to fail with unreachable remote, but it succeeded")
	}

	// Verify error message contains network-related information
	outputStr := string(output)
	if !containsAny(outputStr, []string{"Could not resolve host", "Connection", "Network", "unreachable", "timed out", "unable to fork", "cannot run ssh"}) {
		t.Errorf("Expected network/connection error message, got: %s", outputStr)
	}

	// Verify error can be wrapped with our error type
	netErr := errors.NetworkError("fetch", err)
	if netErr.Type != errors.ErrorTypeNetwork {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeNetwork, netErr.Type)
	}

	// Verify user-friendly message includes recovery suggestion
	userMsg := netErr.UserFriendlyMessage()
	if !strings.Contains(userMsg, "Suggestion") {
		t.Error("Expected user-friendly message to contain recovery suggestion")
	}
	if !strings.Contains(userMsg, "internet connection") {
		t.Error("Expected suggestion to mention checking internet connection")
	}

	t.Logf("Network failure properly detected and handled")
	t.Logf("Error message: %s", userMsg)
}

// TestAuthenticationFailure_InvalidCredentials tests handling of authentication failures
func TestAuthenticationFailure_InvalidCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	workDir := filepath.Join(tmpDir, "work")
	setupLocalGitRepo(t, workDir)

	// Test 1: Invalid SSH key path
	t.Run("InvalidSSHKey", func(t *testing.T) {
		// Set GIT_SSH_COMMAND to use a non-existent key
		invalidKeyPath := filepath.Join(tmpDir, "nonexistent_key")
		remoteURL := "git@github.com:test/test-repo.git"
		runGitCommand(t, workDir, "remote", "add", "github", remoteURL)

		cmd := exec.Command("git", "fetch", "github")
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", invalidKeyPath),
		)
		output, err := cmd.CombinedOutput()

		// Verify authentication failure
		if err == nil {
			t.Log("Note: Fetch succeeded (may have used default credentials)")
		} else {
			outputStr := string(output)
			// Look for authentication-related errors
			if containsAny(outputStr, []string{"Permission denied", "authentication", "publickey", "Could not resolve host"}) {
				t.Logf("Authentication error properly detected: %s", outputStr)
			}
		}

		// Test our error wrapper
		authErr := errors.GitHubAuthFailed(err)
		if authErr.Type != errors.ErrorTypeGitHub {
			t.Errorf("Expected error type %s, got %s", errors.ErrorTypeGitHub, authErr.Type)
		}

		userMsg := authErr.UserFriendlyMessage()
		if !strings.Contains(userMsg, "Suggestion") {
			t.Error("Expected user-friendly message to contain recovery suggestion")
		}
		if !strings.Contains(userMsg, "PAT") || !strings.Contains(userMsg, "Vault") {
			t.Error("Expected suggestion to mention PAT and Vault")
		}

		t.Logf("Authentication error message: %s", userMsg)
	})

	// Test 2: Invalid/expired token simulation
	t.Run("InvalidToken", func(t *testing.T) {
		// Test error wrapping for invalid token (401 error)
		err := fmt.Errorf("401 Bad credentials")
		authErr := errors.GitHubAuthFailed(err)

		// Verify error structure
		if authErr.Type != errors.ErrorTypeGitHub {
			t.Errorf("Expected error type %s, got %s", errors.ErrorTypeGitHub, authErr.Type)
		}
		if authErr.Hint == "" {
			t.Error("Expected hint for authentication failure")
		}

		userMsg := authErr.UserFriendlyMessage()
		if !strings.Contains(userMsg, "scopes") {
			t.Error("Expected message to mention required scopes")
		}

		t.Logf("Invalid token error handled: %s", userMsg)
	})
}

// TestDivergenceScenario_RepositoriesDiverged tests handling of diverged repositories
func TestDivergenceScenario_RepositoriesDiverged(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	// Setup: Create two bare repos and a working repo
	bareRepo1 := filepath.Join(tmpDir, "bare1.git")
	bareRepo2 := filepath.Join(tmpDir, "bare2.git")
	workDir := filepath.Join(tmpDir, "work")

	// Initialize first bare repo
	runCommand(t, "git", "init", "--bare", bareRepo1)

	// Clone and create initial commit
	runCommand(t, "git", "clone", bareRepo1, workDir)
	setupGitUser(t, workDir)
	createTestFile(t, workDir, "file.txt", "initial content")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Initial commit")
	runGitCommand(t, workDir, "push", "origin", "master")

	// Initialize second bare repo with same content
	runCommand(t, "git", "clone", "--bare", workDir, bareRepo2)

	// Add second remote
	runGitCommand(t, workDir, "remote", "add", "secondary", bareRepo2)

	// Create divergence: Make different commits to each remote

	// Commit to bare1 via work directory
	createTestFile(t, workDir, "file1.txt", "content for bare1")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Commit for bare1")
	runGitCommand(t, workDir, "push", "origin", "master")

	// Reset to previous commit
	runGitCommand(t, workDir, "reset", "--hard", "HEAD~1")

	// Create different commit for bare2
	createTestFile(t, workDir, "file2.txt", "content for bare2")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Commit for bare2")
	runGitCommand(t, workDir, "push", "secondary", "master", "-f")

	// Now fetch both remotes
	runGitCommand(t, workDir, "fetch", "origin")
	runGitCommand(t, workDir, "fetch", "secondary")

	// Check for divergence
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", "origin/master...secondary/master")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check divergence: %v\nOutput: %s", err, output)
	}

	outputStr := strings.TrimSpace(string(output))
	parts := strings.Fields(outputStr)
	if len(parts) != 2 {
		t.Fatalf("Unexpected divergence output format: %s", outputStr)
	}

	// Both should have commits the other doesn't (diverged)
	if parts[0] == "0" || parts[1] == "0" {
		t.Error("Expected repositories to be diverged, but they're not")
	}

	t.Logf("Divergence detected: origin has %s unique commits, secondary has %s unique commits", parts[0], parts[1])

	// Test error creation for divergence
	divergeErr := errors.DivergenceDetected(1, 1)
	if divergeErr.Type != errors.ErrorTypeGit {
		t.Errorf("Expected error type %s, got %s", errors.ErrorTypeGit, divergeErr.Type)
	}

	userMsg := divergeErr.UserFriendlyMessage()
	if !strings.Contains(userMsg, "diverged") {
		t.Error("Expected message to mention divergence")
	}
	if !strings.Contains(userMsg, "Manual resolution") {
		t.Error("Expected message to suggest manual resolution")
	}

	t.Logf("Divergence error message: %s", userMsg)
}

// TestConflictScenario_MergeConflicts tests handling of merge conflicts
func TestConflictScenario_MergeConflicts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	workDir := filepath.Join(tmpDir, "work")
	setupLocalGitRepo(t, workDir)

	// Create a file and commit
	testFile := filepath.Join(workDir, "conflict.txt")
	createTestFile(t, workDir, "conflict.txt", "line 1\nline 2\nline 3\n")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Initial commit")

	// Create a branch
	runGitCommand(t, workDir, "checkout", "-b", "feature")

	// Modify file on feature branch
	createTestFile(t, workDir, "conflict.txt", "line 1\nfeature change\nline 3\n")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Feature change")

	// Go back to master and make conflicting change
	runGitCommand(t, workDir, "checkout", "master")
	createTestFile(t, workDir, "conflict.txt", "line 1\nmaster change\nline 3\n")
	runGitCommand(t, workDir, "add", ".")
	runGitCommand(t, workDir, "commit", "-m", "Master change")

	// Attempt to merge - should create conflict
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	// Verify merge conflict occurred
	if err == nil {
		t.Fatal("Expected merge to fail with conflict, but it succeeded")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "conflict") && !strings.Contains(outputStr, "CONFLICT") {
		t.Errorf("Expected merge conflict message, got: %s", outputStr)
	}

	// Verify conflict markers in file
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read conflicted file: %v", err)
	}

	contentStr := string(content)
	hasConflictMarkers := strings.Contains(contentStr, "<<<<<<<") &&
		strings.Contains(contentStr, "=======") &&
		strings.Contains(contentStr, ">>>>>>>")

	if !hasConflictMarkers {
		t.Error("Expected conflict markers in file")
	}

	// Check git status shows conflict
	cmd = exec.Command("git", "status")
	cmd.Dir = workDir
	statusOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Git status during conflict: %s", statusOutput)
	}

	statusStr := string(statusOutput)
	if !containsAny(statusStr, []string{"both modified", "Unmerged paths", "conflict"}) {
		t.Error("Expected git status to show merge conflict state")
	}

	t.Logf("Merge conflict properly detected")
	t.Logf("Status output: %s", statusStr)

	// Test conflict resolution suggestion
	conflictErr := errors.New(errors.ErrorTypeGit, "Merge conflict detected")
	conflictErr = errors.WithHint(conflictErr, "Resolve conflicts in the affected files, then run 'git add <file>' and 'git commit'")

	userMsg := conflictErr.UserFriendlyMessage()
	if !strings.Contains(userMsg, "Resolve conflicts") {
		t.Error("Expected message to include conflict resolution steps")
	}

	t.Logf("Conflict resolution message: %s", userMsg)
}

// TestMissingRepository_NotFound tests handling of missing repositories
func TestMissingRepository_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	// Test 1: Missing from state
	t.Run("MissingFromState", func(t *testing.T) {
		stateDir := filepath.Join(tmpDir, "state")
		stateMgr, err := state.NewManager(stateDir)
		if err != nil {
			t.Fatalf("Failed to create state manager: %v", err)
		}

		// Try to get a non-existent repository
		_, err = stateMgr.GetRepository("nonexistent-repo")
		if err == nil {
			t.Fatal("Expected error when getting non-existent repository")
		}

		// Verify error message
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' in error message, got: %v", err)
		}

		// Test our error wrapper
		repoErr := errors.RepositoryNotFound("nonexistent-repo")
		if repoErr.Type != errors.ErrorTypeState {
			t.Errorf("Expected error type %s, got %s", errors.ErrorTypeState, repoErr.Type)
		}

		userMsg := repoErr.UserFriendlyMessage()
		if !strings.Contains(userMsg, "Suggestion") {
			t.Error("Expected user-friendly message to contain suggestion")
		}
		if !strings.Contains(userMsg, "repo list") {
			t.Error("Expected suggestion to mention 'repo list' command")
		}

		t.Logf("Missing repository error message: %s", userMsg)
	})

	// Test 2: Missing git repository (invalid path)
	t.Run("InvalidGitRepository", func(t *testing.T) {
		nonexistentPath := filepath.Join(tmpDir, "nonexistent")

		cmd := exec.Command("git", "status")
		cmd.Dir = nonexistentPath
		output, err := cmd.CombinedOutput()

		if err == nil {
			t.Fatal("Expected git status to fail in non-existent directory")
		}

		outputStr := string(output)
		t.Logf("Git error for invalid path: %s", outputStr)

		// Test error wrapping
		gitErr := errors.Wrap(errors.ErrorTypeGit, "Repository path does not exist", err)
		gitErr = errors.WithHint(gitErr, "Verify the repository path is correct")

		userMsg := gitErr.UserFriendlyMessage()
		if !strings.Contains(userMsg, "path") {
			t.Error("Expected message to mention path")
		}

		t.Logf("Invalid path error message: %s", userMsg)
	})

	// Test 3: Remote repository not found (404)
	t.Run("RemoteNotFound", func(t *testing.T) {
		workDir := filepath.Join(tmpDir, "work")
		setupLocalGitRepo(t, workDir)

		// Add remote that doesn't exist
		nonexistentRepo := "https://github.com/nonexistent-user-12345/nonexistent-repo-67890.git"
		runGitCommand(t, workDir, "remote", "add", "origin", nonexistentRepo)

		// Try to fetch
		cmd := exec.Command("git", "fetch", "origin")
		cmd.Dir = workDir
		// Set a short timeout to avoid long waits
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		output, err := cmd.CombinedOutput()

		if err == nil {
			t.Log("Note: Fetch succeeded (unexpected)")
		}

		outputStr := string(output)
		if containsAny(outputStr, []string{"not found", "404", "does not exist", "Could not resolve host"}) {
			t.Logf("Remote not found error properly detected: %s", outputStr)
		}

		// Test error handling
		notFoundErr := errors.Wrap(errors.ErrorTypeGitHub, "Remote repository not found", err)
		notFoundErr = errors.WithHint(notFoundErr, "Verify the repository exists and you have access to it")

		userMsg := notFoundErr.UserFriendlyMessage()
		t.Logf("Remote not found error message: %s", userMsg)
	})
}

// TestInvalidState_CorruptedStateFile tests handling of corrupted state files
func TestInvalidState_CorruptedStateFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	// Test 1: Invalid YAML syntax
	t.Run("InvalidYAML", func(t *testing.T) {
		stateDir := filepath.Join(tmpDir, "state-invalid-yaml")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("Failed to create state dir: %v", err)
		}

		stateFile := filepath.Join(stateDir, "state.yaml")
		invalidYAML := `
repositories:
  test-repo:
    path: /path/to/repo
    remote: origin
    created: 2024-01-01T00:00:00Z
    github:
      enabled: true
      user: testuser
      # Invalid YAML - unclosed bracket
      repo: [test
`
		if err := os.WriteFile(stateFile, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("Failed to write invalid state file: %v", err)
		}

		// Try to load corrupted state
		stateMgr, err := state.NewManager(stateDir)
		if err != nil {
			t.Fatalf("Failed to create state manager: %v", err)
		}

		_, err = stateMgr.Load()
		if err == nil {
			t.Fatal("Expected error when loading corrupted state file")
		}

		if !strings.Contains(err.Error(), "unmarshal") && !strings.Contains(err.Error(), "yaml") {
			t.Errorf("Expected unmarshal/yaml error, got: %v", err)
		}

		// Test error wrapping
		stateErr := errors.StateCorrupted(err)
		if stateErr.Type != errors.ErrorTypeState {
			t.Errorf("Expected error type %s, got %s", errors.ErrorTypeState, stateErr.Type)
		}

		userMsg := stateErr.UserFriendlyMessage()
		if !strings.Contains(userMsg, "Suggestion") {
			t.Error("Expected user-friendly message to contain suggestion")
		}
		if !strings.Contains(userMsg, "Backup") {
			t.Error("Expected suggestion to mention backing up state file")
		}
		if !strings.Contains(userMsg, "doctor") {
			t.Error("Expected suggestion to mention doctor command")
		}

		t.Logf("Corrupted state error message: %s", userMsg)
	})

	// Test 2: Missing required fields
	t.Run("MissingFields", func(t *testing.T) {
		stateDir := filepath.Join(tmpDir, "state-missing-fields")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("Failed to create state dir: %v", err)
		}

		stateFile := filepath.Join(stateDir, "state.yaml")
		incompleteYAML := `
repositories:
  test-repo:
    # Missing required fields like path and remote
    created: 2024-01-01T00:00:00Z
`
		if err := os.WriteFile(stateFile, []byte(incompleteYAML), 0644); err != nil {
			t.Fatalf("Failed to write incomplete state file: %v", err)
		}

		stateMgr, err := state.NewManager(stateDir)
		if err != nil {
			t.Fatalf("Failed to create state manager: %v", err)
		}

		loadedState, err := stateMgr.Load()
		if err != nil {
			t.Fatalf("Failed to load state: %v", err)
		}

		// Verify that repository loaded but has missing fields
		repo, exists := loadedState.Repositories["test-repo"]
		if !exists {
			t.Fatal("Expected repository to exist in state")
		}

		if repo.Path != "" || repo.Remote != "" {
			t.Error("Expected path and remote to be empty")
		}

		t.Logf("State loaded with missing fields - validation should occur at usage time")
	})

	// Test 3: File permission errors
	t.Run("PermissionDenied", func(t *testing.T) {
		// Only test on Unix-like systems
		if os.Getenv("GOOS") == "windows" {
			t.Skip("Skipping permission test on Windows")
		}

		// Skip if running as root (root can read files even with 0000 permissions)
		if os.Geteuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		stateDir := filepath.Join(tmpDir, "state-no-perms")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("Failed to create state dir: %v", err)
		}

		stateFile := filepath.Join(stateDir, "state.yaml")
		validYAML := `
repositories:
  test-repo:
    path: /path/to/repo
    remote: origin
    created: 2024-01-01T00:00:00Z
`
		if err := os.WriteFile(stateFile, []byte(validYAML), 0644); err != nil {
			t.Fatalf("Failed to write state file: %v", err)
		}

		// Remove read permissions
		if err := os.Chmod(stateFile, 0000); err != nil {
			t.Fatalf("Failed to change file permissions: %v", err)
		}
		// Restore permissions after test
		defer os.Chmod(stateFile, 0644)

		stateMgr, err := state.NewManager(stateDir)
		if err != nil {
			t.Fatalf("Failed to create state manager: %v", err)
		}

		_, err = stateMgr.Load()
		if err == nil {
			t.Fatal("Expected error when reading file without permissions")
		}

		// Verify we got a permission error
		errStr := err.Error()
		if !strings.Contains(errStr, "permission") && !strings.Contains(errStr, "denied") {
			t.Logf("Note: Got error but not explicitly permission-related: %v", err)
		}

		// Test error wrapping
		fsErr := errors.Wrap(errors.ErrorTypeFileSystem, "Cannot read state file", err)
		fsErr = errors.WithHint(fsErr, "Check file permissions on state file")

		if fsErr.Type != errors.ErrorTypeFileSystem {
			t.Errorf("Expected error type %s, got %s", errors.ErrorTypeFileSystem, fsErr.Type)
		}

		t.Logf("Permission error handled: %s", fsErr.UserFriendlyMessage())
	})

	// Test 4: Empty/uninitialized state (should not error)
	t.Run("EmptyState", func(t *testing.T) {
		stateDir := filepath.Join(tmpDir, "state-empty")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatalf("Failed to create state dir: %v", err)
		}

		// Don't create state file - should load empty state
		stateMgr, err := state.NewManager(stateDir)
		if err != nil {
			t.Fatalf("Failed to create state manager: %v", err)
		}

		loadedState, err := stateMgr.Load()
		if err != nil {
			t.Fatalf("Expected empty state to load successfully, got error: %v", err)
		}

		if len(loadedState.Repositories) != 0 {
			t.Errorf("Expected empty repositories map, got %d entries", len(loadedState.Repositories))
		}

		t.Log("Empty state handled correctly")
	})
}

// TestStateUpdateOnFailure tests that state is properly updated when operations fail
func TestStateUpdateOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTestDir(t)
	defer os.RemoveAll(tmpDir)

	stateDir := filepath.Join(tmpDir, "state")
	stateMgr, err := state.NewManager(stateDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Add a repository to state
	repo := &state.Repository{
		Path:    "/path/to/repo",
		Remote:  "origin",
		Created: time.Now(),
		GitHub: &state.GitHub{
			Enabled:    true,
			User:       "testuser",
			Repo:       "test-repo",
			SyncStatus: "synced",
		},
	}

	err = stateMgr.AddRepository("test-repo", repo)
	if err != nil {
		t.Fatalf("Failed to add repository to state: %v", err)
	}

	// Simulate a sync failure and update state
	errorMsg := "Network error: connection timeout"
	err = stateMgr.UpdateGitHubStatus("test-repo", "error", errorMsg)
	if err != nil {
		t.Fatalf("Failed to update GitHub status: %v", err)
	}

	// Load state and verify error was recorded
	updatedRepo, err := stateMgr.GetRepository("test-repo")
	if err != nil {
		t.Fatalf("Failed to load repository from state: %v", err)
	}

	if updatedRepo.GitHub.SyncStatus != "error" {
		t.Errorf("Expected sync status 'error', got '%s'", updatedRepo.GitHub.SyncStatus)
	}

	if updatedRepo.GitHub.LastError != errorMsg {
		t.Errorf("Expected last error '%s', got '%s'", errorMsg, updatedRepo.GitHub.LastError)
	}

	if !updatedRepo.GitHub.NeedsRetry {
		t.Error("Expected NeedsRetry to be true after error")
	}

	t.Log("State properly updated with error information")
	t.Logf("Sync status: %s", updatedRepo.GitHub.SyncStatus)
	t.Logf("Last error: %s", updatedRepo.GitHub.LastError)
	t.Logf("Needs retry: %v", updatedRepo.GitHub.NeedsRetry)

	// Test recovery: Update state after successful retry
	err = stateMgr.UpdateGitHubStatus("test-repo", "synced", "")
	if err != nil {
		t.Fatalf("Failed to update GitHub status after recovery: %v", err)
	}

	recoveredRepo, err := stateMgr.GetRepository("test-repo")
	if err != nil {
		t.Fatalf("Failed to load recovered repository: %v", err)
	}

	if recoveredRepo.GitHub.SyncStatus != "synced" {
		t.Errorf("Expected sync status 'synced' after recovery, got '%s'", recoveredRepo.GitHub.SyncStatus)
	}

	if recoveredRepo.GitHub.LastError != "" {
		t.Errorf("Expected last error to be cleared, got '%s'", recoveredRepo.GitHub.LastError)
	}

	if recoveredRepo.GitHub.NeedsRetry {
		t.Error("Expected NeedsRetry to be false after successful sync")
	}

	t.Log("State properly updated after recovery")
}

// TestErrorMessageQuality tests that all errors have user-friendly messages and hints
func TestErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name     string
		err      *errors.GitHelperError
		mustHave []string
	}{
		{
			name: "VaultUnreachable",
			err:  errors.VaultUnreachable("http://localhost:8200", fmt.Errorf("connection refused")),
			mustHave: []string{
				"Vault unreachable",
				"Suggestion",
				"doctor",
			},
		},
		{
			name: "GitNotInstalled",
			err:  errors.GitNotInstalled(fmt.Errorf("executable not found")),
			mustHave: []string{
				"not installed",
				"Suggestion",
				"package manager",
			},
		},
		{
			name: "RepositoryNotFound",
			err:  errors.RepositoryNotFound("my-repo"),
			mustHave: []string{
				"not found",
				"Suggestion",
				"repo list",
			},
		},
		{
			name: "GitHubAuthFailed",
			err:  errors.GitHubAuthFailed(fmt.Errorf("401 unauthorized")),
			mustHave: []string{
				"authentication failed",
				"Suggestion",
				"PAT",
				"Vault",
			},
		},
		{
			name: "DivergenceDetected",
			err:  errors.DivergenceDetected(3, 2),
			mustHave: []string{
				"diverged",
				"Suggestion",
				"Manual resolution",
			},
		},
		{
			name: "StateCorrupted",
			err:  errors.StateCorrupted(fmt.Errorf("yaml parse error")),
			mustHave: []string{
				"corrupted",
				"Suggestion",
				"Backup",
				"doctor",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userMsg := tt.err.UserFriendlyMessage()

			for _, required := range tt.mustHave {
				if !strings.Contains(userMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, userMsg)
				}
			}

			// Verify error type is set
			if tt.err.Type == "" {
				t.Error("Error type should not be empty")
			}

			// Verify message is not empty
			if tt.err.Message == "" {
				t.Error("Error message should not be empty")
			}

			t.Logf("Error type: %s", tt.err.Type)
			t.Logf("User message: %s", userMsg)
		})
	}
}

// Helper functions

func setupTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "githelper-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tmpDir
}

func setupLocalGitRepo(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	runGitCommand(t, path, "init")
	setupGitUser(t, path)
}

func setupGitUser(t *testing.T, path string) {
	runGitCommand(t, path, "config", "user.name", "Test User")
	runGitCommand(t, path, "config", "user.email", "test@example.com")
	// Disable commit signing for tests
	runGitCommand(t, path, "config", "commit.gpgsign", "false")
}

func createTestFile(t *testing.T, dir, filename, content string) {
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}

func runCommand(t *testing.T, name string, args ...string) {
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Command failed: %s %v\nError: %v\nOutput: %s", name, args, err, output)
	}
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
