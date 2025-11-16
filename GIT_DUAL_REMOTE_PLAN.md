# Git Dual-Remote Setup Tool - Implementation Plan

## Overview

A bulletproof Go CLI tool that configures and manages dual git remotes: a self-hosted bare repository (primary) and GitHub (backup/collaboration). The tool automates the entire setup process with beautiful output, comprehensive error handling, and informative help.

## Relationship to GitSetup Tool

### Architectural Decision: Standalone Tool (Not Integrated with GitSetup)

**TL;DR**: `git-dual-remote` will be implemented as a **separate, standalone tool** that complements (but does not integrate with) the existing `gitsetup` utility.

### Context: What is GitSetup?

GitSetup is an existing enterprise-grade Go CLI tool (~19,600 LOC) located at `/home/lcgerke/wk/31utilityScripts/gitSetupGo/`. It manages the complete lifecycle of Git repositories on a self-hosted bare repository server:

**GitSetup Capabilities:**
- Creates/converts/clones repositories on bare server (`gitmanager@lcgasgit:/srv/git/`)
- Manages repository lifecycle (create, archive, delete, list)
- Provides operation tracking with undo/rollback
- Offers project templates (Python, Go, Ansible, Docs)
- Handles SSH-based authentication to bare repo server
- Maintains state in `~/.gitsetup/state.yaml`

**GitSetup Architecture:**
- **Single remote model**: Only manages `origin` pointing to bare repository
- **No GitHub integration**: Zero GitHub API or `gh` CLI usage
- **No Vault integration**: Uses existing SSH keys from `~/.ssh/`
- **No multi-remote support**: Architecture assumes one canonical remote

### Why Keep Separate?

#### 1. Fundamentally Different Problem Domains

| Aspect | GitSetup | git-dual-remote |
|--------|----------|-----------------|
| **Primary Goal** | Repository lifecycle management | Remote synchronization strategy |
| **Scope** | Create, archive, delete, track repos | Configure dual-push to GitHub |
| **Remote Model** | Single remote (bare repo) | Dual remotes (bare + GitHub) |
| **Dependencies** | SSH to bare server | Vault + GitHub + gh CLI |
| **State** | Tracks managed repositories | Configures existing repos |
| **User Flow** | `gitsetup create` → new repo | `git-dual-remote setup` → add GitHub |

#### 2. Architectural Incompatibility

**GitSetup's Single-Remote Assumption:**
```go
// From gitsetup codebase - hardcoded single remote
func (r *Repository) AddRemote(name string, url string) error {
    // Always adds exactly one remote named "origin"
    return r.git.CreateRemote(&config.RemoteConfig{
        Name: "origin",  // Hardcoded
        URLs: []string{url},  // Single URL
    })
}
```

**git-dual-remote Requires:**
- Multiple push URLs for same remote (`origin` → both bare + GitHub)
- Explicit second remote (`github`)
- Dual-push configuration via `git remote set-url --add --push`

Retrofitting this into GitSetup would require:
- Rewriting core remote management logic
- Changing 19k+ lines of assumptions
- Breaking existing users' workflows
- Doubling testing surface area

#### 3. Dependency Explosion

**Current GitSetup Dependencies:**
- Go standard library
- SSH client library
- Git operations library
- YAML parsing

**git-dual-remote Adds:**
- HashiCorp Vault API client
- GitHub API client (`go-github`)
- `gh` CLI wrapper
- SSH config parser (`ssh_config`)

Adding these to GitSetup would:
- Increase binary size significantly
- Add security attack surface
- Create version conflicts
- Complicate builds and releases

#### 4. Feature Scope Mismatch

**GitSetup Users Expect:**
- Team repository hosting workflows
- Bare repository server management
- Unix group-based permissions
- Operation audit trails

**git-dual-remote Users Expect:**
- Personal backup workflows
- GitHub integration
- Vault-based key management
- Dual-push convenience

These are **orthogonal concerns** serving different use cases.

#### 5. Maintenance and Testing Complexity

**As Separate Tools:**
- Each tool has focused test suite
- Independent versioning and releases
- Clear separation of concerns
- Single Responsibility Principle

**If Integrated:**
- Combinatorial testing (GitSetup features × dual-remote features)
- Feature flag complexity
- Risk of regression when changing either feature
- Unclear ownership of bugs

### Recommended Complementary Workflow

The tools work **sequentially** in a pipeline:

```bash
# Step 1: Use GitSetup to create/manage repository
gitsetup create myproject --type go --mode complete
cd ~/repos/myproject

# Step 2: Use git-dual-remote to add GitHub backup
git-dual-remote setup --github-user lcgerke

# Result: Repository lifecycle managed by GitSetup,
#         GitHub backup configured by git-dual-remote
```

**Key Insight:** Users who want dual remotes can use **both tools**. Users who only want bare repo management can use **just GitSetup**. This provides maximum flexibility.

### Technical Comparison

#### Remote Configuration After Each Tool

**After `gitsetup create myproject`:**
```bash
$ git remote -v
origin  lcgasgit:/srv/git/myproject.git (fetch)
origin  lcgasgit:/srv/git/myproject.git (push)
```

**After `git-dual-remote setup --github-user lcgerke`:**
```bash
$ git remote -v
origin  lcgasgit:/srv/git/myproject.git (fetch)
origin  lcgasgit:/srv/git/myproject.git (push)
origin  git@github.com:lcgerke/myproject.git (push)  # ADDED
github  git@github.com:lcgerke/myproject.git (fetch) # ADDED
github  git@github.com:lcgerke/myproject.git (push)  # ADDED
```

Notice: `git-dual-remote` **extends** the configuration created by GitSetup without modifying GitSetup's state or behavior.

### Potential Future Integration (Low Priority)

If demand arises, integration could happen via:

1. **GitSetup Plugin System** (requires architecture changes)
   ```bash
   gitsetup plugin install dual-remote
   gitsetup create myproject --with-plugin dual-remote
   ```

2. **Post-Create Hooks**
   ```yaml
   # ~/.gitsetup/config.yaml
   hooks:
     post_create: "/usr/local/bin/git-dual-remote setup --github-user lcgerke"
   ```

3. **Shared Configuration**
   ```yaml
   # ~/.config/git-tools/config.yaml (shared by both tools)
   bare_repo:
     host: lcgasgit
     path: /srv/git/{repo}.git
   github:
     username: lcgerke
   ```

4. **Monorepo with Subcommands**
   ```bash
   git-tools repo create myproject      # GitSetup functionality
   git-tools remote add-github myproject # git-dual-remote functionality
   ```

**Conclusion:** These are possible but **not necessary**. Standalone tools provide sufficient value with lower complexity.

### Decision Summary

**FINAL VERDICT: New unified tool called `githelper` in this repository**

**User decision:** "Change the name to githelper and put it in this repo"

**Rationale:**
- ✅ Single binary to install and manage
- ✅ Clean naming (`githelper` encompasses all Git management)
- ✅ Unified command structure (`githelper repo ...`, `githelper github ...`)
- ✅ Shared state (tracks both bare repos and GitHub)
- ✅ No "tool divorce" - integrated state management
- ✅ Code reuse (SSH, Git, Vault clients)
- ✅ Fresh start without legacy constraints

**Location:** `/home/lcgerke/gitDualRemote/` (this repository, rename from gitDualRemote to githelper)

**Distribution:** Single `githelper` binary

**Architecture:** Unified Git management tool with multiple subcommands

### Tool Architecture - githelper

**New unified tool built from scratch in this repository.**

**Command structure:**

```bash
# Repository management (inspired by gitsetup)
githelper repo create myproject --type go
githelper repo list
githelper repo status myproject
githelper repo archive myproject
githelper repo delete myproject

# GitHub integration (new dual-remote functionality)
githelper github setup myproject --user lcgerke
githelper github status myproject
githelper github sync myproject
githelper github test myproject

# Integrated workflow
githelper repo create myproject --type go --with-github lcgerke
# Creates bare repo + local clone + GitHub dual-push in one command
```

