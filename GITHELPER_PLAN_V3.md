# GitHelper - Unified Git Management Tool - Implementation Plan V3

## Overview

A comprehensive Go CLI tool that manages both bare repository workflows and GitHub dual-remote synchronization. This unified tool (`githelper`) combines repository lifecycle management with seamless GitHub backup integration.

**Date**: 2025-11-16
**Version**: 3.1 (Post-Critique Resolutions)
**Status**: Architecture Finalized with Resolutions, Ready for Phase 0

## Architectural Decisions (From /decide Session)

### Decision Summary

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| 1 | **Tool Scope** | Unified `githelper` with subcommands | Integrated experience, shared state, cohesive UX (~5000 LOC) |
| 2 | **Vault Role** | Everything in Vault + cache-by-default | Single source of truth across machines, centralized config |
| 3 | **Offline Strategy** | Cache with visible staleness indicators | Works offline, shows "Vault (cached 15m ago) ⚡", no blocking |
| 4 | **Authentication** | Global SSH key + optional per-repo override | Simple default (one key), flexible override when needed |
| 5 | **Push Strategy** | Synchronous dual-push | Explicit failures, no silent issues, both remotes or error |
| 6 | **Failure Recovery** | Smart divergence detection | Git compares commit graphs, pushes missing commits naturally |
| 7 | **Platform Support** | Linux only | Focused testing, simple paths, standard Unix permissions |
| 8 | **Output Modes** | TTY detection + manual overrides | Auto JSON for agents/pipes, human for TTY, `--format` overrides |
| 9 | **V1 Scope** | Full plan (no cuts) | Complete feature set: doctor, hooks, tables, all diagnostics |
| 10 | **State Management** | Vault + cache + git config + state file | Centralized cross-repo visibility in `~/.githelper/state.yaml` |
| 11 | **Security Model** | Verification commands as credential tracker | `githelper doctor --credentials` shows where all credentials live |
| 12 | **GitHub Integration** | Direct API via go-github library | Seamless integration, no `gh` CLI dependency, full control |
| 13 | **Hook Installation** | Automatic (no prompt) | Opinionated but reliable, `githelper hooks uninstall` available |
| 14 | **Doctor Command** | Comprehensive diagnostics | Vault, SSH, git config, remotes, credentials, sync status |

## Critique Resolutions (2025-11-16)

Following architectural review, the following clarifications and updates were made:

### Resolution #3: State Management Ownership Model

**Clarification on Decision 10:** Hybrid ownership model to prevent divergence issues.

**Ownership Rules:**
- **Git config (`.git/config`)**: Source of truth for all git operations
  - Remote URLs (authoritative)
  - SSH command configuration (authoritative)
  - Branch tracking information
- **State file (`~/.githelper/state.yaml`)**: Metadata and inventory only
  - Repository inventory (which repos githelper manages)
  - Creation timestamps, repository types
  - Cached sync status (hints, not authoritative)
- **Vault**: Configuration and secrets (fetched as needed)
  - Global configuration
  - SSH keys and PATs
  - Per-repo credential overrides

**Conflict Resolution:** If git config and state file diverge, git config always wins. The tool will detect divergence and auto-repair state file from git config.

**Reconstruction:** State file can be reconstructed by scanning git repositories if lost.

### Resolution #4: Hook Installation Safety

**Update to Decision 13:** Automatic installation with backup protection.

**Behavior:**
1. Check for existing hooks (`.git/hooks/pre-push`, `.git/hooks/post-push`)
2. If exists: Automatically backup to `.git/hooks/<hook>.githelper-backup`
3. Install githelper hooks
4. Output: `✓ Installed hooks (backed up existing pre-push to pre-push.githelper-backup)`
5. User can manually restore from backup if needed

