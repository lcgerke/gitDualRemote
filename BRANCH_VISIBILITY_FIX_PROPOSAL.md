# Branch Divergence Visibility Fix Proposal

## Problem Statement

**Root Cause**: githelper's `status` command displays sync information for the **default branch** in the top-level `Sync` section, but divergence information for the **current branch** is buried in the `Branches` array. This creates poor UX when:

1. User is working on a non-default branch (e.g., `main`)
2. Default branch is different (e.g., `master`)
3. Current branch has divergence from remotes
4. User looks at top-level Sync section and sees "In sync" (referring to `master`)
5. User misses the actual divergence on `main` because it's only shown in the branches array

**Real-World Example** (from actual user experience):
```bash
$ git branch --show-current
main

$ githelper status
🔍 Analyzing repository state...

🔄 Sync Status:
  S1 - In sync           # ← This is showing 'master' status
  Branch: master         # ← Not the current branch!

...

# Divergence on 'main' is hidden in JSON branches array:
"branches": [
  {
    "branch": "main",
    "local_ahead_of_core": 3,
    "local_ahead_of_github": 4
  }
]
```

## Proposed Solutions

### Option 1: Show Current Branch in Top-Level Sync (Recommended)

**Change**: Modify the top-level `Sync` section to show the **current branch** instead of the default branch.

**Rationale**:
- Users care most about the branch they're actively working on
- This matches user expectations (git status shows current branch)
- Reduces cognitive load
- Makes divergence immediately visible

**Implementation**:

**File**: `internal/scenarios/classifier.go` (or similar detection logic)

```go
// Current behavior (BEFORE):
func (c *Classifier) detectSyncState() (SyncState, error) {
    // Gets default branch (e.g., 'master')
    defaultBranch := c.gitClient.GetDefaultBranch()

    // Calculates sync state for DEFAULT branch
    state := calculateSyncForBranch(defaultBranch)
    state.Branch = defaultBranch
    return state, nil
}

// Proposed behavior (AFTER):
func (c *Classifier) detectSyncState() (SyncState, error) {
    // Get CURRENT branch instead of default
    currentBranch, err := c.gitClient.GetCurrentBranch()
    if err != nil {
        // Fallback to default branch if detached HEAD or error
        currentBranch = c.gitClient.GetDefaultBranch()
    }

    // Calculate sync state for CURRENT branch
    state := calculateSyncForBranch(currentBranch)
    state.Branch = currentBranch

    // Optionally: Add field to indicate if this differs from default
    if currentBranch != c.gitClient.GetDefaultBranch() {
        state.IsDefaultBranch = false
    }

    return state, nil
}
```

**File**: `internal/scenarios/types.go`

```go
// Add new optional field to SyncState
type SyncState struct {
    ID          string `json:"id"`
    Description string `json:"description"`

    Branch          string `json:"branch"`          // Now shows CURRENT branch
    IsDefaultBranch bool   `json:"is_default_branch,omitempty"` // NEW: indicates if showing default
    DefaultBranch   string `json:"default_branch,omitempty"`    // NEW: what the default is

    // ... rest of fields unchanged
}
```

**File**: `cmd/githelper/status.go` (lines 183-203)

```go
// Update human-readable output to clarify:
fmt.Println("🔄 Sync Status:")
fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
if state.Sync.Branch != "" {
    branchLabel := state.Sync.Branch
    if !state.Sync.IsDefaultBranch {
        branchLabel = fmt.Sprintf("%s (current, default: %s)",
                                   state.Sync.Branch,
                                   state.Sync.DefaultBranch)
    } else {
        branchLabel = fmt.Sprintf("%s (current & default)", state.Sync.Branch)
    }
    fmt.Printf("  Branch: %s\n", branchLabel)
}
```

**Example Output (AFTER fix)**:
```bash
$ git branch --show-current
main

$ githelper status
🔍 Analyzing repository state...

🔄 Sync Status:
  S3 - Local ahead          # ← Now showing 'main' status!
  Branch: main (current, default: master)
  Local ahead of Core: 3 commits
  Local ahead of GitHub: 4 commits
```

---

### Option 2: Add Prominent Warning for Non-Default Branch Divergence

**Change**: Keep top-level Sync showing default branch, but add a **prominent warning** if the current branch differs and has divergence.

**Implementation**:

**File**: `cmd/githelper/status.go` (add after line 204)

