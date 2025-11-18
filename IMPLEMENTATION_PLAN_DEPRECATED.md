# GitHelper Scenario Classification System - Implementation Plan

**Version:** 1.0
**Date:** 2025-11-18
**Target Completion:** 4-6 weeks
**Estimated LOC:** ~2,500 lines (production + tests)

---

## Executive Summary

### Objective

Implement a comprehensive state classification system for GitHelper that detects and identifies repository states across three locations (Local, Core, GitHub), enabling automated diagnosis, repair suggestions, and intelligent command orchestration.

### Scope

**In Scope:**
- Scenario classification across 5 dimensions (41 distinct scenarios)
- Automated fix suggestion with priority ordering
- Auto-fix capability for 60% of scenarios
- Integration with existing `doctor` command
- New `status`, `repair`, and `scenarios` commands
- Comprehensive test coverage (unit + integration)

**Out of Scope (Future Phases):**
- BFG cleanup automation (manual guide only)
- Multi-repository batch operations
- Submodule state tracking
- Real-time sync monitoring

### Success Criteria

1. ✅ `doctor` command reports scenario IDs (E1-E8, S1-S13, B1-B7, W1-W5, C1-C8)
2. ✅ Auto-fix resolves 60%+ of detected issues without manual intervention
3. ✅ All 41 scenarios have integration tests with >90% coverage
4. ✅ Zero regressions in existing commands
5. ✅ External review approval from 2+ engineers

### Key Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Scenario coverage | 0/41 (0%) | 41/41 (100%) |
| Auto-fixable issues | N/A | 25/41 (61%) |
| Test coverage | ~40% | >90% |
| Mean time to diagnosis | Manual | <2 seconds |
| False positive rate | N/A | <5% |

---

## Architecture Overview

### Component Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        Commands Layer                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  doctor  │  │  status  │  │  repair  │  │scenarios │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
└───────┼─────────────┼─────────────┼─────────────┼──────────┘
        │             │             │             │
        └─────────────┴─────────────┴─────────────┘
                          │
        ┌─────────────────▼─────────────────────────────────┐
        │         internal/scenarios (NEW)                  │
        │  ┌──────────────┐  ┌──────────────┐              │
        │  │  Classifier  │  │  Suggester   │              │
        │  │  (Detect)    │  │ (SuggestFix) │              │
        │  └──────┬───────┘  └──────┬───────┘              │
        │         │                  │                       │
        │  ┌──────▼──────────────────▼───────┐              │
        │  │  types.go  │  tables.go         │              │
        │  └────────────┴────────────────────┘              │
        └───────────────────┬────────────────────────────────┘
                            │
        ┌───────────────────▼────────────────────────────────┐
        │         internal/git (ENHANCED)                    │
        │  ┌────────────────────────────────────────┐        │
        │  │  cli.go + 6 new methods                │        │
        │  │  - GetBranchHash()                     │        │
        │  │  - CountCommitsBetween()               │        │
        │  │  - GetStagedFiles()                    │        │
        │  │  - ... (see Phase 1)                   │        │
        │  └────────────────────────────────────────┘        │
        └────────────────────────────────────────────────────┘
```

### Data Flow

```
User runs: githelper doctor myproject
    │
    ├─> Initialize Classifier(git.Client, github.Client)
    │
    ├─> classifier.Detect()
    │    ├─> detectExistence() → ExistenceState (E1-E8)
    │    ├─> detectWorkingTree() → WorkingTreeState (W1-W5)
    │    ├─> detectCorruption() → CorruptionState (C1-C8)
    │    ├─> detectDefaultBranchSync() → BranchSyncState (S1-S13)
    │    └─> detectBranchTopology() → []BranchState (B1-B7 each)
    │
    ├─> SuggestFixes(state) → []Fix (prioritized)
    │
    ├─> Display findings with scenario IDs
    │
    └─> If --auto-fix: AutoFix(state) → Apply fixable repairs
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2) ~800 LOC

**Goal:** Establish core data structures and git client extensions

#### 1.1: Data Structures (`internal/scenarios/types.go`)

**Priority:** P0 (Critical path)
**LOC:** ~200 lines
**Dependencies:** None

**File:** `internal/scenarios/types.go`

```go
package scenarios

import "time"

// RepositoryState represents complete multi-dimensional state
type RepositoryState struct {
    Repo          string
    DetectedAt    time.Time
    Existence     ExistenceState
    WorkingTree   WorkingTreeState
    Corruption    CorruptionState
    DefaultBranch BranchSyncState
    Branches      []BranchState
}

// ExistenceState (D1: E1-E8)
type ExistenceState struct {
    ID           string  // "E1", "E2", ..., "E8"
    LocalExists  bool
    LocalPath    string
    CoreExists   bool
    CoreURL      string
    GitHubExists bool
    GitHubURL    string
}

// WorkingTreeState (D4: W1-W5)
type WorkingTreeState struct {
    ID                string
    Clean             bool
    StagedFiles       []string
    UnstagedFiles     []string
    WouldConflict     bool
}

// CorruptionState (D5: C1-C8)
type CorruptionState struct {
    ID               string
    HasCorruption    bool
    LocalBinaries    []LargeBinary
    CoreBinaries     []LargeBinary
    GitHubBinaries   []LargeBinary
}

type LargeBinary struct {
    Path       string
    SizeMB     float64
    CommitHash string
    CommitDate time.Time
}

// BranchSyncState (D2: S1-S13)
type BranchSyncState struct {
    ID              string
    Branch          string
    LocalHash       string
    CoreHash        string
    GitHubHash      string

    // Pairwise comparisons
    LocalVsCore     SyncStatus
    LocalVsGitHub   SyncStatus
    CoreVsGitHub    SyncStatus

    // Quantitative deltas
    LocalAheadCore    int
    LocalBehindCore   int
    LocalAheadGitHub  int
    LocalBehindGitHub int
    CoreAheadGitHub   int
    CoreBehindGitHub  int
}

// BranchState (D3: B1-B7)
type BranchState struct {
    ID           string
    Name         string
    LocalExists  bool
    CoreExists   bool
    GitHubExists bool
    SyncState    *BranchSyncState  // Only if on multiple locations
}

// SyncStatus enum
type SyncStatus int

const (
    StatusUnknown SyncStatus = iota
    StatusSynced
    StatusAhead
    StatusBehind
    StatusDiverged
)

func (s SyncStatus) String() string {
    return [...]string{"unknown", "synced", "ahead", "behind", "diverged"}[s]
}
```

**Testing Requirements:**
- Unit test for enum String() method
- Validation that zero values are sensible

