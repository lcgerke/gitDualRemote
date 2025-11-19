package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Client wraps git CLI operations with thread safety.
// All git operations are implemented as methods on this client and organized
// into separate files by responsibility:
// - cli_core.go: Basic git operations (Init, Clone, Add, Commit, Push, Fetch, Config)
// - cli_remote.go: Remote operations (AddRemote, RemoveRemote, SetURL, etc.)
// - cli_branch.go: Branch operations (GetCurrentBranch, ListBranches, etc.)
// - cli_status.go: Status/detection operations (IsRepository, GetStagedFiles, etc.)
// - cli_advanced.go: Advanced operations (CountCommitsBetween, ScanLargeBinaries, etc.)
type Client struct {
	workdir string
	mu      sync.Mutex // Serialize all git operations to prevent races
}

// NewClient creates a new git CLI client
func NewClient(workdir string) *Client {
	return &Client{
		workdir: workdir,
	}
}

// run executes a git command with context, mutex, and safe environment
func (c *Client) run(args ...string) (string, error) {
	return c.runWithContext(context.Background(), args...)
}

// runWithContext executes a git command with explicit context
func (c *Client) runWithContext(ctx context.Context, args ...string) (string, error) {
	// CRITICAL: Serialize all git operations to prevent races
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd := exec.CommandContext(ctx, "git", args...)
	if c.workdir != "" {
		cmd.Dir = c.workdir
	}

	// Force non-interactive mode and stable locale
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // Prevent credential hangs
		"LC_ALL=C",              // Stable output parsing
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
