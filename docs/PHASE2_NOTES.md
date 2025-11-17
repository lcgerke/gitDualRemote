# Phase 2: GitHub Integration - Implementation Notes

## Overview

Phase 2 implements GitHub integration with dual-push functionality - the core feature of githelper.

## What Was Implemented

### 1. GitHub API Client (`internal/github/`)
- OAuth2 authentication with PAT
- Repository creation and management
- Connection testing
- Repository existence checking
- SSH URL retrieval

**Files**: `internal/github/client.go` (~100 lines)

### 2. SSH Key Management (`internal/vault/`)
- SSH key download from Vault to disk
- Automatic 600 permissions
- Per-repo and default key support
- Public key handling

**Files**: `internal/vault/ssh.go` (~70 lines)

### 3. Repository-Local SSH Configuration (`internal/git/`)
- `core.sshCommand` configuration
- Per-repository SSH keys (not global `~/.ssh/config`)
- IdentitiesOnly and StrictHostKeyChecking options

**Files**: `internal/git/ssh.go` (~15 lines)

### 4. **Dual-Push Configuration** (`internal/git/`) ⭐ KEY FEATURE
- Configure remote with multiple push URLs
- Single `git push` → pushes to both bare repo AND GitHub
- Git's native dual-push (not sequential application pushes)
- Verification of dual-push setup

**How it works**:
```bash
git remote set-url --push origin <bare-url>
git remote set-url --add --push origin <github-url>

# Now git push origin → pushes to BOTH URLs atomically
```

**Files**: `internal/git/dualpush.go` (~115 lines)

### 5. Hook Installation with Backup (`internal/hooks/`)
- Pre-push hook: Verifies connectivity before push
- Post-push hook: Updates sync status
- Automatic backup of existing hooks (`.githelper-backup`)
- Clean uninstall

**Files**: `internal/hooks/hooks.go` (~115 lines)

### 6. GitHub Commands (`cmd/githelper/`)

#### `githelper github setup <repo>`
- Retrieves SSH key from Vault
- Downloads to `~/.ssh/github_*`
- Configures repository-local SSH
- Gets PAT from Vault
- Creates GitHub API client
- Optionally creates GitHub repository (`--create`)
- **Sets up dual-push** (the magic!)
- Installs hooks with backup
- Updates state file

**Files**: `cmd/githelper/github_setup.go` (~240 lines)

#### `githelper github status <repo>`
- Shows GitHub integration status
- Displays sync status
- Lists push URLs
- JSON and human formats

**Files**: `cmd/githelper/github_status.go` (~100 lines)

#### `githelper github test <repo>`
- Tests GitHub API connection
- Verifies repository exists
- Used by pre-push hook

**Files**: `cmd/githelper/github_check.go` (~125 lines)

## Commands

### Setup GitHub Integration

```bash
# Basic setup (repo must exist on GitHub)
githelper github setup myproject

# Create GitHub repo automatically
githelper github setup myproject --create --user lcgerke

# Skip hooks
githelper github setup myproject --skip-hooks

# Specify different GitHub repo name
githelper github setup myproject --repo different-name
```

### Check Status

```bash
# Human format
githelper github status myproject

# JSON format
githelper github status myproject --format json
```

### Test Connection

```bash
# Test GitHub connectivity
githelper github test myproject

# Quiet mode (used in hooks)
githelper github test myproject --quiet
```

## Architecture Highlights

### Dual-Push: The Core Feature

**Before Phase 2**:
- User has to manually push to both remotes
- `git push origin && git push github`
- Error-prone, easy to forget

**After Phase 2**:
- Single `git push` → both remotes automatically
- Git handles it natively
- Atomic operation
- No custom code needed

**Configuration**:
```
[remote "origin"]
  url = gitmanager@lcgasgit:/srv/git/myproject.git
  fetch = +refs/heads/*:refs/remotes/origin/*
  pushurl = gitmanager@lcgasgit:/srv/git/myproject.git
  pushurl = git@github.com:lcgerke/myproject.git
```

### Repository-Local SSH

**Why not global `~/.ssh/config`**:
- Different repos can use different SSH keys
- No global pollution
- Repo-specific configuration
- Portable across machines

**How**:
```
git config core.sshCommand "ssh -i ~/.ssh/github_myproject -o IdentitiesOnly=yes"
```

### Hook Safety

**Backup Strategy**:
1. Check if hook exists
2. Rename to `.githelper-backup`
3. Install new hook
4. User can restore manually if needed

**Hooks Installed**:
- `pre-push`: Verifies GitHub connectivity before push (prevents failed pushes)
- `post-push`: Updates state file with sync status

## Testing (Without Vault/GitHub)

Phase 2 requires:
- Vault server (for SSH keys and PATs)
- GitHub account and PAT
- SSH key configured in Vault

**For testing without these**:
- Commands will fail with clear errors
- Build succeeds
- Help commands work
- Architecture is sound

## What's NOT in Phase 2

- ❌ Actual push/sync operations (need Phase 3)
- ❌ Divergence detection and recovery (Phase 3)
- ❌ Doctor command (Phase 4)
- ❌ Comprehensive testing (Phase 5)

## Stats

- **Lines of Code**: ~943 (Phase 2 implementation)
- **Files Added**: 10
- **New Dependencies**: `github.com/google/go-github/v56`, `golang.org/x/oauth2`
- **Commands**: 3 new subcommands

## Next: Phase 3

Phase 3 will add:
- Synchronous dual-push validation
- Smart divergence detection
- Partial failure handling
- `githelper github sync --retry-github`
- State tracking for sync status
