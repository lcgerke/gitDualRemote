# Sync Detection Gap Fix Plan v2

> **Incorporated feedback from GPT-5 and Gemini 2.5 Pro critiques**

## Problem Statement

**Current Behavior:**
When GitHub remote is missing (E2 scenario), the classifier skips sync detection entirely and returns:
```json
"sync": {
  "id": "S1",
  "description": "N/A (not all locations exist)",
  "local_ahead_of_core": 0,
  "local_behind_core": 0,
  ...
}
```

**Real-world Impact:**
User has 9 unpushed commits but githelper doesn't report them because GitHub remote isn't configured.

**Gap:**
Sync detection only runs when all three locations exist (E1). It should run partial sync detection for E2 (local + core) and E3 (local + github) scenarios.

---

## Root Cause Analysis

### File: `internal/scenarios/classifier.go`

**Line 92-95: Sync detection is skipped for non-E1 scenarios**
```go
// Only detect sync if all three locations exist
if state.Existence.ID != "E1" {
    state.Sync = SyncState{
        ID:          "S1",
        Description: "N/A (not all locations exist)",
    }
    return state, nil
}
```

This rigid check prevents useful sync information when only 2 of 3 locations exist.

---

## Proposed Solution

### 1. Expand Sync Detection to E2 and E3

**Modify:** `internal/scenarios/classifier.go:92-95`

**From:**
```go
if state.Existence.ID != "E1" {
    state.Sync = SyncState{
        ID:          "S1",
        Description: "N/A (not all locations exist)",
    }
    return state, nil
}
```

**To:**
```go
// Detect sync for partial configurations
switch state.Existence.ID {
case "E1":
    // Full three-way sync detection
    state.Sync = c.detectDefaultBranchSync(gc, defaultBranch)

case "E2": // Local + Core exist, GitHub missing
    state.Sync = c.detectTwoWaySync(gc, defaultBranch, c.coreRemote, "", "GitHub")

case "E3": // Local + GitHub exist, Core missing
    state.Sync = c.detectTwoWaySync(gc, defaultBranch, "", c.githubRemote, "Core")

default:
    // E4-E8: Not enough locations for sync detection
    state.Sync = SyncState{
        ID:          "S1",
        Description: "N/A (not all locations exist)",
    }
    return state, nil
}
```

### 2. Create New Method: `detectTwoWaySync()`

**Add to:** `internal/scenarios/classifier.go`

**Signature:**
```go
func (c *Classifier) detectTwoWaySync(
    gc GitClient,
    branch string,
    coreRemote string,
    githubRemote string,
    missingRemoteName string,
) SyncState
```

**Parameters:**
- `gc`: Git client for repository operations
- `branch`: Branch name to compare (typically default branch)
- `coreRemote`: Core remote name (empty string if missing)
- `githubRemote`: GitHub remote name (empty string if missing)
- `missingRemoteName`: Human-readable name of missing remote for description

**Detailed Implementation Logic:**