**File structure:**

```
githelper/                              # This repository (renamed from gitDualRemote)
├── cmd/
│   └── githelper/
│       ├── main.go                     # Entry point
│       ├── repo.go                     # Repo subcommand root
│       ├── repo_create.go              # Create command
│       ├── repo_list.go                # List command
│       ├── repo_status.go              # Status command
│       ├── repo_archive.go             # Archive command
│       ├── github.go                   # GitHub subcommand root
│       ├── github_setup.go             # Setup dual-remote
│       ├── github_status.go            # GitHub sync status
│       ├── github_sync.go              # Manual sync
│       └── github_test.go              # Test connectivity
├── internal/
│   ├── config/
│   │   ├── config.go                   # Vault-backed config
│   │   └── cache.go                    # Config cache (--use-cache)
│   ├── vault/
│   │   ├── client.go                   # Vault integration
│   │   └── secrets.go                  # Secret retrieval
│   ├── ssh/
│   │   ├── client.go                   # SSH operations
│   │   ├── keygen.go                   # Key management
│   │   └── config.go                   # Repository-local SSH config
│   ├── git/
│   │   ├── remote.go                   # Remote configuration
│   │   ├── sync.go                     # Dual-push sync
│   │   ├── operations.go               # Basic git operations
│   │   └── url.go                      # URL parsing/validation
│   ├── github/
│   │   ├── client.go                   # GitHub API / gh CLI wrapper
│   │   ├── repo.go                     # Repository management
│   │   └── auth.go                     # PAT authentication
│   ├── ui/
│   │   ├── output.go                   # Colorized output
│   │   ├── table.go                    # Table formatting
│   │   ├── spinner.go                  # Progress indicators
│   │   └── prompt.go                   # User confirmations
│   └── state/
│       ├── state.go                    # State file management
│       └── repo.go                     # Repository state tracking
├── docs/
│   ├── PLAN.md                         # This document (renamed)
│   ├── WORKFLOW.md                     # Daily workflow guide
│   └── VAULT.md                        # Vault setup guide
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

**State file:**

`~/.githelper/state.yaml`:

```yaml
repositories:
  myproject:
    path: /home/user/repos/myproject
    remote: lcgasgit:/srv/git/myproject.git
    created: "2025-11-16T10:00:00Z"
    type: go
    github:
      enabled: true
      user: lcgerke
      repo: myproject
      sync_status: synced
      last_sync: "2025-11-16T14:30:00Z"
      needs_retry: false
      last_error: ""
```

**Benefits:**
1. ✅ **Single installation**: One binary: `githelper`
2. ✅ **Unified state**: Tracks both bare repos and GitHub in one state file
3. ✅ **No tool divorce**: When repo archived, all state cleaned up
4. ✅ **Clean design**: Fresh start, no legacy constraints
5. ✅ **Integrated workflows**: `githelper repo create --with-github` does everything
6. ✅ **Consistent UX**: Same logging, errors, help system across all commands
7. ✅ **Clear naming**: `githelper` encompasses all Git management tasks

**Implementation approach:**

- Build new tool from scratch in this repository
- Two main subcommands: `repo` and `github`
- Shared internal packages (config, vault, ssh, git, github, ui, state)
- State file at `~/.githelper/state.yaml`
- Configuration from Vault (with `--use-cache` flag)
- Repository-local SSH config (no global modifications)
- Dual-push with explicit error handling

## Strategic Decisions (from `/decide` session)

### Decision Matrix

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Primary Remote** | Bare repo (`origin`) | Fast local operations, user controls canonical version |
| **Push Behavior** | Dual push (both remotes) | Every commit backed up to GitHub automatically |
| **GitHub Auth** | SSH key from Vault | Most secure, standard workflow, no tokens in memory |
| **GitHub Repo** | Auto-create if missing | Fully automated setup via `gh repo create` |
| **Pull Behavior** | Bare repo only | Fast default pulls, explicit GitHub sync when needed |

### Resulting Workflow

```bash
# Daily work
git pull              # Fast pull from bare repo (local network)
git commit -m "work"
git push              # Pushes to BOTH bare repo AND GitHub

# When GitHub has changes (CI, collaborators, web edits)
git pull github main  # Explicit pull from GitHub
git push              # Sync merged result to both remotes
```

### Remote Configuration

```
origin (fetch) → gitmanager@lcgasgit:/srv/git/ccTrack.git
origin (push)  → gitmanager@lcgasgit:/srv/git/ccTrack.git
origin (push)  → git@github.com:username/ccTrack.git  [DUAL PUSH]
github (fetch) → git@github.com:username/ccTrack.git
github (push)  → git@github.com:username/ccTrack.git
```

## Tool Architecture

### Name: `git-dual-remote`

**Philosophy**: Declarative configuration meets automated execution with maximum safety.

### Core Responsibilities

1. **Validate Environment**
   - Check for git, gh CLI, vault CLI
   - Verify git repository exists
   - Detect current remote configuration

2. **Credential Management** (⚠️ UPDATED per critique)
   - Retrieve **two secrets** from HashiCorp Vault:
     - GitHub SSH key (for git push/pull operations)
     - GitHub PAT (for GitHub API calls via `gh` CLI)
   - Store SSH key at `~/.ssh/github_<reponame>` with 600 permissions
   - Configure repository-local SSH command (NOT global `~/.ssh/config`)
   - Set `GH_TOKEN` environment variable for `gh` CLI
   - Test both SSH connection and GitHub API authentication

3. **Remote Configuration**
   - Configure `origin` remote (bare repo)
   - Add dual push URL to `origin` (GitHub)
   - Add explicit `github` remote
   - Validate URLs use SCP-style format (colon, not `ssh://`)

4. **GitHub Repository Management**
   - Authenticate `gh` CLI with SSH key
   - Check if GitHub repo exists
   - Create private repo if missing
   - Set up branch protection (optional)

5. **Synchronization** (⚠️ EXPANDED per critique)
   - Push current state to both remotes
   - Handle partial failures (one remote succeeds, other fails)
   - Implement retry/queue mechanism for GitHub failures
   - Verify both remotes are in sync
   - Display sync status with divergence detection
   - Provide `--skip-github` flag for offline work

6. **Documentation**
   - Generate workflow guide
   - Create troubleshooting reference
   - Display next steps

### Command Structure

```
git-dual-remote [command] [flags]

Commands:
  setup       Configure dual remotes (full automated setup)
  status      Check current remote configuration and sync state
  sync        Manually sync both remotes
  test        Test connectivity to both remotes
  doctor      Diagnose configuration issues
  help        Display strategy and usage information

Flags (global):
  --use-cache            Use cached Vault config if Vault unreachable (explicit opt-in)
  --repo-name string     Repository name (default: current directory name)
  --github-user string   GitHub username (required for setup)
  --vault-path string    Vault path for SSH key (default: secret/github/<repo>)
  --bare-remote string   Bare repo remote URL (default: gitmanager@lcgasgit:/srv/git/<repo>.git)
  --dry-run              Show what would be done without executing
  --verbose              Enable verbose logging
  --no-color             Disable colorized output
```

### Configuration Strategy

**⚠️ REVISED (User Preferences: Vault-Centric, Minimal Env Vars, No Silent Failures)**

**Critique recommended**: YAML config file + environment variables
**User constraints**:
- Dislikes environment variable proliferation
- **Hates silent failures** (fallbacks are OK if explicit and loud)

**Final approach**: **Everything in Vault, with explicit caching and loud warnings**

#### Vault Configuration with Explicit Caching (No Silent Failures)

**All configuration stored in Vault** at `secret/git-dual-remote/config`:

