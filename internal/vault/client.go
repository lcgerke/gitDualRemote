package vault

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// Client wraps the Vault API client
type Client struct {
	client *vault.Client
	ctx    context.Context
}

// NewClient creates a new Vault client
// It uses environment variables for configuration:
// - VAULT_ADDR: Vault server address
// - VAULT_TOKEN: Authentication token
func NewClient(ctx context.Context) (*Client, error) {
	config := vault.DefaultConfig()
	if config == nil {
		return nil, fmt.Errorf("failed to create default vault config")
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// GetSecret retrieves a secret from Vault
func (c *Client) GetSecret(path string) (map[string]interface{}, error) {
	secret, err := c.client.KVv2("secret").Get(c.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret at %s: %w", path, err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no data found at %s", path)
	}

	return secret.Data, nil
}

// PutSecret stores a secret in Vault
func (c *Client) PutSecret(path string, data map[string]interface{}) error {
	_, err := c.client.KVv2("secret").Put(c.ctx, path, data)
	if err != nil {
		return fmt.Errorf("failed to write secret at %s: %w", path, err)
	}

	return nil
}

// IsReachable checks if Vault server is reachable
func (c *Client) IsReachable() bool {
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()

	_, err := c.client.Sys().HealthWithContext(ctx)
	return err == nil
}

// GetConfig retrieves githelper configuration from Vault
func (c *Client) GetConfig() (*Config, error) {
	data, err := c.GetSecret("githelper/config")
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	if v, ok := data["github_username"].(string); ok {
		cfg.GitHubUsername = v
	}
	if v, ok := data["bare_repo_pattern"].(string); ok {
		cfg.BareRepoPattern = v
	}
	if v, ok := data["default_visibility"].(string); ok {
		cfg.DefaultVisibility = v
	}
	if v, ok := data["auto_create_github"].(bool); ok {
		cfg.AutoCreateGitHub = v
	}
	if v, ok := data["test_before_push"].(bool); ok {
		cfg.TestBeforePush = v
	}
	if v, ok := data["sync_on_setup"].(bool); ok {
		cfg.SyncOnSetup = v
	}
	if v, ok := data["retry_on_partial_failure"].(bool); ok {
		cfg.RetryOnPartialFailure = v
	}

	return cfg, nil
}

// GetSSHKey retrieves SSH key from Vault
// Tries repo-specific path first, falls back to default
func (c *Client) GetSSHKey(repoName string) (*SSHKey, error) {
	var data map[string]interface{}
	var err error

	// Try repo-specific key first
	if repoName != "" {
		path := fmt.Sprintf("githelper/github/%s/ssh", repoName)
		data, err = c.GetSecret(path)
		if err == nil {
			return parseSSHKey(data)
		}
	}

	// Fall back to default key
	data, err = c.GetSecret("githelper/github/default_ssh")
	if err != nil {
		return nil, fmt.Errorf("no SSH key found (tried repo-specific and default): %w", err)
	}

	return parseSSHKey(data)
}

// GetPAT retrieves GitHub Personal Access Token from Vault
// Tries repo-specific path first, falls back to default
func (c *Client) GetPAT(repoName string) (string, error) {
	var data map[string]interface{}
	var err error

	// Try repo-specific PAT first
	if repoName != "" {
		path := fmt.Sprintf("githelper/github/%s/pat", repoName)
		data, err = c.GetSecret(path)
		if err == nil {
			if token, ok := data["token"].(string); ok {
				return token, nil
			}
		}
	}

	// Fall back to default PAT
	data, err = c.GetSecret("githelper/github/default_pat")
	if err != nil {
		return "", fmt.Errorf("no PAT found (tried repo-specific and default): %w", err)
	}

	if token, ok := data["token"].(string); ok {
		return token, nil
	}

	return "", fmt.Errorf("PAT data missing 'token' field")
}

func parseSSHKey(data map[string]interface{}) (*SSHKey, error) {
	key := &SSHKey{}

	privateKey, ok := data["private_key"].(string)
	if !ok {
		return nil, fmt.Errorf("SSH key data missing 'private_key' field")
	}
	key.PrivateKey = privateKey

	if publicKey, ok := data["public_key"].(string); ok {
		key.PublicKey = publicKey
	}

	return key, nil
}
