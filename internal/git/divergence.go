package git

import (
	"fmt"
	"strconv"
	"strings"
)

// DivergenceStatus represents the sync status between remotes
type DivergenceStatus struct {
	// InSync indicates if both remotes have the same commits
	InSync bool
	// BareAhead is the number of commits bare repo has that GitHub doesn't
	BareAhead int
	// GitHubAhead is the number of commits GitHub has that bare repo doesn't
	GitHubAhead int
	// BareRef is the current commit on the bare remote
	BareRef string
	// GitHubRef is the current commit on the GitHub remote
	GitHubRef string
}

// CheckDivergence checks if the GitHub remote is behind the bare remote
// This uses git's native commit graph comparison
func (c *Client) CheckDivergence(bareRemote, githubRemote, branch string) (*DivergenceStatus, error) {
	bareRef := fmt.Sprintf("%s/%s", bareRemote, branch)
	githubRef := fmt.Sprintf("%s/%s", githubRemote, branch)

	// Fetch from both remotes
	if err := c.Fetch(bareRemote); err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", bareRemote, err)
	}

	if err := c.Fetch(githubRemote); err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", githubRemote, err)
	}

	// Get commit SHAs
	bareCommit, err := c.GetCommit(bareRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit for %s: %w", bareRef, err)
	}

	githubCommit, err := c.GetCommit(githubRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit for %s: %w", githubRef, err)
	}

	// If commits are the same, we're in sync
	if bareCommit == githubCommit {
		return &DivergenceStatus{
			InSync:      true,
			BareAhead:   0,
			GitHubAhead: 0,
			BareRef:     bareCommit,
			GitHubRef:   githubCommit,
		}, nil
	}

	// Count commits that bare has but GitHub doesn't
	bareAhead, err := c.countCommits(githubRef, bareRef)
	if err != nil {
		return nil, fmt.Errorf("failed to count commits ahead on bare: %w", err)
	}

	// Count commits that GitHub has but bare doesn't
	githubAhead, err := c.countCommits(bareRef, githubRef)
	if err != nil {
		return nil, fmt.Errorf("failed to count commits ahead on GitHub: %w", err)
	}

	return &DivergenceStatus{
		InSync:      false,
		BareAhead:   bareAhead,
		GitHubAhead: githubAhead,
		BareRef:     bareCommit,
		GitHubRef:   githubCommit,
	}, nil
}

// countCommits counts commits in 'newer' that aren't in 'older'
// This uses git rev-list to compare commit graphs
func (c *Client) countCommits(older, newer string) (int, error) {
	// git rev-list older..newer --count
	// Returns number of commits in 'newer' that aren't in 'older'
	output, err := c.run("rev-list", "--count", fmt.Sprintf("%s..%s", older, newer))
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetCommit returns the commit SHA for a ref
func (c *Client) GetCommit(ref string) (string, error) {
	output, err := c.run("rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// SyncToGitHub pushes missing commits from bare remote to GitHub remote
func (c *Client) SyncToGitHub(bareRemote, githubRemote, branch string) error {
	// First check divergence
	status, err := c.CheckDivergence(bareRemote, githubRemote, branch)
	if err != nil {
		return fmt.Errorf("failed to check divergence: %w", err)
	}

	if status.InSync {
		return nil // Already in sync
	}

	if status.GitHubAhead > 0 {
		return fmt.Errorf("GitHub has commits that bare doesn't - cannot sync (manual resolution required)")
	}

	// Push from bare remote to GitHub
	// We push from local tracking branch which should match bare
	bareRef := fmt.Sprintf("%s/%s", bareRemote, branch)

	// Push the bare remote's branch to GitHub
	_, err = c.run("push", githubRemote, fmt.Sprintf("%s:%s", bareRef, branch))
	if err != nil {
		return fmt.Errorf("failed to push to GitHub: %w", err)
	}

	return nil
}