```bash
vault kv put secret/git-dual-remote/config \
  github_username="lcgerke" \
  bare_repo_pattern="gitmanager@lcgasgit:/srv/git/{repo}.git" \
  default_visibility="private" \
  auto_create_github=true \
  test_before_push=true \
  sync_on_setup=true \
  retry_on_partial_failure=true
```

**Implementation - Explicit Cache via Flag**:

```go
type Config struct {
    GitHubUsername      string
    BareRepoPattern     string
    DefaultVisibility   string
    AutoCreateGitHub    bool
    TestBeforePush      bool
    SyncOnSetup         bool
    RetryOnPartialFail  bool
}

type CachedConfig struct {
    Config    Config
    FetchedAt time.Time
    Source    string  // "vault" or "cache"
}

func LoadConfig(vaultClient *vault.Client, allowCache bool) (*CachedConfig, error) {
    // Try to read from Vault
    secret, err := vaultClient.Logical().Read("secret/data/git-dual-remote/config")
    if err == nil {
        // SUCCESS: Got config from Vault
        config, err := ParseVaultConfig(secret.Data["data"])
        if err != nil {
            return nil, fmt.Errorf("invalid configuration in Vault: %w", err)
        }

        // Validate required fields
        if config.GitHubUsername == "" {
            return nil, errors.New("github_username is required in Vault config")
        }
        if config.BareRepoPattern == "" {
            return nil, errors.New("bare_repo_pattern is required in Vault config")
        }

        // Cache it for next time
        SaveConfigCache(config)

        return &CachedConfig{
            Config:    config,
            FetchedAt: time.Now(),
            Source:    "vault",
        }, nil
    }

    // Vault failed - check if user explicitly allowed cache
    if !allowCache {
        // User did NOT use --use-cache flag - HARD FAIL
        cached, cacheErr := LoadConfigCache()
        cacheHint := ""
        if cacheErr == nil && time.Since(cached.FetchedAt) < 24*time.Hour {
            cacheHint = fmt.Sprintf("\n\nHint: Cached config available (age: %v)\n"+
                "      Use --use-cache flag to use cached config:\n"+
                "      git-dual-remote --use-cache status",
                time.Since(cached.FetchedAt))
        }

        return nil, fmt.Errorf("Vault unreachable:\n"+
            "  Error: %w%s\n\n"+
            "Fix Vault connection or create config:\n"+
            "  vault kv put secret/git-dual-remote/config "+
            "github_username=... bare_repo_pattern=...", err, cacheHint)
    }

    // User explicitly used --use-cache flag
    cached, cacheErr := LoadConfigCache()
    if cacheErr == nil && time.Since(cached.FetchedAt) < 24*time.Hour {
        // ⚠️ LOUD WARNING - Using cache because user explicitly requested it
        log.Warn("⚠️  Using cached configuration (--use-cache flag)")
        log.Warn("    Vault error: %v", err)
        log.Warn("    Cache age: %v", time.Since(cached.FetchedAt))
        log.Warn("    Config may be stale if changed in Vault")

        cached.Source = "cache"
        return cached, nil
    }

    // User requested cache but it's unavailable/expired
    return nil, fmt.Errorf("cached configuration unavailable:\n"+
        "  Vault error: %w\n"+
        "  Cache error: %v\n\n"+
        "Cannot proceed without Vault access.", err, cacheErr)
}
```

**Cache Location**: `~/.cache/git-dual-remote/config.json`

**Cache Format**:
```json
{
  "config": {
    "github_username": "lcgerke",
    "bare_repo_pattern": "gitmanager@lcgasgit:/srv/git/{repo}.git",
    ...
  },
  "fetched_at": "2025-11-16T14:30:00Z"
}
```

**Benefits:**
- ✅ Zero config files (cache is invisible, automatic)
- ✅ Only 2 env vars (`VAULT_ADDR`, `VAULT_TOKEN`)
- ✅ Vault is source of truth
- ✅ **No automatic fallbacks** - cache requires explicit flag
- ✅ **LOUD warnings when using cache** (user explicitly requested it)
- ✅ Helpful hints when cache available but not used
- ✅ Fast commands when Vault working (config cached in background)

**User Experience with Explicit Cache**:

```bash
# Normal operation (Vault reachable)
$ git-dual-remote status
✓ Configuration loaded from Vault
Remote Configuration:
...

# Vault temporarily down (NO --use-cache flag)
$ git-dual-remote status
🔴 Error: Vault unreachable
  Error: connection refused

Hint: Cached config available (age: 15 minutes)
      Use --use-cache flag to use cached config:
      git-dual-remote --use-cache status

Fix Vault connection or create config:
  vault kv put secret/git-dual-remote/config ...

# Vault down with --use-cache flag (EXPLICIT)
$ git-dual-remote --use-cache status
⚠️  Using cached configuration (--use-cache flag)
    Vault error: connection refused
    Cache age: 15 minutes
    Config may be stale if changed in Vault
Remote Configuration:
...

# Cache expired
$ git-dual-remote --use-cache status
🔴 Error: Cached configuration unavailable
  Vault error: connection refused
  Cache error: cache expired (25 hours old)

Cannot proceed without Vault access.
```

**Vault Secret Paths (Hardcoded in Tool):**

```go
const (
    ConfigPath      = "secret/data/git-dual-remote/config"     // Cached (24h), requires --use-cache
    SSHKeyPathTmpl  = "secret/data/github/%s/ssh"              // NEVER cached (security)
    PATPathTmpl     = "secret/data/github/%s/pat"              // NEVER cached (security)
)
```

**Critical: Secrets are NEVER cached** (SSH keys, PAT tokens always fetched fresh from Vault).

### Global Flag for Cache Usage

```bash
# Add --use-cache as global flag to all commands
git-dual-remote [--use-cache] <command> [args]

# Examples:
git-dual-remote status                    # Fails if Vault unreachable
git-dual-remote --use-cache status        # Uses cache if Vault unreachable
git-dual-remote --use-cache setup ...     # Uses cached config for setup
```

**Flag Behavior:**
- Default: `--use-cache=false` (no automatic fallback)
- When Vault fails WITHOUT flag: Error with hint about `--use-cache`
- When Vault fails WITH flag: Use cache with loud warning
- Secrets (SSH, PAT) always require Vault regardless of flag

## Implementation Details

### Technology Stack

- **Language**: Go 1.21+
- **Logging**: zap (structured, fast, colorized)
- **CLI Framework**: cobra (commands, flags, help)
- **Terminal UI**:
  - `fatih/color` for ANSI colors
  - `briandowns/spinner` for progress indicators
  - `olekukonko/tablewriter` for status tables
- **Git Operations**: `go-git/go-git` (native Go) + shell fallback
- **Vault Client**: `hashicorp/vault/api`
- **GitHub API**: `google/go-github` + `gh` CLI integration

### Output Design Philosophy

**Principles:**
- ✅ **Clear visual hierarchy** (colors, symbols, indentation)
- ✅ **Progress indicators** for long operations
- ✅ **Actionable error messages** with suggested fixes
- ✅ **Summary tables** for status/sync state
- ✅ **Copy-paste commands** in error recovery

**Color Scheme:**
```
🟢 Green   → Success, confirmed state
🟡 Yellow  → Warning, needs attention
🔴 Red     → Error, blocking issue
🔵 Blue    → Info, context, hints
⚪ Gray    → Debug, verbose details
```

**Example Output:**

