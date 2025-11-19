# GitHelper: Replace gh CLI with Direct GitHub API Integration

## Overview

Replace the external `gh` CLI dependency with direct GitHub API integration using the official Go SDK. This eliminates external tool dependencies, improves error handling, enables better pre-flight checks, and provides a foundation for multi-platform support (GitLab, Bitbucket, etc.).

## Problem Statement

**Current Approach** (from FRICTION_ANALYSIS_AND_IMPROVEMENTS_PLAN_v2.md):
- Wraps `gh` CLI using `exec.CommandContext`
- Requires `gh` CLI to be installed and authenticated
- Pre-flight checks verify `gh` existence but don't validate API access
- Error messages come from parsing CLI output
- Hard-coded to GitHub only

**Issues**:
1. External dependency (`gh` must be installed)
2. Authentication tied to `gh auth login` workflow
3. Difficult to programmatically validate API access
4. CLI output parsing is brittle
5. Cannot easily extend to GitLab/Bitbucket/other platforms
6. Subprocess overhead on every API call

## Solution

Use the official **github.com/google/go-github** Go SDK for direct API integration.

## Goals

1. ✅ **Eliminate `gh` CLI dependency** - Use native Go SDK
2. ✅ **Better authentication** - Support multiple token sources (env var, gh config, gitconfig)
3. ✅ **Programmatic validation** - Check API access, permissions, branch protection before operations
4. ✅ **Better error handling** - Structured errors with HTTP status codes
5. ✅ **Foundation for multi-platform** - Abstract interface that can support GitLab/Bitbucket SDKs
6. ✅ **Backward compatibility** - Still read token from `gh` config if available

## Architecture

### New Package Structure

```
internal/
├── git/           # Existing git operations
├── remote/        # NEW: Remote platform abstraction
│   ├── interface.go      # Platform-agnostic interface
│   ├── github/
│   │   ├── client.go     # GitHub API client
│   │   ├── auth.go       # Token resolution
│   │   └── operations.go # Branch, protection, etc.
│   ├── gitlab/           # Future: GitLab support
│   └── factory.go        # Auto-detect platform from remote URL
└── scenarios/     # Existing scenario classification
```

### Interface Design

```go
// internal/remote/interface.go
package remote

type Platform interface {
    // Branch operations
    SetDefaultBranch(branch string) error
    GetDefaultBranch() (string, error)

    // Protection checks
    IsBranchProtected(branch string) (bool, error)
    GetBranchProtection(branch string) (*ProtectionRules, error)

    // Permission checks
    CanPush() (bool, error)
    CanAdmin() (bool, error)

    // Repository info
    GetOwner() string
    GetRepo() string
    GetPlatform() string // "github", "gitlab", "bitbucket"
}

type ProtectionRules struct {
    Enabled              bool
    RequireReviews       bool
    RequireStatusChecks  bool
    EnforceAdmins        bool
    AllowForcePush       bool
}
```

## Implementation

### Phase 1: GitHub Client (Core)

**Estimated Effort**: 6-8 hours

**File**: `internal/remote/github/client.go`