**Acceptance Criteria:**
- [ ] All type definitions compile
- [ ] Package documentation complete
- [ ] Zero values documented

---

#### 1.2: Git Client Extensions (`internal/git/cli.go`)

**Priority:** P0 (Blocks classifier)
**LOC:** ~250 lines
**Dependencies:** Existing git.Client

**File:** `internal/git/cli.go` (append to existing)

**New Methods Required:**

```go
// LocalExists checks if repository exists locally
func (c *Client) LocalExists() (bool, string) {
    // Implementation:
    // 1. Check if c.repoPath exists
    // 2. Check if .git directory/file exists
    // 3. Return (exists, absolutePath)
}

// GetStagedFiles returns list of staged files
func (c *Client) GetStagedFiles() ([]string, error) {
    // git diff --cached --name-only
}

// GetUnstagedFiles returns list of unstaged modifications
func (c *Client) GetUnstagedFiles() ([]string, error) {
    // git diff --name-only
}

// GetBranchHash returns commit hash for local branch
func (c *Client) GetBranchHash(branch string) (string, error) {
    // git rev-parse <branch>
}

// GetRemoteBranchHash returns commit hash for remote branch
func (c *Client) GetRemoteBranchHash(remote, branch string) (string, error) {
    // git rev-parse <remote>/<branch>
    // Handle case where remote branch doesn't exist
}

// CountCommitsBetween counts commits in ref1 not in ref2
func (c *Client) CountCommitsBetween(ref1, ref2 string) (int, error) {
    // git rev-list --count <ref1> ^<ref2>
    // Returns 0 if refs are equal
}

// GetDefaultBranch returns the default branch name
func (c *Client) GetDefaultBranch() (string, error) {
    // Try: git symbolic-ref refs/remotes/origin/HEAD
    // Fallback: Check for main, then master
}

// CanReachRemote tests if remote is accessible
func (c *Client) CanReachRemote(remote string) bool {
    // git ls-remote --exit-code <remote> HEAD
    // Return true if exit code 0, false otherwise
    // Timeout after 5 seconds
}

// ListBranches returns all branches (local and remote)
func (c *Client) ListBranches() (local, remote []string, error) {
    // git branch --format='%(refname:short)'
    // git branch -r --format='%(refname:short)'
}

// GetRemoteURL returns URL for named remote
// Already exists, verify signature matches needs
```

**Implementation Notes:**
- Use existing `c.run()` helper for git commands
- Handle missing refs gracefully (return empty string, no error)
- Timeout on remote operations: 5 seconds
- Cache results where appropriate (e.g., branch lists)

**Testing Requirements:**
- Unit tests with mock git output
- Integration tests with real test repos
- Test edge cases: missing remotes, detached HEAD, empty repos

**Acceptance Criteria:**
- [ ] All 9 methods implemented
- [ ] 100% unit test coverage
- [ ] Integration tests pass
- [ ] Handles all edge cases (documented in tests)

---

#### 1.3: Classification Tables (`internal/scenarios/tables.go`)

**Priority:** P0 (Blocks classifier)
**LOC:** ~150 lines
**Dependencies:** types.go

**File:** `internal/scenarios/tables.go`

```go
package scenarios

// classifyExistence maps 3 booleans to E1-E8
func classifyExistence(local, core, github bool) string {
    switch {
    case local && core && github:
        return "E1"  // Dual-remote managed
    case local && core && !github:
        return "E2"  // Core-only
    case local && !core && github:
        return "E3"  // GitHub-only clone
    case local && !core && !github:
        return "E4"  // Local-only
    case !local && core && github:
        return "E5"  // Missing local copy
    case !local && core && !github:
        return "E6"  // Core orphan
    case !local && !core && github:
        return "E7"  // GitHub orphan
    default:
        return "E8"  // Doesn't exist
    }
}

// classifyWorkingTree maps working tree state to W1-W5
func classifyWorkingTree(clean, hasStaged, hasUnstaged bool) string {
    if clean {
        return "W1"  // Clean
    }
    if hasStaged && !hasUnstaged {
        return "W2"  // Staged only
    }
    if !hasStaged && hasUnstaged {
        return "W3"  // Unstaged only
    }
    return "W4"  // Mixed
    // W5 (conflicts) detected during pull attempts, not here
}

// classifyCorruption maps binary presence to C1-C8
func classifyCorruption(local, core, github bool) string {
    // Same logic as classifyExistence, different IDs
    // C1 = none, C2-C8 = various combinations
}

// classifyBranchTopology maps branch presence to B1-B7
func classifyBranchTopology(local, core, github bool) string {
    switch {
    case local && core && github:
        return "B1"  // Fully tracked
    case local && core && !github:
        return "B2"  // Core-tracked only
    case local && !core && github:
        return "B3"  // GitHub-tracked only
    case local && !core && !github:
        return "B4"  // Local only
    case !local && core && github:
        return "B5"  // Remote branch not fetched
    case !local && core && !github:
        return "B6"  // Core orphan branch
    case !local && !core && github:
        return "B7"  // GitHub orphan branch
    default:
        return "B_INVALID"  // Should never happen
    }
}

// syncStateKey for map lookup
type syncStateKey struct {
    localVsCore   SyncStatus
    localVsGitHub SyncStatus
    coreVsGitHub  SyncStatus
}

// syncStateTable: The authoritative mapping of valid sync states
// IMPORTANT: All 13 valid states MUST be here
var syncStateTable = map[syncStateKey]string{
    // Perfect sync
    {StatusSynced, StatusSynced, StatusSynced}: "S1",

    // Local ahead scenarios
    {StatusAhead, StatusAhead, StatusSynced}:   "S2",  // Local ahead of both
    {StatusAhead, StatusSynced, StatusBehind}:  "S4",  // Local ahead of Core only
    {StatusAhead, StatusBehind, StatusAhead}:   "S8",  // Complex ahead/behind

    // Local behind scenarios
    {StatusBehind, StatusBehind, StatusSynced}: "S6",  // Both remotes ahead
    {StatusBehind, StatusSynced, StatusAhead}:  "S7",  // Core ahead only

    // Remote-only sync scenarios
    {StatusSynced, StatusAhead, StatusBehind}:  "S3",  // Core→GitHub push only
    {StatusSynced, StatusBehind, StatusAhead}:  "S5",  // GitHub has commits

    // Divergence scenarios
    {StatusDiverged, StatusDiverged, StatusSynced}:   "S9",  // Local diverged from synced remotes
    {StatusSynced, StatusDiverged, StatusDiverged}:   "S10", // Remotes diverged, local=core
    {StatusDiverged, StatusSynced, StatusDiverged}:   "S11", // Remotes diverged, local=github
    {StatusAhead, StatusDiverged, StatusDiverged}:    "S12", // All diverged (local ahead)
    {StatusDiverged, StatusDiverged, StatusDiverged}: "S13", // Three-way divergence
}

// classifySyncState performs lookup
func classifySyncState(localVsCore, localVsGitHub, coreVsGitHub SyncStatus) string {
    key := syncStateKey{localVsCore, localVsGitHub, coreVsGitHub}

    if id, exists := syncStateTable[key]; exists {
        return id
    }

    // Log warning for invalid state (should never happen due to transitivity)
    return "S_UNKNOWN"
}
```

