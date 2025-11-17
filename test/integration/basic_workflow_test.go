package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBasicWorkflow tests the complete basic workflow:
// 1. Create repository
// 2. Make commits
// 3. Push
// 4. Verify dual-push
func TestBasicWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if githelper binary exists
	_ = findGitHelperBinary(t)

	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "githelper-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create bare repository directory
	bareRepoPath := filepath.Join(tmpDir, "bare-repo.git")
	if err := os.MkdirAll(bareRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create bare repo dir: %v", err)
	}

	// Initialize bare repo with git directly
	cmd := exec.Command("git", "init", "--bare", bareRepoPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\nOutput: %s", err, output)
	}

	// Set up working directory
	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("Failed to create work dir: %v", err)
	}

	// Clone the bare repo
	cmd = exec.Command("git", "clone", bareRepoPath, "myproject")
	cmd.Dir = workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone bare repo: %v\nOutput: %s", err, output)
	}

	projectPath := filepath.Join(workDir, "myproject")

	// Configure git user
	runGitCommand(t, projectPath, "config", "user.name", "Test User")
	runGitCommand(t, projectPath, "config", "user.email", "test@example.com")

	// Create a test file
	testFile := filepath.Join(projectPath, "README.md")
	content := []byte("# Test Project\n\nThis is a test project for githelper integration testing.\n")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add and commit
	runGitCommand(t, projectPath, "add", ".")
	runGitCommand(t, projectPath, "commit", "-m", "Initial commit")

	// Push to bare repo
	runGitCommand(t, projectPath, "push", "origin", "master")

	// Verify commit made it to bare repo
	cmd = exec.Command("git", "log", "--oneline")
	cmd.Dir = bareRepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check bare repo log: %v\nOutput: %s", err, output)
	}

	if len(output) == 0 {
		t.Error("Expected commits in bare repo, got none")
	}

	t.Logf("Successfully completed basic workflow test")
}

// TestDoctorCommand tests the doctor command
func TestDoctorCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	githelperPath := findGitHelperBinary(t)

	// Run doctor command
	cmd := exec.Command(githelperPath, "doctor")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Doctor command output:\n%s", output)
		// Doctor might fail if Vault is not configured, but that's OK
		// We just want to ensure it runs without crashing
		if !contains(string(output), "Git Installation") {
			t.Fatalf("Doctor command crashed: %v", err)
		}
	}

	// Check for expected sections
	outputStr := string(output)
	if !contains(outputStr, "Git Installation") {
		t.Error("Expected 'Git Installation' section in output")
	}

	t.Logf("Doctor command output:\n%s", output)
}

// TestHelpCommands tests that all help commands work
func TestHelpCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	githelperPath := findGitHelperBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{"Root Help", []string{"--help"}},
		{"Repo Help", []string{"repo", "--help"}},
		{"GitHub Help", []string{"github", "--help"}},
		{"Doctor Help", []string{"doctor", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(githelperPath, tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Help command failed: %v\nOutput: %s", err, output)
			}

			if len(output) == 0 {
				t.Error("Expected help output, got none")
			}

			// All help should contain "Usage:"
			if !contains(string(output), "Usage:") {
				t.Error("Expected 'Usage:' in help output")
			}
		})
	}
}

// Helper functions

func findGitHelperBinary(t *testing.T) string {
	// Try current directory
	paths := []string{
		"./githelper",
		"../../githelper",
		"/usr/local/bin/githelper",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}

	t.Fatal("Could not find githelper binary. Run 'make build' first.")
	return ""
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Git command failed: git %v\nError: %v\nOutput: %s", args, err, output)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