```go
func (c *Classifier) detectTwoWaySync(
    gc GitClient,
    branch string,
    coreRemote string,
    githubRemote string,
    missingRemoteName string,
) SyncState {
    // Determine which remote is available
    remoteName := coreRemote
    if remoteName == "" {
        remoteName = githubRemote
    }

    // 1. FETCH from available remote (ensures fresh data)
    err := gc.Fetch(remoteName)
    if err != nil {
        // Remote unreachable - return error state
        return SyncState{
            ID:          "S_UNAVAILABLE",
            Description: fmt.Sprintf("Cannot connect to %s", remoteName),
            Branch:      branch,
            PartialSync: true,
            Error:       err.Error(),
        }
    }

    // 2. Get commit hashes
    localHash, err := gc.GetBranchHash(branch)
    if err != nil {
        return SyncState{
            ID:          "S_NA_DETACHED",
            Description: "Sync N/A (detached HEAD or missing branch)",
            PartialSync: true,
            Error:       err.Error(),
        }
    }

    remoteHash, err := gc.GetRemoteBranchHash(remoteName, branch)
    if err != nil {
        return SyncState{
            ID:          "S_UNAVAILABLE",
            Description: fmt.Sprintf("Remote branch %s/%s not found", remoteName, branch),
            Branch:      branch,
            PartialSync: true,
            Error:       err.Error(),
        }
    }

    // 3. Calculate ahead/behind counts (reuse existing helper)
    ahead, behind, err := gc.CountCommitsBetween(localHash, remoteHash)
    if err != nil {
        return SyncState{
            ID:          "S_UNAVAILABLE",
            Description: "Could not calculate sync status",
            Branch:      branch,
            PartialSync: true,
            Error:       err.Error(),
        }
    }

    // 4. Determine sync scenario and build description
    var scenarioID string
    var description string
    diverged := ahead > 0 && behind > 0

    if ahead == 0 && behind == 0 {
        scenarioID = "S1"
        description = fmt.Sprintf("In sync with %s (%s N/A)", remoteName, missingRemoteName)
    } else if ahead > 0 && behind == 0 {
        scenarioID = "S2"
        description = fmt.Sprintf("Local ahead of %s (%s N/A)", remoteName, missingRemoteName)
    } else if ahead == 0 && behind > 0 {
        scenarioID = "S3"
        description = fmt.Sprintf("Local behind %s (%s N/A)", remoteName, missingRemoteName)
    } else {
        scenarioID = "S4"
        description = fmt.Sprintf("Diverged from %s (%s N/A)", remoteName, missingRemoteName)
    }

    // 5. Build SyncState with appropriate fields populated
    state := SyncState{
        ID:          scenarioID,
        Description: description,
        Branch:      branch,
        PartialSync: true,
        LocalHash:   localHash,
        Diverged:    diverged,
    }

    // Populate remote-specific fields
    if coreRemote != "" {
        state.CoreHash = remoteHash
        state.LocalAheadOfCore = ahead
        state.LocalBehindCore = behind
        state.AvailableRemote = coreRemote
    } else {
        state.GitHubHash = remoteHash
        state.LocalAheadOfGitHub = ahead
        state.LocalBehindGitHub = behind
        state.AvailableRemote = githubRemote
    }

    return state
}
```

**Key Design Decisions:**
- **Always fetch first** - Ensures comparison against latest remote state
- **Comprehensive error handling** - Returns specific error states (S_UNAVAILABLE, S_NA_DETACHED)
- **Reuses existing helpers** - `CountCommitsBetween()` for ahead/behind calculation
- **Handles divergence** - Detects when both ahead AND behind (S4 scenario)
- **Populates only relevant fields** - Core OR GitHub fields, not both

### 3. Update Sync State Type

**Modify:** `internal/scenarios/types.go:59-78`

**Add fields:**
```go
type SyncState struct {
    ID          string `json:"id"`
    Description string `json:"description"`

    Branch string `json:"branch"`

    PartialSync     bool   `json:"partial_sync,omitempty"`      // NEW: true for E2/E3 scenarios
    AvailableRemote string `json:"available_remote,omitempty"`  // NEW: name of available remote
    Error           string `json:"error,omitempty"`             // NEW: error message if fetch/compare failed

    LocalHash  string `json:"local_hash,omitempty"`
    CoreHash   string `json:"core_hash,omitempty"`
    GitHubHash string `json:"github_hash,omitempty"`

    LocalAheadOfCore   int `json:"local_ahead_of_core"`
    LocalBehindCore    int `json:"local_behind_core"`
    LocalAheadOfGitHub int `json:"local_ahead_of_github"`
    LocalBehindGitHub  int `json:"local_behind_github"`
    CoreAheadOfGitHub  int `json:"core_ahead_of_github"`
    CoreBehindGitHub   int `json:"core_behind_github"`

    Diverged bool `json:"diverged"`
}
```

**New Fields Explained:**
- `PartialSync`: Indicates E2/E3 partial sync detection (vs E1 full)
- `AvailableRemote`: Name of the remote that was actually compared (origin/github)
- `Error`: If sync detection failed, contains error message

### 4. Update Scenario Table

**Modify:** `internal/scenarios/tables.go`

**Add new error scenario:**
```go
"S_UNAVAILABLE": {
    ID:          "S_UNAVAILABLE",
    Name:        "Remote Unavailable",
    Description: "Cannot connect to remote or determine sync status",
    Category:    "sync",
    Severity:    "error",
    AutoFixable: false,
},
"S_NA_DETACHED": {
    ID:          "S_NA_DETACHED",
    Name:        "Sync N/A (Detached HEAD)",
    Description: "Repository in detached HEAD state - sync comparison not applicable",
    Category:    "sync",
    Severity:    "warning",
    AutoFixable: false,
},
```

**Update existing S2/S3 descriptions** to note they apply to partial sync too.

---

## Implementation Steps

