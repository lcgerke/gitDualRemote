package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lcgerke/githelper/internal/constants"
)

// cli_status.go contains status and detection operations: IsRepository, LocalExists,
// IsDetachedHEAD, IsShallowClone, GetStagedFiles, GetUnstagedFiles, GetUntrackedFiles,
// GetConflictFiles, GetOrphanedSubmodules

// IsRepository checks if the current directory is a git repository
func (c *Client) IsRepository() bool {
	_, err := c.run("rev-parse", "--git-dir")
	return err == nil
}

// LocalExists checks if repository exists locally
// Returns true and repository root if in a git repository (from any subdirectory)
func (c *Client) LocalExists() (bool, string) {
	if c.workdir == "" {
		return false, ""
	}

	// Use git to find repository root - works from any subdirectory
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = c.workdir
	output, err := cmd.Output()
	if err == nil {
		// Normal git repository (non-bare)
		repoRoot := strings.TrimSpace(string(output))
		return true, repoRoot
	}

	// Check if workdir itself is a bare repository
	cmd = exec.Command("git", "rev-parse", "--is-bare-repository")
	cmd.Dir = c.workdir
	output, err = cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "true" {
		return true, c.workdir
	}

	return false, ""
}

// IsDetachedHEAD checks for detached HEAD state
func (c *Client) IsDetachedHEAD() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.BranchOperationTimeout)
	defer cancel()

	_, err := c.runWithContext(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		// Exit code 1 = detached HEAD
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// IsShallowClone checks if repository is a shallow clone
func (c *Client) IsShallowClone() (bool, error) {
	shallowPath := filepath.Join(c.workdir, ".git", "shallow")
	_, err := os.Stat(shallowPath)
	return err == nil, nil
}

// GetStagedFiles returns list of staged files
func (c *Client) GetStagedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	output, err := c.runWithContext(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// GetUnstagedFiles returns list of unstaged modifications
func (c *Client) GetUnstagedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	output, err := c.runWithContext(ctx, "diff", "--name-only")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// GetUntrackedFiles returns list of untracked files
func (c *Client) GetUntrackedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	output, err := c.runWithContext(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// GetConflictFiles returns list of files with merge conflicts
func (c *Client) GetConflictFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	output, err := c.runWithContext(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// OrphanedSubmodule represents a submodule that's in the index but not in .gitmodules
type OrphanedSubmodule struct {
	Path string
	Hash string
}

// GetOrphanedSubmodules detects submodules registered in git index but missing from .gitmodules
// This happens when submodules are not properly removed or .gitmodules gets out of sync
func (c *Client) GetOrphanedSubmodules() ([]OrphanedSubmodule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	// Get all gitlinks (submodules) from the index
	// Mode 160000 indicates a gitlink/submodule entry
	output, err := c.runWithContext(ctx, "ls-files", "--stage")
	if err != nil {
		return nil, err
	}

	var orphaned []OrphanedSubmodule
	if output == "" {
		return orphaned, nil
	}

	// Parse ls-files output to find gitlinks (mode 160000)
	lines := strings.Split(output, "\n")
	var gitlinks []OrphanedSubmodule
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: <mode> <hash> <stage>\t<path>
		// Example: 160000 abc123... 0\tpath/to/submodule
		parts := strings.Fields(line)
		if len(parts) >= 4 && parts[0] == "160000" {
			gitlinks = append(gitlinks, OrphanedSubmodule{
				Path: parts[3],
				Hash: parts[1],
			})
		}
	}

	if len(gitlinks) == 0 {
		return orphaned, nil
	}

	// Check which gitlinks are NOT in .gitmodules
	for _, gitlink := range gitlinks {
		// Try to get submodule config
		_, err := c.run("config", "--file", ".gitmodules", "--get", fmt.Sprintf("submodule.%s.path", gitlink.Path))
		if err != nil {
			// If we can't find it in .gitmodules, it's orphaned
			// But we need to distinguish between "not found" and other errors
			// git config exits with 1 if key not found
			orphaned = append(orphaned, gitlink)
		}
	}

	return orphaned, nil
}