**Testing Requirements:**
- **Critical:** Test all 13 valid sync state combinations
- Test all 8 existence combinations
- Test all 5 working tree combinations
- Test all 7 branch topology combinations
- Verify invalid sync states return "S_UNKNOWN"

**Acceptance Criteria:**
- [ ] All 13 sync states tested
- [ ] Table lookup performance <1µs
- [ ] Unknown states handled gracefully
- [ ] Documentation explains each scenario

---

### Phase 2: Classifier Core (Week 2-3) ~600 LOC

**Goal:** Implement hierarchical state detection

#### 2.1: Classifier Implementation (`internal/scenarios/classifier.go`)

**Priority:** P0 (Core feature)
**LOC:** ~400 lines
**Dependencies:** types.go, tables.go, git/cli.go extensions

**File:** `internal/scenarios/classifier.go`

```go
package scenarios

import (
    "context"
    "fmt"
    "time"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/lcgerke/githelper/internal/github"
)

type Classifier struct {
    repoPath     string
    gitClient    *git.Client
    githubClient *github.Client
    ctx          context.Context
}

func NewClassifier(ctx context.Context, repoPath string, gc *git.Client, ghc *github.Client) *Classifier {
    return &Classifier{
        ctx:          ctx,
        repoPath:     repoPath,
        gitClient:    gc,
        githubClient: ghc,
    }
}

// Detect performs full hierarchical state detection
func (c *Classifier) Detect() (*RepositoryState, error) {
    state := &RepositoryState{
        Repo:       c.repoPath,
        DetectedAt: time.Now(),
    }

    // D1: Existence (REQUIRED for all other checks)
    var err error
    state.Existence, err = c.detectExistence()
    if err != nil {
        return nil, fmt.Errorf("existence check failed: %w", err)
    }

    // Early return if local doesn't exist
    if !state.Existence.LocalExists {
        return state, nil
    }

    // D4: Working tree (independent of remotes)
    state.WorkingTree, err = c.detectWorkingTree()
    if err != nil {
        return nil, fmt.Errorf("working tree check failed: %w", err)
    }

    // D5: Corruption (expensive, can be skipped in quick mode)
    state.Corruption, err = c.detectCorruption()
    if err != nil {
        // Log warning but don't fail entire detection
        state.Corruption = CorruptionState{ID: "C_UNKNOWN"}
    }

    // D2: Default branch sync (requires remotes)
    if state.Existence.CoreExists || state.Existence.GitHubExists {
        state.DefaultBranch, err = c.detectDefaultBranchSync()
        if err != nil {
            return nil, fmt.Errorf("default branch sync check failed: %w", err)
        }
    }

    // D3: Per-branch topology
    state.Branches, err = c.detectBranchTopology()
    if err != nil {
        // Log warning, return partial state
        state.Branches = []BranchState{}
    }

    return state, nil
}

// detectExistence checks where repository exists (E1-E8)
func (c *Classifier) detectExistence() (ExistenceState, error) {
    state := ExistenceState{}

    // Check local
    state.LocalExists, state.LocalPath = c.gitClient.LocalExists()

    // Early return if local doesn't exist
    if !state.LocalExists {
        state.ID = classifyExistence(false, false, false)
        return state, nil
    }

    // Check Core remote
    coreURL, err := c.gitClient.GetRemoteURL("origin")
    if err == nil && coreURL != "" {
        state.CoreURL = coreURL
        state.CoreExists = c.gitClient.CanReachRemote("origin")
    }

    // Check GitHub remote
    githubURL, err := c.gitClient.GetRemoteURL("github")
    if err == nil && githubURL != "" {
        state.GitHubURL = githubURL
        state.GitHubExists = c.gitClient.CanReachRemote("github")
    }

    // Classify
    state.ID = classifyExistence(state.LocalExists, state.CoreExists, state.GitHubExists)

    return state, nil
}

// detectWorkingTree checks for uncommitted changes (W1-W5)
func (c *Classifier) detectWorkingTree() (WorkingTreeState, error) {
    state := WorkingTreeState{}

    staged, err := c.gitClient.GetStagedFiles()
    if err != nil {
        return state, err
    }
    state.StagedFiles = staged

    unstaged, err := c.gitClient.GetUnstagedFiles()
    if err != nil {
        return state, err
    }
    state.UnstagedFiles = unstaged

    state.Clean = len(staged) == 0 && len(unstaged) == 0

    // Classify
    state.ID = classifyWorkingTree(state.Clean, len(staged) > 0, len(unstaged) > 0)

    return state, nil
}

// detectCorruption scans for large binaries (C1-C8)
func (c *Classifier) detectCorruption() (CorruptionState, error) {
    state := CorruptionState{
        ID: "C1",  // Default: no corruption
        HasCorruption: false,
    }

    // Scan local history for large files (>10MB threshold)
    // git rev-list --objects --all | \
    //   git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' | \
    //   awk '$1 == "blob" && $3 >= 10485760'

    binaries, err := c.scanLargeBinaries()
    if err != nil {
        return state, err
    }

    if len(binaries) > 0 {
        state.LocalBinaries = binaries
        state.HasCorruption = true
        // For now, assume if local has them, remotes might too
        // Full detection would require scanning remotes (expensive)
        state.ID = classifyCorruption(true, false, false)
    }

    return state, nil
}

// scanLargeBinaries finds files >10MB in history
func (c *Classifier) scanLargeBinaries() ([]LargeBinary, error) {
    // Implementation:
    // 1. Run git rev-list --objects --all
    // 2. Pipe to git cat-file --batch-check
    // 3. Parse output for large blobs
    // 4. For each, get commit hash and date
    // Return list of LargeBinary structs

    // Threshold: 10MB = 10485760 bytes
    const threshold = 10 * 1024 * 1024

    // TODO: Implement using gitClient
    return []LargeBinary{}, nil
}

// detectDefaultBranchSync determines sync state (S1-S13)
func (c *Classifier) detectDefaultBranchSync() (BranchSyncState, error) {
    state := BranchSyncState{}

    // Get default branch name
    defaultBranch, err := c.gitClient.GetDefaultBranch()
    if err != nil {
        return state, err
    }
    state.Branch = defaultBranch

    // Get commit hashes
    state.LocalHash, _ = c.gitClient.GetBranchHash(defaultBranch)
    state.CoreHash, _ = c.gitClient.GetRemoteBranchHash("origin", defaultBranch)
    state.GitHubHash, _ = c.gitClient.GetRemoteBranchHash("github", defaultBranch)

    // Compare pairwise
    state.LocalVsCore, state.LocalAheadCore, state.LocalBehindCore =
        c.compareBranches(state.LocalHash, state.CoreHash)

    state.LocalVsGitHub, state.LocalAheadGitHub, state.LocalBehindGitHub =
        c.compareBranches(state.LocalHash, state.GitHubHash)

    state.CoreVsGitHub, state.CoreAheadGitHub, state.CoreBehindGitHub =
        c.compareBranches(state.CoreHash, state.GitHubHash)

    // Classify using lookup table
    state.ID = classifySyncState(state.LocalVsCore, state.LocalVsGitHub, state.CoreVsGitHub)

    return state, nil
}

// compareBranches determines relationship between two refs
func (c *Classifier) compareBranches(ref1, ref2 string) (SyncStatus, int, int) {
    if ref1 == "" || ref2 == "" {
        return StatusUnknown, 0, 0
    }

    if ref1 == ref2 {
        return StatusSynced, 0, 0
    }

    // Count commits ref1 has that ref2 doesn't
    ahead, err := c.gitClient.CountCommitsBetween(ref1, ref2)
    if err != nil {
        return StatusUnknown, 0, 0
    }

    // Count commits ref2 has that ref1 doesn't
    behind, err := c.gitClient.CountCommitsBetween(ref2, ref1)
    if err != nil {
        return StatusUnknown, 0, 0
    }

    if ahead > 0 && behind > 0 {
        return StatusDiverged, ahead, behind
    } else if ahead > 0 {
        return StatusAhead, ahead, 0
    } else if behind > 0 {
        return StatusBehind, 0, behind
    }

    return StatusSynced, 0, 0
}

// detectBranchTopology analyzes all branches (B1-B7 each)
func (c *Classifier) detectBranchTopology() ([]BranchState, error) {
    localBranches, remoteBranches, err := c.gitClient.ListBranches()
    if err != nil {
        return nil, err
    }

    // Build map of all unique branch names
    branchMap := make(map[string]*BranchState)

    // Add local branches
    for _, branch := range localBranches {
        branchMap[branch] = &BranchState{
            Name:        branch,
            LocalExists: true,
        }
    }

    // Add remote branches
    for _, remoteBranch := range remoteBranches {
        // Parse "origin/branch" or "github/branch"
        parts := strings.SplitN(remoteBranch, "/", 2)
        if len(parts) != 2 {
            continue
        }
        remote, branch := parts[0], parts[1]

        if _, exists := branchMap[branch]; !exists {
            branchMap[branch] = &BranchState{Name: branch}
        }

        if remote == "origin" {
            branchMap[branch].CoreExists = true
        } else if remote == "github" {
            branchMap[branch].GitHubExists = true
        }
    }

    // Classify each branch and convert to slice
    branches := []BranchState{}
    for _, state := range branchMap {
        state.ID = classifyBranchTopology(
            state.LocalExists,
            state.CoreExists,
            state.GitHubExists,
        )

        // If branch exists on multiple locations, check sync state
        if state.ID == "B1" {  // Fully tracked
            // TODO: Optionally check per-branch sync state
            // For now, skip to avoid O(n) git operations per branch
        }

        branches = append(branches, *state)
    }

    return branches, nil
}
```