### Phase 1: Expand Detection Logic (TDD)

**Duration: 60 minutes**

1. **Write failing tests** - `internal/scenarios/classifier_test.go`

```go
// Test E2 scenario: local ahead of core
func TestDetectTwoWaySync_E2_LocalAhead(t *testing.T) {
    mockGC := &MockGitClient{
        localHash:  "abc123",
        coreHash:   "def456",
        ahead:      9,
        behind:     0,
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

    assert.Equal(t, "S2", state.ID)
    assert.True(t, state.PartialSync)
    assert.Equal(t, 9, state.LocalAheadOfCore)
    assert.Equal(t, 0, state.LocalBehindCore)
    assert.Equal(t, "origin", state.AvailableRemote)
}

// Test E2 scenario: local behind core
func TestDetectTwoWaySync_E2_LocalBehind(t *testing.T) {
    mockGC := &MockGitClient{
        localHash:  "abc123",
        coreHash:   "xyz789",
        ahead:      0,
        behind:     5,
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

    assert.Equal(t, "S3", state.ID)
    assert.True(t, state.PartialSync)
    assert.Equal(t, 0, state.LocalAheadOfCore)
    assert.Equal(t, 5, state.LocalBehindCore)
}

// Test E2 scenario: diverged (both ahead and behind)
func TestDetectTwoWaySync_E2_Diverged(t *testing.T) {
    mockGC := &MockGitClient{
        localHash:  "abc123",
        coreHash:   "def456",
        ahead:      3,
        behind:     2,
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

    assert.Equal(t, "S4", state.ID)
    assert.True(t, state.PartialSync)
    assert.True(t, state.Diverged)
    assert.Equal(t, 3, state.LocalAheadOfCore)
    assert.Equal(t, 2, state.LocalBehindCore)
}

// Test E2 scenario: remote fetch fails
func TestDetectTwoWaySync_E2_RemoteUnavailable(t *testing.T) {
    mockGC := &MockGitClient{
        fetchError: errors.New("ssh: connect to host core.example.com port 22: Connection refused"),
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

    assert.Equal(t, "S_UNAVAILABLE", state.ID)
    assert.True(t, state.PartialSync)
    assert.Contains(t, state.Error, "Connection refused")
}

// Test E2 scenario: detached HEAD
func TestDetectTwoWaySync_E2_DetachedHEAD(t *testing.T) {
    mockGC := &MockGitClient{
        branchHashError: errors.New("not on a branch"),
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "origin", "", "GitHub")

    assert.Equal(t, "S_NA_DETACHED", state.ID)
    assert.True(t, state.PartialSync)
}

// Test E3 scenario: local ahead of github
func TestDetectTwoWaySync_E3_LocalAhead(t *testing.T) {
    mockGC := &MockGitClient{
        localHash:  "abc123",
        githubHash: "def456",
        ahead:      4,
        behind:     0,
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectTwoWaySync(mockGC, "main", "", "github", "Core")

    assert.Equal(t, "S2", state.ID)
    assert.True(t, state.PartialSync)
    assert.Equal(t, 4, state.LocalAheadOfGitHub)
    assert.Equal(t, 0, state.LocalBehindGitHub)
    assert.Equal(t, "github", state.AvailableRemote)
}

// Test E1 regression: ensure full sync still works
func TestDetectDefaultBranchSync_E1_NoRegression(t *testing.T) {
    mockGC := &MockGitClient{
        localHash:  "abc123",
        coreHash:   "abc123",
        githubHash: "abc123",
    }

    classifier := NewClassifier(mockGC, "origin", "github", DefaultDetectionOptions())
    state := classifier.detectDefaultBranchSync(mockGC, "main")

    assert.Equal(t, "S1", state.ID)
    assert.False(t, state.PartialSync) // Full sync, not partial
    assert.Equal(t, 0, state.LocalAheadOfCore)
}
```

2. **Implement `detectTwoWaySync()`** - Following the detailed logic above

3. **Update classifier switch** - Add E2/E3 cases

4. **Run tests** - Verify all new tests pass

### Phase 2: Update Suggester

**Duration: 30 minutes**

**Modify:** `internal/scenarios/suggester.go:113-251`

**Fix suggester logic to work with Option B (PartialSync flag):**