```
🚀 Git Dual-Remote Setup
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Repository: ccTrack
Strategy:   Bare repo primary, GitHub backup
Mode:       Dual-push (both remotes on every push)

📋 Preflight Checks
  ✓ Git repository detected
  ✓ git command available (v2.43.0)
  ✓ gh CLI available (v2.40.1)
  ✓ vault CLI available (v1.15.0)

🔐 Vault SSH Key Retrieval
  → Authenticating with vault...
  ✓ Connected to vault at http://localhost:8200
  → Retrieving secret/github/ccTrack...
  ✓ SSH key retrieved (ED25519, 256 bits)
  → Writing to ~/.ssh/github_cctrack...
  ✓ Key saved with permissions 600

📝 SSH Configuration
  → Checking ~/.ssh/config...
  ⚠ No github.com entry found
  → Adding github.com host configuration...
  ✓ SSH config updated

  Host github.com
    IdentityFile ~/.ssh/github_cctrack
    IdentitiesOnly yes

🔌 Connection Tests
  → Testing bare repo (gitmanager@lcgasgit)...
  ✓ Bare repo reachable (latency: 12ms)
  → Testing GitHub (git@github.com)...
  ✓ GitHub SSH authenticated as lcgerke

🔧 Remote Configuration
  Current remotes:
  ┌────────┬────────────────────────────────────────────────┬────────┐
  │ NAME   │ URL                                            │ TYPE   │
  ├────────┼────────────────────────────────────────────────┼────────┤
  │ origin │ ssh://gitmanager@lcgasgit/srv/git/ccTrack.git │ fetch  │
  │ origin │ ssh://gitmanager@lcgasgit/srv/git/ccTrack.git │ push   │
  └────────┴────────────────────────────────────────────────┴────────┘

  → Updating origin to SCP-style format...
  ✓ origin (fetch): gitmanager@lcgasgit:/srv/git/ccTrack.git
  → Adding dual push URL for GitHub...
  ✓ origin (push): git@github.com:lcgerke/ccTrack.git
  → Adding explicit github remote...
  ✓ github remote configured

  Updated remotes:
  ┌────────┬────────────────────────────────────────────┬────────┐
  │ NAME   │ URL                                        │ TYPE   │
  ├────────┼────────────────────────────────────────────┼────────┤
  │ origin │ gitmanager@lcgasgit:/srv/git/ccTrack.git  │ fetch  │
  │ origin │ gitmanager@lcgasgit:/srv/git/ccTrack.git  │ push   │
  │ origin │ git@github.com:lcgerke/ccTrack.git        │ push   │
  │ github │ git@github.com:lcgerke/ccTrack.git        │ both   │
  └────────┴────────────────────────────────────────────┴────────┘

🐙 GitHub Repository
  → Checking if lcgerke/ccTrack exists...
  ⚠ Repository not found
  → Creating private repository...
  ✓ Repository created: https://github.com/lcgerke/ccTrack
  → Setting default branch to main...
  ✓ Default branch configured

🔄 Initial Sync
  → Pushing to bare repo...
  ✓ Pushed main to gitmanager@lcgasgit (3 commits)
  → Pushing to GitHub...
  ✓ Pushed main to github.com (3 commits)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Setup Complete!

Your repository now has dual remotes configured:

  📍 Primary (origin):  gitmanager@lcgasgit:/srv/git/ccTrack.git
  🔄 Backup (github):   git@github.com:lcgerke/ccTrack.git

Daily Workflow:
  git pull              # Pull from bare repo (fast, local)
  git commit -m "work"
  git push              # Push to BOTH remotes automatically

When GitHub has changes:
  git pull github main  # Explicit GitHub sync
  git push              # Push merged result to both

Next Steps:
  • Run 'git-dual-remote status' to verify sync state
  • See 'git-dual-remote help workflow' for full guide
  • Documentation: .git-dual-remote-workflow.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Command Specifications

### `git-dual-remote setup`

**Purpose**: Full automated setup of dual remotes

**Steps:**
1. Validate environment (git, gh, vault)
2. Retrieve SSH key from Vault
3. Configure SSH for github.com
4. Test connections to both remotes
5. Configure git remotes with dual push
6. Create GitHub repo if missing
7. Perform initial sync push
8. Generate workflow documentation

**Flags:**
- `--github-user` (required)
- `--vault-path` (default: `secret/github/<repo>`)
- `--bare-remote` (default: `gitmanager@lcgasgit:/srv/git/<repo>.git`)
- `--skip-sync` (don't push on setup)
- `--force` (reconfigure even if already set up)

**Safety Checks:**
- Confirm before overwriting existing remotes
- Backup `.git/config` before modifications
- Rollback on failure
- Dry-run mode shows all changes without applying

### `git-dual-remote status`

**Purpose**: Display current remote configuration and sync state

**Output:**
```
Remote Configuration:
┌────────┬──────────────────────────────────┬────────┬─────────┐
│ REMOTE │ URL                              │ TYPE   │ STATUS  │
├────────┼──────────────────────────────────┼────────┼─────────┤
│ origin │ gitmanager@lcgasgit:/srv/git/... │ fetch  │ ✓ reach │
│ origin │ gitmanager@lcgasgit:/srv/git/... │ push   │ ✓ reach │
│ origin │ git@github.com:lcgerke/ccTrack   │ push   │ ✓ reach │
│ github │ git@github.com:lcgerke/ccTrack   │ both   │ ✓ reach │
└────────┴──────────────────────────────────┴────────┴─────────┘

Sync State (main branch):
┌──────────┬────────┬────────────────────┬─────────┐
│ REMOTE   │ AHEAD  │ BEHIND             │ STATUS  │
├──────────┼────────┼────────────────────┼─────────┤
│ origin   │ 0      │ 0                  │ ✓ synced│
│ github   │ 0      │ 0                  │ ✓ synced│
└──────────┴────────┴────────────────────┴─────────┘

SSH Keys:
  ✓ ~/.ssh/github_cctrack exists (ED25519)
  ✓ SSH config has github.com entry
  ✓ GitHub authentication working

Everything is in sync! ✅
```

**Checks:**
- Remote URLs configured correctly
- Remotes are reachable
- Branch sync state (ahead/behind)
- SSH key exists and works
- Dual push URLs present

### `git-dual-remote sync`

**Purpose**: Manually synchronize both remotes

**Behavior:**
1. Fetch from both remotes
2. Show divergence if any
3. Push current branch to both
4. Verify sync completed

**Flags:**
- `--force` (force push if needed)
- `--branch` (specific branch, default: current)
- `--pull-first` (pull from origin before pushing)
- `--skip-github` (only sync to bare repo, skip GitHub)
- `--retry-github` (retry failed GitHub push from previous sync)

## Dual-Push Error Handling Strategy

### ⚠️ NEW SECTION (Per Critique)

**Problem**: When `git push` pushes to multiple remotes, if one succeeds and another fails, the remotes are left in an inconsistent state.

### Error Scenarios

#### Scenario 1: Bare Repo Succeeds, GitHub Fails

```
$ git push
Pushing to gitmanager@lcgasgit:/srv/git/myrepo.git
 ✓ main -> main (3 commits pushed)

Pushing to git@github.com:lcgerke/myrepo.git
 ✗ Error: Failed to connect to github.com
```

**System State:**
- Bare repo: ✅ Up to date
- GitHub: ❌ Behind by 3 commits
- User's local: Committed but partially synced

**Tool Response:**
1. **Don't fail silently** - clearly report partial success
2. **Mark GitHub as "needs sync"** in internal state
3. **Provide recovery command**: `git-dual-remote sync --retry-github`

**Output:**
```
⚠️  Partial Push Success

✅ Bare repo: Pushed 3 commits to gitmanager@lcgasgit
❌ GitHub: Push failed (network unreachable)

Status:
  Bare repo is authoritative and up-to-date.
  GitHub is behind by 3 commits.

Recovery:
  When GitHub is reachable, run:
    git-dual-remote sync --retry-github

  Or continue working normally. Next successful push will sync both.
```

#### Scenario 2: Both Remotes Fail

```
$ git push
Pushing to gitmanager@lcgasgit:/srv/git/myrepo.git
 ✗ Error: ssh: connect to host lcgasgit port 2222: Connection refused

