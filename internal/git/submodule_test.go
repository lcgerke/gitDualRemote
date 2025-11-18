package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetOrphanedSubmodules(t *testing.T) {
	// Create a temporary directory for test repo
	tmpDir, err := os.MkdirTemp("", "git-orphaned-submodule-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "config", "user.email", "test@example.com").Dir = tmpDir
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write README: %v", err)
	}
	exec.Command("git", "add", "README.md").Dir = tmpDir
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	// Test 1: No submodules - should return empty list
	t.Run("no submodules", func(t *testing.T) {
		client := NewClient(tmpDir)
		orphaned, err := client.GetOrphanedSubmodules()
		if err != nil {
			t.Fatalf("GetOrphanedSubmodules failed: %v", err)
		}
		if len(orphaned) != 0 {
			t.Errorf("Expected 0 orphaned submodules, got %d", len(orphaned))
		}
	})

	// Test 2: Create an orphaned submodule by directly adding a gitlink to index
	t.Run("orphaned submodule detected", func(t *testing.T) {
		// Create a fake submodule directory
		submodulePath := filepath.Join(tmpDir, "fake-submodule")
		os.MkdirAll(submodulePath, 0755)

		// Initialize it as a git repo
		exec.Command("git", "init").Dir = submodulePath
		exec.Command("git", "config", "user.email", "test@example.com").Dir = submodulePath
		exec.Command("git", "config", "user.name", "Test User").Run()

		subReadme := filepath.Join(submodulePath, "README.md")
		os.WriteFile(subReadme, []byte("# Submodule\n"), 0644)
		exec.Command("git", "add", "README.md").Dir = submodulePath
		exec.Command("git", "commit", "-m", "Sub commit").Run()

		// Get the commit hash
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = submodulePath
		hashOutput, err := hashCmd.Output()
		if err != nil {
			t.Fatalf("Failed to get hash: %v", err)
		}
		hash := string(hashOutput)
		if len(hash) > 0 && hash[len(hash)-1] == '\n' {
			hash = hash[:len(hash)-1]
		}

		// Manually add as gitlink (mode 160000) to parent repo's index
		// This simulates an orphaned submodule situation
		updateIndexCmd := exec.Command("git", "update-index", "--add",
			"--cacheinfo", "160000", hash, "fake-submodule")
		updateIndexCmd.Dir = tmpDir
		output, err := updateIndexCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to create gitlink: %v\nOutput: %s", err, output)
		}

		// Now test - should detect one orphaned submodule
		client := NewClient(tmpDir)
		orphaned, err := client.GetOrphanedSubmodules()
		if err != nil {
			t.Fatalf("GetOrphanedSubmodules failed: %v", err)
		}

		if len(orphaned) != 1 {
			t.Fatalf("Expected 1 orphaned submodule, got %d", len(orphaned))
		}

		if orphaned[0].Path != "fake-submodule" {
			t.Errorf("Expected path 'fake-submodule', got '%s'", orphaned[0].Path)
		}

		if orphaned[0].Hash != hash {
			t.Errorf("Expected hash '%s', got '%s'", hash, orphaned[0].Hash)
		}
	})

	// Test 3: Properly configured submodule in .gitmodules should NOT be orphaned
	t.Run("proper submodule not orphaned", func(t *testing.T) {
		// Clean up from previous test
		os.RemoveAll(tmpDir)
		tmpDir, _ = os.MkdirTemp("", "git-proper-submodule-test-*")
		defer os.RemoveAll(tmpDir)

		exec.Command("git", "init").Dir = tmpDir
		exec.Command("git", "config", "user.email", "test@example.com").Dir = tmpDir
		exec.Command("git", "config", "user.name", "Test User").Run()

		readmePath := filepath.Join(tmpDir, "README.md")
		os.WriteFile(readmePath, []byte("# Test\n"), 0644)
		exec.Command("git", "add", "README.md").Dir = tmpDir
		exec.Command("git", "commit", "-m", "Initial").Run()

		// Add a proper submodule using git submodule command
		// (This would normally add to .gitmodules)
		// For testing, we'll manually create .gitmodules
		gitmodulesPath := filepath.Join(tmpDir, ".gitmodules")
		gitmodulesContent := `[submodule "proper-sub"]
	path = proper-sub
	url = https://github.com/example/repo.git
`
		os.WriteFile(gitmodulesPath, []byte(gitmodulesContent), 0644)

		// Create the submodule directory and gitlink
		subPath := filepath.Join(tmpDir, "proper-sub")
		os.MkdirAll(subPath, 0755)
		exec.Command("git", "init").Dir = subPath
		exec.Command("git", "config", "user.email", "test@example.com").Dir = subPath
		exec.Command("git", "config", "user.name", "Test").Run()
		os.WriteFile(filepath.Join(subPath, "file.txt"), []byte("content"), 0644)
		exec.Command("git", "add", ".").Dir = subPath
		exec.Command("git", "commit", "-m", "Sub").Run()

		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = subPath
		hashOutput, err := hashCmd.Output()
		if err != nil {
			t.Fatalf("Failed to get hash: %v", err)
		}
		hash := string(hashOutput)
		if len(hash) > 0 && hash[len(hash)-1] == '\n' {
			hash = hash[:len(hash)-1]
		}

		updateCmd := exec.Command("git", "update-index", "--add", "--cacheinfo", "160000", hash, "proper-sub")
		updateCmd.Dir = tmpDir
		updateCmd.Run()
		exec.Command("git", "add", ".gitmodules").Dir = tmpDir
		exec.Command("git", "commit", "-m", "Add submodule").Run()

		// Test - should NOT detect as orphaned
		client := NewClient(tmpDir)
		orphaned, err := client.GetOrphanedSubmodules()
		if err != nil {
			t.Fatalf("GetOrphanedSubmodules failed: %v", err)
		}

		if len(orphaned) != 0 {
			t.Errorf("Expected 0 orphaned submodules (properly configured), got %d", len(orphaned))
		}
	})
}
