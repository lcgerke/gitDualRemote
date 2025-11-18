package github

import (
	"context"
	"fmt"
	"os/exec"

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

// CheckGHCLIAvailable checks if the gh CLI is installed and available
func CheckGHCLIAvailable() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

// CheckGHAuthenticated checks if gh CLI is authenticated
func CheckGHAuthenticated() bool {
	if !CheckGHCLIAvailable() {
		return false
	}

	cmd := exec.Command("gh", "auth", "status")
	err := cmd.Run()
	return err == nil
}

// CreateRepositoryViaGH creates a GitHub repository using the gh CLI
// This is an alternative to CreateRepository that doesn't require a PAT
func (c *Client) CreateRepositoryViaGH(name, description string, private bool) error {
	if name == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	if !CheckGHCLIAvailable() {
		return fmt.Errorf("gh CLI is not installed")
	}

	if !CheckGHAuthenticated() {
		return fmt.Errorf("gh CLI is not authenticated - run 'gh auth login'")
	}

	// Build the gh repo create command
	args := []string{"repo", "create", name}

	// Add description if provided
	if description != "" {
		args = append(args, "--description", description)
	}

	// Add visibility flag
	if private {
		args = append(args, "--private")
	} else {
		args = append(args, "--public")
	}

	// Execute the command
	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create repository via gh: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// CreateRepositoryViaGHWithCheck creates a repository via gh CLI with existence check
func (c *Client) CreateRepositoryViaGHWithCheck(owner, name, description string, private bool) error {
	// First check if repository already exists
	exists, err := c.RepositoryExists(owner, name)
	if err != nil {
		// If we can't check (e.g., no PAT), try to create anyway
		// gh CLI will fail gracefully if it already exists
		return c.CreateRepositoryViaGH(name, description, private)
	}

	if exists {
		return fmt.Errorf("repository %s/%s already exists", owner, name)
	}

	return c.CreateRepositoryViaGH(name, description, private)
}
