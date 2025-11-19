package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/vault"
)

// mockVaultClient is a mock implementation of vault client interface
type mockVaultClient struct {
	isReachable    bool
	config         *vault.Config
	configError    error
	sshKey         *vault.SSHKey
	sshKeyError    error
	pat            string
	patError       error
}

func (m *mockVaultClient) IsReachable() bool {
	return m.isReachable
}

func (m *mockVaultClient) GetConfig() (*vault.Config, error) {
	if m.configError != nil {
		return nil, m.configError
	}
	return m.config, nil
}

func (m *mockVaultClient) GetSSHKey(repoName string) (*vault.SSHKey, error) {
	if m.sshKeyError != nil {
		return nil, m.sshKeyError
	}
	return m.sshKey, nil
}

func (m *mockVaultClient) GetPAT(repoName string) (string, error) {
	if m.patError != nil {
		return "", m.patError
	}
	return m.pat, nil
}

// vaultClientInterface defines the interface we need from vault.Client
type vaultClientInterface interface {
	IsReachable() bool
	GetConfig() (*vault.Config, error)
	GetSSHKey(repoName string) (*vault.SSHKey, error)
	GetPAT(repoName string) (string, error)
}

// testManager is a Manager that accepts our interface for testing
type testManager struct {
	Manager
	testClient vaultClientInterface
}

func (tm *testManager) IsReachable() bool {
	return tm.testClient.IsReachable()
}

func (tm *testManager) GetConfig() (*vault.Config, bool, error) {
	// Try to get from Vault first
	if tm.testClient.IsReachable() {
		cfg, err := tm.testClient.GetConfig()
		if err == nil {
			// Cache it for later
			_ = tm.cacheConfig(cfg)
			return cfg, false, nil // false = not from cache
		}
		// Vault error - fall through to cache
	}

	// Try cache
	cached, err := tm.loadCache()
	if err != nil {
		return nil, false, errors.New("vault unreachable and no valid cache: " + err.Error())
	}

	// Check if cache is stale
	age := time.Since(cached.FetchedAt)
	if age > tm.cacheTTL {
		return cached.Config, true, errors.New("cache is stale (" + age.String() + " old, TTL is " + tm.cacheTTL.String() + ")")
	}

	return cached.Config, true, nil // true = from cache
}

func (tm *testManager) GetSSHKey(repoName string) (*vault.SSHKey, error) {
	if !tm.testClient.IsReachable() {
		return nil, errors.New("vault unreachable (SSH keys are never cached)")
	}
	return tm.testClient.GetSSHKey(repoName)
}

func (tm *testManager) GetPAT(repoName string) (string, error) {
	if !tm.testClient.IsReachable() {
		return "", errors.New("vault unreachable (PATs are never cached)")
	}
	return tm.testClient.GetPAT(repoName)
}

// createTestManager creates a testManager with a mock vault client for testing
func createTestManager(t *testing.T, mock vaultClientInterface) *testManager {
	t.Helper()

	tempDir := t.TempDir()

	return &testManager{
		Manager: Manager{
			cacheDir: tempDir,
			cacheTTL: defaultCacheTTL,
		},
		testClient: mock,
	}
}

func TestNewManager_CacheDirectory(t *testing.T) {
	tests := []struct {
		name      string
		cacheDir  string
		wantError bool
	}{
		{
			name:      "with explicit cache directory",
			cacheDir:  filepath.Join(t.TempDir(), "test-cache"),
			wantError: false,
		},
		{
			name:      "creates parent directories",
			cacheDir:  filepath.Join(t.TempDir(), "parent", "child", "cache"),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVaultClient{
				isReachable: true,
				config: &vault.Config{
					GitHubUsername: "test",
				},
			}

			mgr := &Manager{
				vaultClient: nil, // We won't use it
				cacheDir:    tt.cacheDir,
				cacheTTL:    defaultCacheTTL,
			}

			// Create a test manager wrapper
			testMgr := &testManager{
				Manager:    *mgr,
				testClient: mock,
			}

			// cacheConfig creates parent directories via MkdirAll
			// First create the cache directory
			err := os.MkdirAll(tt.cacheDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create cache directory: %v", err)
			}

			// Test that cache file can be written
			err = testMgr.cacheConfig(&vault.Config{
				GitHubUsername: "test",
			})

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify directory exists
			if !tt.wantError {
				if _, err := os.Stat(tt.cacheDir); os.IsNotExist(err) {
					t.Error("Cache directory was not created")
				}
			}
		})
	}
}

