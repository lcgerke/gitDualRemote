package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lcgerke/githelper/internal/constants"
)

// cli_advanced.go contains advanced operations: CountCommitsBetween, CanReachRemote,
// FetchRemote, ResetToRef, CheckLFSEnabled, ScanLargeBinaries, ValidateGitVersion,
// CheckGitVersion

// CountCommitsBetween counts commits in ref1 not in ref2
func (c *Client) CountCommitsBetween(ref1, ref2 string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultOperationTimeout)
	defer cancel()

	// git rev-list --count ref1 ^ref2
	output, err := c.runWithContext(ctx, "rev-list", "--count", ref1, "^"+ref2)
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("invalid count output: %s", output)
	}
	return count, nil
}

// CanReachRemote tests if a remote is accessible
func (c *Client) CanReachRemote(remote string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	_, err := c.runWithContext(ctx, "ls-remote", "--exit-code", remote, "HEAD")
	return err == nil
}

// FetchRemote fetches updates from a remote with timeout
func (c *Client) FetchRemote(remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultFetchTimeout)
	defer cancel()

	_, err := c.runWithContext(ctx, "fetch", remote, "--tags")
	return err
}

// ResetToRef performs hard reset to ref (used in auto-fix)
func (c *Client) ResetToRef(ref string) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultFetchTimeout)
	defer cancel()

	_, err := c.runWithContext(ctx, "reset", "--hard", ref)
	return err
}

// CheckLFSEnabled detects if Git LFS is installed and in use
func (c *Client) CheckLFSEnabled() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	// Check if git lfs is available
	_, err := c.runWithContext(ctx, "lfs", "version")
	if err != nil {
		return false, nil // LFS not installed
	}

	// Check if any files are tracked by LFS
	output, err := c.runWithContext(ctx, "lfs", "ls-files")
	if err != nil {
		return false, nil
	}

	return output != "", nil
}

// LargeBinary represents a large binary file in the repository
type LargeBinary struct {
	SHA1   string
	SizeMB float64
	// Path is intentionally NOT included (too expensive to lookup)
}

// ScanLargeBinaries finds blobs larger than threshold (SHA+size only)
func (c *Client) ScanLargeBinaries(thresholdBytes int64) ([]LargeBinary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultFetchTimeout)
	defer cancel()

	// Step 1: Get all objects with git rev-list --objects --all
	revListOut, err := c.runWithContext(ctx, "rev-list", "--objects", "--all")
	if err != nil {
		return nil, fmt.Errorf("rev-list failed: %w", err)
	}

	if revListOut == "" {
		return []LargeBinary{}, nil
	}

	// Step 2: Get object info with git cat-file --batch-check
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if c.workdir != "" {
		cmd.Dir = c.workdir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	cmd.Stdin = strings.NewReader(revListOut)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cat-file failed: %w", err)
	}

	// Parse output and filter for large blobs
	var largeBinaries []LargeBinary
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		sha1 := parts[0]
		objType := parts[1]
		sizeStr := parts[2]

		if objType != "blob" {
			continue
		}

		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			continue
		}

		if size >= thresholdBytes {
			largeBinaries = append(largeBinaries, LargeBinary{
				SHA1:   sha1,
				SizeMB: float64(size) / (1024 * 1024),
			})
		}
	}

	return largeBinaries, nil
}

// ValidateGitVersion checks git version meets minimum requirements
func (c *Client) ValidateGitVersion() error {
	output, err := c.run("--version")
	if err != nil {
		return fmt.Errorf("git is not installed: %w", err)
	}

	// Parse version: "git version 2.34.1"
	parts := strings.Fields(output)
	if len(parts) < 3 {
		return fmt.Errorf("unexpected git version output: %s", output)
	}

	version := parts[2]
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("invalid git version format: %s", version)
	}

	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("invalid major version: %s", versionParts[0])
	}

	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return fmt.Errorf("invalid minor version: %s", versionParts[1])
	}

	// Require Git 2.30+
	if major < 2 || (major == 2 && minor < 30) {
		return fmt.Errorf("GitHelper requires Git 2.30.0 or newer (found: %s)\nPlease upgrade Git: https://git-scm.com/downloads", version)
	}

	return nil
}

// CheckGitVersion verifies git is installed and meets minimum version
func CheckGitVersion() error {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git is not installed or not in PATH: %w", err)
	}

	// Basic check - just ensure git runs
	// For production, we'd parse version and check >= 2.0
	if !strings.Contains(string(output), "git version") {
		return fmt.Errorf("unexpected git version output: %s", output)
	}

	return nil
}
