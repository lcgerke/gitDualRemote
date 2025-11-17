# Phase 1 Testing Guide

## Overview

Phase 1 implements the core infrastructure for githelper:
- ✅ Project scaffolding with Cobra CLI
- ✅ Vault integration with caching
- ✅ State file management
- ✅ TTY detection and output modes
- ✅ Git CLI wrapper
- ✅ Basic `githelper repo create` command

## Prerequisites

1. **Git** (>= 2.0) must be installed
2. **Go** (>= 1.21) for building
3. **Vault configuration** (either running Vault server or cached config)

## Setup Test Environment

### 1. Build githelper

```bash
go build -o githelper ./cmd/githelper
```

### 2. Create test cache (simulates Vault)

```bash
mkdir -p ~/.githelper/cache
cat > ~/.githelper/cache/config.json << 'EOF'
{
  "config": {
    "github_username": "lcgerke",
    "bare_repo_pattern": "/tmp/bare-repos/{repo}.git",
    "default_visibility": "private",
    "auto_create_github": false,
    "test_before_push": true,
    "sync_on_setup": true,
    "retry_on_partial_failure": true
  },
  "fetched_at": "2025-11-16T21:00:00Z"
}
EOF
```

### 3. Create bare repos directory

```bash
mkdir -p /tmp/bare-repos
```

### 4. Configure git user (if not already done)

```bash
git config --global user.name "Your Name"
git config --global user.email "your@email.com"
```

## Test Commands

### Test 1: Help and Version

```bash
./githelper --help
./githelper repo --help
./githelper repo create --help
```

**Expected**: Help text displays correctly

### Test 2: List Empty Repositories

```bash
./githelper repo list --format human
```

**Expected Output**:
```
No repositories found.
Create one with: githelper repo create <name>
```

**JSON format**:
```bash
./githelper repo list --format json
```

**Expected**:
```json
{
  "repositories": []
}
```

### Test 3: Create Repository (Basic)

```bash
./githelper repo create demo-repo \
  --clone-dir /tmp/demo-clone \
  --format human
```

**Expected Output**:
```
🚀 Creating Repository: demo-repo
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠ Configuration: Vault (cached Xm ago) ⚡

Creating bare repository at /tmp/bare-repos/demo-repo.git...
✓ Created bare repository: /tmp/bare-repos/demo-repo.git
Cloning to /tmp/demo-clone...
✓ Cloned to: /tmp/demo-clone
✓ Created initial commit
✓ Pushed initial commit to bare repository

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✓ Repository ready! cd /tmp/demo-clone
```

**Verify**:
```bash
# Check clone directory exists
ls -la /tmp/demo-clone/

# Check bare repository exists and has commit
cd /tmp/bare-repos/demo-repo.git && git log --oneline
```

### Test 4: Create Go Repository

```bash
./githelper repo create go-project \
  --type go \
  --clone-dir /tmp/go-project \
  --format human
```

**Expected**: Additional output line:
```
Initializing Go module...
✓ Initialized Go repository
```

**Verify files created**:
```bash
ls /tmp/go-project/
# Should show: README.md go.mod .gitignore

cat /tmp/go-project/go.mod
# Should show: module github.com/lcgerke/go-project
```

### Test 5: List Repositories

```bash
./githelper repo list --format human
```

**Expected Output**:
```
Managed Repositories

📁 demo-repo
   Path:    /tmp/demo-clone
   Remote:  /tmp/bare-repos/demo-repo.git
   Created: 2025-11-16 21:12:11
   Type:

📁 go-project
   Path:    /tmp/go-project
   Remote:  /tmp/bare-repos/go-project.git
   Created: 2025-11-16 21:15:23
   Type:    go

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total: 2 repositories
```

### Test 6: JSON Output

```bash
./githelper repo list --format json
```

**Expected**: Structured JSON output with repository details

### Test 7: State File

```bash
cat ~/.githelper/state.yaml
```

**Expected**: YAML file containing repository state

## What Works in Phase 1

✅ **CLI Framework**
- Cobra-based command structure
- Global flags (--format, --no-color, --quiet, --verbose)
- Subcommands (repo create, repo list)

✅ **Configuration**
- Vault integration with fallback to cache
- 24-hour cache TTL
- Staleness indicators

✅ **Output Formatting**
- TTY detection (automatic JSON for pipes)
- Manual format override (--format human|json)
- Colorized output (with --no-color option)
- Success/Error/Warning/Info methods

✅ **State Management**
- YAML state file at ~/.githelper/state.yaml
- Repository inventory tracking
- Metadata storage (path, remote, created, type)

✅ **Git Operations**
- Git CLI wrapper
- Bare repository creation
- Repository cloning
- Initial commit and push
- Git version check

✅ **Repository Types**
- Generic repositories (README only)
- Go repositories (go.mod, .gitignore)
- Extensible type system

## What's NOT in Phase 1

❌ GitHub integration (Phase 2)
❌ Dual-push configuration (Phase 2)
❌ SSH key management (Phase 2)
❌ Doctor command (Phase 4)
❌ Hook installation (Phase 2)
❌ Sync/recovery commands (Phase 3)

## Troubleshooting

### Error: "vault unreachable and no valid cache"

**Solution**: Create the cache file as shown in Setup step 2

### Error: "git is not installed"

**Solution**: Install git (>= 2.0)

### Error: "failed to create bare repository"

**Solution**: Ensure parent directory exists:
```bash
mkdir -p /tmp/bare-repos
```

### Error: "directory already exists"

**Solution**: Choose a different --clone-dir or remove existing directory

## Cleanup

```bash
# Remove test repositories
rm -rf /tmp/demo-clone /tmp/go-project /tmp/bare-repos

# Remove state file
rm ~/.githelper/state.yaml

# Remove cache
rm -rf ~/.githelper/cache
```

## Phase 1 Success Criteria

All criteria met ✅:

- [x] Project builds successfully
- [x] Help commands display correctly
- [x] Can create bare repositories
- [x] Can clone repositories locally
- [x] State file tracks repositories
- [x] Configuration cached from Vault
- [x] Multiple output formats work
- [x] Repository types (go) initialized correctly
- [x] Git operations work via CLI wrapper

## Next Steps

**Phase 2** will add:
- GitHub API integration
- SSH key retrieval from Vault
- Dual-push remote configuration
- `githelper github setup` command
- Hook installation
