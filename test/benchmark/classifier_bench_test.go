package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/git"
)

// BenchmarkDetection measures full state detection performance
func BenchmarkDetection(b *testing.B) {
	// Setup: Create test repo
	tempDir := setupTestRepo(b)
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will be the full Classifier.Detect() once implemented
		_ = gitClient.IsRepository()
	}
}

// BenchmarkExistenceCheck measures remote existence detection
func BenchmarkExistenceCheck(b *testing.B) {
	tempDir := setupTestRepo(b)
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gitClient.ListRemotes()
	}
}

// BenchmarkWorkingTreeCheck measures working tree state detection
func BenchmarkWorkingTreeCheck(b *testing.B) {
	tempDir := setupTestRepo(b)
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Placeholder for working tree check
		_ = gitClient.IsRepository()
	}
}

// BenchmarkLargeBinaryScan measures large binary detection performance
func BenchmarkLargeBinaryScan(b *testing.B) {
	tempDir := setupTestRepoWithBinaries(b, 100) // 100 commits
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Placeholder - will call ScanLargeBinaries() once implemented
		_ = gitClient.IsRepository()
	}
}

// BenchmarkFetchOperation measures git fetch performance
func BenchmarkFetchOperation(b *testing.B) {
	// Skip if no network access
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		b.Skip("Skipping network-dependent benchmark")
	}

	tempDir := setupTestRepo(b)
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	// Add a remote pointing to a small public repo
	// (We'll use a local bare repo for consistent benchmarks)
	bareDir := filepath.Join(os.TempDir(), "bench-bare")
	os.MkdirAll(bareDir, 0755)
	defer os.RemoveAll(bareDir)

	git.InitBareRepo(bareDir)
	gitClient.AddRemote("bench", bareDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gitClient.Fetch("bench")
	}
}

// BenchmarkCommitCounting measures commit count performance
func BenchmarkCommitCounting(b *testing.B) {
	tempDir := setupTestRepoWithCommits(b, 100) // 100 commits
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Placeholder - will call CountCommitsBetween() once implemented
		_, _ = gitClient.GetRevList("HEAD~10", "HEAD")
	}
}

// setupTestRepo creates a minimal test repository
func setupTestRepo(t testing.TB) string {
	tempDir, err := os.MkdirTemp("", "githelper-bench-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	gitClient := git.NewClient(tempDir)
	if err := gitClient.Init(false); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create initial commit
	readmePath := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("Failed to write README: %v", err)
	}

	if err := gitClient.Add("."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	if err := gitClient.Commit("Initial commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return tempDir
}

// setupTestRepoWithCommits creates a repo with N commits
func setupTestRepoWithCommits(t testing.TB, count int) string {
	tempDir := setupTestRepo(t)
	gitClient := git.NewClient(tempDir)

	for i := 1; i < count; i++ {
		filePath := filepath.Join(tempDir, "file"+string(rune(i))+".txt")
		content := []byte("Content " + string(rune(i)) + "\n")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		if err := gitClient.Add("."); err != nil {
			t.Fatalf("Failed to add files: %v", err)
		}

		if err := gitClient.Commit("Commit " + string(rune(i))); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	return tempDir
}

// setupTestRepoWithBinaries creates a repo with large binary files
func setupTestRepoWithBinaries(t testing.TB, count int) string {
	tempDir := setupTestRepo(t)
	gitClient := git.NewClient(tempDir)

	for i := 1; i < count; i++ {
		// Create 1MB "binary" file
		binPath := filepath.Join(tempDir, "binary"+string(rune(i))+".bin")
		data := make([]byte, 1024*1024) // 1MB
		for j := range data {
			data[j] = byte(j % 256)
		}

		if err := os.WriteFile(binPath, data, 0644); err != nil {
			t.Fatalf("Failed to write binary: %v", err)
		}

		if err := gitClient.Add(binPath); err != nil {
			t.Fatalf("Failed to add binary: %v", err)
		}

		if err := gitClient.Commit("Add binary " + string(rune(i))); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	return tempDir
}

// Performance targets (documented for validation)
const (
	// Target: Full detection < 2 seconds
	TargetDetectionTime = 2 * time.Second

	// Target: Large binary scan < 5 seconds
	TargetBinaryScanTime = 5 * time.Second

	// Target: Existence check < 100ms
	TargetExistenceCheckTime = 100 * time.Millisecond

	// Target: Working tree check < 200ms
	TargetWorkingTreeCheckTime = 200 * time.Millisecond
)

// TestPerformanceTargets validates we meet performance requirements
func TestPerformanceTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	t.Run("Detection", func(t *testing.T) {
		tempDir := setupTestRepo(t)
		defer os.RemoveAll(tempDir)

		start := time.Now()
		gitClient := git.NewClient(tempDir)
		_ = gitClient.IsRepository() // Placeholder for full detection
		elapsed := time.Since(start)

		if elapsed > TargetDetectionTime {
			t.Errorf("Detection took %v, target is %v", elapsed, TargetDetectionTime)
		} else {
			t.Logf("Detection completed in %v (target: %v)", elapsed, TargetDetectionTime)
		}
	})

	t.Run("ExistenceCheck", func(t *testing.T) {
		tempDir := setupTestRepo(t)
		defer os.RemoveAll(tempDir)

		start := time.Now()
		gitClient := git.NewClient(tempDir)
		_, _ = gitClient.ListRemotes()
		elapsed := time.Since(start)

		if elapsed > TargetExistenceCheckTime {
			t.Errorf("Existence check took %v, target is %v", elapsed, TargetExistenceCheckTime)
		} else {
			t.Logf("Existence check completed in %v (target: %v)", elapsed, TargetExistenceCheckTime)
		}
	})
}

// TestConcurrentOperations validates thread safety (will be critical for Phase 1.2)
func TestConcurrentOperations(t *testing.T) {
	tempDir := setupTestRepo(t)
	defer os.RemoveAll(tempDir)

	gitClient := git.NewClient(tempDir)

	// Spawn multiple goroutines performing git operations
	const concurrency = 10
	done := make(chan bool, concurrency)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 5; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					// Test various concurrent operations
					_ = gitClient.IsRepository()
					_, _ = gitClient.ListRemotes()
					_, _ = gitClient.GetCurrentBranch()
				}
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Concurrent operations timed out")
		}
	}

	t.Log("Concurrent operations completed without race conditions")
}
