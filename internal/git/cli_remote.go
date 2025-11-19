package git

import (
	"fmt"
	"strings"
)

// cli_remote.go contains remote operations: AddRemote, RemoveRemote, SetURL, AddPushURL,
// ConfigureDualPush, GetRemoteURL, GetPushURLs, ListRemotes

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