```go
package github

import (
    "context"
    "fmt"
    "net/url"
    "strings"
    "time"

    "github.com/google/go-github/v58/github"
    "golang.org/x/oauth2"
)

type Client struct {
    client *github.Client
    owner  string
    repo   string
    ctx    context.Context
}

// NewClient creates a GitHub client from a remote URL
// Supports: https://github.com/owner/repo.git, git@github.com:owner/repo.git
func NewClient(remoteURL string) (*Client, error) {
    owner, repo, err := parseGitHubURL(remoteURL)
    if err != nil {
        return nil, fmt.Errorf("invalid GitHub URL: %w", err)
    }

    token, err := getGitHubToken()
    if err != nil {
        return nil, fmt.Errorf("GitHub authentication required: %w", err)
    }

    ctx := context.Background()
    ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
    tc := oauth2.NewClient(ctx, ts)

    return &Client{
        client: github.NewClient(tc),
        owner:  owner,
        repo:   repo,
        ctx:    ctx,
    }, nil
}

// NewClientWithTimeout creates a client with custom timeout
func NewClientWithTimeout(remoteURL string, timeout time.Duration) (*Client, error) {
    client, err := NewClient(remoteURL)
    if err != nil {
        return nil, err
    }

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    // Note: caller should defer cancel()
    client.ctx = ctx

    return client, nil
}

// parseGitHubURL extracts owner and repo from various GitHub URL formats
func parseGitHubURL(remoteURL string) (owner, repo string, err error) {
    // Handle SSH URLs: git@github.com:owner/repo.git
    if strings.HasPrefix(remoteURL, "git@github.com:") {
        parts := strings.TrimPrefix(remoteURL, "git@github.com:")
        parts = strings.TrimSuffix(parts, ".git")

        split := strings.Split(parts, "/")
        if len(split) != 2 {
            return "", "", fmt.Errorf("invalid SSH URL format")
        }
        return split[0], split[1], nil
    }

    // Handle HTTPS URLs: https://github.com/owner/repo.git
    u, err := url.Parse(remoteURL)
    if err != nil {
        return "", "", err
    }

    if u.Host != "github.com" {
        return "", "", fmt.Errorf("not a GitHub URL: %s", u.Host)
    }

    path := strings.TrimPrefix(u.Path, "/")
    path = strings.TrimSuffix(path, ".git")

    parts := strings.Split(path, "/")
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid GitHub path: %s", path)
    }

    return parts[0], parts[1], nil
}

func (c *Client) GetOwner() string {
    return c.owner
}

func (c *Client) GetRepo() string {
    return c.repo
}

func (c *Client) GetPlatform() string {
    return "github"
}
```

**Tests**: `internal/remote/github/client_test.go`

```go
func TestParseGitHubURL(t *testing.T) {
    tests := []struct {
        url   string
        owner string
        repo  string
        err   bool
    }{
        {"https://github.com/lcgerke/gitDualRemote.git", "lcgerke", "gitDualRemote", false},
        {"git@github.com:lcgerke/gitDualRemote.git", "lcgerke", "gitDualRemote", false},
        {"https://github.com/lcgerke/gitDualRemote", "lcgerke", "gitDualRemote", false},
        {"https://gitlab.com/owner/repo.git", "", "", true}, // Not GitHub
        {"invalid", "", "", true},
    }

    for _, tt := range tests {
        owner, repo, err := parseGitHubURL(tt.url)
        if tt.err && err == nil {
            t.Errorf("Expected error for %s", tt.url)
        }
        if !tt.err && err != nil {
            t.Errorf("Unexpected error for %s: %v", tt.url, err)
        }
        if owner != tt.owner || repo != tt.repo {
            t.Errorf("Got %s/%s, want %s/%s", owner, repo, tt.owner, tt.repo)
        }
    }
}
```

---

### Phase 2: Authentication (Token Resolution)

**Estimated Effort**: 4-5 hours

**File**: `internal/remote/github/auth.go`

```go
package github

import (
    "fmt"
    "os"
    "path/filepath"

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
// Priority: GITHUB_TOKEN env var > gh CLI config > git config
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
        return fmt.Errorf("token validation failed: %w\n\n" +
            "Your token may be expired or lack required permissions.\n" +
            "Required scopes: repo (full control of private repositories)")
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

type RepositoryPermissions struct {
    Pull  bool
    Push  bool
    Admin bool
}
```

**Tests**: Test token resolution priority and error messages

---

### Phase 3: Repository Operations

**Estimated Effort**: 5-6 hours

**File**: `internal/remote/github/operations.go`

