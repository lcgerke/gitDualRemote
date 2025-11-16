package git

import (
	"fmt"
)

// SetupDualPush configures a remote to push to multiple URLs
// This is the core dual-push feature: one `git push` → pushes to both remotes
func (c *Client) SetupDualPush(remoteName, fetchURL, bareURL, githubURL string) error {
	// Check if remote exists
	remotes, err := c.ListRemotes()
	if err != nil {
		return fmt.Errorf("failed to list remotes: %w", err)
	}

	remoteExists := false
	for _, r := range remotes {
		if r == remoteName {
			remoteExists = true
			break
		}
	}

	// If remote doesn't exist, create it
	if !remoteExists {
		if err := c.AddRemote(remoteName, fetchURL); err != nil {
			return fmt.Errorf("failed to add remote: %w", err)
		}
	} else {
		// Update fetch URL
		if err := c.SetURL(remoteName, fetchURL); err != nil {
			return fmt.Errorf("failed to set fetch URL: %w", err)
		}
	}

	// Clear existing push URLs by setting to the first URL
	// This is necessary because we need to control all push URLs
	_, err = c.run("remote", "set-url", "--push", remoteName, bareURL)
	if err != nil {
		return fmt.Errorf("failed to set initial push URL: %w", err)
	}

	// Add GitHub as second push URL
	if err := c.AddPushURL(remoteName, githubURL); err != nil {
		return fmt.Errorf("failed to add GitHub push URL: %w", err)
	}

	return nil
}

// GetPushURLs returns all push URLs for a remote
func (c *Client) GetPushURLs(remoteName string) ([]string, error) {
	output, err := c.run("remote", "get-url", "--push", "--all", remoteName)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return splitLines(output), nil
}

// VerifyDualPush checks that dual-push is configured correctly
func (c *Client) VerifyDualPush(remoteName, bareURL, githubURL string) (bool, error) {
	pushURLs, err := c.GetPushURLs(remoteName)
	if err != nil {
		return false, err
	}

	if len(pushURLs) != 2 {
		return false, nil
	}

	// Check that both URLs are present (order might vary)
	hasBare := false
	hasGitHub := false

	for _, url := range pushURLs {
		if url == bareURL {
			hasBare = true
		}
		if url == githubURL {
			hasGitHub = true
		}
	}

	return hasBare && hasGitHub, nil
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}

	var lines []string
	current := ""
	for _, r := range s {
		if r == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