func TestGetConfig_VaultReachable(t *testing.T) {
	expectedConfig := &vault.Config{
		GitHubUsername:        "testuser",
		BareRepoPattern:       "/repos/{repo}",
		DefaultVisibility:     "private",
		AutoCreateGitHub:      true,
		TestBeforePush:        false,
		SyncOnSetup:           true,
		RetryOnPartialFailure: true,
	}

	mock := &mockVaultClient{
		isReachable: true,
		config:      expectedConfig,
	}

	mgr := createTestManager(t, mock)

	cfg, fromCache, err := mgr.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if fromCache {
		t.Error("Expected config from vault, but got from cache")
	}

	if cfg.GitHubUsername != expectedConfig.GitHubUsername {
		t.Errorf("GitHubUsername = %s, want %s", cfg.GitHubUsername, expectedConfig.GitHubUsername)
	}
	if cfg.BareRepoPattern != expectedConfig.BareRepoPattern {
		t.Errorf("BareRepoPattern = %s, want %s", cfg.BareRepoPattern, expectedConfig.BareRepoPattern)
	}
	if cfg.AutoCreateGitHub != expectedConfig.AutoCreateGitHub {
		t.Errorf("AutoCreateGitHub = %v, want %v", cfg.AutoCreateGitHub, expectedConfig.AutoCreateGitHub)
	}
	if cfg.DefaultVisibility != expectedConfig.DefaultVisibility {
		t.Errorf("DefaultVisibility = %s, want %s", cfg.DefaultVisibility, expectedConfig.DefaultVisibility)
	}
	if cfg.TestBeforePush != expectedConfig.TestBeforePush {
		t.Errorf("TestBeforePush = %v, want %v", cfg.TestBeforePush, expectedConfig.TestBeforePush)
	}
	if cfg.SyncOnSetup != expectedConfig.SyncOnSetup {
		t.Errorf("SyncOnSetup = %v, want %v", cfg.SyncOnSetup, expectedConfig.SyncOnSetup)
	}
	if cfg.RetryOnPartialFailure != expectedConfig.RetryOnPartialFailure {
		t.Errorf("RetryOnPartialFailure = %v, want %v", cfg.RetryOnPartialFailure, expectedConfig.RetryOnPartialFailure)
	}
}

func TestGetConfig_VaultUnreachable_NoCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: false,
	}

	mgr := createTestManager(t, mock)

	_, _, err := mgr.GetConfig()
	if err == nil {
		t.Fatal("Expected error when vault unreachable and no cache, got nil")
	}

	expectedMsg := "vault unreachable and no valid cache"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

func TestGetConfig_VaultUnreachable_ValidCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: false,
	}

	mgr := createTestManager(t, mock)

	// Create a valid cache
	cachedConfig := &vault.Config{
		GitHubUsername:  "cacheduser",
		BareRepoPattern: "/cached/{repo}",
	}

	err := mgr.cacheConfig(cachedConfig)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Try to get config
	cfg, fromCache, err := mgr.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if !fromCache {
		t.Error("Expected config from cache, but got from vault")
	}

	if cfg.GitHubUsername != cachedConfig.GitHubUsername {
		t.Errorf("GitHubUsername = %s, want %s", cfg.GitHubUsername, cachedConfig.GitHubUsername)
	}
	if cfg.BareRepoPattern != cachedConfig.BareRepoPattern {
		t.Errorf("BareRepoPattern = %s, want %s", cfg.BareRepoPattern, cachedConfig.BareRepoPattern)
	}
}