```go
package github

import (
    "fmt"
    "net/http"

    "github.com/google/go-github/v58/github"
)

// SetDefaultBranch updates the repository's default branch
func (c *Client) SetDefaultBranch(branch string) error {
    _, _, err := c.client.Repositories.Edit(c.ctx, c.owner, c.repo, &github.Repository{
        DefaultBranch: github.String(branch),
    })

    if err != nil {
        return fmt.Errorf("failed to set default branch: %w", err)
    }

    return nil
}

// GetDefaultBranch returns the current default branch
func (c *Client) GetDefaultBranch() (string, error) {
    repo, _, err := c.client.Repositories.Get(c.ctx, c.owner, c.repo)
    if err != nil {
        return "", fmt.Errorf("failed to get repository: %w", err)
    }

    return repo.GetDefaultBranch(), nil
}

// IsBranchProtected checks if a branch has protection rules
func (c *Client) IsBranchProtected(branch string) (bool, error) {
    _, resp, err := c.client.Repositories.GetBranchProtection(c.ctx, c.owner, c.repo, branch)

    if err != nil {
        // 404 means no protection
        if resp != nil && resp.StatusCode == http.StatusNotFound {
            return false, nil
        }
        return false, fmt.Errorf("failed to check branch protection: %w", err)
    }

    return true, nil
}

// GetBranchProtection returns detailed protection rules
func (c *Client) GetBranchProtection(branch string) (*ProtectionRules, error) {
    protection, resp, err := c.client.Repositories.GetBranchProtection(c.ctx, c.owner, c.repo, branch)

    if err != nil {
        if resp != nil && resp.StatusCode == http.StatusNotFound {
            return &ProtectionRules{Enabled: false}, nil
        }
        return nil, fmt.Errorf("failed to get branch protection: %w", err)
    }

    return &ProtectionRules{
        Enabled:             true,
        RequireReviews:      protection.GetRequiredPullRequestReviews() != nil,
        RequireStatusChecks: protection.GetRequiredStatusChecks() != nil,
        EnforceAdmins:       protection.GetEnforceAdmins().Enabled,
        AllowForcePush:      protection.GetAllowForcePushes().Enabled,
    }, nil
}

// CanPush checks if authenticated user can push to repository
func (c *Client) CanPush() (bool, error) {
    perms, err := c.CheckPermissions()
    if err != nil {
        return false, err
    }
    return perms.Push, nil
}

// CanAdmin checks if authenticated user has admin access
func (c *Client) CanAdmin() (bool, error) {
    perms, err := c.CheckPermissions()
    if err != nil {
        return false, err
    }
    return perms.Admin, nil
}

// BranchExists checks if a branch exists on GitHub
func (c *Client) BranchExists(branch string) (bool, error) {
    _, resp, err := c.client.Repositories.GetBranch(c.ctx, c.owner, c.repo, branch, false)

    if err != nil {
        if resp != nil && resp.StatusCode == http.StatusNotFound {
            return false, nil
        }
        return false, fmt.Errorf("failed to check branch: %w", err)
    }

    return true, nil
}

type ProtectionRules struct {
    Enabled             bool
    RequireReviews      bool
    RequireStatusChecks bool
    EnforceAdmins       bool
    AllowForcePush      bool
}
```

---

### Phase 4: Platform Factory (Auto-Detection)

**Estimated Effort**: 3-4 hours

**File**: `internal/remote/factory.go`

```go
package remote

import (
    "fmt"
    "strings"

    "github.com/lcgerke/githelper/internal/remote/github"
)

// NewClient creates appropriate platform client based on remote URL
func NewClient(remoteURL string) (Platform, error) {
    platform := detectPlatform(remoteURL)

    switch platform {
    case "github":
        return github.NewClient(remoteURL)
    case "gitlab":
        return nil, fmt.Errorf("GitLab support not yet implemented")
    case "bitbucket":
        return nil, fmt.Errorf("Bitbucket support not yet implemented")
    default:
        return nil, fmt.Errorf("unsupported platform: %s", platform)
    }
}

// detectPlatform identifies the platform from remote URL
func detectPlatform(remoteURL string) string {
    switch {
    case strings.Contains(remoteURL, "github.com"):
        return "github"
    case strings.Contains(remoteURL, "gitlab.com"):
        return "gitlab"
    case strings.Contains(remoteURL, "bitbucket.org"):
        return "bitbucket"
    default:
        return "unknown"
    }
}

// IsPlatformSupported checks if a remote URL points to a supported platform
func IsPlatformSupported(remoteURL string) bool {
    platform := detectPlatform(remoteURL)
    return platform == "github" // Expand as more platforms are added
}
```

---

### Phase 5: Update migrate-to-main Command

**Estimated Effort**: 4-5 hours

**File**: `cmd/githelper/migrate.go` (refactored)