**Implementation Notes:**
- Use context for cancellation
- Log all errors via structured logger
- Cache expensive operations (large binary scan)
- Handle partial failures gracefully

**Testing Requirements:**
- Unit tests with mocked git client
- Integration tests for all 41 scenarios
- Performance test: <2s for typical repo
- Error handling: network failures, missing remotes

**Acceptance Criteria:**
- [ ] Detects all 41 scenarios correctly
- [ ] Handles partial failures (e.g., one remote unreachable)
- [ ] Performance: <2s for 10-branch repo
- [ ] Zero panics on malformed input

---

### Phase 3: Fix Suggestion Engine (Week 3-4) ~500 LOC

**Goal:** Suggest and auto-apply fixes

#### 3.1: Fix Suggester (`internal/scenarios/suggester.go`)

**Priority:** P1 (High value)
**LOC:** ~300 lines
**Dependencies:** types.go, classifier.go

**File:** `internal/scenarios/suggester.go`

```go
package scenarios

import (
    "fmt"
    "sort"
    "strings"

    "github.com/lcgerke/githelper/internal/git"
)

// Fix represents a suggested repair action
type Fix struct {
    ScenarioID  string
    Description string
    Command     string
    AutoFixable bool
    Priority    int     // Lower = more urgent (1-10)
    Reason      string
}

// SuggestFixes analyzes state and returns prioritized fixes
func SuggestFixes(state *RepositoryState) []Fix {
    fixes := []Fix{}

    // D1: Existence issues
    fixes = append(fixes, suggestExistenceFixes(state.Existence)...)

    // D4: Working tree
    if state.WorkingTree.ID != "W1" {
        fixes = append(fixes, Fix{
            ScenarioID:  state.WorkingTree.ID,
            Description: fmt.Sprintf("Working tree has uncommitted changes (%d staged, %d unstaged)",
                len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles)),
            Command:     "git stash  # or: git commit -am 'WIP'",
            AutoFixable: false,  // Requires user decision
            Priority:    2,
            Reason:      "Automated sync requires clean working tree",
        })
    }

    // D5: Corruption
    if state.Corruption.HasCorruption {
        fixes = append(fixes, Fix{
            ScenarioID:  state.Corruption.ID,
            Description: fmt.Sprintf("Large binaries in history (%d files, %.1f MB total)",
                len(state.Corruption.LocalBinaries), totalSizeMB(state.Corruption.LocalBinaries)),
            Command:     "See docs/BFG_CLEANUP.md for removal procedure",
            AutoFixable: false,
            Priority:    3,
            Reason:      "Large files should be removed before syncing",
        })
    }

    // D2: Default branch sync
    fixes = append(fixes, suggestSyncFixes(state.DefaultBranch)...)

    // D3: Branch topology
    for _, branch := range state.Branches {
        fixes = append(fixes, suggestBranchFixes(branch)...)
    }

    // Sort by priority
    sort.Slice(fixes, func(i, j int) bool {
        return fixes[i].Priority < fixes[j].Priority
    })

    return fixes
}

// suggestExistenceFixes handles E1-E8
func suggestExistenceFixes(state ExistenceState) []Fix {
    switch state.ID {
    case "E1":
        return nil  // All good

    case "E2":  // Core exists, GitHub doesn't
        return []Fix{{
            ScenarioID:  "E2",
            Description: "GitHub remote not configured",
            Command:     "githelper github setup --create",
            AutoFixable: true,
            Priority:    1,
            Reason:      "Dual-push requires both Core and GitHub",
        }}

    case "E3":  // GitHub exists, Core doesn't
        return []Fix{{
            ScenarioID:  "E3",
            Description: "Core remote not configured",
            Command:     "git remote add origin <core-url>",
            AutoFixable: false,  // Requires URL from config
            Priority:    1,
            Reason:      "Primary remote missing",
        }}

    case "E4":  // Local only
        return []Fix{{
            ScenarioID:  "E4",
            Description: "No remotes configured",
            Command:     "githelper repo create <name>",
            AutoFixable: false,
            Priority:    1,
            Reason:      "Repository not integrated with system",
        }}

    default:
        return nil
    }
}

// suggestSyncFixes handles S1-S13
func suggestSyncFixes(state BranchSyncState) []Fix {
    switch state.ID {
    case "S1":
        return nil  // Perfect sync

    case "S2":  // Local ahead of both
        return []Fix{{
            ScenarioID:  "S2",
            Description: fmt.Sprintf("Local ahead of both remotes (%d commits)",
                state.LocalAheadCore),
            Command:     "git push",
            AutoFixable: true,
            Priority:    4,
            Reason:      "Dual-push will sync both remotes",
        }}

    case "S3":  // Core pushed to GitHub without local
        return []Fix{{
            ScenarioID:  "S3",
            Description: "Remote sync happened without local update",
            Command:     "git fetch origin",
            AutoFixable: true,
            Priority:    4,
            Reason:      "Local needs update from core",
        }}

    case "S4":  // Local ahead of GitHub only
        return []Fix{{
            ScenarioID:  "S4",
            Description: fmt.Sprintf("Local ahead of GitHub (%d commits)",
                state.LocalAheadGitHub),
            Command:     "githelper github sync",
            AutoFixable: true,
            Priority:    4,
            Reason:      "GitHub behind, needs selective sync",
        }}

    case "S5":  // GitHub has commits
        return []Fix{{
            ScenarioID:  "S5",
            Description: fmt.Sprintf("GitHub has commits not in local/core (%d commits)",
                state.CoreBehindGitHub),
            Command:     "git fetch github && git merge github/" + state.Branch,
            AutoFixable: false,  // Might have conflicts
            Priority:    5,
            Reason:      "Manual merge required",
        }}

    case "S6":  // Both remotes ahead
        return []Fix{{
            ScenarioID:  "S6",
            Description: fmt.Sprintf("Both remotes ahead (%d commits)",
                state.LocalBehindCore),
            Command:     "git pull",
            AutoFixable: true,
            Priority:    4,
            Reason:      "Fast-forward pull will sync",
        }}

    case "S7":  // Core ahead only
        return []Fix{{
            ScenarioID:  "S7",
            Description: fmt.Sprintf("Core ahead of local (%d commits)",
                state.LocalBehindCore),
            Command:     "git pull origin " + state.Branch,
            AutoFixable: true,
            Priority:    4,
            Reason:      "Fast-forward from core",
        }}

    case "S8":  // Complex ahead/behind
        return []Fix{{
            ScenarioID:  "S8",
            Description: "Complex sync state: local ahead of GitHub, behind Core",
            Command:     "git pull origin && git push github",
            AutoFixable: false,  // Requires careful ordering
            Priority:    5,
            Reason:      "Manual resolution recommended",
        }}

    case "S9", "S10", "S11", "S12", "S13":  // Divergence
        return []Fix{{
            ScenarioID:  state.ID,
            Description: "Divergent histories detected",
            Command:     "Manual merge required - see docs/DIVERGENCE.md",
            AutoFixable: false,
            Priority:    5,
            Reason:      "Divergence requires conflict resolution",
        }}

    default:
        return nil
    }
}

// suggestBranchFixes handles B1-B7
func suggestBranchFixes(state BranchState) []Fix {
    switch state.ID {
    case "B1":
        return nil  // Fully tracked

    case "B2", "B6":  // On Core but not GitHub
        return []Fix{{
            ScenarioID:  state.ID,
            Description: fmt.Sprintf("Branch '%s' not on GitHub", state.Name),
            Command:     fmt.Sprintf("git push github %s", state.Name),
            AutoFixable: true,
            Priority:    6,
            Reason:      "Complete dual-remote coverage",
        }}

    case "B3", "B7":  // On GitHub but not Core
        return []Fix{{
            ScenarioID:  state.ID,
            Description: fmt.Sprintf("Branch '%s' not on Core", state.Name),
            Command:     fmt.Sprintf("git push origin %s", state.Name),
            AutoFixable: true,
            Priority:    6,
            Reason:      "Core should have all branches",
        }}

    case "B4":  // Local only
        return []Fix{{
            ScenarioID:  "B4",
            Description: fmt.Sprintf("Branch '%s' not pushed", state.Name),
            Command:     fmt.Sprintf("git push origin %s", state.Name),
            AutoFixable: false,  // User decides when to push
            Priority:    7,
            Reason:      "Local work in progress",
        }}

    case "B5":  // Remote not fetched
        return []Fix{{
            ScenarioID:  "B5",
            Description: fmt.Sprintf("Remote branch '%s' not checked out", state.Name),
            Command:     fmt.Sprintf("git checkout -t origin/%s", state.Name),
            AutoFixable: true,
            Priority:    8,
            Reason:      "Optional: fetch remote branch",
        }}

    default:
        return nil
    }
}

// AutoFix applies all auto-fixable fixes
func AutoFix(state *RepositoryState, gitClient *git.Client) ([]Fix, []error) {
    fixes := SuggestFixes(state)
    applied := []Fix{}
    errors := []error{}

    for _, fix := range fixes {
        if !fix.AutoFixable {
            continue
        }

        // Apply fix based on scenario type
        var err error
        switch {
        case strings.HasPrefix(fix.ScenarioID, "E"):
            err = applyExistenceFix(fix, gitClient)
        case strings.HasPrefix(fix.ScenarioID, "S"):
            err = applySyncFix(fix, gitClient)
        case strings.HasPrefix(fix.ScenarioID, "B"):
            err = applyBranchFix(fix, gitClient)
        }

        if err != nil {
            errors = append(errors, fmt.Errorf("%s: %w", fix.ScenarioID, err))
        } else {
            applied = append(applied, fix)
        }
    }

    return applied, errors
}

// applyExistenceFix handles auto-fixable existence issues
func applyExistenceFix(fix Fix, gitClient *git.Client) error {
    switch fix.ScenarioID {
    case "E2":  // Setup GitHub
        // This requires github client, so delegate to command layer
        return fmt.Errorf("requires github setup command")
    default:
        return fmt.Errorf("not auto-fixable")
    }
}

// applySyncFix handles auto-fixable sync issues
func applySyncFix(fix Fix, gitClient *git.Client) error {
    switch fix.ScenarioID {
    case "S2":  // Local ahead of both
        return gitClient.Push("origin", "HEAD")
    case "S3":  // Fetch from origin
        return gitClient.Fetch("origin")
    case "S6":  // Pull from both
        return gitClient.Pull("origin")
    case "S7":  // Pull from origin
        return gitClient.Pull("origin")
    default:
        return fmt.Errorf("not auto-fixable")
    }
}

// applyBranchFix handles auto-fixable branch issues
func applyBranchFix(fix Fix, gitClient *git.Client) error {
    // Parse branch name from command
    // Execute git push/checkout as needed
    return fmt.Errorf("not implemented")
}

// Helper: calculate total size of binaries
func totalSizeMB(binaries []LargeBinary) float64 {
    total := 0.0
    for _, b := range binaries {
        total += b.SizeMB
    }
    return total
}
```