**Rationale:** Balances automation (no prompts) with safety (preserves user's work). More straightforward than hook chaining while preventing data loss.

### Resolution #6: SSH Key Management Philosophy

**Clarification on Decision 11:** Vault optimized for convenience, not security theater.

**Design Goal:** Use Vault as centralized configuration database for multi-machine convenience, not as ephemeral secrets engine.

**Workflow:**
- SSH keys stored in Vault: `secret/githelper/github/default_ssh`
- On first use: Download to `~/.ssh/github_*` with 600 permissions
- Subsequent uses: Key remains on disk, git uses directly
- New machine: `githelper doctor` auto-downloads keys from Vault

**Accepted Trade-off:** Keys exist on disk (not ephemeral). This is acceptable because:
- SSH keys are long-lived credentials (not rotated frequently)
- Vault provides convenience (configure once, use everywhere)
- Standard SSH security practices apply (encrypted disk, 600 permissions)

### Resolution #8: Testing Strategy

**Expanded Decision:** Comprehensive testing with full edge case coverage.

**Testing Approach:**
- **Unit tests**: All internal packages with extensive mocking
- **Edge case coverage**: All failure modes tested
  - Vault unreachable
  - GitHub API failures and rate limits
  - SSH connection failures (partial and complete)
  - Divergence scenarios (ahead, behind, diverged)
  - Filesystem permission issues
  - Corrupted state files
- **Integration tests**: End-to-end workflows with mocked external services
- **Test infrastructure**: Mock Vault server, GitHub API, SSH connections

**Timeline Impact:** Allocate 3-4 weeks for comprehensive test development (Phase 5 extended).

### Resolution #9: Git Operations Implementation

**Update to Technology Stack:** Pure go-git with Phase 0 validation.

**Decision:** Use `github.com/go-git/go-git/v5` for all git operations (no CLI shelling).

**Benefits:**
- No external git binary dependency
- Programmatic control over all operations
- Better error handling and testability
- Consistent behavior across environments

**Risk Mitigation:** Add Phase 0 spike to validate go-git capabilities before architecture commitment.

**Phase 0 Deliverables:**
- Proof-of-concept: Remote management with go-git
- Validation: Dual-push functionality
- Test: SSH key configuration via go-git
- Test: Divergence detection (commit graph comparison)
- Decision point: Proceed with go-git or fall back to CLI wrapper

**Timeline:** 2-3 days before Phase 1.

## Tool Architecture

### Command Structure

```bash
# Repository management
githelper repo create myproject --type go
githelper repo list
githelper repo status myproject
githelper repo archive myproject
githelper repo delete myproject

# GitHub integration
githelper github setup myproject --user lcgerke
githelper github status myproject
githelper github sync myproject
githelper github test myproject

# Unified workflow (create bare repo + GitHub dual-push in one command)
githelper repo create myproject --type go --with-github lcgerke

# Diagnostics and maintenance
githelper doctor                        # Full health check
githelper doctor --credentials          # Credential inventory (where everything is stored)
githelper doctor --repo myproject       # Repo-specific deep dive

# Hooks
githelper hooks install    # Manually install (already done automatically)
githelper hooks uninstall  # Remove hooks if desired
```

### File Structure

```
githelper/                              # This repository (renamed from gitDualRemote)
├── cmd/
│   └── githelper/
│       ├── main.go                     # Entry point, cobra root
│       ├── repo.go                     # Repo subcommand root
│       ├── repo_create.go              # Create command
│       ├── repo_list.go                # List command
│       ├── repo_status.go              # Status command
│       ├── repo_archive.go             # Archive command
│       ├── github.go                   # GitHub subcommand root
│       ├── github_setup.go             # Setup dual-remote
│       ├── github_status.go            # GitHub sync status
│       ├── github_sync.go              # Manual sync
│       ├── github_test.go              # Test connectivity
│       ├── doctor.go                   # Doctor command
│       └── hooks.go                    # Hook management
├── internal/
│   ├── config/
│   │   ├── config.go                   # Vault-backed config
│   │   └── cache.go                    # Config cache (24h TTL)
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
│   │   ├── client.go                   # GitHub API (go-github)
│   │   ├── repo.go                     # Repository management
│   │   └── auth.go                     # PAT authentication
│   ├── ui/
│   │   ├── output.go                   # Colorized output (TTY detection)
│   │   ├── table.go                    # Table formatting
│   │   ├── spinner.go                  # Progress indicators
│   │   └── prompt.go                   # User confirmations
│   ├── state/
│   │   ├── state.go                    # State file management
│   │   └── repo.go                     # Repository state tracking
│   └── doctor/
│       ├── diagnostics.go              # Health checks
│       ├── credentials.go              # Credential inventory
│       └── fixes.go                    # Auto-fix operations
├── docs/
│   ├── PLAN.md                         # This document
│   ├── WORKFLOW.md                     # Daily workflow guide
│   └── VAULT.md                        # Vault setup guide
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Configuration & State Management

### Vault Configuration (Source of Truth)

**All configuration stored in Vault:**

```bash
vault kv put secret/githelper/config \
  github_username="lcgerke" \
  bare_repo_pattern="gitmanager@lcgasgit:/srv/git/{repo}.git" \
  default_visibility="private" \
  auto_create_github=true \
  test_before_push=true \
  sync_on_setup=true \
  retry_on_partial_failure=true
```

**Authentication secrets (per-repo):**

```bash
# Global default SSH key (used for all repos unless overridden)
vault kv put secret/githelper/github/default_ssh \
  private_key=@~/.ssh/github_default \
  public_key=@~/.ssh/github_default.pub

# Global default PAT
vault kv put secret/githelper/github/default_pat \
  token="ghp_xxxxxxxxxxxx" \
  scopes="repo,admin:repo_hook"

# Per-repo override (only if needed for multi-account scenarios)
vault kv put secret/githelper/github/myrepo/ssh \
  private_key=@~/.ssh/github_myrepo \
  public_key=@~/.ssh/github_myrepo.pub

vault kv put secret/githelper/github/myrepo/pat \
  token="ghp_yyyyyyyyyyyy"
```

**Lookup order:**
1. Check `secret/githelper/github/<repo>/ssh` (repo-specific)
2. Fall back to `secret/githelper/github/default_ssh` (global)
3. Same pattern for PAT

### Local Cache (Automatic, 24h TTL)

**Location:** `~/.githelper/cache/config.json`

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

**Cache behavior:**
- Automatically updated when Vault accessible
- Used automatically with staleness indicator
- Shows age: "Configuration: Vault (cached 15m ago) ⚡"
- 24-hour TTL (configurable)
- **Secrets NEVER cached** (SSH keys, PATs always fetched fresh)

### State File (Cross-Repo Tracking)

**Location:** `~/.githelper/state.yaml`

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

### Git Config (Repository-Local)

Each repository gets:

```bash
# Repository-local SSH command (NOT global ~/.ssh/config)
git config core.sshCommand "ssh -i ~/.ssh/github_myrepo -o IdentitiesOnly=yes"

# Dual-push remotes
[remote "origin"]
  url = gitmanager@lcgasgit:/srv/git/myproject.git
  fetch = +refs/heads/*:refs/remotes/origin/*
  pushurl = gitmanager@lcgasgit:/srv/git/myproject.git
  pushurl = git@github.com:lcgerke/myproject.git

[remote "github"]
  url = git@github.com:lcgerke/myproject.git
  fetch = +refs/heads/*:refs/remotes/github/*
```

## Dual-Push Strategy

### Synchronous Push (Decision 5)

```bash
$ git push
# Pushes to both remotes synchronously:
# 1. Push to bare repo (fast, local, ~5ms)
# 2. Push to GitHub (slow, internet, ~200ms)
# Both must succeed or clear error shown
```

### Smart Divergence Detection (Decision 6)

When `githelper github sync --retry-github` is run:

1. Fetch from both remotes
2. Compare commit graphs (`git rev-list origin/main..github/main`)
3. Push only missing commits
4. No force pushes, no manual state tracking
5. Git handles it naturally

**Example scenario:**

```
User: Makes 3 commits, pushes
Bare repo: ✅ Receives 3 commits
GitHub: ❌ Network down

User: Makes 2 more commits

User: Runs 'githelper github sync --retry-github'
Tool: Fetches both remotes
      Detects GitHub is 5 commits behind
      Pushes all 5 commits to GitHub
      Verifies sync
```

### Partial Failure Handling

```bash
$ git push
✅ Bare repo: Pushed 3 commits to gitmanager@lcgasgit
❌ GitHub: Push failed (network unreachable)

⚠️  Partial Push Success

Status:
  Bare repo is authoritative and up-to-date.
  GitHub is behind by 3 commits.

Recovery:
  When GitHub is reachable, run:
    githelper github sync --retry-github

  Or continue working normally. Next successful push will sync both.
```

## Output Modes (Decision 8)

### TTY Detection (Automatic)

```go
func getOutputFormat() string {
    if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
        return "human"  // Interactive terminal
    }
    return "json"  // Piped to file/script/agent
}
```

**Behavior:**
- `githelper status` in terminal → colorized tables
- `githelper status | jq` → JSON output
- Agent calling tool → JSON automatically

### Manual Overrides

```bash
--format=json        # Force JSON even in TTY
--format=human       # Force human even when piped
--no-color           # Human format without ANSI codes
--quiet              # Minimal output
--verbose            # Detailed output
```

**Example outputs:**

**Human (TTY):**
```
Remote Configuration:
┌────────┬────────────────────────────────┬────────┬─────────┐
│ REMOTE │ URL                            │ TYPE   │ STATUS  │
├────────┼────────────────────────────────┼────────┼─────────┤
│ origin │ gitmanager@lcgasgit:/srv/git/…│ fetch  │ ✓ reach │
│ github │ git@github.com:lcgerke/myrepo  │ both   │ ✓ reach │
└────────┴────────────────────────────────┴────────┴─────────┘
```

**JSON (piped/agent):**
```json
{
  "remotes": [
    {"name": "origin", "url": "gitmanager@lcgasgit:/srv/git/myrepo.git", "type": "fetch", "status": "reachable"},
    {"name": "github", "url": "git@github.com:lcgerke/myrepo.git", "type": "both", "status": "reachable"}
  ]
}
```

## Doctor Command (Decision 14)

### Comprehensive Diagnostics

```bash
$ githelper doctor

🔍 GitHelper Diagnostic Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Configuration:
  ✓ Vault connectivity (http://core:8200)
  ✓ Configuration loaded (cached 15m ago)

Credentials:
  ✓ Global SSH key: secret/githelper/github/default_ssh
    → Disk location: ~/.ssh/github_default (600)
  ✓ Global PAT: secret/githelper/github/default_pat
  ✓ Repo override: secret/githelper/github/myrepo/ssh
    → Disk location: ~/.ssh/github_myrepo (600)

Git Configuration:
  ✓ Repository detected: /home/user/myrepo
  ✓ core.sshCommand configured
  ✓ Dual-push remotes configured

Remote Connectivity:
  ✓ Bare repo reachable (12ms)
  ✓ GitHub SSH authenticated as lcgerke

Sync Status:
  ✓ All remotes synchronized
  ✓ No pending GitHub retries

Hooks:
  ✓ pre-push hook installed
  ✓ post-push hook installed

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ All systems healthy
```

### Credential Inventory (Decision 11)

```bash
$ githelper doctor --credentials

📋 Credential Inventory
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Vault Secrets:
┌─────────────────────────────────────────────┬──────────────┬──────────────┐
│ PATH                                        │ TYPE         │ LAST USED    │
├─────────────────────────────────────────────┼──────────────┼──────────────┤
│ secret/githelper/github/default_ssh         │ SSH Key      │ 2h ago       │
│ secret/githelper/github/default_pat         │ GitHub PAT   │ 2h ago       │
│ secret/githelper/github/myrepo/ssh          │ SSH Key      │ 30m ago      │
└─────────────────────────────────────────────┴──────────────┴──────────────┘

Disk Locations:
┌─────────────────────────────────┬──────────┬──────────────┐
│ FILE                            │ PERMS    │ USED BY      │
├─────────────────────────────────┼──────────┼──────────────┤
│ ~/.ssh/github_default           │ 600      │ 3 repos      │
│ ~/.ssh/github_myrepo            │ 600      │ myrepo only  │
└─────────────────────────────────┴──────────┴──────────────┘

Repositories Using Each Credential:
  github_default: project1, project2, project3
  github_myrepo: myrepo

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Purpose: Track where all credentials live (not security audit)
```

## Hook Installation (Decision 13 + Resolution #4)

### Automatic Installation with Backup

Hooks installed automatically during `githelper github setup`:

**Installation Process:**
1. Check for existing hooks in `.git/hooks/`
2. If found, backup to `.git/hooks/<hook>.githelper-backup`
3. Install githelper hook
4. Display backup confirmation

**Example output:**
```bash
$ githelper github setup myproject --user lcgerke
...
✓ Backed up existing pre-push hook to pre-push.githelper-backup
✓ Installed pre-push hook
✓ Installed post-push hook
```

**`.git/hooks/pre-push`:**
```bash
#!/bin/bash
# Verify both remotes reachable before push
githelper github test --quiet || {
    echo "⚠️  Remote connectivity issue detected"
    echo "Run 'githelper github test' for details"
    exit 1
}
```

**`.git/hooks/post-push`:**
```bash
#!/bin/bash
# Update sync status in state file
githelper github status --update-state
```

### Manual Hook Management

**Restore from backup:**
```bash
# User manually restores
$ mv .git/hooks/pre-push.githelper-backup .git/hooks/pre-push
```

**Uninstall:**
```bash
$ githelper hooks uninstall
✓ Removed .git/hooks/pre-push
✓ Removed .git/hooks/post-push
Note: Backups remain at *.githelper-backup if you need to restore
```

**Reinstall (overwrites existing):**
```bash
$ githelper hooks install
✓ Backed up existing pre-push hook to pre-push.githelper-backup
✓ Installed hooks
```

## Platform Support (Decision 7)

**Linux only** for V1:
- Focused testing matrix
- Standard Unix paths (`~/.ssh/`, `~/.githelper/`)
- Direct use of `chmod 600`, standard permissions
- No cross-platform abstraction complexity

**Future:** macOS support (similar enough, minimal changes needed)

## Technology Stack

```go
require (
    github.com/spf13/cobra v1.8.0           // CLI framework
    go.uber.org/zap v1.26.0                 // Logging
    github.com/fatih/color v1.16.0          // Colors
    github.com/briandowns/spinner v1.23.0   // Spinners
    github.com/olekukonko/tablewriter v0.0.5 // Tables
    github.com/hashicorp/vault/api v1.10.0  // Vault client
    github.com/google/go-github/v56 v56.0.0 // GitHub API
    gopkg.in/yaml.v3 v3.0.1                 // State file
    github.com/go-git/go-git/v5 v5.11.0     // Git operations
)
```

## Security Model (Decision 11)

### What We Store Where

**Vault (source of truth):**
- Configuration (cached for 24h)
- SSH keys (never cached, always fetched fresh)
- GitHub PATs (never cached, always fetched fresh)

**Disk:**
- SSH keys: `~/.ssh/github_*` with 600 permissions
- Configuration cache: `~/.githelper/cache/config.json`
- State: `~/.githelper/state.yaml`

**Git config:**
- Repository-local SSH command
- Remote URLs

### User Responsibilities

- Secure Vault server
- Encrypted disk for `~/.ssh/`
- Rotate SSH keys and PATs periodically
- Protect `VAULT_TOKEN` environment variable

### Credential Tracking (Not Security Theater)

The doctor command's `--credentials` flag is explicitly for **tracking where credentials are**, not security auditing. This helps answer:
- "Where did I store that key?"
- "Which repos use which credentials?"
- "What's in Vault vs on disk?"

## Implementation Phases

### Phase 0: Go-git Validation Spike (2-3 days)
- [ ] Proof-of-concept: Create and clone bare repository using go-git
- [ ] Validation: Configure multiple push URLs (dual-push setup)
- [ ] Test: SSH key configuration and authentication via go-git
- [ ] Test: Fetch from multiple remotes and compare commit graphs
- [ ] Test: Push to multiple remotes programmatically
- [ ] Edge cases: Handle auth failures, network errors, divergence
- [ ] **Decision point**: Proceed with go-git or pivot to CLI wrapper
- [ ] Document findings and API patterns

### Phase 1: Core Infrastructure (Week 1)
- [ ] Project scaffolding (cobra, zap, structure)
- [ ] Vault integration (config retrieval, caching)
- [ ] State file management (`~/.githelper/state.yaml`)
  - [ ] Implement conflict detection (state file vs git config)
  - [ ] Auto-repair: sync state file from git config when diverged
- [ ] TTY detection and output modes (human/JSON)
- [ ] Basic `githelper repo create` (bare repo only)

### Phase 2: GitHub Integration (Week 2)
- [ ] GitHub API client (go-github)
- [ ] SSH key retrieval from Vault (global + per-repo)
- [ ] PAT retrieval from Vault
- [ ] Repository-local SSH config (`core.sshCommand`)
- [ ] Dual-push remote configuration
- [ ] `githelper github setup` command
- [ ] Hook installation with backup logic
  - [ ] Detect existing hooks
  - [ ] Backup to `.githelper-backup` suffix
  - [ ] Install githelper hooks

### Phase 3: Sync & Recovery (Week 3)
- [ ] Synchronous dual-push implementation
- [ ] Smart divergence detection
- [ ] Partial failure handling
- [ ] State tracking (sync status, retry flags)
- [ ] `githelper github sync --retry-github`

### Phase 4: Diagnostics & Polish (Week 4)
- [ ] Doctor command (full health check)
- [ ] Credential inventory (`--credentials`)
- [ ] Repo-specific diagnostics (`--repo`)
- [ ] Auto-fix operations
- [ ] Comprehensive error messages
- [ ] Help system

### Phase 5: Testing & Documentation (3-4 weeks)
- [ ] **Unit tests** (each internal package with mocks)
  - [ ] Vault client (mock Vault server)
  - [ ] GitHub client (mock API responses)
  - [ ] SSH operations (mock connections)
  - [ ] Git operations (mock go-git interactions)
  - [ ] State management (conflict resolution)
- [ ] **Edge case tests** (all failure modes)
  - [ ] Vault unreachable (use cache)
  - [ ] GitHub API failures (rate limits, auth failures)
  - [ ] SSH failures (partial and complete)
  - [ ] Divergence scenarios (ahead, behind, diverged)
  - [ ] Filesystem permission issues
  - [ ] Corrupted state files
  - [ ] Hook backup scenarios (existing hooks)
- [ ] **Integration tests** (end-to-end workflows)
  - [ ] Full repository creation + GitHub setup
  - [ ] Dual-push success and partial failure
  - [ ] Sync recovery after divergence
  - [ ] Doctor diagnostics with various failure modes
- [ ] **Test infrastructure**
  - [ ] Mock Vault server setup
  - [ ] Mock GitHub API server
  - [ ] Test fixture repositories
- [ ] **Documentation**
  - [ ] Workflow guide (`docs/WORKFLOW.md`)
  - [ ] Vault setup guide (`docs/VAULT.md`)
  - [ ] README with examples
  - [ ] Release automation

## Success Criteria

### Functional Requirements
- ✅ Create bare repos and local clones
- ✅ Set up GitHub dual-remote automatically
- ✅ Retrieve credentials from Vault (global + per-repo override)
- ✅ Configure repository-local SSH (no global changes)
- ✅ Synchronous dual-push with clear errors
- ✅ Smart divergence detection and recovery
- ✅ Comprehensive diagnostics
- ✅ Credential inventory tracking
- ✅ Works offline with cached config

### Non-Functional Requirements
- ✅ Beautiful human output (colors, tables, spinners)
- ✅ Machine-readable JSON output (automatic TTY detection)
- ✅ Fast execution (< 10 seconds for setup)
- ✅ Comprehensive doctor command
- ✅ Linux-only, focused platform support
- ✅ Well-tested (unit + integration)
- ✅ Clear, maintainable code

## Usage Examples

### Initial Setup

```bash
# Create bare repo + GitHub dual-remote in one command
$ githelper repo create myproject --type go --with-github lcgerke

🚀 Creating Repository: myproject
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Configuration: Vault (live) ✓

✓ Created bare repository: gitmanager@lcgasgit:/srv/git/myproject.git
✓ Cloned to: /home/user/repos/myproject
✓ Retrieved SSH key from Vault (global default)
✓ Configured repository-local SSH
✓ Created GitHub repository: https://github.com/lcgerke/myproject
✓ Configured dual-push remotes
✓ Installed pre-push and post-push hooks
✓ Pushed initial commit to both remotes

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Repository ready! cd repos/myproject
```

### Daily Workflow

```bash
# Check status
$ githelper github status myproject
Configuration: Vault (cached 15m ago) ⚡

Sync State:
  ✓ Bare repo: up to date
  ✓ GitHub: up to date

# Work normally
$ cd repos/myproject
$ git commit -m "Add feature"
$ git push  # Pushes to both remotes automatically

# If GitHub was down
$ githelper github sync --retry-github
✓ Pushed 3 commits to GitHub
✓ All remotes synchronized
```

### Diagnostics

```bash
# Full health check
$ githelper doctor

# Credential inventory
$ githelper doctor --credentials

# Repo-specific deep dive
$ githelper doctor --repo myproject
```

---

**Version**: 3.1
**Status**: Architecture Finalized with Critique Resolutions
**Ready**: Implementation Phase 0 (go-git validation spike)
**Timeline**: Phase 0 (2-3 days) → Phases 1-5 (7-8 weeks)
**Author**: lcgerke + Claude (post-critique resolutions)

## Change Summary (3.0 → 3.1)

1. **State Management**: Clarified hybrid ownership (git config authoritative, state file reconstructable)
2. **Hook Installation**: Added automatic backup mechanism for safety
3. **SSH Keys**: Documented Vault-for-convenience philosophy (not ephemeral secrets)
4. **Testing**: Committed to comprehensive edge case coverage (3-4 weeks)
5. **Git Operations**: Pure go-git with Phase 0 validation spike before commitment
6. **Timeline**: Extended Phase 5 for comprehensive testing, added Phase 0
