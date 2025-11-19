package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cli_core.go contains basic git operations: Init, Clone, Add, Commit, Push, Fetch, Config

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