```go
func preflightChecksMigrate(gitClient *git.Client) error {
    // Get github remote URL
    remoteURL, err := gitClient.GetRemoteURL("github")
    if err != nil {
        return fmt.Errorf("no 'github' remote found: %w", err)
    }

    // Check if platform is supported
    if !remote.IsPlatformSupported(remoteURL) {
        return fmt.Errorf("remote '%s' is not a supported platform\n" +
            "Currently supported: GitHub\n" +
            "Coming soon: GitLab, Bitbucket", remoteURL)
    }

    // Create GitHub client (validates authentication)
    ghClient, err := remote.NewClient(remoteURL)
    if err != nil {
        return err
    }

    // Validate token and permissions
    if validator, ok := ghClient.(interface{ ValidateToken() error }); ok {
        if err := validator.ValidateToken(); err != nil {
            return err
        }
    }

    // Check admin permissions (required to change default branch)
    canAdmin, err := ghClient.CanAdmin()
    if err != nil {
        return fmt.Errorf("failed to check permissions: %w", err)
    }
    if !canAdmin {
        return fmt.Errorf("admin access required to change default branch\n" +
            "You have: %s\n" +
            "Required: admin", getPermissionLevel(ghClient))
    }

    // Check if master branch is protected
    protected, err := ghClient.IsBranchProtected("master")
    if err != nil {
        return fmt.Errorf("failed to check branch protection: %w", err)
    }
    if protected {
        rules, _ := ghClient.GetBranchProtection("master")
        return fmt.Errorf("'master' branch is protected on GitHub\n" +
            "Protection rules:\n" +
            "  - Enforce admins: %v\n" +
            "  - Require reviews: %v\n" +
            "  - Allow force push: %v\n\n" +
            "Temporarily disable protection or use --force to override",
            rules.EnforceAdmins, rules.RequireReviews, rules.AllowForcePush)
    }

    // Check push access to remotes
    remotes, _ := gitClient.ListRemotes()
    for _, remoteName := range remotes {
        if err := gitClient.CheckPushAccess(remoteName); err != nil {
            return fmt.Errorf("cannot push to remote '%s': %w", remoteName, err)
        }
    }

    return nil
}

func setGitHubDefaultBranch(gitClient *git.Client, branch string) error {
    remoteURL, err := gitClient.GetRemoteURL("github")
    if err != nil {
        return err
    }

    ghClient, err := remote.NewClient(remoteURL)
    if err != nil {
        return err
    }

    return ghClient.SetDefaultBranch(branch)
}

func getPermissionLevel(client remote.Platform) string {
    if canAdmin, _ := client.CanAdmin(); canAdmin {
        return "admin"
    }
    if canPush, _ := client.CanPush(); canPush {
        return "push"
    }
    return "pull"
}
```

---

### Phase 6: Add Diagnostic Command

**Estimated Effort**: 2-3 hours

**File**: `cmd/githelper/auth.go` (new)