```go
case "S2": // Local ahead (full or partial)
    var remoteName string
    var aheadCount int

    // Determine which remote to suggest pushing to
    if state.Sync.PartialSync {
        remoteName = state.Sync.AvailableRemote
        if state.Sync.LocalAheadOfCore > 0 {
            aheadCount = state.Sync.LocalAheadOfCore
        } else {
            aheadCount = state.Sync.LocalAheadOfGitHub
        }
    } else {
        // Full sync - push to both remotes
        remoteName = coreRemote
        aheadCount = state.Sync.LocalAheadOfCore
    }

    return []Fix{{
        ScenarioID:  "S2",
        Description: fmt.Sprintf("Local has %d unpushed commits to %s", aheadCount, remoteName),
        Command:     fmt.Sprintf("git push %s %s", remoteName, state.Sync.Branch),
        Operation: &PushOperation{
            Remote:  remoteName,
            Refspec: state.Sync.Branch,
        },
        AutoFixable: true,
        Priority:    4,
        Reason:      "Push local commits to remote",
    }}

case "S3": // Local behind (full or partial)
    var remoteName string
    var behindCount int

    if state.Sync.PartialSync {
        remoteName = state.Sync.AvailableRemote
        if state.Sync.LocalBehindCore > 0 {
            behindCount = state.Sync.LocalBehindCore
        } else {
            behindCount = state.Sync.LocalBehindGitHub
        }
    } else {
        remoteName = coreRemote
        behindCount = state.Sync.LocalBehindCore
    }

    return []Fix{{
        ScenarioID:  "S3",
        Description: fmt.Sprintf("Remote %s has %d commits not in local", remoteName, behindCount),
        Command:     fmt.Sprintf("git pull %s %s", remoteName, state.Sync.Branch),
        Operation: &PullOperation{
            Remote: remoteName,
            Branch: state.Sync.Branch,
        },
        AutoFixable: true,
        Priority:    4,
        Reason:      "Pull updates from remote",
    }}

case "S4": // Diverged
    var remoteName string
    if state.Sync.PartialSync {
        remoteName = state.Sync.AvailableRemote
    } else {
        remoteName = "remotes"
    }

    return []Fix{{
        ScenarioID:  "S4",
        Description: fmt.Sprintf("Local diverged from %s - manual merge required", remoteName),
        Command:     fmt.Sprintf("git pull %s %s --rebase", state.Sync.AvailableRemote, state.Sync.Branch),
        Operation:   nil,
        AutoFixable: false,
        Priority:    2,
        Reason:      "Branch has diverged - manual intervention needed",
    }}

case "S_UNAVAILABLE":
    return []Fix{{
        ScenarioID:  "S_UNAVAILABLE",
        Description: "Remote unavailable - check network/credentials",
        Command:     "Check remote configuration: git remote -v",
        Operation:   nil,
        AutoFixable: false,
        Priority:    1,
        Reason:      "Cannot determine sync status without remote access",
    }}

case "S_NA_DETACHED":
    return []Fix{{
        ScenarioID:  "S_NA_DETACHED",
        Description: "Detached HEAD - checkout a branch",
        Command:     "git checkout main",
        Operation:   nil,
        AutoFixable: false,
        Priority:    3,
        Reason:      "Sync detection requires being on a branch",
    }}
```

**Add tests for suggester:**

```go
func TestSuggestFixes_S2_Partial(t *testing.T) {
    state := &RepositoryState{
        Sync: SyncState{
            ID:               "S2",
            PartialSync:      true,
            AvailableRemote:  "origin",
            LocalAheadOfCore: 9,
            Branch:           "main",
        },
    }

    fixes := SuggestFixes(state)

    assert.Len(t, fixes, 1)
    assert.Equal(t, "S2", fixes[0].ScenarioID)
    assert.Contains(t, fixes[0].Command, "git push origin main")
    assert.True(t, fixes[0].AutoFixable)
}
```

### Phase 3: Update Human Output

**Duration: 20 minutes**

**Modify:** `cmd/githelper/status.go`

**Display partial sync info clearly:**