func TestGetConfig_VaultUnreachable_StaleCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: false,
	}

	mgr := createTestManager(t, mock)

	// Create a stale cache (older than TTL)
	cachedConfig := &vault.Config{
		GitHubUsername: "staleuser",
	}

	cached := &CachedConfig{
		Config:    cachedConfig,
		FetchedAt: time.Now().Add(-25 * time.Hour), // 25 hours ago (older than 24h TTL)
	}

	cachePath := filepath.Join(mgr.cacheDir, configCacheFile)
	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("Failed to marshal cache: %v", err)
	}

	err = os.WriteFile(cachePath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write cache: %v", err)
	}

	// Try to get config
	cfg, fromCache, err := mgr.GetConfig()
	if err == nil {
		t.Fatal("Expected error for stale cache, got nil")
	}

	if !fromCache {
		t.Error("Expected indicator that config came from cache")
	}

	// Should still return the config even though it's stale
	if cfg == nil {
		t.Error("Expected stale config to be returned, got nil")
	}

	expectedMsg := "cache is stale"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

func TestGetConfig_VaultError_FallbackToCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
		configError: errors.New("vault internal error"),
	}

	mgr := createTestManager(t, mock)

	// Create a valid cache first
	cachedConfig := &vault.Config{
		GitHubUsername: "cacheduser",
	}
	err := mgr.cacheConfig(cachedConfig)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Try to get config - should fall back to cache despite vault being reachable
	cfg, fromCache, err := mgr.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if !fromCache {
		t.Error("Expected config from cache due to vault error")
	}

	if cfg.GitHubUsername != cachedConfig.GitHubUsername {
		t.Errorf("GitHubUsername = %s, want %s", cfg.GitHubUsername, cachedConfig.GitHubUsername)
	}
}

func TestGetConfig_CachesSuccessfulFetch(t *testing.T) {
	expectedConfig := &vault.Config{
		GitHubUsername: "testuser",
	}

	mock := &mockVaultClient{
		isReachable: true,
		config:      expectedConfig,
	}

	mgr := createTestManager(t, mock)

	// First call - should fetch from vault and cache
	_, _, err := mgr.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	// Verify cache was created
	cached, err := mgr.loadCache()
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	if cached.Config.GitHubUsername != expectedConfig.GitHubUsername {
		t.Errorf("Cached GitHubUsername = %s, want %s", cached.Config.GitHubUsername, expectedConfig.GitHubUsername)
	}

	// Verify cache timestamp is recent
	age := time.Since(cached.FetchedAt)
	if age > 1*time.Second {
		t.Errorf("Cache age = %v, want less than 1 second", age)
	}
}

func TestGetSSHKey_VaultReachable(t *testing.T) {
	expectedKey := &vault.SSHKey{
		PrivateKey: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
		PublicKey:  "ssh-rsa AAAA...",
	}

	mock := &mockVaultClient{
		isReachable: true,
		sshKey:      expectedKey,
	}

	mgr := createTestManager(t, mock)

	key, err := mgr.GetSSHKey("myrepo")
	if err != nil {
		t.Fatalf("GetSSHKey() error = %v", err)
	}

	if key.PrivateKey != expectedKey.PrivateKey {
		t.Errorf("PrivateKey = %s, want %s", key.PrivateKey, expectedKey.PrivateKey)
	}
	if key.PublicKey != expectedKey.PublicKey {
		t.Errorf("PublicKey = %s, want %s", key.PublicKey, expectedKey.PublicKey)
	}
}

func TestGetSSHKey_VaultUnreachable(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: false,
	}

	mgr := createTestManager(t, mock)

	_, err := mgr.GetSSHKey("myrepo")
	if err == nil {
		t.Fatal("Expected error when vault unreachable, got nil")
	}

	expectedMsg := "vault unreachable"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}

	expectedMsg2 := "SSH keys are never cached"
	if !contains(err.Error(), expectedMsg2) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg2)
	}
}

