package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lcgerke/githelper/internal/constants"
)

// cli_branch.go contains branch operations: GetCurrentBranch, GetBranchHash,
// GetRemoteBranchHash, ListBranches, IsAncestor, GetDefaultBranch

// GetCurrentBranch returns the current branch name
func (c *Client) GetCurrentBranch() (string, error) {
	return c.run("rev-parse", "--abbrev-ref", "HEAD")
}

// GetBranchHash returns commit hash for a local branch
func (c *Client) GetBranchHash(branch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.BranchOperationTimeout)
	defer cancel()

	output, err := c.runWithContext(ctx, "rev-parse", branch)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetRemoteBranchHash returns commit hash for a remote branch
func (c *Client) GetRemoteBranchHash(remote, branch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.BranchOperationTimeout)
	defer cancel()

	ref := fmt.Sprintf("%s/%s", remote, branch)
	output, err := c.runWithContext(ctx, "rev-parse", ref)
	if err != nil {
		// Branch doesn't exist on remote, not necessarily an error
		return "", nil
	}
	return output, nil
}

// ListBranches returns all local and remote branches
func (c *Client) ListBranches() (local, remote []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultOperationTimeout)
	defer cancel()

	// Local branches
	localOut, err := c.runWithContext(ctx, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, nil, err
	}
	if localOut != "" {
		local = strings.Split(localOut, "\n")
	}

	// Remote branches
	remoteOut, err := c.runWithContext(ctx, "branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return nil, nil, err
	}
	if remoteOut != "" {
		remote = strings.Split(remoteOut, "\n")
	}

	return local, remote, nil
}

// IsAncestor checks if commit1 is an ancestor of commit2 (for fast-forward validation)
func (c *Client) IsAncestor(commit1, commit2 string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.QuickOperationTimeout)
	defer cancel()

	// git merge-base --is-ancestor returns exit code 0 if ancestor, 1 if not
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", commit1, commit2)
	if c.workdir != "" {
		cmd.Dir = c.workdir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")

	err := cmd.Run()
	if err == nil {
		return true, nil // Is ancestor
	}

	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil // Not ancestor (not an error)
	}

	return false, fmt.Errorf("merge-base failed: %w", err)
}

// GetDefaultBranch determines the default branch for a remote (local cache first)
func (c *Client) GetDefaultBranch(remote string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultOperationTimeout)
	defer cancel()

	// Try 1: git symbolic-ref refs/remotes/<remote>/HEAD (fast, local cache)
	output, err := c.runWithContext(ctx, "symbolic-ref", fmt.Sprintf("refs/remotes/%s/HEAD", remote))
	if err == nil && output != "" {
		// Output: "refs/remotes/origin/main"
		parts := strings.Split(output, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Try 2: git remote show <remote> | grep "HEAD branch" (network fallback)
	output, err = c.runWithContext(ctx, "remote", "show", remote)
	if err == nil {
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "HEAD branch:") {
				branch := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
				return branch, nil
			}
		}
	}

	// Try 3: Check for main
	_, err = c.runWithContext(ctx, "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", constants.DefaultBranch))
	if err == nil {
		return constants.DefaultBranch, nil
	}

	// Try 4: Check for master
	_, err = c.runWithContext(ctx, "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", constants.MasterBranch))
	if err == nil {
		return constants.MasterBranch, nil
	}

	return "", fmt.Errorf("could not determine default branch for remote %s", remote)
}
