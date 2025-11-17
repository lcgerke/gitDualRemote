package errors

import (
	"fmt"
)

// Error types for better error handling
type ErrorType string

const (
	ErrorTypeVault       ErrorType = "vault"
	ErrorTypeGit         ErrorType = "git"
	ErrorTypeConfig      ErrorType = "config"
	ErrorTypeState       ErrorType = "state"
	ErrorTypeGitHub      ErrorType = "github"
	ErrorTypeNetwork     ErrorType = "network"
	ErrorTypeFileSystem  ErrorType = "filesystem"
	ErrorTypeValidation  ErrorType = "validation"
)

// GitHelperError represents a structured error with context
type GitHelperError struct {
	Type    ErrorType
	Message string
	Hint    string
	Err     error
}

func (e *GitHelperError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *GitHelperError) Unwrap() error {
	return e.Err
}

// UserFriendlyMessage returns a user-friendly error message with hint
func (e *GitHelperError) UserFriendlyMessage() string {
	msg := e.Message
	if e.Hint != "" {
		msg += "\n\nSuggestion: " + e.Hint
	}
	return msg
}

// New creates a new GitHelperError
func New(errType ErrorType, message string) *GitHelperError {
	return &GitHelperError{
		Type:    errType,
		Message: message,
	}
}

// Wrap wraps an existing error with context
func Wrap(errType ErrorType, message string, err error) *GitHelperError {
	return &GitHelperError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// WithHint adds a hint to an error
func WithHint(err *GitHelperError, hint string) *GitHelperError {
	err.Hint = hint
	return err
}

// Common error constructors

func VaultUnreachable(addr string, err error) *GitHelperError {
	return WithHint(
		Wrap(ErrorTypeVault, fmt.Sprintf("Vault unreachable at %s", addr), err),
		"Check that Vault is running and the address is correct. Run 'githelper doctor' for diagnostics.",
	)
}

func GitNotInstalled(err error) *GitHelperError {
	return WithHint(
		Wrap(ErrorTypeGit, "Git is not installed or not in PATH", err),
		"Install git using your package manager (apt, yum, brew, etc.)",
	)
}

func RepositoryNotFound(name string) *GitHelperError {
	return WithHint(
		New(ErrorTypeState, fmt.Sprintf("Repository '%s' not found in state", name)),
		fmt.Sprintf("Run 'githelper repo list' to see configured repositories or 'githelper repo create %s' to create it.", name),
	)
}

func GitHubAuthFailed(err error) *GitHelperError {
	return WithHint(
		Wrap(ErrorTypeGitHub, "GitHub authentication failed", err),
		"Check that your GitHub PAT is valid and has the required scopes (repo). Update in Vault at secret/githelper/github/default_pat",
	)
}

func SSHKeyNotFound(repoName string) *GitHelperError {
	hint := "Check that the SSH key exists in Vault"
	if repoName != "default" {
		hint = fmt.Sprintf("No SSH key found for repository '%s'. Using default key. To set a repo-specific key, add it to Vault at secret/githelper/github/%s/ssh", repoName, repoName)
	}
	return WithHint(
		New(ErrorTypeVault, fmt.Sprintf("SSH key not found for '%s'", repoName)),
		hint,
	)
}

func RemoteNotConfigured(remoteName string) *GitHelperError {
	return WithHint(
		New(ErrorTypeGit, fmt.Sprintf("Remote '%s' not configured", remoteName)),
		fmt.Sprintf("Run 'git remote add %s <url>' or 'githelper github setup' to configure GitHub integration.", remoteName),
	)
}

func DualPushNotConfigured() *GitHelperError {
	return WithHint(
		New(ErrorTypeGit, "Dual-push not configured"),
		"Run 'githelper github setup <repo-name>' to configure dual-push for GitHub integration.",
	)
}

func SyncRequired(repoName string, commitsAhead int) *GitHelperError {
	return WithHint(
		New(ErrorTypeGitHub, fmt.Sprintf("GitHub is %d commit(s) behind", commitsAhead)),
		fmt.Sprintf("Run 'githelper github sync %s' to sync missing commits to GitHub.", repoName),
	)
}

func DivergenceDetected(bareAhead, githubAhead int) *GitHelperError {
	return WithHint(
		New(ErrorTypeGit, fmt.Sprintf("Repositories have diverged: bare has %d unique commits, GitHub has %d unique commits", bareAhead, githubAhead)),
		"Manual resolution required. Fetch both remotes and merge or rebase as appropriate.",
	)
}

func StateCorrupted(err error) *GitHelperError {
	return WithHint(
		Wrap(ErrorTypeState, "State file is corrupted or invalid", err),
		"Backup and delete ~/.githelper/state.yaml, then run 'githelper doctor' to rebuild state.",
	)
}

func NetworkError(operation string, err error) *GitHelperError {
	return WithHint(
		Wrap(ErrorTypeNetwork, fmt.Sprintf("Network error during %s", operation), err),
		"Check your internet connection. If the problem persists, the remote server may be down.",
	)
}

func InvalidConfiguration(key, reason string) *GitHelperError {
	return WithHint(
		New(ErrorTypeConfig, fmt.Sprintf("Invalid configuration for '%s': %s", key, reason)),
		"Run 'githelper doctor --credentials' to check your configuration.",
	)
}