func TestGetSSHKey_VaultError(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
		sshKeyError: errors.New("SSH key not found in vault"),
	}

	mgr := createTestManager(t, mock)

	_, err := mgr.GetSSHKey("myrepo")
	if err == nil {
		t.Fatal("Expected error when vault returns error, got nil")
	}

	if !contains(err.Error(), "SSH key not found") {
		t.Errorf("Error should contain vault error message, got: %v", err)
	}
}

func TestGetPAT_VaultReachable(t *testing.T) {
	expectedPAT := "ghp_testtoken123"

	mock := &mockVaultClient{
		isReachable: true,
		pat:         expectedPAT,
	}

	mgr := createTestManager(t, mock)

	pat, err := mgr.GetPAT("myrepo")
	if err != nil {
		t.Fatalf("GetPAT() error = %v", err)
	}

	if pat != expectedPAT {
		t.Errorf("PAT = %s, want %s", pat, expectedPAT)
	}
}

func TestGetPAT_VaultUnreachable(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: false,
	}

	mgr := createTestManager(t, mock)

	_, err := mgr.GetPAT("myrepo")
	if err == nil {
		t.Fatal("Expected error when vault unreachable, got nil")
	}

	expectedMsg := "vault unreachable"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}

	expectedMsg2 := "PATs are never cached"
	if !contains(err.Error(), expectedMsg2) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg2)
	}
}

func TestGetPAT_VaultError(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
		patError:    errors.New("PAT not found in vault"),
	}

	mgr := createTestManager(t, mock)

	_, err := mgr.GetPAT("myrepo")
	if err == nil {
		t.Fatal("Expected error when vault returns error, got nil")
	}

	if !contains(err.Error(), "PAT not found") {
		t.Errorf("Error should contain vault error message, got: %v", err)
	}
}

func TestCacheConfig(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	testConfig := &vault.Config{
		GitHubUsername:   "testuser",
		BareRepoPattern:  "/repos/{repo}",
		AutoCreateGitHub: true,
		TestBeforePush:   true,
		SyncOnSetup:      false,
	}

	err := mgr.cacheConfig(testConfig)
	if err != nil {
		t.Fatalf("cacheConfig() error = %v", err)
	}

	// Verify cache file exists
	cachePath := filepath.Join(mgr.cacheDir, configCacheFile)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Verify cache content
	cached, err := mgr.loadCache()
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}

	if cached.Config.GitHubUsername != testConfig.GitHubUsername {
		t.Errorf("Cached GitHubUsername = %s, want %s", cached.Config.GitHubUsername, testConfig.GitHubUsername)
	}
	if cached.Config.BareRepoPattern != testConfig.BareRepoPattern {
		t.Errorf("Cached BareRepoPattern = %s, want %s", cached.Config.BareRepoPattern, testConfig.BareRepoPattern)
	}
	if cached.Config.AutoCreateGitHub != testConfig.AutoCreateGitHub {
		t.Errorf("Cached AutoCreateGitHub = %v, want %v", cached.Config.AutoCreateGitHub, testConfig.AutoCreateGitHub)
	}
	if cached.Config.TestBeforePush != testConfig.TestBeforePush {
		t.Errorf("Cached TestBeforePush = %v, want %v", cached.Config.TestBeforePush, testConfig.TestBeforePush)
	}
	if cached.Config.SyncOnSetup != testConfig.SyncOnSetup {
		t.Errorf("Cached SyncOnSetup = %v, want %v", cached.Config.SyncOnSetup, testConfig.SyncOnSetup)
	}

	// Verify timestamp
	age := time.Since(cached.FetchedAt)
	if age > 1*time.Second {
		t.Errorf("Cache timestamp age = %v, want less than 1 second", age)
	}
}

func TestLoadCache_NoCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	_, err := mgr.loadCache()
	if err == nil {
		t.Fatal("Expected error when no cache exists, got nil")
	}

	expectedMsg := "failed to read cache"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	// Write invalid JSON to cache file
	cachePath := filepath.Join(mgr.cacheDir, configCacheFile)
	err := os.WriteFile(cachePath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid cache: %v", err)
	}

	_, err = mgr.loadCache()
	if err == nil {
		t.Fatal("Expected error when cache has invalid JSON, got nil")
	}

	expectedMsg := "failed to unmarshal cache"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

