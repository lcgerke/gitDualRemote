package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v58/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client and implements the Platform interface
type Client struct {
	client *github.Client
	owner  string
	repo   string
	ctx    context.Context
}

// ProtectionRules represents branch protection settings
// This is a local copy to avoid import cycles
type ProtectionRules struct {
	Enabled             bool
	RequireReviews      bool
	RequireStatusChecks bool
	EnforceAdmins       bool
	AllowForcePush      bool
}

// NewClient creates a GitHub client from a remote URL
// Supports: https://github.com/owner/repo.git, git@github.com:owner/repo.git
func NewClient(remoteURL string) (*Client, error) {
	owner, repo, err := parseGitHubURL(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	token, err := getGitHubToken()
	if err != nil {
		return nil, fmt.Errorf("GitHub authentication required: %w", err)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		owner:  owner,
		repo:   repo,
		ctx:    ctx,
	}, nil
}

// NewClientWithTimeout creates a client with custom timeout
// Returns client and a cancel function that must be called when done
func NewClientWithTimeout(remoteURL string, timeout time.Duration) (*Client, context.CancelFunc, error) {
	client, err := NewClient(remoteURL)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	client.ctx = ctx

	return client, cancel, nil
}

// parseGitHubURL extracts owner and repo from various GitHub URL formats
func parseGitHubURL(remoteURL string) (owner, repo string, err error) {
	// Handle SSH URLs: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		parts := strings.TrimPrefix(remoteURL, "git@github.com:")
		parts = strings.TrimSuffix(parts, ".git")

		split := strings.Split(parts, "/")
		if len(split) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL format")
		}
		return split[0], split[1], nil
	}

	// Handle HTTPS URLs: https://github.com/owner/repo.git
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", err
	}

	if u.Host != "github.com" {
		return "", "", fmt.Errorf("not a GitHub URL: %s", u.Host)
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GitHub path: %s", path)
	}

	return parts[0], parts[1], nil
}

// GetOwner returns the repository owner
func (c *Client) GetOwner() string {
	return c.owner
}

// GetRepo returns the repository name
func (c *Client) GetRepo() string {
	return c.repo
}

// GetPlatform returns "github"
func (c *Client) GetPlatform() string {
	return "github"
}

// SetDefaultBranch updates the repository's default branch
func (c *Client) SetDefaultBranch(branch string) error {
	_, _, err := c.client.Repositories.Edit(c.ctx, c.owner, c.repo, &github.Repository{
		DefaultBranch: github.String(branch),
	})

	if err != nil {
		return fmt.Errorf("failed to set default branch: %w", err)
	}

	return nil
}

// GetDefaultBranch returns the current default branch
func (c *Client) GetDefaultBranch() (string, error) {
	repo, _, err := c.client.Repositories.Get(c.ctx, c.owner, c.repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}

	return repo.GetDefaultBranch(), nil
}

// IsBranchProtected checks if a branch has protection rules
func (c *Client) IsBranchProtected(branch string) (bool, error) {
	_, resp, err := c.client.Repositories.GetBranchProtection(c.ctx, c.owner, c.repo, branch)

	if err != nil {
		// 404 means no protection
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check branch protection: %w", err)
	}

	return true, nil
}

// GetBranchProtection returns detailed protection rules
func (c *Client) GetBranchProtection(branch string) (*ProtectionRules, error) {
	protection, resp, err := c.client.Repositories.GetBranchProtection(c.ctx, c.owner, c.repo, branch)

	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return &ProtectionRules{Enabled: false}, nil
		}
		return nil, fmt.Errorf("failed to get branch protection: %w", err)
	}

	return &ProtectionRules{
		Enabled:             true,
		RequireReviews:      protection.GetRequiredPullRequestReviews() != nil,
		RequireStatusChecks: protection.GetRequiredStatusChecks() != nil,
		EnforceAdmins:       protection.GetEnforceAdmins().Enabled,
		AllowForcePush:      protection.GetAllowForcePushes().Enabled,
	}, nil
}

// CanPush checks if authenticated user can push to repository
func (c *Client) CanPush() (bool, error) {
	perms, err := c.CheckPermissions()
	if err != nil {
		return false, err
	}
	return perms.Push, nil
}

// CanAdmin checks if authenticated user has admin access
func (c *Client) CanAdmin() (bool, error) {
	perms, err := c.CheckPermissions()
	if err != nil {
		return false, err
	}
	return perms.Admin, nil
}
