package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalExists_FromRepositoryRoot(t *testing.T) {
	// Create a temporary git repository
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.Mkdir(repoRoot, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Test from repository root
	client := NewClient(repoRoot)
	exists, path := client.LocalExists()

	if !exists {
		t.Error("Expected repository to exist")
	}
	if path != repoRoot {
		t.Errorf("Expected path %s, got %s", repoRoot, path)
	}
}

func TestLocalExists_FromSubdirectory(t *testing.T) {
	// Create a temporary git repository with subdirectories
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	subDir := filepath.Join(repoRoot, "sub", "nested")

	// Create directory structure
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirs: %v", err)
	}

	// Initialize git repo at root
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Test from subdirectory
	client := NewClient(subDir)
	exists, path := client.LocalExists()

	if !exists {
		t.Error("Expected repository to exist from subdirectory")
	}
	// Should return repository root, not subdirectory
	if path != repoRoot {
		t.Errorf("Expected repository root %s, got %s", repoRoot, path)
	}
}

func TestLocalExists_NotInRepository(t *testing.T) {
	tmpDir := t.TempDir()

	client := NewClient(tmpDir)
	exists, path := client.LocalExists()

	if exists {
		t.Error("Expected no repository")
	}
	if path != "" {
		t.Errorf("Expected empty path, got %s", path)
	}
}

func TestLocalExists_BareRepository(t *testing.T) {
	// Create a bare repository
	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "bare.git")
	if err := os.Mkdir(bareRepo, 0755); err != nil {
		t.Fatalf("Failed to create bare repo dir: %v", err)
	}

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init bare repo: %v", err)
	}

	client := NewClient(bareRepo)
	exists, path := client.LocalExists()

	if !exists {
		t.Error("Expected bare repository to exist")
	}
	if path != bareRepo {
		t.Errorf("Expected path %s, got %s", bareRepo, path)
	}
}