**Testing Requirements:**
- Unit test for each scenario ID
- Verify priority ordering
- Test auto-fix for safe scenarios
- Ensure non-fixable scenarios return error

**Acceptance Criteria:**
- [ ] All 41 scenarios have fix suggestions
- [ ] 25/41 scenarios are auto-fixable
- [ ] Priority ordering is correct
- [ ] No false positive fixes

---

### Phase 4: Command Integration (Week 4-5) ~400 LOC

**Goal:** Integrate classifier into user-facing commands

#### 4.1: Enhance `doctor` Command

**Priority:** P0 (User-facing)
**LOC:** ~150 lines
**Dependencies:** classifier.go, suggester.go

**File:** `cmd/githelper/doctor.go` (enhance existing)

**Changes Required:**

```go
// Add to existing runDoctor function

func runDoctor(cmd *cobra.Command, args []string) error {
    // ... existing setup ...

    // NEW: Create classifier
    classifier := scenarios.NewClassifier(ctx, repoPath, gitClient, ghClient)

    // NEW: Detect state
    state, err := classifier.Detect()
    if err != nil {
        return fmt.Errorf("state detection failed: %w", err)
    }

    // NEW: Enhanced output with scenario IDs
    if !out.IsJSON() {
        out.Header("Repository State Analysis: " + repoName)
        out.Separator()

        // Existence
        out.Infof("Existence:    %s (%s)",
            state.Existence.ID,
            explainScenario(state.Existence.ID))

        // Working Tree
        icon := "✓"
        if state.WorkingTree.ID != "W1" {
            icon = "⚠"
        }
        out.Infof("Working Tree: %s %s",
            icon,
            state.WorkingTree.ID)

        // Corruption
        icon = "✓"
        if state.Corruption.HasCorruption {
            icon = "⚠"
        }
        out.Infof("Integrity:    %s %s (%d large files)",
            icon,
            state.Corruption.ID,
            len(state.Corruption.LocalBinaries))

        // Default Branch Sync
        icon = "✓"
        if state.DefaultBranch.ID != "S1" {
            icon = "⚠"
        }
        out.Infof("Sync State:   %s %s (%s)",
            icon,
            state.DefaultBranch.ID,
            state.DefaultBranch.Branch)

        // Branches
        orphanCount := countOrphanBranches(state.Branches)
        icon = "✓"
        if orphanCount > 0 {
            icon = "⚠"
        }
        out.Infof("Branches:     %s %d tracked, %d orphan",
            icon,
            len(state.Branches)-orphanCount,
            orphanCount)
    }

    // NEW: Get fix suggestions
    fixes := scenarios.SuggestFixes(state)

    if len(fixes) == 0 {
        out.Success("\n✅ All systems healthy")
        return nil
    }

    // Display issues
    out.Separator()
    out.Warningf("Found %d issue(s):\n", len(fixes))

    for i, fix := range fixes {
        icon := "⚠"
        if fix.AutoFixable {
            icon = "✓"
        }
        out.Infof("  %d. [%s] %s: %s",
            i+1,
            icon,
            fix.ScenarioID,
            fix.Description)
        out.Infof("     Fix: %s", fix.Command)
        if verbose {
            out.Infof("     Reason: %s", fix.Reason)
        }
    }

    // Auto-fix if requested
    if autoFix {
        out.Separator()
        out.Info("Applying auto-fixes...\n")

        applied, errors := scenarios.AutoFix(state, gitClient)

        if len(applied) > 0 {
            out.Successf("Applied %d fix(es):", len(applied))
            for _, fix := range applied {
                out.Infof("  ✓ %s: %s", fix.ScenarioID, fix.Description)
            }
        }

        if len(errors) > 0 {
            out.Warningf("\n%d fix(es) failed:", len(errors))
            for _, err := range errors {
                out.Errorf("  ✗ %v", err)
            }
        }

        // Re-detect to show final state
        out.Info("\nRe-checking state...")
        newState, _ := classifier.Detect()
        newFixes := scenarios.SuggestFixes(newState)

        if len(newFixes) == 0 {
            out.Success("✅ All issues resolved!")
        } else {
            out.Warningf("⚠ %d issue(s) remaining (require manual fix)", len(newFixes))
        }
    }

    return nil
}

// Helper: explain scenario in human terms
func explainScenario(id string) string {
    // Map scenario IDs to brief descriptions
    explanations := map[string]string{
        "E1": "Dual-remote managed",
        "E2": "Core-only (GitHub missing)",
        "E3": "GitHub-only (Core missing)",
        // ... all 41 scenarios
    }
    return explanations[id]
}
```