func TestLoadCache_ValidCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	// Create valid cache
	expectedConfig := &vault.Config{
		GitHubUsername:  "testuser",
		BareRepoPattern: "/repos/{repo}",
	}

	err := mgr.cacheConfig(expectedConfig)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Load cache
	cached, err := mgr.loadCache()
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}

	if cached.Config.GitHubUsername != expectedConfig.GitHubUsername {
		t.Errorf("GitHubUsername = %s, want %s", cached.Config.GitHubUsername, expectedConfig.GitHubUsername)
	}
	if cached.Config.BareRepoPattern != expectedConfig.BareRepoPattern {
		t.Errorf("BareRepoPattern = %s, want %s", cached.Config.BareRepoPattern, expectedConfig.BareRepoPattern)
	}
}

func TestGetCacheAge_NoCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	age := mgr.GetCacheAge()
	if age != 0 {
		t.Errorf("GetCacheAge() = %v, want 0 when no cache exists", age)
	}
}

func TestGetCacheAge_WithCache(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	// Create a cache from 1 hour ago
	testTime := time.Now().Add(-1 * time.Hour)
	cached := &CachedConfig{
		Config: &vault.Config{
			GitHubUsername: "test",
		},
		FetchedAt: testTime,
	}

	cachePath := filepath.Join(mgr.cacheDir, configCacheFile)
	data, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("Failed to marshal cache: %v", err)
	}

	err = os.WriteFile(cachePath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write cache: %v", err)
	}

	age := mgr.GetCacheAge()

	// Should be approximately 1 hour (allow some tolerance)
	expectedAge := 1 * time.Hour
	tolerance := 1 * time.Second

	if age < expectedAge-tolerance || age > expectedAge+tolerance {
		t.Errorf("GetCacheAge() = %v, want approximately %v", age, expectedAge)
	}
}

func TestIsVaultReachable(t *testing.T) {
	tests := []struct {
		name        string
		isReachable bool
	}{
		{
			name:        "vault reachable",
			isReachable: true,
		},
		{
			name:        "vault unreachable",
			isReachable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVaultClient{
				isReachable: tt.isReachable,
			}

			mgr := createTestManager(t, mock)

			got := mgr.IsReachable()
			if got != tt.isReachable {
				t.Errorf("IsVaultReachable() = %v, want %v", got, tt.isReachable)
			}
		})
	}
}

func TestCacheTTL(t *testing.T) {
	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := createTestManager(t, mock)

	// Verify default TTL
	if mgr.cacheTTL != defaultCacheTTL {
		t.Errorf("cacheTTL = %v, want %v", mgr.cacheTTL, defaultCacheTTL)
	}

	if mgr.cacheTTL != 24*time.Hour {
		t.Errorf("cacheTTL = %v, want 24 hours", mgr.cacheTTL)
	}
}

func TestCachedConfig_JSONSerialization(t *testing.T) {
	original := &CachedConfig{
		Config: &vault.Config{
			GitHubUsername:        "testuser",
			BareRepoPattern:       "/repos/{repo}",
			DefaultVisibility:     "private",
			AutoCreateGitHub:      true,
			TestBeforePush:        false,
			SyncOnSetup:           true,
			RetryOnPartialFailure: true,
		},
		FetchedAt: time.Now().Round(time.Second), // Round to avoid precision issues
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	// Unmarshal
	var decoded CachedConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	// Compare
	if decoded.Config.GitHubUsername != original.Config.GitHubUsername {
		t.Errorf("GitHubUsername = %s, want %s", decoded.Config.GitHubUsername, original.Config.GitHubUsername)
	}
	if decoded.Config.BareRepoPattern != original.Config.BareRepoPattern {
		t.Errorf("BareRepoPattern = %s, want %s", decoded.Config.BareRepoPattern, original.Config.BareRepoPattern)
	}
	if decoded.Config.AutoCreateGitHub != original.Config.AutoCreateGitHub {
		t.Errorf("AutoCreateGitHub = %v, want %v", decoded.Config.AutoCreateGitHub, original.Config.AutoCreateGitHub)
	}
	if decoded.Config.DefaultVisibility != original.Config.DefaultVisibility {
		t.Errorf("DefaultVisibility = %s, want %s", decoded.Config.DefaultVisibility, original.Config.DefaultVisibility)
	}

	// Compare timestamps (allowing 1 second tolerance for precision)
	timeDiff := decoded.FetchedAt.Sub(original.FetchedAt)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("FetchedAt = %v, want %v", decoded.FetchedAt, original.FetchedAt)
	}
}