```go
package main

import (
    "fmt"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/lcgerke/githelper/internal/remote"
    "github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
    Use:   "auth",
    Short: "Check GitHub authentication status",
    Long:  `Diagnose GitHub API authentication and permissions.`,
    RunE:  runAuth,
}

func runAuth(cmd *cobra.Command, args []string) error {
    gitClient := git.NewClient(".")

    if !gitClient.IsRepository() {
        return fmt.Errorf("not a git repository")
    }

    // Get github remote
    remoteURL, err := gitClient.GetRemoteURL("github")
    if err != nil {
        fmt.Println("❌ No 'github' remote found")
        fmt.Println()
        fmt.Println("This repository does not have a GitHub remote.")
        fmt.Println("Add one with: git remote add github <url>")
        return nil
    }

    fmt.Printf("GitHub Remote: %s\n", remoteURL)
    fmt.Println()

    // Create client (tests token resolution)
    fmt.Println("🔍 Checking authentication...")
    ghClient, err := remote.NewClient(remoteURL)
    if err != nil {
        fmt.Printf("❌ Authentication failed: %v\n", err)
        return nil
    }

    // Show token source
    if gh, ok := ghClient.(interface{ GetTokenSource() string }); ok {
        fmt.Printf("✓ Token found: %s\n", gh.GetTokenSource())
    }

    // Validate token
    fmt.Println()
    fmt.Println("🔍 Validating token...")
    if validator, ok := ghClient.(interface{ ValidateToken() error }); ok {
        if err := validator.ValidateToken(); err != nil {
            fmt.Printf("❌ Token validation failed: %v\n", err)
            return nil
        }
    }
    fmt.Println("✓ Token is valid")

    // Check permissions
    fmt.Println()
    fmt.Println("🔍 Checking repository permissions...")

    canPull, _ := ghClient.CanPush()
    canPush, _ := ghClient.CanPush()
    canAdmin, _ := ghClient.CanAdmin()

    fmt.Printf("  Pull:  %v\n", canPull)
    fmt.Printf("  Push:  %v\n", canPush)
    fmt.Printf("  Admin: %v\n", canAdmin)

    // Check default branch
    fmt.Println()
    fmt.Println("🔍 Repository information...")
    defaultBranch, err := ghClient.GetDefaultBranch()
    if err == nil {
        fmt.Printf("  Default branch: %s\n", defaultBranch)

        // Check if protected
        protected, _ := ghClient.IsBranchProtected(defaultBranch)
        if protected {
            fmt.Printf("  Protection: ✓ Enabled\n")

            if rules, err := ghClient.GetBranchProtection(defaultBranch); err == nil {
                fmt.Printf("    - Require reviews: %v\n", rules.RequireReviews)
                fmt.Printf("    - Require status checks: %v\n", rules.RequireStatusChecks)
                fmt.Printf("    - Enforce admins: %v\n", rules.EnforceAdmins)
            }
        } else {
            fmt.Printf("  Protection: ✗ Disabled\n")
        }
    }

    fmt.Println()
    fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    fmt.Println("✅ GitHub authentication is working correctly")

    return nil
}
```

**User Experience**:
```bash
$ githelper auth
GitHub Remote: git@github.com:lcgerke/gitDualRemote.git

🔍 Checking authentication...
✓ Token found: ~/.config/gh/hosts.yml

🔍 Validating token...
✓ Token is valid

🔍 Checking repository permissions...
  Pull:  true
  Push:  true
  Admin: true

🔍 Repository information...
  Default branch: main
  Protection: ✓ Enabled
    - Require reviews: false
    - Require status checks: true
    - Enforce admins: false

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ GitHub authentication is working correctly
```

---

## Dependencies

**Add to `go.mod`**:
```
require (
    github.com/google/go-github/v58 v58.0.0
    golang.org/x/oauth2 v0.15.0
    gopkg.in/yaml.v3 v3.0.1
)
```

**Install**:
```bash
go get github.com/google/go-github/v58/github
go get golang.org/x/oauth2
go get gopkg.in/yaml.v3
```

---

## Testing Strategy

### Unit Tests
- ✅ URL parsing (SSH, HTTPS, edge cases)
- ✅ Token resolution priority
- ✅ Platform detection
- ✅ Error message formatting

### Integration Tests
- ✅ Real GitHub API calls (using test token)
- ✅ Permission validation
- ✅ Branch protection queries
- ✅ Default branch updates

### Mock Tests
- ✅ Mock GitHub client for testing without API calls
- ✅ Test error scenarios (401, 403, 404)
- ✅ Test rate limiting handling

**Example Integration Test**:
```go
func TestGitHubClient_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    token := os.Getenv("GITHUB_TEST_TOKEN")
    if token == "" {
        t.Skip("GITHUB_TEST_TOKEN not set")
    }

    // Use a real test repository
    client, err := github.NewClient("https://github.com/lcgerke/test-repo.git")
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }

    // Test getting default branch
    branch, err := client.GetDefaultBranch()
    if err != nil {
        t.Fatalf("Failed to get default branch: %v", err)
    }

    if branch == "" {
        t.Error("Default branch is empty")
    }
}
```

---

## Migration Path

### Backward Compatibility

**Phase 1**: Add new API client alongside existing `gh` CLI wrapper
- Both methods available
- Use feature flag to toggle: `--use-api` vs `--use-gh-cli`

**Phase 2**: Default to API client, keep `gh` CLI as fallback
- If API fails (no token), fall back to `gh` CLI
- Log which method is being used

**Phase 3**: Remove `gh` CLI dependency entirely
- Remove fallback code
- Update documentation