**Acceptance Criteria:**
- [ ] Shows all 5 dimensions with scenario IDs
- [ ] Lists fixes with priority
- [ ] Auto-fix works for safe scenarios
- [ ] No regression in existing doctor functionality

---

#### 4.2: New `status` Command

**Priority:** P1 (Quick reference)
**LOC:** ~100 lines
**Dependencies:** classifier.go

**File:** `cmd/githelper/status.go` (NEW)

```go
package main

import (
    "fmt"
    "os"

    "github.com/lcgerke/githelper/internal/scenarios"
    "github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
    Use:   "status <repo-name>",
    Short: "Quick repository state summary",
    Long: `Shows repository state across all dimensions with scenario IDs.

Example:
  githelper status myproject

Output shows:
  - Existence state (E1-E8)
  - Sync state (S1-S13)
  - Working tree (W1-W5)
  - Integrity (C1-C8)
  - Branch status`,
    Args: cobra.ExactArgs(1),
    RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
    repoName := args[0]

    // Initialize (similar to doctor)
    // ...

    // Detect state
    classifier := scenarios.NewClassifier(ctx, repoPath, gitClient, ghClient)
    state, err := classifier.Detect()
    if err != nil {
        return err
    }

    // Quick summary output
    fmt.Printf("Repository: %s\n", repoName)
    fmt.Printf("  Existence:  %s (%s)\n",
        formatScenario(state.Existence.ID),
        scenarios.ExplainExistence(state.Existence.ID))
    fmt.Printf("  Sync State: %s (%s)\n",
        formatScenario(state.DefaultBranch.ID),
        scenarios.ExplainSync(state.DefaultBranch.ID))
    fmt.Printf("  Branches:   %d tracked, %d orphan\n",
        len(state.Branches)-countOrphans(state.Branches),
        countOrphans(state.Branches))
    fmt.Printf("  Tree:       %s\n", formatScenario(state.WorkingTree.ID))
    fmt.Printf("  Integrity:  %s\n", formatScenario(state.Corruption.ID))

    // Quick fix hint
    fixes := scenarios.SuggestFixes(state)
    if len(fixes) > 0 {
        fmt.Printf("\nQuick fix: %s\n", fixes[0].Command)
    }

    return nil
}

func formatScenario(id string) string {
    // Add color: green for good states, yellow for warnings
    // E1, S1, W1, C1 = green
    // Others = yellow
    goodStates := map[string]bool{
        "E1": true, "S1": true, "W1": true, "C1": true, "B1": true,
    }

    if goodStates[id] {
        return color.GreenString(id + " ✓")
    }
    return color.YellowString(id + " ⚠")
}
```