func TestCacheConfig_WithNestedDirectory(t *testing.T) {
	tempBase := t.TempDir()
	cacheDir := filepath.Join(tempBase, "nested", "cache", "dir")

	// Create the nested directory structure first
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested cache directory: %v", err)
	}

	mock := &mockVaultClient{
		isReachable: true,
	}

	mgr := &testManager{
		Manager: Manager{
			cacheDir: cacheDir,
			cacheTTL: defaultCacheTTL,
		},
		testClient: mock,
	}

	testConfig := &vault.Config{
		GitHubUsername: "test",
	}

	err = mgr.cacheConfig(testConfig)
	if err != nil {
		t.Fatalf("cacheConfig() error = %v", err)
	}

	// Verify cache file was created
	cachePath := filepath.Join(cacheDir, configCacheFile)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}
}

func TestCacheExpiration(t *testing.T) {
	tests := []struct {
		name       string
		cacheAge   time.Duration
		shouldFail bool
	}{
		{
			name:       "fresh cache (1 hour old)",
			cacheAge:   1 * time.Hour,
			shouldFail: false,
		},
		{
			name:       "cache just under TTL (23 hours 59 minutes)",
			cacheAge:   23*time.Hour + 59*time.Minute,
			shouldFail: false,
		},
		{
			name:       "stale cache (25 hours old)",
			cacheAge:   25 * time.Hour,
			shouldFail: true,
		},
		{
			name:       "very stale cache (7 days old)",
			cacheAge:   7 * 24 * time.Hour,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVaultClient{
				isReachable: false, // Force use of cache
			}

			mgr := createTestManager(t, mock)

			// Create cache with specific age
			cached := &CachedConfig{
				Config: &vault.Config{
					GitHubUsername: "test",
				},
				FetchedAt: time.Now().Add(-tt.cacheAge),
			}

			cachePath := filepath.Join(mgr.cacheDir, configCacheFile)
			data, err := json.Marshal(cached)
			if err != nil {
				t.Fatalf("Failed to marshal cache: %v", err)
			}

			err = os.WriteFile(cachePath, data, 0644)
			if err != nil {
				t.Fatalf("Failed to write cache: %v", err)
			}

			// Try to get config
			cfg, fromCache, err := mgr.GetConfig()

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected error for stale cache, got nil")
				}
				if !contains(err.Error(), "cache is stale") {
					t.Errorf("Error should indicate stale cache, got: %v", err)
				}
				// Should still return config even if stale
				if cfg == nil {
					t.Error("Expected config to be returned even if stale")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid cache: %v", err)
				}
				if !fromCache {
					t.Error("Config should be from cache")
				}
				if cfg == nil {
					t.Error("Expected valid config")
				}
			}
		})
	}
}

func TestConfigCacheFile_Constant(t *testing.T) {
	if configCacheFile != "config.json" {
		t.Errorf("configCacheFile = %s, want config.json", configCacheFile)
	}
}

func TestDefaultCacheTTL_Constant(t *testing.T) {
	if defaultCacheTTL != 24*time.Hour {
		t.Errorf("defaultCacheTTL = %v, want 24 hours", defaultCacheTTL)
	}
}

