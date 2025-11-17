# GitHelper Workflow Guide

**Version:** 4.0
**Date:** 2025-11-16

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Initial Setup](#initial-setup)
- [Daily Workflows](#daily-workflows)
- [Recovery Scenarios](#recovery-scenarios)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

GitHelper provides a unified workflow for managing repositories with automatic GitHub backup through dual-push. This guide covers common workflows from initial setup through daily use and recovery scenarios.

## Prerequisites

### Required Software

- **Git**: Version 2.0 or later
- **Go**: Version 1.21 or later (for building)
- **Vault** (Optional): For centralized configuration

### Environment Variables

```bash
# Vault configuration (if using Vault)
export VAULT_ADDR="http://your-vault-server:8200"
export VAULT_TOKEN="your-vault-token"
```

### Building GitHelper

```bash
# Clone repository
git clone https://github.com/lcgerke/githelper
cd githelper

# Build
make build

# Install (optional)
sudo cp githelper /usr/local/bin/
```

## Initial Setup

### 1. Configure Vault (if using centralized config)

Store default configuration in Vault:

```bash
# Default SSH key for GitHub
vault kv put secret/githelper/github/default_ssh \
  private_key=@~/.ssh/id_ed25519 \
  public_key=@~/.ssh/id_ed25519.pub

# Default GitHub PAT
vault kv put secret/githelper/github/default_pat \
  token="ghp_your_personal_access_token_here"

# Global configuration
vault kv put secret/githelper/config \
  github_username="your-github-username" \
  bare_repo_pattern="gitmanager@server:/srv/git/{name}.git" \
  default_visibility="private" \
  auto_create_github=true
```

### 2. Verify Installation

```bash
# Check githelper is working
githelper --version

# Run diagnostics
githelper doctor

# Expected output:
# 🔍 GitHelper Diagnostic Report
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Git Installation:
#   ✓ Git installed and accessible
# ...
# ✅ All systems healthy
```

## Daily Workflows

### Creating a New Repository

#### Option 1: Create with GitHub Integration

```bash
# Create repository with automatic GitHub setup
githelper repo create myproject --type go --with-github yourusername

# This will:
# 1. Create bare repository on your server
# 2. Clone locally
# 3. Initialize with project structure
# 4. Set up SSH keys
# 5. Create GitHub repository
# 6. Configure dual-push
# 7. Install hooks
# 8. Make initial commit
# 9. Push to both remotes

# Navigate to your new repo
cd repos/myproject
```

#### Option 2: Create Bare Repo, Add GitHub Later

```bash
# Create bare repo only
githelper repo create myproject --type go

# Work locally...
cd repos/myproject

# Add GitHub integration when ready
githelper github setup myproject \
  --user yourusername \
  --repo myproject \
  --create \
  --private
```

### Working with Dual-Push Repositories

Once set up, dual-push is **completely transparent**:

```bash
# Normal git workflow - no changes!
git add .
git commit -m "Add new feature"
git push

# Pushes to BOTH:
# - Your bare repository (fast, local)
# - GitHub (backup, collaboration)
```

**What Happens Behind the Scenes:**

1. **Pre-push hook** checks connectivity to both remotes
2. **Git's native dual-push** pushes to both URLs sequentially
3. **Post-push hook** updates sync status in state file

### Checking Status

```bash
# Check GitHub integration status
githelper github status myproject

# Output:
# GitHub Status: myproject
# GitHub: yourusername/myproject
# Status: synced
# Last Sync: 2025-11-16 14:30:00
#
# Push URLs:
#   1. gitmanager@server:/srv/git/myproject.git
#   2. git@github.com:yourusername/myproject.git
```

### Testing Connectivity

```bash
# Test both remotes
githelper github check myproject

# Output:
# Testing GitHub connectivity for: myproject
# ✓ Bare repo reachable (5ms)
# ✓ GitHub SSH authenticated as yourusername
# ✅ All remotes accessible
```

### Listing Repositories

```bash
# List all managed repositories
githelper repo list

# JSON output for automation
githelper repo list --format=json
```

## Recovery Scenarios

### Scenario 1: GitHub Push Failed (Network Issues)

**Situation:** You pushed commits but GitHub was unreachable.

```bash
# Symptoms:
$ git push
✅ Bare repo: Pushed 3 commits to gitmanager@server
❌ GitHub: Network unreachable
error: failed to push some refs
```

**Recovery:**

```bash
# Wait until network is available, then sync
githelper github sync myproject

# Output:
# 🔄 Syncing GitHub for: myproject
# Checking divergence between bare and GitHub (branch: main)...
# ⚠ Divergence detected:
#   Bare is ahead by 3 commit(s)
# Syncing 3 commit(s) to GitHub...
# Verifying sync...
# ✅ Synced 3 commit(s) to GitHub
#   Commit: a1b2c3d4
```

### Scenario 2: Check Divergence Status

```bash
# See if GitHub is behind
githelper github status myproject --update-state

# Or manually check with sync (without pushing)
githelper github sync myproject --branch main
```

### Scenario 3: Hooks Accidentally Deleted

```bash
# Run doctor with auto-fix
githelper doctor --auto-fix

# Output:
# 🔧 Auto-Fix:
#   Found 2 fixable issue(s):
#     1. [low] myproject - Hook not installed: pre-push
#     2. [low] myproject - Hook not installed: post-push
#   ✓ Fixed 2 issue(s)
```

### Scenario 4: Divergent Histories (Manual Resolution Needed)

**Situation:** Commits were pushed directly to GitHub (bypassing bare repo).

```bash
# Attempt sync
githelper github sync myproject

# Output:
# ❌ GitHub has commits that bare repository doesn't have
#    Manual resolution required
#
# This typically means commits were pushed directly to GitHub.
# Resolve manually by:
#   1. git fetch github
#   2. git merge github/main (or rebase)
#   3. git push
```

**Manual Resolution:**

```bash
# Fetch from GitHub
git fetch github

# Check what's different
git log github/main..main    # What bare has that GitHub doesn't
git log main..github/main    # What GitHub has that bare doesn't

# Merge or rebase
git merge github/main
# or
git rebase github/main

# Push to both (will sync automatically)
git push
```

## Troubleshooting

### Running Diagnostics

```bash
# Full system health check
githelper doctor

# Check specific repository
githelper doctor --repo myproject

# Show all credentials
githelper doctor --credentials

# Auto-fix common issues
githelper doctor --auto-fix
```

### Common Issues

#### Issue: "Repository not found"

```bash
# List all repos to see what's configured
githelper repo list

# The repo might be named differently than you think
```

#### Issue: "SSH authentication failed"

```bash
# Check SSH keys
githelper doctor --credentials

# Test SSH manually
ssh -T git@github.com

# Ensure SSH key is in Vault (if using Vault)
vault kv get secret/githelper/github/default_ssh
```

#### Issue: "Vault unreachable"

GitHelper will use cached configuration:

```bash
# Check if cache is available
ls ~/.githelper/cache/

# Doctor will show cache age
githelper doctor
# Output: ⚠ Vault not reachable (will use cache if available)
```

#### Issue: "Dual-push not configured"

```bash
# Set up GitHub integration
githelper github setup myproject \
  --user yourusername \
  --create
```

### Debug Mode

```bash
# Verbose output
githelper -v github status myproject

# JSON output for parsing
githelper --format=json doctor
```

## Best Practices

### 1. Regular Health Checks

```bash
# Run weekly
githelper doctor --credentials --auto-fix

# Add to crontab
0 9 * * 1 /usr/local/bin/githelper doctor --quiet --auto-fix
```

### 2. Always Test Connectivity Before Important Pushes

```bash
# Before pushing critical work
githelper github check myproject

# Then push
git push
```

### 3. Use Sync After Network Outages

```bash
# After being offline or having network issues
for repo in $(githelper repo list --format=json | jq -r '.repositories[].name'); do
  githelper github sync "$repo"
done
```

### 4. Backup State File

```bash
# State file location
~/.githelper/state.yaml

# Backup regularly
cp ~/.githelper/state.yaml ~/.githelper/state.yaml.backup
```

### 5. Repository-Specific SSH Keys

For repositories requiring different GitHub accounts:

```bash
# Store repo-specific SSH key in Vault
vault kv put secret/githelper/github/myproject/ssh \
  private_key=@~/.ssh/id_work \
  public_key=@~/.ssh/id_work.pub

# GitHelper will automatically use the override
```

### 6. Keep Hooks Updated

Hooks are backed up automatically before update:

```bash
# Hooks are at: .git/hooks/pre-push, .git/hooks/post-push
# Backups at: .git/hooks/pre-push.githelper-backup

# Restore from backup if needed
cp .git/hooks/pre-push.githelper-backup .git/hooks/pre-push
```

## Advanced Workflows

### Migrating Existing Repository to GitHelper

```bash
# 1. Add bare repository as remote (if not already)
git remote add origin gitmanager@server:/srv/git/myproject.git
git push -u origin main

# 2. Add to githelper state
# (Currently manual - add to ~/.githelper/state.yaml)

# 3. Set up GitHub integration
cd /path/to/myproject
githelper github setup myproject \
  --user yourusername \
  --create

# 4. Verify
githelper doctor --repo myproject
```

### Using in CI/CD

```bash
# In your CI pipeline
export VAULT_ADDR="https://vault.company.com"
export VAULT_TOKEN="${CI_VAULT_TOKEN}"

# Test configuration
githelper doctor --format=json > diagnostic.json

# Check specific repo
githelper github check myproject --quiet || {
  echo "GitHub not reachable from CI"
  exit 1
}
```

### Multi-Branch Workflow

```bash
# GitHelper works with any branch
git checkout -b feature/new-feature
git commit -m "Work on feature"
git push origin feature/new-feature

# Sync specific branch
githelper github sync myproject --branch feature/new-feature
```

## Reference

### Command Quick Reference

```bash
# Repository Management
githelper repo create <name> [--type go|python] [--with-github user]
githelper repo list [--format json]

# GitHub Integration
githelper github setup <repo> --user <user> [--create] [--private]
githelper github status <repo> [--update-state]
githelper github sync <repo> [--branch main] [--retry-github]
githelper github check <repo>

# Diagnostics
githelper doctor [--credentials] [--repo name] [--auto-fix]

# Global Flags
--format json       # Machine-readable output
--no-color          # Disable colors
-q, --quiet         # Minimal output
-v, --verbose       # Detailed output
```

### State File Format

Location: `~/.githelper/state.yaml`

```yaml
repositories:
  myproject:
    path: /home/user/repos/myproject
    remote: origin
    created: "2025-11-16T10:00:00Z"
    type: go
    github:
      enabled: true
      user: yourusername
      repo: myproject
      sync_status: synced
      last_sync: "2025-11-16T14:30:00Z"
      needs_retry: false
      last_error: ""
```

### Exit Codes

- `0` - Success
- `1` - General error
- `2` - Configuration error
- Non-zero - Command-specific error

## Getting Help

```bash
# Command help
githelper --help
githelper doctor --help
githelper github --help

# Run diagnostics
githelper doctor --credentials

# Check specific issue
githelper github check myproject -v
```

## Next Steps

After mastering these workflows:

1. Explore automation with JSON output
2. Set up organization-wide Vault configuration
3. Integrate with your deployment pipeline
4. Customize hooks for your workflow

For more information, see:
- `README.md` - Feature overview
- `docs/VAULT.md` - Vault setup guide
- `docs/PHASE*.md` - Implementation details