**Acceptance Criteria:**
- [ ] Shows concise state summary
- [ ] Colorized output (green=good, yellow=warning)
- [ ] Suggests quickest fix
- [ ] Runs in <1 second

---

### Phase 5: Testing & Documentation (Week 5-6) ~600 LOC

**Goal:** Comprehensive test coverage and documentation

#### 5.1: Unit Tests

**Files to create:**
- `internal/scenarios/types_test.go`
- `internal/scenarios/tables_test.go`
- `internal/scenarios/classifier_test.go`
- `internal/scenarios/suggester_test.go`

**Critical Tests:**

```go
// tables_test.go - Test all 13 valid sync states
func TestSyncStateClassification(t *testing.T) {
    tests := []struct {
        name          string
        localVsCore   SyncStatus
        localVsGitHub SyncStatus
        coreVsGitHub  SyncStatus
        expected      string
    }{
        {"S1: Perfect sync", StatusSynced, StatusSynced, StatusSynced, "S1"},
        {"S2: Local ahead both", StatusAhead, StatusAhead, StatusSynced, "S2"},
        // ... all 13 valid states
        {"Invalid state", StatusAhead, StatusBehind, StatusBehind, "S_UNKNOWN"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := classifySyncState(tt.localVsCore, tt.localVsGitHub, tt.coreVsGitHub)
            if result != tt.expected {
                t.Errorf("got %s, want %s", result, tt.expected)
            }
        })
    }
}

// suggester_test.go - Test fix generation
func TestSuggestFixes(t *testing.T) {
    tests := []struct {
        name          string
        state         *RepositoryState
        expectedCount int
        expectedFirst string
    }{
        {
            name: "E2: GitHub missing",
            state: &RepositoryState{
                Existence: ExistenceState{ID: "E2"},
            },
            expectedCount: 1,
            expectedFirst: "E2",
        },
        // ... test each scenario
    }
}
```

**Acceptance Criteria:**
- [ ] 100% coverage of tables.go
- [ ] All 41 scenarios tested
- [ ] All auto-fixable scenarios have tests
- [ ] Edge cases covered (missing remotes, network failures)

---

#### 5.2: Integration Tests

**File:** `test/integration/scenarios_test.go` (NEW)

**Approach:** Create real test repositories in known states

```go
func TestScenarioDetection_E1_DualRemote(t *testing.T) {
    // Setup: Create temp repo with both remotes
    testRepo := setupTestRepo(t,
        withLocalRepo(),
        withCoreRemote(),
        withGitHubRemote(),
    )
    defer testRepo.Cleanup()

    // Detect
    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, ghClient)
    state, err := classifier.Detect()
    require.NoError(t, err)

    // Assert
    assert.Equal(t, "E1", state.Existence.ID)
}

func TestScenarioDetection_S2_LocalAhead(t *testing.T) {
    testRepo := setupTestRepo(t,
        withDualRemote(),
        withCommit("local commit"),  // Not pushed
    )
    defer testRepo.Cleanup()

    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, ghClient)
    state, err := classifier.Detect()
    require.NoError(t, err)

    assert.Equal(t, "S2", state.DefaultBranch.ID)
    assert.Equal(t, 1, state.DefaultBranch.LocalAheadCore)
}

// Test all 41 scenarios with real repos
```

**Acceptance Criteria:**
- [ ] All 41 scenarios have integration tests
- [ ] Tests use real git operations
- [ ] Tests clean up temp repos
- [ ] Tests run in <30 seconds

---

#### 5.3: Documentation

**Files to create/update:**

1. **`docs/SCENARIOS.md`** (NEW) - User-facing scenario reference
   - Explanation of each scenario ID
   - How to interpret `doctor` output
   - Common troubleshooting paths

2. **`docs/ARCHITECTURE.md`** (NEW) - Developer reference
   - System architecture
   - Adding new scenarios
   - Extending the classifier

3. **`internal/scenarios/README.md`** (NEW) - Package documentation
   - API overview
   - Usage examples
   - Design decisions

4. **Update `README.md`** - Add scenario system to features

**Acceptance Criteria:**
- [ ] All public APIs documented with godoc
- [ ] User guide covers all 41 scenarios
- [ ] Architecture doc explains design
- [ ] Examples for common use cases

---

## Integration Plan

### Integration Points with Existing Code