**Configuration**:
```yaml
# ~/.githelper/config.yaml
github:
  auth_method: auto  # auto, api, gh-cli
  token_source: auto # env, gh-config, git-config
```

---

## Error Messages

**Before** (wrapping gh CLI):
```
❌ Migration failed
Error: exit status 1
gh: command not found
```

**After** (direct API):
```
❌ GitHub authentication required

No GitHub token found.

Please authenticate using one of:
  1. Set GITHUB_TOKEN environment variable
  2. Run: gh auth login (token will be read from gh config)
  3. Run: git config --global github.token YOUR_TOKEN

To create a token:
  https://github.com/settings/tokens/new
  Required scopes: repo (full control of private repositories)
```

---

## Benefits Summary

| Aspect | gh CLI Wrapper | Direct API | Improvement |
|--------|---------------|------------|-------------|
| External deps | Requires `gh` CLI | Go SDK only | ✅ Simpler install |
| Authentication | `gh auth login` only | Multiple sources | ✅ More flexible |
| Error messages | Parse CLI output | Structured errors | ✅ Better UX |
| Pre-flight checks | Limited | Full API access | ✅ Fail fast |
| Performance | Subprocess overhead | Native HTTP | ✅ Faster |
| Multi-platform | GitHub only | Extensible | ✅ Future-proof |
| Testing | Mock CLI hard | Mock HTTP easy | ✅ Better tests |

---

## Implementation Timeline

**Week 1** (13-15 hours):
- Phase 1: GitHub client core (6-8 hrs)
- Phase 2: Authentication (4-5 hrs)
- Phase 3: Repository operations (3-4 hrs)

**Week 2** (9-12 hours):
- Phase 4: Platform factory (3-4 hrs)
- Phase 5: Update migrate command (4-5 hrs)
- Phase 6: Auth diagnostic command (2-3 hrs)

**Week 3** (6-8 hours):
- Integration testing with real GitHub API
- Documentation updates
- Backward compatibility testing
- Remove `gh` CLI dependency

**Total Effort**: 28-35 hours

---

## Future Enhancements

### GitLab Support
```go
// internal/remote/gitlab/client.go
type Client struct {
    client *gitlab.Client
    project string
}

func (c *Client) SetDefaultBranch(branch string) error {
    // Use gitlab.com/gitlab-org/api/client-go
    return c.client.Projects.EditProject(c.project, &gitlab.EditProjectOptions{
        DefaultBranch: &branch,
    })
}
```

### Bitbucket Support
```go
// internal/remote/bitbucket/client.go
type Client struct {
    client *bitbucket.Client
    workspace string
    repo string
}
```

### Rate Limiting
```go
// Check rate limit before operations
func (c *Client) CheckRateLimit() error {
    rate, _, err := c.client.RateLimits(c.ctx)
    if err != nil {
        return err
    }

    if rate.Core.Remaining < 10 {
        return fmt.Errorf("GitHub API rate limit low: %d/%d remaining",
            rate.Core.Remaining, rate.Core.Limit)
    }

    return nil
}
```

---

## Success Criteria

- ✅ Zero dependency on `gh` CLI
- ✅ Works with GITHUB_TOKEN, gh config, and git config
- ✅ Pre-flight checks catch all common failures
- ✅ Clear error messages with recovery steps
- ✅ Comprehensive test coverage (unit + integration)
- ✅ 100% backward compatible (users don't notice change)
- ✅ Foundation for GitLab/Bitbucket support
- ✅ Performance improvement (no subprocess overhead)

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking users who rely on `gh` auth | Read token from gh config, maintain compatibility |
| GitHub API changes | Use official SDK (handles versioning) |
| Rate limiting | Check limits before operations, use conditional requests |
| Missing permissions | Clear pre-flight checks with actionable errors |
| Token expiration | Validate token before operations, guide to re-auth |

---

## Rollback Plan

If issues arise:
1. Revert to `gh` CLI wrapper (keep code for 1-2 releases)
2. Add feature flag to toggle between methods
3. Collect feedback and iterate
4. Fix issues in API client
5. Re-attempt migration

**Feature Flag**:
```bash
# Force use of gh CLI
githelper migrate-to-main --use-gh-cli

# Force use of API (default)
githelper migrate-to-main --use-api
```