Pushing to git@github.com:lcgerke/myrepo.git
 ✗ Error: Failed to connect to github.com
```

**Tool Response:**
- Report total failure
- Advise checking network/SSH
- Commits remain in local repository (safe)

#### Scenario 3: GitHub Succeeds, Bare Repo Fails (Edge Case)

**This should be impossible** with dual-push URL configuration because git pushes to `origin` first (bare repo), then the additional push URL (GitHub). If bare repo fails, GitHub push never happens.

However, with explicit `git push github`, this could occur.

**Tool Response:**
- Warn that bare repo (canonical source) is behind
- Require user to manually fix bare repo before next dual push

### Implementation Strategy

#### 1. Post-Push Verification

After every `git push`, verify sync state:

```go
func VerifyPostPush() error {
    // Check both remotes for branch HEAD
    bareHead := exec.Command("git", "ls-remote", "origin", "HEAD").Output()
    githubHead := exec.Command("git", "ls-remote", "github", "HEAD").Output()

    if bareHead != githubHead {
        // Remotes diverged!
        return ErrPartialPushFailure
    }
    return nil
}
```

#### 2. State Tracking

Track failed GitHub pushes in `~/.git-dual-remote/state.yaml`:

```yaml
repositories:
  /home/user/myrepo:
    last_sync: "2025-11-16T14:30:00Z"
    github_needs_retry: true
    github_last_error: "network unreachable"
    commits_pending: 3
```

#### 3. Retry Mechanism

```bash
# User runs after network is restored
git-dual-remote sync --retry-github

# Tool behavior:
# 1. Check state file for pending GitHub push
# 2. Fetch from both remotes to compare
# 3. Push local commits to GitHub only
# 4. Verify sync, clear retry flag
```

#### 4. Offline Mode Flag

For users who know GitHub is unavailable:

```bash
# Skip GitHub push entirely, update bare repo only
git push --push-option=skip-github

# Or via tool
git-dual-remote sync --skip-github
```

**Implementation**: Use git push options or environment variable:
```bash
export GIT_DUAL_REMOTE_SKIP_GITHUB=1
git push  # Only pushes to bare repo
```

### Status Command Integration

`git-dual-remote status` must show divergence:

```
Sync State (main branch):
┌──────────┬────────┬────────────────────┬──────────────┐
│ REMOTE   │ AHEAD  │ BEHIND             │ STATUS       │
├──────────┼────────┼────────────────────┼──────────────┤
│ origin   │ 0      │ 0                  │ ✓ synced     │
│ github   │ 0      │ 3                  │ ⚠ needs sync │
└──────────┴────────┴────────────────────┴──────────────┘

⚠ GitHub is behind by 3 commits

Last sync attempt: 2025-11-16 14:30:00
Last error: network unreachable

Run 'git-dual-remote sync --retry-github' to retry.
```

### Git Hook Integration (Optional)

Install a `post-push` hook to automatically detect partial failures:

```bash
# .git/hooks/post-push
#!/bin/bash
git-dual-remote verify-sync || {
    echo "⚠️  Warning: Remotes are out of sync"
    echo "Run 'git-dual-remote status' for details"
}
```

### User Documentation

Add to workflow guide:

**Q: What if GitHub is down?**

A: Your work is safe. The bare repo (authoritative) receives your push. When GitHub is back, run `git-dual-remote sync --retry-github`.

**Q: Can I work offline?**

A: Yes. Use `--skip-github` flag or set `GIT_DUAL_REMOTE_SKIP_GITHUB=1`. Your commits push to the bare repo only. Sync to GitHub when online.

**Q: What if both remotes fail?**

A: Your commits stay in your local repository (safe). Fix network/SSH, then push again. Nothing is lost.

### `git-dual-remote test`

**Purpose**: Test connectivity and authentication to both remotes

**Tests:**
- SSH connection to bare repo host
- SSH authentication to github.com
- Git remote connectivity (`git ls-remote`)
- Push permission (dry-run push)
- Vault accessibility (if testing key retrieval)

**Output:** Pass/fail for each test with troubleshooting hints

### `git-dual-remote doctor`

**Purpose**: Diagnose and fix common configuration issues

**Diagnostics:**
- Git repository validity
- Remote URL format (SCP vs ssh://)
- SSH key permissions
- SSH config correctness
- Vault connectivity
- GitHub authentication
- Diverged remotes

**Auto-fix:**
- Fix SSH key permissions (chmod 600)
- Convert ssh:// URLs to SCP format
- Add missing SSH config entries
- Suggest commands for manual fixes

### `git-dual-remote help`

**Enhanced help system:**

```
git-dual-remote help           # Standard help
git-dual-remote help strategy  # Display full strategy document
git-dual-remote help workflow  # Daily workflow guide
git-dual-remote help vault     # Vault integration guide
git-dual-remote help troubleshoot # Common issues
```

**Strategy output** (embedded in binary):
- Full decision matrix from this planning doc
- Visual diagram of remote configuration
- Workflow examples
- Architecture rationale

## Error Handling Strategy

### Categories

1. **Validation Errors** (before any changes)
   - Missing dependencies
   - Invalid repository
   - Wrong directory

2. **Configuration Errors** (during setup)
   - Vault connection failed
   - SSH key invalid
   - Remote unreachable

3. **Runtime Errors** (during operations)
   - Push failed
   - Merge conflicts
   - Network timeout

### Error Message Template

```
🔴 Error: GitHub SSH Key Retrieval Failed

Problem:
  Unable to retrieve SSH key from vault path: secret/github/ccTrack

Cause:
  Vault returned: permission denied on secret/github/ccTrack