```go
// Display sync status
if state.Sync.Error != "" {
    fmt.Printf("🔄 Sync Status:\n")
    fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
    fmt.Printf("  ⚠️  Error: %s\n", state.Sync.Error)
} else if state.Sync.PartialSync {
    fmt.Printf("🔄 Sync Status (partial):\n")
    fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
    fmt.Printf("  Branch: %s\n", state.Sync.Branch)

    // Show available remote stats
    if state.Sync.AvailableRemote != "" {
        if state.Sync.LocalAheadOfCore > 0 || state.Sync.LocalBehindCore > 0 {
            fmt.Printf("  Local vs %s: ", state.Sync.AvailableRemote)
            if state.Sync.LocalAheadOfCore > 0 {
                fmt.Printf("%d ahead", state.Sync.LocalAheadOfCore)
            }
            if state.Sync.LocalBehindCore > 0 {
                if state.Sync.LocalAheadOfCore > 0 {
                    fmt.Printf(", ")
                }
                fmt.Printf("%d behind", state.Sync.LocalBehindCore)
            }
            fmt.Printf("\n")
        } else if state.Sync.LocalAheadOfGitHub > 0 || state.Sync.LocalBehindGitHub > 0 {
            fmt.Printf("  Local vs %s: ", state.Sync.AvailableRemote)
            if state.Sync.LocalAheadOfGitHub > 0 {
                fmt.Printf("%d ahead", state.Sync.LocalAheadOfGitHub)
            }
            if state.Sync.LocalBehindGitHub > 0 {
                if state.Sync.LocalAheadOfGitHub > 0 {
                    fmt.Printf(", ")
                }
                fmt.Printf("%d behind", state.Sync.LocalBehindGitHub)
            }
            fmt.Printf("\n")
        }
    }
} else {
    // Full three-way sync
    fmt.Printf("🔄 Sync Status:\n")
    fmt.Printf("  %s - %s\n", state.Sync.ID, state.Sync.Description)
    fmt.Printf("  Branch: %s\n", state.Sync.Branch)
}
```

### Phase 4: Comprehensive Tests

**Duration: 40 minutes**

**Test coverage checklist:**

- [x] E2 with local ahead (9 commits)
- [x] E2 with local behind (5 commits)
- [x] E2 with local synced (0 ahead/behind)
- [x] E2 with diverged state (ahead AND behind)
- [x] E2 with remote fetch failure
- [x] E2 with detached HEAD
- [x] E3 with local ahead
- [x] E3 with local behind
- [x] E3 with remote fetch failure
- [x] E1 regression test (ensure no changes to full sync)
- [x] JSON marshaling includes partial_sync, available_remote, error fields
- [x] Suggester returns correct fixes for S2 partial
- [x] Suggester returns correct fixes for S3 partial
- [x] Suggester returns error handling for S_UNAVAILABLE
- [x] CLI output displays partial sync correctly
- [x] CLI output displays errors correctly

**Integration test:**

```bash
# Test E2 scenario with real repo
cd ~/wk  # Has origin but no github
githelper status

# Expected output:
# 🔄 Sync Status (partial):
#   S2 - Local ahead of origin (GitHub N/A)
#   Branch: main
#   Local vs origin: 9 ahead
```

---

## Validation Criteria

**Success means:**

1. **E2 scenario detection:**
```bash
$ cd ~/wk  # Has origin but no github remote
$ githelper status

🔄 Sync Status (partial):
  S2 - Local ahead of origin (GitHub N/A)
  Branch: main
  Local vs origin: 9 ahead
```

2. **JSON output includes sync data:**
```json
{
  "existence": { "id": "E2" },
  "sync": {
    "id": "S2",
    "partial_sync": true,
    "available_remote": "origin",
    "branch": "main",
    "local_ahead_of_core": 9,
    "local_behind_core": 0,
    "local_ahead_of_github": 0,
    "local_behind_github": 0,
    "github_hash": ""
  }
}
```

3. **Suggester provides fix:**
```bash
$ githelper status --show-fixes

Suggested Fixes:
  [S2] Local has 9 unpushed commits to origin
    Command: git push origin main
    Auto-fixable: true
```

4. **Error handling:**
```bash
$ githelper status  # When remote is unreachable

🔄 Sync Status:
  S_UNAVAILABLE - Cannot connect to origin
  ⚠️  Error: ssh: connect to host core.example.com port 22: Connection refused
```

5. **E1 scenarios unaffected:**
   - Full three-way sync detection still works
   - No regression in existing corpus tests

---

## Files to Modify

