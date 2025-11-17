package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lcgerke/githelper/internal/vault"
)

const (
	defaultCacheTTL = 24 * time.Hour
	configCacheFile = "config.json"
)

// Manager handles configuration from Vault with local caching
type Manager struct {
	vaultClient *vault.Client
	cacheDir    string
	cacheTTL    time.Duration
}

// CachedConfig represents cached configuration with timestamp
type CachedConfig struct {
	Config    *vault.Config `json:"config"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// NewManager creates a new config manager
func NewManager(ctx context.Context, cacheDir string) (*Manager, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".githelper", "cache")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	vaultClient, err := vault.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	return &Manager{
		vaultClient: vaultClient,
		cacheDir:    cacheDir,
		cacheTTL:    defaultCacheTTL,
	}, nil
}

// GetConfig retrieves config from Vault or cache
func (m *Manager) GetConfig() (*vault.Config, bool, error) {
	// Try to get from Vault first
	if m.vaultClient.IsReachable() {
		cfg, err := m.vaultClient.GetConfig()
		if err == nil {
			// Cache it for later
			_ = m.cacheConfig(cfg)
			return cfg, false, nil // false = not from cache
		}
		// Vault error - fall through to cache
	}

	// Try cache
	cached, err := m.loadCache()
	if err != nil {
		return nil, false, fmt.Errorf("vault unreachable and no valid cache: %w", err)
	}

	// Check if cache is stale
	age := time.Since(cached.FetchedAt)
	if age > m.cacheTTL {
		return cached.Config, true, fmt.Errorf("cache is stale (%s old, TTL is %s)", age, m.cacheTTL)
	}

	return cached.Config, true, nil // true = from cache
}

// GetSSHKey retrieves SSH key from Vault (never cached)
func (m *Manager) GetSSHKey(repoName string) (*vault.SSHKey, error) {
	if !m.vaultClient.IsReachable() {
		return nil, fmt.Errorf("vault unreachable (SSH keys are never cached)")
	}

	return m.vaultClient.GetSSHKey(repoName)
}

// GetPAT retrieves PAT from Vault (never cached)
func (m *Manager) GetPAT(repoName string) (string, error) {
	if !m.vaultClient.IsReachable() {
		return "", fmt.Errorf("vault unreachable (PATs are never cached)")
	}

	return m.vaultClient.GetPAT(repoName)
}

// cacheConfig saves config to cache
func (m *Manager) cacheConfig(cfg *vault.Config) error {
	cached := &CachedConfig{
		Config:    cfg,
		FetchedAt: time.Now(),
	}

	cachePath := filepath.Join(m.cacheDir, configCacheFile)
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	return nil
}

// loadCache loads config from cache
func (m *Manager) loadCache() (*CachedConfig, error) {
	cachePath := filepath.Join(m.cacheDir, configCacheFile)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}

	var cached CachedConfig
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	return &cached, nil
}

// GetCacheAge returns how old the cache is (or 0 if no cache)
func (m *Manager) GetCacheAge() time.Duration {
	cached, err := m.loadCache()
	if err != nil {
		return 0
	}
	return time.Since(cached.FetchedAt)
}

// IsVaultReachable checks if Vault is reachable
func (m *Manager) IsVaultReachable() bool {
	return m.vaultClient.IsReachable()
}
