# Phase 3: Sync & Recovery - Implementation Notes

**Date:** 2025-11-16
**Status:** Complete

## Overview

Phase 3 implements intelligent sync and recovery mechanisms for dual-push repositories. This allows automatic detection and recovery from partial push failures.

## Features Implemented

### 1. Divergence Detection (`internal/git/divergence.go`)

Smart commit graph comparison using git's native tools:

```go
// Checks if GitHub remote is behind bare remote
status, err := gitClient.CheckDivergence("origin", "github", "main")

// Returns:
// - InSync: bool - are remotes synchronized?
// - BareAhead: int - commits bare has that GitHub doesn't
// - GitHubAhead: int - commits GitHub has that bare doesn't
// - BareRef: string - current commit on bare
// - GitHubRef: string - current commit on GitHub
```

**How it works:**
1. Fetches from both remotes
2. Gets commit SHAs for both remotes
3. Uses `git rev-list <older>..<newer> --count` to count divergence
4. Returns detailed status

**Edge cases handled:**
- In sync: Both remotes have same commit
- Bare ahead: GitHub is behind, can auto-sync
- GitHub ahead: Manual resolution required (diverged state)
- Both ahead: True divergence, manual merge needed

### 2. Sync Command (`cmd/githelper/github_sync.go`)

Manual sync recovery after partial failures:

```bash
# Check and sync GitHub with bare repo
githelper github sync myproject

# Sync specific branch
githelper github sync myproject --branch develop

# Force retry after known failure
githelper github sync myproject --retry-github
```

**Workflow:**
1. Loads repository state
2. Checks for GitHub integration
3. Detects divergence between bare and GitHub
4. If GitHub is behind: pushes missing commits
5. If GitHub is ahead: reports error, requires manual resolution
6. Verifies sync and updates state

**Temporary Remote Handling:**
- For dual-push configurations (single origin with multiple push URLs)
- Creates temporary `github-temp` remote for comparison
- Automatically cleans up after sync
- Allows sync checking without breaking dual-push config

### 3. Enhanced State Tracking

State file (`~/.githelper/state.yaml`) now tracks:

```yaml
repositories:
  myproject:
    github:
      enabled: true
      user: lcgerke
      repo: myproject
      sync_status: "synced"  # synced|behind|diverged|unknown
      last_sync: "2025-11-16T14:30:00Z"
      needs_retry: false
      last_error: ""
```

**Sync States:**
- `synced`: Both remotes have same commits
- `behind`: GitHub is behind bare repo (needs sync)
- `diverged`: GitHub has different commits (manual resolution)
- `unknown`: Status not yet determined

### 4. Enhanced Status Command

The `github status` command now supports state updates:

```bash
# Show current status
githelper github status myproject

# Check real divergence and update state
githelper github status myproject --update-state
```

**Post-Push Hook Integration:**
- Post-push hook calls `githelper github status --update-state`
- Automatically tracks sync status after every push
- Updates state file with current divergence

### 5. Partial Failure Handling

**How Dual-Push Failures Work:**

With git's native dual-push (multiple push URLs), the behavior is:
1. Git tries to push to all URLs sequentially
2. If ANY push fails, the entire operation fails
3. Git reports which URL failed in the error message
4. Some remotes may have succeeded before the failure

**Example Scenario:**

```bash
$ git push
# Git pushes to URL 1: git@lcgasgit:/srv/git/myproject.git ✅
# Git pushes to URL 2: git@github.com:lcgerke/myproject.git ❌
# Push fails, but bare repo has the commits

$ githelper github sync myproject
# Detects GitHub is 3 commits behind
# Pushes only the missing commits
# Verifies sync ✅
```

**Recovery Strategy:**
1. User attempts push (pre-push hook checks connectivity)
2. If push fails, user can see which remote failed from git output
3. User runs `githelper github sync` to recover
4. Sync detects divergence and pushes only missing commits
5. State updated to "synced"

**Why Not Automatic?**
- Git's dual-push is atomic from its perspective
- We can't hook into git's push process mid-execution
- Clean separation: git handles push, githelper handles recovery
- User stays in control

## New Git Operations

Added to `internal/git/cli.go`:

```go
// RemoveRemote removes a git remote
func (c *Client) RemoveRemote(name string) error

// GetPushURLs gets all push URLs for a remote
func (c *Client) GetPushURLs(remote string) ([]string, error)
```

Added to `internal/git/divergence.go`:

```go
// CheckDivergence checks sync status between remotes
func (c *Client) CheckDivergence(bareRemote, githubRemote, branch string) (*DivergenceStatus, error)

// countCommits counts commits in one branch that aren't in another
func (c *Client) countCommits(older, newer string) (int, error)

// GetCommit returns commit SHA for a ref
func (c *Client) GetCommit(ref string) (string, error)

// Fetch fetches from a remote
func (c *Client) Fetch(remote string) error

// SyncToGitHub pushes missing commits to GitHub
func (c *Client) SyncToGitHub(bareRemote, githubRemote, branch string) error
```

## Testing Recommendations

### Manual Testing

1. **Normal sync flow:**
   ```bash
   git commit -m "test"
   git push  # Should succeed to both
   githelper github sync myproject  # Should report "in sync"
   ```

2. **Simulated partial failure:**
   ```bash
   # Disconnect network
   git commit -m "test"
   git push  # Will fail (both remotes unreachable)

   # Reconnect network
   githelper github sync myproject  # Should sync
   ```

3. **Divergence detection:**
   ```bash
   # Make commit on GitHub directly (via web UI)
   githelper github sync myproject  # Should report divergence
   ```

### Edge Cases to Test

- Empty repositories (no commits)
- Non-main branches (develop, feature/*, etc.)
- Multiple remotes with same URL
- Network failures mid-push
- State file corruption/deletion
- Missing .git/config entries

## Success Criteria

- ✅ Divergence detection using git rev-list
- ✅ Manual sync command with retry
- ✅ State tracking for sync status
- ✅ Enhanced status command with --update-state
- ✅ Temporary remote handling for dual-push configs
- ✅ Post-push hook integration
- ✅ Clear error messages for divergence

## Known Limitations

1. **Branch Detection:** Currently defaults to `main` branch. Need to detect current branch automatically.

2. **Dual-Push State:** Can't easily detect "partial success" because git treats it as complete failure. Recovery is manual via sync command.

3. **Network Timing:** Pre-push connectivity check can't predict network failures during push.

4. **Merge Conflicts:** If GitHub has diverged, user must manually resolve. Tool doesn't attempt automatic merges.

## Next Steps (Phase 4)

- Doctor command for comprehensive health checks
- Auto-fix operations
- Credential inventory
- Improved error messages
- Help system

## Files Modified/Added

**New Files:**
- `internal/git/divergence.go` (152 lines) - Divergence detection logic
- `cmd/githelper/github_sync.go` (250 lines) - Sync command implementation
- `docs/PHASE3_NOTES.md` - This file

**Modified Files:**
- `internal/git/cli.go` - Added RemoveRemote, GetPushURLs
- `cmd/githelper/github.go` - Added sync subcommand
- `cmd/githelper/github_status.go` - Added --update-state support

**Total New Code:** ~450 lines
