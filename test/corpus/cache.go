package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Cache manages repository caching for corpus testing
type Cache struct {
	baseDir string
	ttl     time.Duration
	enabled bool
}

// NewCache creates a new cache instance
func NewCache(enabled bool, ttl time.Duration) (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".cache", "githelper-corpus")

	if enabled {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	return &Cache{
		baseDir: baseDir,
		ttl:     ttl,
		enabled: enabled,
	}, nil
}

// Get retrieves a cached repo or clones it
// Returns: (localPath, fromCache, error)
func (c *Cache) Get(url string) (string, bool, error) {
	if !c.enabled {
		// No caching - use temp directory
		tmpDir, err := os.MkdirTemp("", "githelper-corpus-*")
		if err != nil {
			return "", false, fmt.Errorf("failed to create temp dir: %w", err)
		}

		if err := c.clone(url, tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", false, err
		}

		return tmpDir, false, nil
	}

	// Generate cache key from URL
	cacheKey := c.generateCacheKey(url)
	cachePath := filepath.Join(c.baseDir, cacheKey)

	// Check if cached version exists and is valid
	if c.isCacheValid(cachePath) {
		// Update via fetch
		if err := c.update(cachePath); err != nil {
			// Fetch failed, but we can still use stale cache
			fmt.Printf("Warning: fetch failed for %s, using stale cache: %v\n", url, err)
		}
		return cachePath, true, nil
	}

	// Cache miss or expired - clone fresh
	if err := os.RemoveAll(cachePath); err != nil && !os.IsNotExist(err) {
		return "", false, fmt.Errorf("failed to remove stale cache: %w", err)
	}

	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return "", false, fmt.Errorf("failed to create cache dir: %w", err)
	}

	if err := c.clone(url, cachePath); err != nil {
		os.RemoveAll(cachePath)
		return "", false, err
	}

	// Create timestamp file
	timestampFile := filepath.Join(cachePath, ".cache-timestamp")
	if err := os.WriteFile(timestampFile, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		// Non-fatal
		fmt.Printf("Warning: failed to write timestamp: %v\n", err)
	}

	return cachePath, false, nil
}

// Clear removes all cached repositories
func (c *Cache) Clear() error {
	if !c.enabled {
		return nil
	}
	return os.RemoveAll(c.baseDir)
}

// ClearExpired removes only expired cache entries
func (c *Cache) ClearExpired() error {
	if !c.enabled {
		return nil
	}

	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cachePath := filepath.Join(c.baseDir, entry.Name())
		if !c.isCacheValid(cachePath) {
			if err := os.RemoveAll(cachePath); err != nil {
				fmt.Printf("Warning: failed to remove expired cache %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}

// generateCacheKey creates a stable directory name from URL
func (c *Cache) generateCacheKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes (32 hex chars)
}

// isCacheValid checks if cache exists and is within TTL
func (c *Cache) isCacheValid(path string) bool {
	// Check if .git exists
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return false
	}

	// Check timestamp file
	timestampFile := filepath.Join(path, ".cache-timestamp")
	data, err := os.ReadFile(timestampFile)
	if err != nil {
		// No timestamp - assume expired
		return false
	}

	timestamp, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return false
	}

	return time.Since(timestamp) < c.ttl
}

// clone performs git clone
func (c *Cache) clone(url, destPath string) error {
	cmd := exec.Command("git", "clone", "--quiet", url, destPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// update performs git fetch to update cached repo
func (c *Cache) update(repoPath string) error {
	cmd := exec.Command("git", "fetch", "--all", "--tags", "--quiet")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Stats returns cache statistics
func (c *Cache) Stats() (int, int64, error) {
	if !c.enabled {
		return 0, 0, nil
	}

	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	var totalSize int64
	count := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		count++

		// Get size (approximate)
		cachePath := filepath.Join(c.baseDir, entry.Name())
		size, _ := c.dirSize(cachePath)
		totalSize += size
	}

	return count, totalSize, nil
}

// dirSize calculates directory size recursively
func (c *Cache) dirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