```go
// After printing Sync section, check for current branch divergence
func printStatusReport(out *ui.Output, state *scenarios.RepositoryState, showFixes bool) {
    // ... existing code ...

    // NEW: Check if current branch differs from default and has divergence
    currentBranch, _ := getCurrentBranch() // implement this
    if currentBranch != "" && currentBranch != state.DefaultBranch {
        // Find current branch in branches array
        for _, branchState := range state.Branches {
            if branchState.Branch == currentBranch {
                hasDivergence := branchState.LocalAheadOfCore > 0 ||
                                branchState.LocalBehindCore > 0 ||
                                branchState.LocalAheadOfGitHub > 0 ||
                                branchState.LocalBehindGitHub > 0 ||
                                branchState.Diverged

                if hasDivergence {
                    out.Warning("⚠️  CURRENT BRANCH DIVERGENCE:")
                    out.Warning(fmt.Sprintf("    Branch '%s' is out of sync with remotes:", currentBranch))
                    if branchState.LocalAheadOfCore > 0 {
                        out.Warning(fmt.Sprintf("      Ahead of %s: %d commits", state.CoreRemote, branchState.LocalAheadOfCore))
                    }
                    if branchState.LocalAheadOfGitHub > 0 {
                        out.Warning(fmt.Sprintf("      Ahead of %s: %d commits", state.GitHubRemote, branchState.LocalAheadOfGitHub))
                    }
                    if branchState.Diverged {
                        out.Error("      ⚠️  DIVERGED - manual merge required")
                    }
                    fmt.Println()
                }
                break
            }
        }
    }

    // ... rest of existing code ...
}
```

---

### Option 3: Show Both Default and Current Branch Status

**Change**: Display **both** default branch and current branch in the Sync section when they differ.

**Example Output**:
```bash
🔄 Sync Status:
  Default Branch (master):
    S1 - In sync

  Current Branch (main):
    S3 - Local ahead
    Local ahead of Core: 3 commits
    Local ahead of GitHub: 4 commits
```

---

## Recommendation

**Implement Option 1** (Show Current Branch in Top-Level Sync) because:

1. ✅ **Matches user expectations**: Users naturally expect to see status for the branch they're on
2. ✅ **Reduces cognitive load**: No need to scan branches array or correlate information
3. ✅ **Aligns with git conventions**: `git status` shows current branch
4. ✅ **Simpler implementation**: Single change to detection logic
5. ✅ **Backward compatible**: JSON output structure remains similar (just different branch shown)

**Optional Enhancement**: Also implement Option 2's warning as a **safety net** for users who might still expect default branch behavior.

---

## Implementation Checklist

- [ ] Update `internal/scenarios/classifier.go` to use current branch instead of default
- [ ] Add `IsDefaultBranch` and `DefaultBranch` fields to `SyncState` struct
- [ ] Update `cmd/githelper/status.go` to show "(current)" or "(current & default)" label
- [ ] Add optional warning for divergence when current != default
- [ ] Update tests to reflect new behavior
- [ ] Update documentation/examples
- [ ] Test with:
  - [ ] Current branch = default branch (no regression)
  - [ ] Current branch ≠ default branch, in sync
  - [ ] Current branch ≠ default branch, diverged (this was the bug)
  - [ ] Detached HEAD state
  - [ ] New repo with only one branch

---

## Files to Modify

1. **`internal/scenarios/classifier.go`** (or equivalent detection logic)
   - Change default branch detection to current branch detection

2. **`internal/scenarios/types.go`** (line 60-83)
   - Add `IsDefaultBranch bool` field to `SyncState`
   - Add `DefaultBranch string` field to `SyncState`

3. **`cmd/githelper/status.go`** (lines 183-203)
   - Update human-readable output formatting
   - Add optional divergence warning

4. **`internal/git/cli.go`** (or equivalent)
   - Ensure `GetCurrentBranch()` method exists and handles edge cases

---

## Testing Scenarios

### Test Case 1: Current = Default (master)
```bash
$ git checkout master
$ githelper status
# Expected: Shows "Branch: master (current & default)"
```

### Test Case 2: Current ≠ Default, In Sync
```bash
$ git checkout main
$ githelper status
# Expected: Shows "Branch: main (current, default: master)"
#           Status shows main is in sync
```

### Test Case 3: Current ≠ Default, Diverged (THE BUG)
```bash
$ git checkout main
# (main is 3 ahead of origin, 4 ahead of github)
$ githelper status
# Expected: Shows "Branch: main (current, default: master)"
#           Status shows "Local ahead of Core: 3 commits"
#           Status shows "Local ahead of GitHub: 4 commits"
```

### Test Case 4: Detached HEAD
```bash
$ git checkout HEAD~1
$ githelper status
# Expected: Falls back to showing default branch (master)
#           Shows warning about detached HEAD
```

---

## Migration Notes

**Breaking Change?** Minimal. The JSON structure remains the same, only the branch shown in top-level `Sync` changes from default → current.

**Users relying on top-level Sync being default branch**: Very unlikely, as this behavior was not documented and contradicts user expectations.

**Migration Path**:
1. Add `IsDefaultBranch` field to JSON output (backward compatible - new field)
2. Change behavior in minor version bump
3. Document in changelog: "Top-level Sync now shows current branch instead of default branch"
