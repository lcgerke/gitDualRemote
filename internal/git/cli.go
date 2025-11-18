package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client wraps git CLI operations with thread safety
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

// Init initializes a git repository
func (c *Client) Init(bare bool) error {
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}

	_, err := c.run(args...)
	return err
}

// Clone clones a repository
func Clone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}

// AddRemote adds a remote
func (c *Client) AddRemote(name, url string) error {
	_, err := c.run("remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote
func (c *Client) RemoveRemote(name string) error {
	_, err := c.run("remote", "remove", name)
	return err
}

// SetURL sets the fetch URL for a remote
func (c *Client) SetURL(remote, url string) error {
	_, err := c.run("remote", "set-url", remote, url)
	return err
}

// AddPushURL adds a push URL to a remote
func (c *Client) AddPushURL(remote, url string) error {
	_, err := c.run("remote", "set-url", "--add", "--push", remote, url)
	return err
}

// ConfigureDualPush sets up dual-push for a remote
// This configures the remote to push to multiple URLs
func (c *Client) ConfigureDualPush(remote, bareURL, githubURL string) error {
	// First, set the fetch URL (usually the bare repo)
	if err := c.SetURL(remote, bareURL); err != nil {
		return fmt.Errorf("failed to set fetch URL: %w", err)
	}

	// Add both push URLs
	if err := c.AddPushURL(remote, bareURL); err != nil {
		return fmt.Errorf("failed to add bare push URL: %w", err)
	}

	if err := c.AddPushURL(remote, githubURL); err != nil {
		return fmt.Errorf("failed to add GitHub push URL: %w", err)
	}

	return nil
}

// ConfigSet sets a git config value
func (c *Client) ConfigSet(key, value string) error {
	_, err := c.run("config", key, value)
	return err
}

// ConfigGet gets a git config value
func (c *Client) ConfigGet(key string) (string, error) {
	return c.run("config", "--get", key)
}

// SetSSHCommand sets the SSH command for git operations
func (c *Client) SetSSHCommand(keyPath string) error {
	sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", keyPath)
	return c.ConfigSet("core.sshCommand", sshCmd)
}

// Add stages files
func (c *Client) Add(files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := c.run(args...)
	return err
}

// Commit creates a commit
func (c *Client) Commit(message string) error {
	_, err := c.run("commit", "-m", message)
	return err
}

// Push pushes to a remote
func (c *Client) Push(remote, refspec string) error {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
	}
	if refspec != "" {
		args = append(args, refspec)
	}

	_, err := c.run(args...)
	return err
}

// PushSetUpstream pushes and sets upstream
func (c *Client) PushSetUpstream(remote, branch string) error {
	_, err := c.run("push", "-u", remote, branch)
	return err
}

// Fetch fetches from a remote
func (c *Client) Fetch(remote string) error {
	_, err := c.run("fetch", remote)
	return err
}

// GetCurrentBranch returns the current branch name
func (c *Client) GetCurrentBranch() (string, error) {
	return c.run("rev-parse", "--abbrev-ref", "HEAD")
}

// GetRemoteURL gets the URL for a remote
func (c *Client) GetRemoteURL(remote string) (string, error) {
	return c.run("remote", "get-url", remote)
}

// GetPushURLs gets all push URLs for a remote
func (c *Client) GetPushURLs(remote string) ([]string, error) {
	output, err := c.run("remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// ListRemotes lists all remotes
func (c *Client) ListRemotes() ([]string, error) {
	output, err := c.run("remote")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// GetRevList gets commits between two refs
func (c *Client) GetRevList(ref1, ref2 string) ([]string, error) {
	rangeSpec := fmt.Sprintf("%s..%s", ref1, ref2)
	output, err := c.run("rev-list", rangeSpec)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// IsRepository checks if the current directory is a git repository
func (c *Client) IsRepository() bool {
	_, err := c.run("rev-parse", "--git-dir")
	return err == nil
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

// InitBareRepo creates a bare repository at the specified path
func InitBareRepo(path string) error {
	// Check if parent directory exists
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory does not exist: %s", parentDir)
	}

	// Create the directory for the bare repo
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create bare repo directory: %w", err)
	}

	// Initialize as bare
	client := NewClient(path)
	return client.Init(true)
}

// ============================================================================
// PHASE 1.2: New methods for scenario classifier
// ============================================================================

// IsAncestor checks if commit1 is an ancestor of commit2 (for fast-forward validation)
func (c *Client) IsAncestor(commit1, commit2 string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	_, err = c.runWithContext(ctx, "rev-parse", "--verify", "refs/heads/main")
	if err == nil {
		return "main", nil
	}

	// Try 4: Check for master
	_, err = c.runWithContext(ctx, "rev-parse", "--verify", "refs/heads/master")
	if err == nil {
		return "master", nil
	}

	return "", fmt.Errorf("could not determine default branch for remote %s", remote)
}

// FetchRemote fetches updates from a remote with timeout
func (c *Client) FetchRemote(remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := c.runWithContext(ctx, "fetch", remote, "--tags")
	return err
}

// ResetToRef performs hard reset to ref (used in auto-fix)
func (c *Client) ResetToRef(ref string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := c.runWithContext(ctx, "reset", "--hard", ref)
	return err
}

// GetBranchHash returns commit hash for a local branch
func (c *Client) GetBranchHash(branch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output, err := c.runWithContext(ctx, "rev-parse", branch)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetRemoteBranchHash returns commit hash for a remote branch
func (c *Client) GetRemoteBranchHash(remote, branch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ref := fmt.Sprintf("%s/%s", remote, branch)
	output, err := c.runWithContext(ctx, "rev-parse", ref)
	if err != nil {
		// Branch doesn't exist on remote, not necessarily an error
		return "", nil
	}
	return output, nil
}

// CountCommitsBetween counts commits in ref1 not in ref2
func (c *Client) CountCommitsBetween(ref1, ref2 string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.runWithContext(ctx, "ls-remote", "--exit-code", remote, "HEAD")
	return err == nil
}

// LocalExists checks if repository exists locally
func (c *Client) LocalExists() (bool, string) {
	if c.workdir == "" {
		return false, ""
	}

	gitDir := filepath.Join(c.workdir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true, c.workdir
	}

	// Check if workdir itself is a git directory (bare repo)
	if _, err := os.Stat(filepath.Join(c.workdir, "HEAD")); err == nil {
		if _, err := os.Stat(filepath.Join(c.workdir, "refs")); err == nil {
			return true, c.workdir
		}
	}

	return false, ""
}

// IsDetachedHEAD checks for detached HEAD state
func (c *Client) IsDetachedHEAD() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// ListBranches returns all local and remote branches
func (c *Client) ListBranches() (local, remote []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// CheckLFSEnabled detects if Git LFS is installed and in use
func (c *Client) CheckLFSEnabled() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
