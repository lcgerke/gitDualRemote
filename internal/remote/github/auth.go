package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TokenSource represents where the token was found
type TokenSource string

const (
	SourceEnvVar    TokenSource = "GITHUB_TOKEN"
	SourceGhConfig  TokenSource = "~/.config/gh/hosts.yml"
	SourceGitConfig TokenSource = "git config github.token"
)

// TokenInfo contains token and its source
type TokenInfo struct {
	Token  string
	Source TokenSource
}

// getGitHubToken attempts to find GitHub token from multiple sources
// Priority: GITHUB_TOKEN env var > GH_TOKEN env var > gh CLI config > git config
func getGitHubToken() (string, error) {
	info, err := getGitHubTokenInfo()
	if err != nil {
		return "", err
	}
	return info.Token, nil
}

// getGitHubTokenInfo returns token with source information (useful for diagnostics)
func getGitHubTokenInfo() (*TokenInfo, error) {
	// 1. Try GITHUB_TOKEN environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return &TokenInfo{Token: token, Source: SourceEnvVar}, nil
	}

	// 2. Try GH_TOKEN (alternative env var)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return &TokenInfo{Token: token, Source: SourceEnvVar}, nil
	}

	// 3. Try gh CLI config (~/.config/gh/hosts.yml)
	if token, err := readGhConfigToken(); err == nil && token != "" {
		return &TokenInfo{Token: token, Source: SourceGhConfig}, nil
	}

	// 4. Try git config (github.token)
	if token, err := readGitConfigToken(); err == nil && token != "" {
		return &TokenInfo{Token: token, Source: SourceGitConfig}, nil
	}

	return nil, fmt.Errorf("no GitHub token found\n\n" +
		"Please authenticate using one of:\n" +
		"  1. Set GITHUB_TOKEN environment variable\n" +
		"  2. Run: gh auth login\n" +
		"  3. Run: git config --global github.token YOUR_TOKEN")
}

// readGhConfigToken reads token from gh CLI config
func readGhConfigToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(home, ".config", "gh", "hosts.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config map[string]map[string]string
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", err
	}

	if ghConfig, ok := config["github.com"]; ok {
		if token, ok := ghConfig["oauth_token"]; ok {
			return token, nil
		}
	}

	return "", fmt.Errorf("no token in gh config")
}

// readGitConfigToken reads token from git config
func readGitConfigToken() (string, error) {
	cmd := exec.Command("git", "config", "--global", "github.token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("git config github.token is empty")
	}

	return token, nil
}

// ValidateToken checks if token has required scopes
func (c *Client) ValidateToken() error {
	// Try a simple API call to verify token works
	_, _, err := c.client.Users.Get(c.ctx, "")
	if err != nil {
		return fmt.Errorf("token validation failed: %w\n\n"+
			"Your token may be expired or lack required permissions.\n"+
			"Required scopes: repo (full control of private repositories)", err)
	}

	return nil
}

// CheckPermissions verifies required repository permissions
func (c *Client) CheckPermissions() (*RepositoryPermissions, error) {
	repo, _, err := c.client.Repositories.Get(c.ctx, c.owner, c.repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	perms := repo.GetPermissions()
	return &RepositoryPermissions{
		Pull:  perms["pull"],
		Push:  perms["push"],
		Admin: perms["admin"],
	}, nil
}

// RepositoryPermissions represents user's permissions on a repository
type RepositoryPermissions struct {
	Pull  bool
	Push  bool
	Admin bool
}
