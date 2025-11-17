package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v56/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	ctx    context.Context
	token  string
}

// NewClient creates a new GitHub client with PAT authentication
func NewClient(ctx context.Context, token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		ctx:    ctx,
		token:  token,
	}
}

// CreateRepository creates a new GitHub repository
func (c *Client) CreateRepository(name, description string, private bool) (*github.Repository, error) {
	repo := &github.Repository{
		Name:        github.String(name),
		Description: github.String(description),
		Private:     github.Bool(private),
		AutoInit:    github.Bool(false), // We'll push our own initial commit
	}

	createdRepo, _, err := c.client.Repositories.Create(c.ctx, "", repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return createdRepo, nil
}

// GetRepository retrieves repository information
func (c *Client) GetRepository(owner, repo string) (*github.Repository, error) {
	repository, _, err := c.client.Repositories.Get(c.ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repository, nil
}

// RepositoryExists checks if a repository exists
func (c *Client) RepositoryExists(owner, repo string) (bool, error) {
	_, resp, err := c.client.Repositories.Get(c.ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check repository existence: %w", err)
	}

	return true, nil
}

// GetAuthenticatedUser returns the currently authenticated user
func (c *Client) GetAuthenticatedUser() (*github.User, error) {
	user, _, err := c.client.Users.Get(c.ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated user: %w", err)
	}

	return user, nil
}

// TestConnection tests the GitHub API connection
func (c *Client) TestConnection() error {
	_, err := c.GetAuthenticatedUser()
	if err != nil {
		return fmt.Errorf("GitHub API connection test failed: %w", err)
	}
	return nil
}

// GetSSHURL returns the SSH clone URL for a repository
func (c *Client) GetSSHURL(owner, repo string) (string, error) {
	repository, err := c.GetRepository(owner, repo)
	if err != nil {
		return "", err
	}

	if repository.SSHURL == nil {
		return "", fmt.Errorf("repository has no SSH URL")
	}

	return *repository.SSHURL, nil
}