| Existing Component | Integration Required | Risk Level |
|--------------------|---------------------|------------|
| `cmd/githelper/doctor.go` | Enhance with classifier | Low |
| `cmd/githelper/github_sync.go` | Add pre-condition checks | Medium |
| `internal/git/cli.go` | Add 9 new methods | Low |
| `internal/state/state.go` | None (read-only) | None |
| `internal/github/client.go` | None (already used) | None |

### Migration Strategy

**Phase 1-2:** Build in isolation (no risk to existing code)

**Phase 3:** Integrate with `doctor`:
1. Add classifier call behind feature flag
2. Run both old and new code paths
3. Compare outputs in logs
4. Validate in staging for 1 week
5. Enable for all users

**Phase 4:** Enhance other commands
- `github sync`: Add pre-conditions
- `github status`: Show scenario ID
- `repo list`: Add health column

### Rollback Plan

If critical issue discovered:
1. Disable feature flag (1 minute)
2. Revert PR (5 minutes)
3. Deploy previous version (10 minutes)

**Rollback criteria:**
- >5% false positive rate
- Any data corruption
- Performance regression >2x
- User complaints >10/day

---

## Risk Assessment & Mitigation

### Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| **Performance regression** | High | Medium | Benchmark tests, caching, lazy loading |
| **False positives in detection** | High | Medium | Comprehensive integration tests (all 41 scenarios) |
| **Network timeout during remote checks** | Medium | High | 5-second timeout, graceful degradation |
| **Breaking changes to git client** | Medium | Low | Unit tests for all git operations |
| **Auto-fix causes data loss** | Critical | Low | Never auto-fix diverged states, require confirmation |

### Mitigation Strategies

1. **Performance:**
   - Benchmark: `Detect()` must complete in <2s for typical repo
   - Cache expensive operations (large binary scan)
   - Offer `--quick` mode (skip corruption check)

2. **Correctness:**
   - Integration test for every single scenario (41 tests)
   - Property-based testing for sync state transitivity
   - Manual QA with real repositories

3. **Safety:**
   - Auto-fix only for idempotent operations (fetch, push to empty branch)
   - Require `--confirm` for destructive operations
   - Log all auto-fix actions for audit trail

4. **Usability:**
   - Clear error messages with scenario IDs
   - Help text explains each scenario
   - `--explain S4` command for detailed info

---

## Acceptance Criteria

### Functional Requirements

- [ ] **Scenario Detection:** All 41 scenarios correctly identified
- [ ] **Fix Suggestions:** All scenarios have appropriate fix suggestions
- [ ] **Auto-Fix:** 25/41 scenarios can be auto-fixed safely
- [ ] **Performance:** State detection completes in <2 seconds
- [ ] **Accuracy:** False positive rate <5%

### Non-Functional Requirements

- [ ] **Test Coverage:** >90% line coverage, 100% scenario coverage
- [ ] **Documentation:** All public APIs documented, user guide complete
- [ ] **Error Handling:** Graceful degradation on network failures
- [ ] **Logging:** Structured logging for all operations
- [ ] **Backward Compatibility:** Zero regressions in existing commands

### User Experience

- [ ] **Clarity:** Scenario IDs are self-explanatory with help text
- [ ] **Actionability:** Every warning includes clear fix command
- [ ] **Speed:** `status` command feels instant (<1s)
- [ ] **Confidence:** Auto-fix never causes data loss

---

## Delivery Milestones

### Milestone 1: Foundation (End of Week 2)
- [ ] All data structures defined
- [ ] Git client extensions complete
- [ ] Classification tables implemented
- [ ] Unit tests passing

### Milestone 2: Core Classifier (End of Week 3)
- [ ] Classifier detects all 5 dimensions
- [ ] All 41 scenarios testable
- [ ] Integration tests for existence + sync states

### Milestone 3: Fix Engine (End of Week 4)
- [ ] Fix suggester complete
- [ ] Auto-fix for safe scenarios
- [ ] Integration tests for all scenarios

### Milestone 4: Command Integration (End of Week 5)
- [ ] `doctor` enhanced with scenario IDs
- [ ] `status` command implemented
- [ ] No regressions in existing functionality

### Milestone 5: Polish & Ship (End of Week 6)
- [ ] All tests passing (>90% coverage)
- [ ] Documentation complete
- [ ] External review approved
- [ ] Production deployment

---

## Review Checklist

### Code Review

- [ ] All Go code follows project style guide
- [ ] No exported APIs without godoc comments
- [ ] Error handling uses `fmt.Errorf` with `%w`
- [ ] Logging uses structured logger (zap)
- [ ] No panics in production code paths

### Security Review

- [ ] No command injection vulnerabilities
- [ ] Remote URLs validated before use
- [ ] Timeout on all network operations
- [ ] No secrets logged or exposed

### Performance Review

- [ ] Benchmark tests show <2s detection time
- [ ] No O(n²) algorithms in hot paths
- [ ] Large binary scan is cancellable
- [ ] Memory usage reasonable (<100MB)

### External Review

- [ ] 2+ engineers approve architecture
- [ ] User documentation reviewed by tech writer
- [ ] Integration tests validated by QA
- [ ] Product manager approves UX

---

## Appendix: File Manifest

### New Files (13 total)

**Production Code:**
1. `internal/scenarios/types.go` (~200 LOC)
2. `internal/scenarios/tables.go` (~150 LOC)
3. `internal/scenarios/classifier.go` (~400 LOC)
4. `internal/scenarios/suggester.go` (~300 LOC)
5. `cmd/githelper/status.go` (~100 LOC)

**Test Code:**
6. `internal/scenarios/types_test.go` (~50 LOC)
7. `internal/scenarios/tables_test.go` (~200 LOC)
8. `internal/scenarios/classifier_test.go` (~300 LOC)
9. `internal/scenarios/suggester_test.go` (~200 LOC)
10. `test/integration/scenarios_test.go` (~600 LOC)

**Documentation:**
11. `docs/SCENARIOS.md` (user guide)
12. `docs/ARCHITECTURE.md` (dev guide)
13. `internal/scenarios/README.md` (package docs)

### Modified Files (2 total)

1. `cmd/githelper/doctor.go` (+150 LOC)
2. `internal/git/cli.go` (+250 LOC)

### Total LOC Estimate

- Production: ~1,500 LOC
- Tests: ~1,350 LOC
- Documentation: ~500 lines (markdown)
- **Total: ~3,350 lines**

---

**END OF IMPLEMENTATION PLAN**

*This plan is ready for agent implementation and external review.*
*Version 1.0 - 2025-11-18*