| File | Change | Lines | Complexity |
|------|--------|-------|------------|
| `internal/scenarios/classifier.go` | Add `detectTwoWaySync()` (80 lines), update switch (15 lines) | +95 | Medium |
| `internal/scenarios/types.go` | Add 3 new fields (`PartialSync`, `AvailableRemote`, `Error`) | +3 | Low |
| `internal/scenarios/tables.go` | Add S_UNAVAILABLE, S_NA_DETACHED scenarios | +20 | Low |
| `internal/scenarios/suggester.go` | Update S2/S3/S4 fix logic, add error scenarios | +60 | Medium |
| `cmd/githelper/status.go` | Display partial sync indicator and error handling | +30 | Low |
| `internal/scenarios/classifier_test.go` | Add 10+ test cases for E2/E3 scenarios | +250 | High |
| `internal/scenarios/suggester_test.go` | Add test cases for partial sync fixes | +50 | Medium |

**Total:** ~508 lines of changes (up from 216 in v1)

---

## Decision Point

**Resolved: Using Option B** - Reuse S2/S3 IDs with `PartialSync: true` flag

**Rationale:**
- ✓ Simpler to implement and maintain
- ✓ Fewer scenario definitions to document
- ✓ Fix suggestions can check `PartialSync` flag and `AvailableRemote` field
- ✓ More flexible for future enhancements (e.g., E3 scenarios)

---

## Rollout

1. **Implement** - Follow Phase 1-4 in order (TDD approach)
2. **Unit Test** - Run all new classifier and suggester tests
3. **Corpus Test** - Verify corpus validator still passes (E1 scenarios)
4. **Integration Test** - Test on ~/wk repo (E2 scenario with 9 commits ahead)
5. **Error Test** - Test with unreachable remote to verify error handling
6. **Commit** - TDD style with comprehensive tests
7. **Build & Deploy** - `make build && install ~/.local/bin/githelper`
8. **Validate** - User runs `githelper status` in both E1 and E2 repos

---

## Risk Assessment

**Low Risk:**
- E1 scenarios (fully configured) - Regression test ensures no changes
- E4-E8 scenarios (insufficient locations) - Still return N/A as before

**Medium Risk:**
- E2/E3 scenarios - New detection logic with error handling
- Mitigation: Comprehensive test suite including error cases

**Identified Risks (from critiques):**
- ❌ **Stale remote data** - MITIGATED: `detectTwoWaySync()` fetches before comparing
- ❌ **Remote connectivity failures** - MITIGATED: New `S_UNAVAILABLE` error state
- ❌ **Detached HEAD** - MITIGATED: New `S_NA_DETACHED` state
- ❌ **Diverged branches** - MITIGATED: New `S4` scenario for two-way divergence
- ❌ **Suggester inconsistency** - MITIGATED: Fixed to use `AvailableRemote` field

**Zero Risk:**
- Corpus validation - Only tests E1 scenarios, won't be affected

---

## Timeline Estimate

**Revised timeline based on critique feedback:**

- Phase 1 (Detection + Tests): **60 min** (was 30 min)
  - Implement `detectTwoWaySync()` with error handling: 30 min
  - Write 10+ test cases: 30 min

- Phase 2 (Suggester + Tests): **30 min** (was 15 min)
  - Fix suggester logic for Option B: 15 min
  - Add suggester tests: 15 min

- Phase 3 (Display): **20 min** (was 10 min)
  - Add partial sync display: 10 min
  - Add error display: 10 min

- Phase 4 (Integration Tests): **40 min** (was 20 min)
  - Integration tests on real repos: 20 min
  - Error scenario testing: 20 min

**Total: ~150 minutes (2.5 hours)** - up from 75 minutes in v1

**Buffer:** +30 minutes for unexpected issues

**Final estimate: 3 hours** for complete TDD implementation with comprehensive testing

---

## Changes from v1

**Key improvements incorporated from critiques:**

1. **Detailed Git operations** - Full implementation of `detectTwoWaySync()` with exact logic
2. **Error handling** - New error states (`S_UNAVAILABLE`, `S_NA_DETACHED`) for unreachable remotes
3. **Divergence detection** - Added `S4` scenario for two-way divergence
4. **Fresh data guarantee** - Always fetch before comparing to avoid stale remote data
5. **Fixed suggester logic** - Corrected to use `AvailableRemote` field (matches Option B)
6. **Expanded test coverage** - 10+ test cases including error scenarios, detached HEAD, divergence
7. **Realistic timeline** - Doubled from 75 min to 150 min based on implementation complexity
8. **New SyncState fields** - `AvailableRemote` and `Error` for better context
9. **Better CLI output** - Clear indication of partial sync and error messages
10. **Integration testing** - Explicit validation on real repositories

**Line count increased** from 216 to 508 lines due to:
- Comprehensive error handling
- More test cases
- Detailed suggester logic
- CLI output improvements
