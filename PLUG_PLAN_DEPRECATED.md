# Sync Detection Gap Fix Plan

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
    state.Sync = c.detectTwoWaySync(gc, defaultBranch, c.coreRemote, "")

case "E3": // Local + GitHub exist, Core missing
    state.Sync = c.detectTwoWaySync(gc, defaultBranch, "", c.githubRemote)

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
func (c *Classifier) detectTwoWaySync(gc GitClient, branch, coreRemote, githubRemote string) SyncState
```

**Logic:**
- If `coreRemote != ""`: Compare local vs core (for E2 scenarios)
- If `githubRemote != ""`: Compare local vs github (for E3 scenarios)
- Return partial sync states with only relevant fields populated

**Scenarios to detect:**
- **S2-partial**: Local ahead of available remote
- **S3-partial**: Local behind available remote
- **S1-partial**: Local synced with available remote

### 3. Update Sync State Type

**Modify:** `internal/scenarios/types.go:59-78`

**Add field:**
```go
type SyncState struct {
    ID          string `json:"id"`
    Description string `json:"description"`

    Branch string `json:"branch"`

    PartialSync bool `json:"partial_sync,omitempty"` // NEW: true for E2/E3 scenarios

    LocalHash  string `json:"local_hash,omitempty"`
    CoreHash   string `json:"core_hash,omitempty"`
    GitHubHash string `json:"github_hash,omitempty"`

    // ... rest unchanged
}
```

### 4. Update Scenario Table

**Modify:** `internal/scenarios/tables.go`

**Add new scenario IDs:**
- **S2a**: Local ahead of Core (GitHub N/A)
- **S2b**: Local ahead of GitHub (Core N/A)
- **S3a**: Local behind Core (GitHub N/A)
- **S3b**: Local behind GitHub (Core N/A)

**Or reuse existing S2/S3 with `PartialSync: true` flag**

---

## Implementation Steps

### Phase 1: Expand Detection Logic (TDD)

1. **Write failing test** - `classifier_test.go`
```go
func TestDetectTwoWaySync_E2_LocalAhead(t *testing.T) {
    // Mock: Local at abc123, Core at def456, GitHub doesn't exist
    // Expected: S2a (or S2 with PartialSync=true)
}
```

2. **Implement `detectTwoWaySync()`**
   - Extract local hash
   - Extract remote hash (core OR github)
   - Calculate ahead/behind counts
   - Return appropriate SyncState

3. **Update classifier switch** - Use new method for E2/E3

4. **Run test** - Verify detection works

### Phase 2: Update Suggester

**Modify:** `internal/scenarios/suggester.go:113-251`

**Update S2/S3 fix suggestions:**
```go
case "S2", "S2a", "S2b": // Local ahead (full or partial)
    remote := coreRemote
    if sync.ID == "S2b" {
        remote = githubRemote
    }
    return []Fix{{
        ScenarioID:  sync.ID,
        Description: fmt.Sprintf("Local has unpushed commits to %s", remote),
        Command:     fmt.Sprintf("git push %s %s", remote, sync.Branch),
        // ...
    }}
```

### Phase 3: Update Human Output

**Modify:** `cmd/githelper/status.go`

**Display partial sync info:**
```go
if state.Sync.PartialSync {
    fmt.Printf("🔄 Sync Status (partial - %s missing):\n", missingRemote)
} else {
    fmt.Printf("🔄 Sync Status:\n")
}
```

### Phase 4: Tests

**Test coverage:**
- ✓ E2 with local ahead (9 commits)
- ✓ E2 with local behind (5 commits)
- ✓ E2 with local synced (0 ahead/behind)
- ✓ E3 scenarios (same but with GitHub)
- ✓ JSON marshaling includes partial_sync field
- ✓ Suggester returns correct fixes for S2a/S3a

---

## Validation Criteria

**Success means:**

1. **E2 scenario detection:**
```bash
$ cd ~/wk  # Has origin but no github remote
$ githelper status

🔄 Sync Status:
  S2 - Local ahead of Core
  Branch: main
  Local: 9 commits ahead of origin
  GitHub: N/A (not configured)
```

2. **JSON output includes sync data:**
```json
{
  "existence": { "id": "E2" },
  "sync": {
    "id": "S2",
    "partial_sync": true,
    "local_ahead_of_core": 9,
    "local_ahead_of_github": 0,
    "github_hash": ""
  }
}
```

3. **Suggester provides fix:**
```bash
$ githelper status --show-fixes

Suggested Fixes:
  [S2] Local has unpushed commits to origin
    Command: git push origin main
    Auto-fixable: true
```

4. **E1 scenarios unaffected:**
   - Full three-way sync detection still works
   - No regression in existing tests

---

## Files to Modify

| File | Change | Lines |
|------|--------|-------|
| `internal/scenarios/classifier.go` | Add `detectTwoWaySync()`, update switch | +80 |
| `internal/scenarios/types.go` | Add `PartialSync bool` field | +1 |
| `internal/scenarios/tables.go` | Add S2a/S3a scenarios (optional) | +20 |
| `internal/scenarios/suggester.go` | Update S2/S3 fix logic | +10 |
| `cmd/githelper/status.go` | Display partial sync indicator | +5 |
| `internal/scenarios/classifier_test.go` | Add E2/E3 test cases | +100 |

**Total:** ~216 lines of changes

---

## Alternative: Simpler Approach

Instead of new scenario IDs, keep S2/S3 but:
- Always populate available fields (local_ahead_of_core even in E2)
- Add `PartialSync: true` flag
- Update description to indicate partial state

**Example:**
```json
{
  "sync": {
    "id": "S2",
    "description": "Local ahead of Core (GitHub N/A)",
    "partial_sync": true,
    "local_ahead_of_core": 9,
    "local_ahead_of_github": 0,
    "github_hash": ""
  }
}
```

This avoids scenario ID proliferation and keeps the fix suggestions simpler.

---

## Decision Point

**Option A:** New scenario IDs (S2a, S2b, S3a, S3b)
- ✓ More explicit
- ✗ More scenarios to maintain

**Option B:** Reuse S2/S3 with `PartialSync` flag
- ✓ Simpler
- ✓ Fewer scenario definitions
- ✓ Fix suggestions can check flag instead of ID

**Recommendation:** **Option B** - Use existing S2/S3 IDs with `PartialSync: true`

---

## Rollout

1. **Implement** - Follow Phase 1-4
2. **Test** - Run corpus validator (should still pass)
3. **Test** - Run on ~/wk repo (E2 scenario)
4. **Commit** - TDD style with tests first
5. **Deploy** - Rebuild and reinstall to ~/.local/bin
6. **Validate** - User runs `githelper status` in ~/wk

---

## Risk Assessment

**Low Risk:**
- E1 scenarios (fully configured) - No changes to detection logic
- E4-E8 scenarios (insufficient locations) - Still return N/A

**Medium Risk:**
- E2/E3 scenarios - New detection logic could have bugs
- Mitigation: Comprehensive tests before deployment

**Zero Risk:**
- Corpus validation - Only tests E1 scenarios, won't be affected

---

## Timeline Estimate

- Phase 1 (Detection): 30 min
- Phase 2 (Suggester): 15 min
- Phase 3 (Display): 10 min
- Phase 4 (Tests): 20 min
- **Total: ~75 minutes**