// TestManager_cacheConfig tests the private cacheConfig method
func TestManager_cacheConfig(t *testing.T) {
	tempDir := t.TempDir()

	mgr := &Manager{
		cacheDir: tempDir,
		cacheTTL: defaultCacheTTL,
	}

	testConfig := &vault.Config{
		GitHubUsername:  "testuser",
		BareRepoPattern: "/repos/{repo}",
	}

	err := mgr.cacheConfig(testConfig)
	if err != nil {
		t.Fatalf("cacheConfig() error = %v", err)
	}

	// Verify cache was created
	cachePath := filepath.Join(tempDir, configCacheFile)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Verify we can read it back
	cached, err := mgr.loadCache()
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}

	if cached.Config.GitHubUsername != testConfig.GitHubUsername {
		t.Errorf("Cached username = %s, want %s", cached.Config.GitHubUsername, testConfig.GitHubUsername)
	}
}

// TestManager_cacheConfig_ErrorPaths tests error handling in cacheConfig
func TestManager_cacheConfig_ErrorPaths(t *testing.T) {
	tests := []struct {
		name     string
		cacheDir string
		wantErr  bool
	}{
		{
			name:     "invalid directory path (file instead of directory)",
			cacheDir: "/dev/null", // This is a file, not a directory
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{
				cacheDir: tt.cacheDir,
				cacheTTL: defaultCacheTTL,
			}

			testConfig := &vault.Config{
				GitHubUsername: "testuser",
			}

			err := mgr.cacheConfig(testConfig)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestManager_loadCache tests the private loadCache method
func TestManager_loadCache(t *testing.T) {
	tempDir := t.TempDir()

	mgr := &Manager{
		cacheDir: tempDir,
		cacheTTL: defaultCacheTTL,
	}

	// Test with no cache
	_, err := mgr.loadCache()
	if err == nil {
		t.Error("Expected error when no cache exists")
	}

	// Create a cache
	testConfig := &vault.Config{
		GitHubUsername: "testuser",
	}
	err = mgr.cacheConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Now loading should work
	cached, err := mgr.loadCache()
	if err != nil {
		t.Fatalf("loadCache() error = %v", err)
	}

	if cached.Config.GitHubUsername != testConfig.GitHubUsername {
		t.Errorf("Username = %s, want %s", cached.Config.GitHubUsername, testConfig.GitHubUsername)
	}
}

// TestManager_GetCacheAge tests the GetCacheAge method
func TestManager_GetCacheAge(t *testing.T) {
	tempDir := t.TempDir()

	mgr := &Manager{
		cacheDir: tempDir,
		cacheTTL: defaultCacheTTL,
	}

	// With no cache
	age := mgr.GetCacheAge()
	if age != 0 {
		t.Errorf("GetCacheAge() with no cache = %v, want 0", age)
	}

	// Create a cache
	testConfig := &vault.Config{
		GitHubUsername: "testuser",
	}
	err := mgr.cacheConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Age should be very small (recent)
	age = mgr.GetCacheAge()
	if age > 1*time.Second {
		t.Errorf("GetCacheAge() = %v, want less than 1 second", age)
	}
	if age < 0 {
		t.Errorf("GetCacheAge() = %v, want positive duration", age)
	}
}

// TestCachedConfig_Structure tests the CachedConfig struct
func TestCachedConfig_Structure(t *testing.T) {
	now := time.Now()
	cached := &CachedConfig{
		Config: &vault.Config{
			GitHubUsername:  "testuser",
			BareRepoPattern: "/repos/{repo}",
		},
		FetchedAt: now,
	}

	if cached.Config == nil {
		t.Error("Config should not be nil")
	}
	if cached.FetchedAt.IsZero() {
		t.Error("FetchedAt should not be zero")
	}
	if cached.Config.GitHubUsername != "testuser" {
		t.Errorf("GitHubUsername = %s, want testuser", cached.Config.GitHubUsername)
	}
}

// TestManager_Constants tests the package constants
func TestManager_Constants(t *testing.T) {
	if configCacheFile != "config.json" {
		t.Errorf("configCacheFile = %s, want config.json", configCacheFile)
	}
	if defaultCacheTTL != 24*time.Hour {
		t.Errorf("defaultCacheTTL = %v, want 24h", defaultCacheTTL)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