Fix:
  1. Verify vault credentials in ~/VAULT_CREDENTIALS.txt
  2. Check vault policy allows reading secret/github/*
  3. Ensure key exists at the path:

     vault kv get secret/github/ccTrack

  4. If key doesn't exist, create it:

     ssh-keygen -t ed25519 -f /tmp/github_key -N ""
     vault kv put secret/github/ccTrack \
       private_key=@/tmp/github_key \
       public_key=@/tmp/github_key.pub
     rm /tmp/github_key*

Need more help? Run: git-dual-remote help vault
```

### Recovery Actions

**Rollback Mechanism:**
- Backup `.git/config` before modifications → `.git/config.backup-<timestamp>`
- Transaction-style: all changes or none
- Explicit rollback command on failure
- User confirmation before destructive operations

**Idempotent Operations:**
- Running setup multiple times is safe
- Detects existing configuration
- Updates only what's changed
- Preserves user customizations

## Vault Integration

### ⚠️ UPDATED: Authentication Flow (Per Critique + User Preference)

**Original flaw**: Custom `~/VAULT_CREDENTIALS.txt` file is non-standard and creates security contradiction ("never written to disk").

**Critique recommendation**: Use standard Vault environment variables.

**User constraint**: Minimize environment variables (user strongly dislikes env var proliferation).

**Revised approach**: Use ONLY Vault env vars (`VAULT_ADDR`, `VAULT_TOKEN`), store everything else IN Vault.

### Vault-Centric Architecture (Minimal Env Vars)

**Philosophy**: Only `VAULT_ADDR` and `VAULT_TOKEN` as environment variables. Everything else retrieved from Vault at runtime.

```go
// 1. Create vault client using ONLY standard Vault env vars
config := vault.DefaultConfig() // Reads VAULT_ADDR from environment
client, err := vault.NewClient(config)
if err != nil {
    return fmt.Errorf("failed to create Vault client: %w", err)
}
// VAULT_TOKEN is automatically used by the client

// 2. Verify Vault connectivity
_, err = client.Sys().Health()
if err != nil {
    return fmt.Errorf("cannot connect to Vault at %s: %w",
        os.Getenv("VAULT_ADDR"), err)
}

// 3. Read configuration from Vault (replaces config file and env vars)
configSecret, err := client.Logical().Read("secret/data/git-dual-remote/config")
if err != nil {
    // Fallback to defaults if config doesn't exist
    config = DefaultConfig()
} else {
    config = ParseVaultConfig(configSecret.Data["data"])
}

// 4. Retrieve SSH key for this specific repo
sshPath := fmt.Sprintf("secret/data/github/%s/ssh", repoName)
secret, err := client.Logical().Read(sshPath)
if err != nil {
    return fmt.Errorf("failed to read SSH key from Vault: %w", err)
}
privateKey := secret.Data["data"].(map[string]interface{})["private_key"].(string)

// 5. Retrieve GitHub PAT for this specific repo
patPath := fmt.Sprintf("secret/data/github/%s/pat", repoName)
secret, err = client.Logical().Read(patPath)
if err != nil {
    return fmt.Errorf("failed to read GitHub PAT from Vault: %w", err)
}
pat := secret.Data["data"].(map[string]interface{})["token"].(string)

// 6. Use PAT directly - NO GH_TOKEN env var needed
// Pass to gh CLI via stdin instead of environment
cmd := exec.Command("gh", "auth", "login", "--with-token")
cmd.Stdin = strings.NewReader(pat)
cmd.Run()
```

### Benefits of This Approach

1. **Minimal Environment Variables**: Only 2 env vars total (both Vault-related)
   - `VAULT_ADDR` - Vault server address
   - `VAULT_TOKEN` - Vault authentication token
   - NO `GH_TOKEN`, NO `PATH`, NO custom vars

2. **Configuration in Vault**: Tool configuration stored in Vault, not files
   ```
   secret/git-dual-remote/config
     ├── github_username: "lcgerke"
     ├── bare_repo_pattern: "gitmanager@lcgasgit:/srv/git/{repo}.git"
     ├── default_visibility: "private"
     └── auto_create_github: true
   ```

3. **Centralized Secret Management**: All secrets in one place
   ```
   secret/github/myrepo/
     ├── ssh/private_key
     ├── ssh/public_key
     └── pat/token
   ```

4. **No Token Environment Pollution**: `gh` CLI receives token via stdin, not env var

### Environment Setup (User Responsibility)

**ONLY TWO environment variables needed** (user preference: minimize env vars):

```bash
# 1. Vault address (only set once, rarely changes)
export VAULT_ADDR="http://localhost:8200"

# 2. Vault token (set automatically by 'vault login')
vault login
# This sets VAULT_TOKEN automatically - you don't manage it manually

# Verify Vault access
vault status
```

**Everything else comes from Vault:**
- Tool configuration → `secret/git-dual-remote/config`
- SSH keys → `secret/github/<repo>/ssh`
- GitHub PAT → `secret/github/<repo>/pat`
- User preferences → `secret/git-dual-remote/config`

### Pre-flight Check Updates

Tool must verify environment before proceeding:

```go
func CheckVaultEnvironment() error {
    // Check VAULT_ADDR is set
    if os.Getenv("VAULT_ADDR") == "" {
        return errors.New("VAULT_ADDR environment variable not set")
    }

    // Check VAULT_TOKEN is set
    if os.Getenv("VAULT_TOKEN") == "" {
        return errors.New("VAULT_TOKEN environment variable not set")
    }

    // Verify Vault is reachable
    client, err := vault.NewClient(vault.DefaultConfig())
    if err != nil {
        return fmt.Errorf("failed to create Vault client: %w", err)
    }

    health, err := client.Sys().Health()
    if err != nil {
        return fmt.Errorf("Vault health check failed: %w", err)
    }

    if !health.Initialized {
        return errors.New("Vault is not initialized")
    }

    return nil
}
```

**Error message when Vault env vars missing:**

```
🔴 Error: Vault Environment Not Configured

Required environment variables are missing:
  ✗ VAULT_ADDR not set
  ✗ VAULT_TOKEN not set

Setup:
  1. Set Vault address:
     export VAULT_ADDR="http://localhost:8200"

  2. Authenticate with Vault:
     vault login
     (This sets VAULT_TOKEN automatically)

  3. Verify access:
     vault status

  4. Re-run git-dual-remote setup

For more help: git-dual-remote help vault
```

### Complete Vault Secret Structure

**Global tool configuration** (optional - defaults used if missing):

```bash
vault kv put secret/git-dual-remote/config \
  github_username="lcgerke" \
  bare_repo_pattern="gitmanager@lcgasgit:/srv/git/{repo}.git" \
  default_visibility="private" \
  auto_create_github=true \
  test_before_push=true \
  sync_on_setup=true
```

**Per-repository secrets** (required for each repo):

```bash
# SSH key for git operations
vault kv put secret/github/ccTrack/ssh \
  private_key=@~/.ssh/github_cctrack \
  public_key=@~/.ssh/github_cctrack.pub \
  created="2025-11-16" \
  purpose="Git push/pull operations"

# GitHub PAT for API operations
vault kv put secret/github/ccTrack/pat \
  token="ghp_xxxxxxxxxxxxxxxxxxxx" \
  created="2025-11-16" \
  scopes="repo,workflow" \
  purpose="GitHub API via gh CLI"
```

### Vault Secret Hierarchy

```
secret/
├── git-dual-remote/
│   └── config                    # Global tool configuration
└── github/
    ├── repo1/
    │   ├── ssh/                  # SSH keypair for repo1
    │   │   ├── private_key
    │   │   └── public_key
    │   └── pat/                  # GitHub PAT for repo1
    │       └── token
    ├── repo2/
    │   ├── ssh/
    │   └── pat/
    └── ...
```

### Error Handling

- Vault unreachable → suggest checking `VAULT_ADDR`
- Token expired → suggest running `vault login` again
- Secret not found → provide creation commands
- Malformed key → validate format before writing
- Config missing → use sensible defaults, inform user

## SSH Configuration Management

### ⚠️ CRITICAL CHANGE: Repository-Local SSH Config (Per Critique)

**Problem with original approach**: Modifying global `~/.ssh/config` is dangerous and can break users with complex multi-account GitHub setups.

**New safer approach**: Use repository-local git configuration instead.

### Target Configuration (Repository-Local)

```bash
# Do NOT modify ~/.ssh/config
# Instead, configure per-repository SSH command:
git config core.sshCommand "ssh -i ~/.ssh/github_<reponame> -o IdentitiesOnly=yes"
```

### Implementation Strategy

1. **Write SSH key to filesystem**
   - Store at `~/.ssh/github_<reponame>`
   - Set permissions: `chmod 600 ~/.ssh/github_<reponame>`

2. **Configure repository-local SSH**
   - Run: `git config core.sshCommand "ssh -i ~/.ssh/github_<reponame> -o IdentitiesOnly=yes"`
   - This scopes the SSH key to THIS repository only
   - Global `~/.ssh/config` remains untouched

3. **Validation**
   - Test SSH connection: `ssh -T git@github.com -i ~/.ssh/github_<reponame>`
   - Expected output: "Hi <username>! You've successfully authenticated..."

4. **Benefits of this approach**
   - ✅ No risk of breaking user's existing SSH setup
   - ✅ No conflicts with multi-account GitHub workflows
   - ✅ No need to parse/modify complex SSH config files
   - ✅ Easy to undo: `git config --unset core.sshCommand`
   - ✅ Clear isolation: key only used for this repo

## GitHub Repository Management

### ⚠️ CRITICAL FIX: GitHub Authentication (Per Critique)

**Original flaw**: Plan confused SSH keys (for git) with GitHub PAT (for API).

**Corrected understanding**:
- **SSH key** → Used for `git push`/`git pull` operations
- **GitHub PAT** → Used for `gh` CLI API calls (repo creation, status, etc.)

### Updated Creation Logic

```go
// 1. Authenticate gh CLI with PAT from Vault
// Retrieve GitHub PAT from Vault
secret, err := vaultClient.Logical().Read("secret/data/github/<repo>/pat")
pat := secret.Data["data"].(map[string]interface{})["token"].(string)

// Set GH_TOKEN environment variable for gh CLI
os.Setenv("GH_TOKEN", pat)

// Verify authentication works
output := exec.Command("gh", "auth", "status").CombinedOutput()
if err != nil {
    return fmt.Errorf("GitHub authentication failed: %w", err)
}

// 2. Check if repo exists
output := exec.Command("gh", "repo", "view", "lcgerke/ccTrack").CombinedOutput()
if strings.Contains(string(output), "not found") {
    // 3. Create private repo
    exec.Command("gh", "repo", "create", "lcgerke/ccTrack",
        "--private",
        "--source=.",
        "--remote=github").Run()
}
```

### Vault Secret Structure (Updated)

**Two secrets required per repository:**

```bash
# 1. SSH key for git operations
vault kv put secret/github/ccTrack/ssh \
  private_key=@~/.ssh/github_cctrack \
  public_key=@~/.ssh/github_cctrack.pub \
  created="2025-11-16" \
  purpose="Git push/pull operations"

# 2. GitHub PAT for API operations
vault kv put secret/github/ccTrack/pat \
  token="ghp_xxxxxxxxxxxxxxxxxxxx" \
  created="2025-11-16" \
  scopes="repo,workflow" \
  purpose="GitHub API via gh CLI"
```

### Pre-flight Checks (Updated)

```bash
# Check 1: Vault connectivity
vault status

# Check 2: GitHub PAT exists and works
export GH_TOKEN=$(vault kv get -field=token secret/github/ccTrack/pat)
gh auth status  # Must succeed

# Check 3: SSH key exists
vault kv get secret/github/ccTrack/ssh

# Check 4: gh CLI available
gh --version
```

### Repository Settings

**On creation:**
- Visibility: Private (default)
- Initialize: Empty (we push existing code)
- Default branch: `main` (set explicitly)
- README: No (preserve existing)

**Optional enhancements:**
- `.gitignore` template (user choice)
- License (user choice)
- Branch protection rules (optional flag)

## Testing Strategy

### Unit Tests

**Modules to test:**
- Vault client (mock vault server)
- SSH config parser (fixture files)
- Remote URL parser/validator
- Git operations (test git repo)

### Integration Tests

**Scenarios:**
- Fresh setup (no existing remotes)
- Migration (existing origin, add github)
- Reconfiguration (fix broken remotes)
- Vault key retrieval (test vault container)
- GitHub repo creation (test GitHub account)

### Manual Test Plan

```bash
# 1. Clean slate test
rm -rf test-repo
git init test-repo && cd test-repo
git-dual-remote setup --github-user lcgerke

# 2. Existing repo test
cd existing-project
git-dual-remote setup --github-user lcgerke

# 3. Broken config repair
# Manually mess up remotes
git-dual-remote doctor

# 4. Offline mode (no GitHub)
# Disconnect network
git-dual-remote status

# 5. Sync diverged remotes
# Make commits on GitHub via web UI
git-dual-remote sync
```

## File Structure

```
git-dual-remote/
├── cmd/
│   └── git-dual-remote/
│       ├── main.go              # Entry point, cobra root
│       ├── setup.go             # Setup command
│       ├── status.go            # Status command
│       ├── sync.go              # Sync command
│       ├── test.go              # Test command
│       └── doctor.go            # Doctor command
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration struct and loading
│   │   └── defaults.go          # Default values
│   ├── vault/
│   │   ├── client.go            # Vault API integration
│   │   └── ssh_key.go           # SSH key retrieval/storage
│   ├── ssh/
│   │   ├── config.go            # SSH config parsing/modification
│   │   └── keygen.go            # SSH key operations
│   ├── git/
│   │   ├── remote.go            # Remote configuration
│   │   ├── sync.go              # Sync operations
│   │   └── url.go               # URL parsing/validation
│   ├── github/
│   │   ├── client.go            # GitHub API / gh CLI wrapper
│   │   └── repo.go              # Repository management
│   ├── ui/
│   │   ├── output.go            # Colorized output utilities
│   │   ├── table.go             # Table formatting
│   │   ├── spinner.go           # Progress indicators
│   │   └── prompt.go            # User confirmations
│   └── doctor/
│       ├── diagnostics.go       # Health checks
│       └── fixes.go             # Auto-fix operations
├── pkg/
│   └── strategy/
│       └── strategy.go          # Embedded strategy documentation
├── docs/
│   ├── STRATEGY.md              # This document
│   ├── WORKFLOW.md              # Daily workflow guide
│   ├── TROUBLESHOOTING.md       # Common issues
│   └── VAULT.md                 # Vault integration guide
├── test/
│   ├── fixtures/                # Test SSH configs, git repos
│   ├── integration/             # Integration test scenarios
│   └── mocks/                   # Mock vault/GitHub servers
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Build and Distribution

### Makefile Targets

```makefile
.PHONY: build test install clean

build:
	go build -o bin/git-dual-remote cmd/git-dual-remote/main.go

test:
	go test -v ./...

integration:
	go test -v -tags=integration ./test/integration

install:
	go install ./cmd/git-dual-remote

clean:
	rm -rf bin/

lint:
	golangci-lint run

release:
	goreleaser release --clean
```

### Binary Embedding

Embed strategy docs in binary:

```go
//go:embed docs/*.md
var docsFS embed.FS

func PrintStrategy() {
    content, _ := docsFS.ReadFile("docs/STRATEGY.md")
    fmt.Println(string(content))
}
```

### Distribution

- **Local install**: `go install` or copy to `~/bin`
- **Releases**: GitHub releases with binaries for linux/darwin/windows
- **Package managers**: Future: homebrew, apt, etc.

## Dependencies

```go
require (
    github.com/spf13/cobra v1.8.0           // CLI framework
    go.uber.org/zap v1.26.0                 // Logging
    github.com/fatih/color v1.16.0          // Colors
    github.com/briandowns/spinner v1.23.0   // Spinners
    github.com/olekukonko/tablewriter v0.0.5 // Tables
    github.com/hashicorp/vault/api v1.10.0  // Vault client
    github.com/google/go-github/v56 v56.0.0 // GitHub API
    github.com/kevinburke/ssh_config v1.2.0 // SSH config parsing
    gopkg.in/yaml.v3 v3.0.1                 // Config file
    github.com/go-git/go-git/v5 v5.11.0     // Git operations
)
```

## Development Phases

### Phase 1: Core Setup (MVP)
- [x] Planning and design (this doc)
- [ ] Project scaffolding (cobra, zap, structure)
- [ ] Vault integration (key retrieval)
- [ ] SSH config management
- [ ] Git remote configuration
- [ ] Basic setup command
- [ ] Status command

### Phase 2: GitHub Integration
- [ ] gh CLI wrapper
- [ ] Repository creation
- [ ] Authentication handling
- [ ] Sync operations
- [ ] Test command

### Phase 3: Polish and Safety
- [ ] Doctor command with diagnostics
- [ ] Enhanced error messages
- [ ] Rollback mechanism
- [ ] Dry-run mode
- [ ] Comprehensive help system

### Phase 4: Documentation and Distribution
- [ ] Embedded strategy docs
- [ ] Workflow guide generation
- [ ] Integration tests
- [ ] Release automation
- [ ] README and examples

## Usage Examples

### First-time setup

```bash
# Basic setup
git-dual-remote setup --github-user lcgerke

# Custom bare repo location
git-dual-remote setup \
    --github-user lcgerke \
    --bare-remote gitmanager@customhost:/git/myrepo.git

# Dry-run to preview changes
git-dual-remote setup --github-user lcgerke --dry-run

# Verbose output for debugging
git-dual-remote setup --github-user lcgerke --verbose
```

### Daily operations

```bash
# Check sync state
git-dual-remote status

# Manual sync if needed
git-dual-remote sync

# Test connectivity
git-dual-remote test

# Fix configuration issues
git-dual-remote doctor
```

### Help and documentation

```bash
# Full strategy document
git-dual-remote help strategy

# Workflow guide
git-dual-remote help workflow

# Troubleshooting
git-dual-remote help troubleshoot

# Vault integration
git-dual-remote help vault
```

## Success Criteria

### Functional Requirements

- ✅ Automates entire dual-remote setup in one command
- ✅ Retrieves SSH key from Vault securely
- ✅ Configures git remotes with dual push URLs
- ✅ Creates GitHub repo if missing
- ✅ Tests connectivity to both remotes
- ✅ Provides clear status visibility
- ✅ Handles errors gracefully with actionable messages
- ✅ Supports dry-run mode
- ✅ Idempotent operations (safe to re-run)

### Non-Functional Requirements

- ✅ Beautiful, colorized output
- ✅ Fast execution (< 10 seconds for setup)
- ✅ Comprehensive help system
- ✅ Bulletproof error handling
- ✅ No dependencies beyond Go stdlib + listed packages
- ✅ Cross-platform (Linux, macOS)
- ✅ Well-tested (unit + integration)
- ✅ Clear, maintainable code

## Security Considerations

### ⚠️ UPDATED (Per Critique - Removed Contradictions)

1. **SSH Key Handling**
   - Keys written with 600 permissions (`chmod 600 ~/.ssh/github_<repo>`)
   - No keys logged or displayed in output
   - Keys stored on disk only at `~/.ssh/github_<repo>` (necessary for git operations)
   - Key material never passed via CLI args (no process list exposure)

2. **Vault Token**
   - **Managed via environment variables** (`VAULT_TOKEN`)
   - User responsibility: Set via `vault login` or CI/CD secrets
   - Tool NEVER writes token to disk
   - Not stored in git config or tool config
   - Validated before use via Vault health check

3. **GitHub PAT (Personal Access Token)**
   - Retrieved from Vault at runtime
   - Set in `GH_TOKEN` environment variable (in-memory only)
   - Never written to disk by this tool
   - `gh` CLI may cache token in `~/.config/gh/` (standard gh behavior)
   - Requires `repo` and `workflow` scopes

4. **Git Config Backup**
   - Always backup before modification (`.git/config.backup-<timestamp>`)
   - Timestamped backups for safe rollback
   - Manual restore on failure (fail-safe approach for V1)

5. **Repository-Local SSH Config**
   - Uses `git config core.sshCommand` (local to repo)
   - Does NOT modify global `~/.ssh/config`
   - Cannot interfere with user's other SSH setups
   - Isolated blast radius

### Threat Model

**What we protect against:**
- ✅ Accidental credential exposure in logs/output
- ✅ Process list exposure (no secrets in CLI args)
- ✅ Breaking user's existing SSH/GitHub workflows
- ✅ Credentials surviving in memory after tool exits

**What we DON'T protect against** (out of scope):
- ❌ Compromised Vault server
- ❌ Malicious code in `gh` CLI or `git`
- ❌ Filesystem access by other users (rely on Unix permissions)
- ❌ Memory dumps while tool is running

**User Responsibilities:**
- Secure Vault server and maintain token rotation
- Protect `~/.ssh/` directory with proper permissions
- Rotate GitHub PAT periodically
- Keep system and dependencies updated

## Future Enhancements

1. **Multi-repo management**
   - Operate on multiple repos at once
   - Workspace-level configuration

2. **Advanced sync strategies**
   - Conflict resolution helpers
   - Auto-merge strategies
   - Branch-specific dual-remote configs

3. **Monitoring and alerts**
   - Detect drift between remotes
   - Email/Slack alerts on sync failures
   - Prometheus metrics

4. **GUI/TUI**
   - Interactive setup wizard
   - Real-time sync status dashboard
   - Visual git graph with dual remotes

5. **CI/CD integration**
   - GitHub Actions workflow template
   - Auto-sync hooks for CI

---

## Critique Integration Summary

**Date Updated**: 2025-11-16
**Critiques Applied**: GPT-5 and Gemini 2.5 Pro parallel critiques

### Critical Fixes Applied

1. **✅ GitHub Authentication Fixed**
   - Original flaw: Confused SSH keys with GitHub PAT
   - Fix: Separated into two Vault secrets (SSH key for git, PAT for API)
   - Updated: GitHub Repository Management section, Vault Integration

2. **✅ SSH Config Approach Changed**
   - Original risk: Modifying global `~/.ssh/config` dangerous
   - Fix: Use repository-local `git config core.sshCommand`
   - Updated: SSH Configuration Management section

3. **✅ Dual-Push Error Handling Added**
   - Original gap: No strategy for partial failures
   - Fix: Added complete error handling section with retry mechanism
   - Added: Dual-Push Error Handling Strategy section, `--skip-github` and `--retry-github` flags

4. **✅ Vault Authentication Standardized + User Preferences Applied**
   - Original contradiction: Claimed "never written to disk" but used `~/VAULT_CREDENTIALS.txt`
   - Critique fix: Use standard `VAULT_ADDR` and `VAULT_TOKEN` environment variables
   - User preference #1: Minimize environment variables → Only 2 env vars total (both Vault)
   - User preference #2: No silent failures → Cache requires explicit `--use-cache` flag
   - Critique recommendation: Cache for offline/speed → Accepted but NOT automatic
   - Final approach: Vault-centric, cache via explicit flag only, helpful hints, secrets never cached
   - Updated: Vault Integration section, Configuration Strategy section, added `--use-cache` global flag

5. **✅ Security Claims Corrected**
   - Original issue: Contradictory security statements
   - Fix: Honest threat model, clear user responsibilities
   - Updated: Security Considerations section

### Tool Naming and Location Decision

**✅ Decision**: New unified tool called `githelper` in this repository
- User decision: "Change the name to githelper and put it in this repo"
- Gemini critique recommended: Code reuse approach
- Implementation: New tool with `githelper repo` and `githelper github` subcommands
- Benefits: Unified state, no tool divorce, clean design, clear naming
- Location: `/home/lcgerke/gitDualRemote/` (repository will be renamed to githelper)

### User Preferences Applied

Beyond the critique fixes, the following user preferences shaped the final design:

1. **Minimal Environment Variables**
   - User constraint: "Hates environment variables"
   - Solution: Only 2 env vars (`VAULT_ADDR`, `VAULT_TOKEN`)
   - All other config/secrets stored IN Vault
   - No `GH_TOKEN`, no custom env vars
   - GitHub PAT passed to `gh` CLI via stdin, not env var

2. **No Silent Failures** (Critical Constraint)
   - User constraint: "Hates silent failures" - must be explicit about fallbacks
   - Solution: Cache requires explicit `--use-cache` flag (no automatic fallback)
   - Default behavior: Fail immediately if Vault unreachable
   - With `--use-cache`: Use 24-hour cache with LOUD warning
   - Helpful hint shown when cache available but flag not used
   - Secrets NEVER cached (SSH keys, PAT always fresh from Vault)
   - Explicit > Implicit, User Choice > Automatic

3. **Vault-Centric Architecture**
   - Configuration: `secret/git-dual-remote/config` (cached 24h, requires `--use-cache`)
   - SSH keys: `secret/github/<repo>/ssh` (never cached)
   - GitHub PAT: `secret/github/<repo>/pat` (never cached)
   - Cache at `~/.cache/git-dual-remote/config.json` (updated on Vault success)
   - Single source of truth: Vault (cache is explicit emergency fallback)

---

**Document Version**: 2.2 (Critique-Revised + User Preferences + Integration Decision)
**Author**: lcgerke + Claude
**Date**: 2025-11-16
**Status**: Ready for Implementation (Integration into GitSetup)
