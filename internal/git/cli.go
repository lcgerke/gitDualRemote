package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps git CLI operations
type Client struct {
	workdir string
}

// NewClient creates a new git CLI client
func NewClient(workdir string) *Client {
	return &Client{
		workdir: workdir,
	}
}

// run executes a git command
func (c *Client) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if c.workdir != "" {
		cmd.Dir = c.workdir
	}

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
