package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestGitHelperError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GitHelperError
		expected string
	}{
		{
			name: "error without wrapped error",
			err: &GitHelperError{
				Type:    ErrorTypeGit,
				Message: "test error",
			},
			expected: "git: test error",
		},
		{
			name: "error with wrapped error",
			err: &GitHelperError{
				Type:    ErrorTypeVault,
				Message: "vault connection failed",
				Err:     errors.New("connection refused"),
			},
			expected: "vault: vault connection failed (caused by: connection refused)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGitHelperError_Unwrap(t *testing.T) {
	tests := []struct {
		name     string
		err      *GitHelperError
		expected error
	}{
		{
			name: "error without wrapped error",
			err: &GitHelperError{
				Type:    ErrorTypeGit,
				Message: "test error",
			},
			expected: nil,
		},
		{
			name: "error with wrapped error",
			err: &GitHelperError{
				Type:    ErrorTypeVault,
				Message: "vault connection failed",
				Err:     errors.New("connection refused"),
			},
			expected: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Unwrap()
			if tt.expected == nil {
				if got != nil {
					t.Errorf("Unwrap() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("Unwrap() = nil, want %v", tt.expected)
				} else if got.Error() != tt.expected.Error() {
					t.Errorf("Unwrap() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}

func TestGitHelperError_UserFriendlyMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      *GitHelperError
		expected string
	}{
		{
			name: "error without hint",
			err: &GitHelperError{
				Type:    ErrorTypeGit,
				Message: "test error",
			},
			expected: "test error",
		},
		{
			name: "error with hint",
			err: &GitHelperError{
				Type:    ErrorTypeVault,
				Message: "vault connection failed",
				Hint:    "Check vault configuration",
			},
			expected: "vault connection failed\n\nSuggestion: Check vault configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.UserFriendlyMessage()
			if got != tt.expected {
				t.Errorf("UserFriendlyMessage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	err := New(ErrorTypeConfig, "configuration invalid")

	if err.Type != ErrorTypeConfig {
		t.Errorf("New() Type = %v, want %v", err.Type, ErrorTypeConfig)
	}
	if err.Message != "configuration invalid" {
		t.Errorf("New() Message = %q, want %q", err.Message, "configuration invalid")
	}
	if err.Hint != "" {
		t.Errorf("New() Hint = %q, want empty string", err.Hint)
	}
	if err.Err != nil {
		t.Errorf("New() Err = %v, want nil", err.Err)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(ErrorTypeNetwork, "network operation failed", originalErr)

	if err.Type != ErrorTypeNetwork {
		t.Errorf("Wrap() Type = %v, want %v", err.Type, ErrorTypeNetwork)
	}
	if err.Message != "network operation failed" {
		t.Errorf("Wrap() Message = %q, want %q", err.Message, "network operation failed")
	}
	if err.Err != originalErr {
		t.Errorf("Wrap() Err = %v, want %v", err.Err, originalErr)
	}
	if err.Hint != "" {
		t.Errorf("Wrap() Hint = %q, want empty string", err.Hint)
	}
}

func TestWithHint(t *testing.T) {
	err := New(ErrorTypeGit, "git command failed")
	hintedErr := WithHint(err, "try running git status")

	if hintedErr.Hint != "try running git status" {
		t.Errorf("WithHint() Hint = %q, want %q", hintedErr.Hint, "try running git status")
	}
	// Ensure it's the same error object (modified in place)
	if hintedErr != err {
		t.Error("WithHint() should return the same error instance")
	}
}

func TestVaultUnreachable(t *testing.T) {
	originalErr := errors.New("connection timeout")
	err := VaultUnreachable("http://localhost:8200", originalErr)

	if err.Type != ErrorTypeVault {
		t.Errorf("VaultUnreachable() Type = %v, want %v", err.Type, ErrorTypeVault)
	}
	if !strings.Contains(err.Message, "Vault unreachable at http://localhost:8200") {
		t.Errorf("VaultUnreachable() Message = %q, want it to contain 'Vault unreachable at http://localhost:8200'", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("VaultUnreachable() Err = %v, want %v", err.Err, originalErr)
	}
	if !strings.Contains(err.Hint, "githelper doctor") {
		t.Errorf("VaultUnreachable() Hint = %q, want it to contain 'githelper doctor'", err.Hint)
	}
}

func TestGitNotInstalled(t *testing.T) {
	originalErr := errors.New("executable not found")
	err := GitNotInstalled(originalErr)

	if err.Type != ErrorTypeGit {
		t.Errorf("GitNotInstalled() Type = %v, want %v", err.Type, ErrorTypeGit)
	}
	if !strings.Contains(err.Message, "Git is not installed") {
		t.Errorf("GitNotInstalled() Message = %q, want it to contain 'Git is not installed'", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("GitNotInstalled() Err = %v, want %v", err.Err, originalErr)
	}
	if !strings.Contains(err.Hint, "package manager") {
		t.Errorf("GitNotInstalled() Hint = %q, want it to contain 'package manager'", err.Hint)
	}
}

func TestRepositoryNotFound(t *testing.T) {
	err := RepositoryNotFound("myrepo")

	if err.Type != ErrorTypeState {
		t.Errorf("RepositoryNotFound() Type = %v, want %v", err.Type, ErrorTypeState)
	}
	if !strings.Contains(err.Message, "myrepo") {
		t.Errorf("RepositoryNotFound() Message = %q, want it to contain 'myrepo'", err.Message)
	}
	if !strings.Contains(err.Hint, "githelper repo list") {
		t.Errorf("RepositoryNotFound() Hint = %q, want it to contain 'githelper repo list'", err.Hint)
	}
	if !strings.Contains(err.Hint, "githelper repo create myrepo") {
		t.Errorf("RepositoryNotFound() Hint = %q, want it to contain 'githelper repo create myrepo'", err.Hint)
	}
}

func TestGitHubAuthFailed(t *testing.T) {
	originalErr := errors.New("401 Unauthorized")
	err := GitHubAuthFailed(originalErr)

	if err.Type != ErrorTypeGitHub {
		t.Errorf("GitHubAuthFailed() Type = %v, want %v", err.Type, ErrorTypeGitHub)
	}
	if !strings.Contains(err.Message, "GitHub authentication failed") {
		t.Errorf("GitHubAuthFailed() Message = %q, want it to contain 'GitHub authentication failed'", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("GitHubAuthFailed() Err = %v, want %v", err.Err, originalErr)
	}
	if !strings.Contains(err.Hint, "GitHub PAT") {
		t.Errorf("GitHubAuthFailed() Hint = %q, want it to contain 'GitHub PAT'", err.Hint)
	}
}

func TestSSHKeyNotFound(t *testing.T) {
	tests := []struct {
		name             string
		repoName         string
		expectedInHint   string
		unexpectedInHint string
	}{
		{
			name:             "default repository",
			repoName:         "default",
			expectedInHint:   "Check that the SSH key exists in Vault",
			unexpectedInHint: "No SSH key found for repository",
		},
		{
			name:             "specific repository",
			repoName:         "myrepo",
			expectedInHint:   "No SSH key found for repository 'myrepo'",
			unexpectedInHint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SSHKeyNotFound(tt.repoName)

			if err.Type != ErrorTypeVault {
				t.Errorf("SSHKeyNotFound() Type = %v, want %v", err.Type, ErrorTypeVault)
			}
			if !strings.Contains(err.Message, tt.repoName) {
				t.Errorf("SSHKeyNotFound() Message = %q, want it to contain %q", err.Message, tt.repoName)
			}
			if !strings.Contains(err.Hint, tt.expectedInHint) {
				t.Errorf("SSHKeyNotFound() Hint = %q, want it to contain %q", err.Hint, tt.expectedInHint)
			}
			if tt.unexpectedInHint != "" && strings.Contains(err.Hint, tt.unexpectedInHint) {
				t.Errorf("SSHKeyNotFound() Hint = %q, should not contain %q", err.Hint, tt.unexpectedInHint)
			}
		})
	}
}

func TestRemoteNotConfigured(t *testing.T) {
	err := RemoteNotConfigured("github")

	if err.Type != ErrorTypeGit {
		t.Errorf("RemoteNotConfigured() Type = %v, want %v", err.Type, ErrorTypeGit)
	}
	if !strings.Contains(err.Message, "github") {
		t.Errorf("RemoteNotConfigured() Message = %q, want it to contain 'github'", err.Message)
	}
	if !strings.Contains(err.Hint, "git remote add github") {
		t.Errorf("RemoteNotConfigured() Hint = %q, want it to contain 'git remote add github'", err.Hint)
	}
}

func TestDualPushNotConfigured(t *testing.T) {
	err := DualPushNotConfigured()

	if err.Type != ErrorTypeGit {
		t.Errorf("DualPushNotConfigured() Type = %v, want %v", err.Type, ErrorTypeGit)
	}
	if !strings.Contains(err.Message, "Dual-push not configured") {
		t.Errorf("DualPushNotConfigured() Message = %q, want it to contain 'Dual-push not configured'", err.Message)
	}
	if !strings.Contains(err.Hint, "githelper github setup") {
		t.Errorf("DualPushNotConfigured() Hint = %q, want it to contain 'githelper github setup'", err.Hint)
	}
}

func TestSyncRequired(t *testing.T) {
	err := SyncRequired("myrepo", 5)

	if err.Type != ErrorTypeGitHub {
		t.Errorf("SyncRequired() Type = %v, want %v", err.Type, ErrorTypeGitHub)
	}
	if !strings.Contains(err.Message, "5 commit(s) behind") {
		t.Errorf("SyncRequired() Message = %q, want it to contain '5 commit(s) behind'", err.Message)
	}
	if !strings.Contains(err.Hint, "githelper github sync myrepo") {
		t.Errorf("SyncRequired() Hint = %q, want it to contain 'githelper github sync myrepo'", err.Hint)
	}
}

func TestDivergenceDetected(t *testing.T) {
	err := DivergenceDetected(3, 2)

	if err.Type != ErrorTypeGit {
		t.Errorf("DivergenceDetected() Type = %v, want %v", err.Type, ErrorTypeGit)
	}
	if !strings.Contains(err.Message, "bare has 3 unique commits") {
		t.Errorf("DivergenceDetected() Message = %q, want it to contain 'bare has 3 unique commits'", err.Message)
	}
	if !strings.Contains(err.Message, "GitHub has 2 unique commits") {
		t.Errorf("DivergenceDetected() Message = %q, want it to contain 'GitHub has 2 unique commits'", err.Message)
	}
	if !strings.Contains(err.Hint, "Manual resolution required") {
		t.Errorf("DivergenceDetected() Hint = %q, want it to contain 'Manual resolution required'", err.Hint)
	}
}

func TestStateCorrupted(t *testing.T) {
	originalErr := errors.New("yaml parse error")
	err := StateCorrupted(originalErr)

	if err.Type != ErrorTypeState {
		t.Errorf("StateCorrupted() Type = %v, want %v", err.Type, ErrorTypeState)
	}
	if !strings.Contains(err.Message, "State file is corrupted") {
		t.Errorf("StateCorrupted() Message = %q, want it to contain 'State file is corrupted'", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("StateCorrupted() Err = %v, want %v", err.Err, originalErr)
	}
	if !strings.Contains(err.Hint, "githelper doctor") {
		t.Errorf("StateCorrupted() Hint = %q, want it to contain 'githelper doctor'", err.Hint)
	}
}

func TestNetworkError(t *testing.T) {
	originalErr := errors.New("connection timed out")
	err := NetworkError("fetch", originalErr)

	if err.Type != ErrorTypeNetwork {
		t.Errorf("NetworkError() Type = %v, want %v", err.Type, ErrorTypeNetwork)
	}
	if !strings.Contains(err.Message, "Network error during fetch") {
		t.Errorf("NetworkError() Message = %q, want it to contain 'Network error during fetch'", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("NetworkError() Err = %v, want %v", err.Err, originalErr)
	}
	if !strings.Contains(err.Hint, "internet connection") {
		t.Errorf("NetworkError() Hint = %q, want it to contain 'internet connection'", err.Hint)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	err := InvalidConfiguration("vault.address", "must be a valid URL")

	if err.Type != ErrorTypeConfig {
		t.Errorf("InvalidConfiguration() Type = %v, want %v", err.Type, ErrorTypeConfig)
	}
	if !strings.Contains(err.Message, "vault.address") {
		t.Errorf("InvalidConfiguration() Message = %q, want it to contain 'vault.address'", err.Message)
	}
	if !strings.Contains(err.Message, "must be a valid URL") {
		t.Errorf("InvalidConfiguration() Message = %q, want it to contain 'must be a valid URL'", err.Message)
	}
	if !strings.Contains(err.Hint, "githelper doctor --credentials") {
		t.Errorf("InvalidConfiguration() Hint = %q, want it to contain 'githelper doctor --credentials'", err.Hint)
	}
}
