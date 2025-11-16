package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultStateFile = "state.yaml"
)

// Manager handles the state file
type Manager struct {
	stateFile string
	mu        sync.RWMutex
}

// State represents the entire state file
type State struct {
	Repositories map[string]*Repository `yaml:"repositories"`
}

// Repository represents a single repository's state
type Repository struct {
	Path    string    `yaml:"path"`
	Remote  string    `yaml:"remote"`
	Created time.Time `yaml:"created"`
	Type    string    `yaml:"type,omitempty"`
	GitHub  *GitHub   `yaml:"github,omitempty"`
}

// GitHub represents GitHub integration state
type GitHub struct {
	Enabled    bool      `yaml:"enabled"`
	User       string    `yaml:"user"`
	Repo       string    `yaml:"repo"`
	SyncStatus string    `yaml:"sync_status"` // "synced", "ahead", "behind", "diverged", "unknown"
	LastSync   time.Time `yaml:"last_sync,omitempty"`
	NeedsRetry bool      `yaml:"needs_retry"`
	LastError  string    `yaml:"last_error,omitempty"`
}

// NewManager creates a new state manager
func NewManager(stateDir string) (*Manager, error) {
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		stateDir = filepath.Join(home, ".githelper")
	}

	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, defaultStateFile)

	return &Manager{
		stateFile: stateFile,
	}, nil
}

// Load loads the state from file
func (m *Manager) Load() (*State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If file doesn't exist, return empty state
	if _, err := os.Stat(m.stateFile); os.IsNotExist(err) {
		return &State{
			Repositories: make(map[string]*Repository),
		}, nil
	}

	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Initialize map if nil
	if state.Repositories == nil {
		state.Repositories = make(map[string]*Repository)
	}

	return &state, nil
}

// Save saves the state to file
func (m *Manager) Save(state *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(m.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// AddRepository adds or updates a repository in state
func (m *Manager) AddRepository(name string, repo *Repository) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	state.Repositories[name] = repo
	return m.Save(state)
}

// GetRepository retrieves a repository from state
func (m *Manager) GetRepository(name string) (*Repository, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	repo, exists := state.Repositories[name]
	if !exists {
		return nil, fmt.Errorf("repository %s not found in state", name)
	}

	return repo, nil
}

// ListRepositories returns all repositories
func (m *Manager) ListRepositories() (map[string]*Repository, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	return state.Repositories, nil
}

// DeleteRepository removes a repository from state
func (m *Manager) DeleteRepository(name string) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	delete(state.Repositories, name)
	return m.Save(state)
}

// UpdateGitHubStatus updates the GitHub sync status for a repository
func (m *Manager) UpdateGitHubStatus(name, syncStatus string, lastError string) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	repo, exists := state.Repositories[name]
	if !exists {
		return fmt.Errorf("repository %s not found", name)
	}

	if repo.GitHub == nil {
		repo.GitHub = &GitHub{}
	}

	repo.GitHub.SyncStatus = syncStatus
	repo.GitHub.LastSync = time.Now()
	repo.GitHub.LastError = lastError
	repo.GitHub.NeedsRetry = (lastError != "")

	return m.Save(state)
}
