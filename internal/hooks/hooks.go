package hooks

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	backupSuffix = ".githelper-backup"
)

// prePushHookTemplate is the pre-push hook template
const prePushHookTemplate = `#!/bin/bash
# GitHelper pre-push hook
# Verifies both remotes are reachable before push

githelper github test --quiet %s || {
    echo "⚠️  Remote connectivity issue detected"
    echo "Run 'githelper github test %s' for details"
    exit 1
}
`

// postPushHookTemplate is the post-push hook template
const postPushHookTemplate = `#!/bin/bash
# GitHelper post-push hook
# Updates sync status in state file

githelper github status --quiet %s > /dev/null 2>&1 || true
`

// Manager handles git hook installation
type Manager struct {
	repoPath string
	repoName string
	hooksDir string
}

// NewManager creates a new hooks manager
func NewManager(repoPath string) *Manager {
	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	// Extract repo name from path
	repoName := filepath.Base(repoPath)
	return &Manager{
		repoPath: repoPath,
		repoName: repoName,
		hooksDir: hooksDir,
	}
}

// Install installs githelper hooks
func (m *Manager) Install() error {
	// Ensure hooks directory exists
	if err := os.MkdirAll(m.hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Generate hooks with repo name
	prePushHook := fmt.Sprintf(prePushHookTemplate, m.repoName, m.repoName)
	postPushHook := fmt.Sprintf(postPushHookTemplate, m.repoName)

	// Install pre-push hook
	if err := m.installHook("pre-push", prePushHook); err != nil {
		return fmt.Errorf("failed to install pre-push hook: %w", err)
	}

	// Install post-push hook
	if err := m.installHook("post-push", postPushHook); err != nil {
		return fmt.Errorf("failed to install post-push hook: %w", err)
	}

	return nil
}

// installHook installs a single hook with backup
func (m *Manager) installHook(name, content string) error {
	hookPath := filepath.Join(m.hooksDir, name)
	backupPath := hookPath + backupSuffix

	// Check if hook already exists
	if _, err := os.Stat(hookPath); err == nil {
		// Backup existing hook
		if err := os.Rename(hookPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing %s hook: %w", name, err)
		}
	}

	// Write new hook
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		// Try to restore backup if write failed
		if _, statErr := os.Stat(backupPath); statErr == nil {
			_ = os.Rename(backupPath, hookPath)
		}
		return fmt.Errorf("failed to write %s hook: %w", name, err)
	}

	return nil
}

// Uninstall removes githelper hooks
func (m *Manager) Uninstall() error {
	hooks := []string{"pre-push", "post-push"}

	for _, hook := range hooks {
		hookPath := filepath.Join(m.hooksDir, hook)

		// Remove hook if it exists
		if _, err := os.Stat(hookPath); err == nil {
			if err := os.Remove(hookPath); err != nil {
				return fmt.Errorf("failed to remove %s hook: %w", hook, err)
			}
		}
	}

	return nil
}

// GetBackupPath returns the backup path for a hook
func (m *Manager) GetBackupPath(hookName string) string {
	return filepath.Join(m.hooksDir, hookName+backupSuffix)
}

// HasBackup checks if a backup exists for a hook
func (m *Manager) HasBackup(hookName string) bool {
	backupPath := m.GetBackupPath(hookName)
	_, err := os.Stat(backupPath)
	return err == nil
}

// IsInstalled checks if githelper hooks are installed
func (m *Manager) IsInstalled() bool {
	prePushPath := filepath.Join(m.hooksDir, "pre-push")
	postPushPath := filepath.Join(m.hooksDir, "post-push")

	_, prePushErr := os.Stat(prePushPath)
	_, postPushErr := os.Stat(postPushPath)

	return prePushErr == nil && postPushErr == nil
}
