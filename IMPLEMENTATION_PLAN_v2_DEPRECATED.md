# GitHelper Scenario Classification System - Implementation Plan

**Version:** 2.0
**Date:** 2025-11-18
**Target Completion:** 5-7 weeks (adjusted for complexity)
**Estimated LOC:** ~3,000 lines (production + tests)

**Changelog from v1.0:**
- Added mandatory `git fetch` before sync state detection (Gemini critique)
- Made remote names configurable instead of hardcoded (both critiques)
- Simplified large binary detection to existence-only (Gemini critique)
- Refactored AutoFix to use structured operations (Gemini critique)
- Improved `GetDefaultBranch` fallback logic (Gemini critique)
- Completed all fix suggestion implementations (GPT critique)
- Added detailed integration test strategy (GPT critique)
- Added micro-benchmarking early in implementation (GPT critique)
- Extended timeline to account for additional complexity
- Added git submodules to known complexities requiring future work (user request)

---

## Executive Summary

### Objective

Implement a comprehensive state classification system for GitHelper that detects and identifies repository states across three locations (Local, Core, GitHub), enabling automated diagnosis, repair suggestions, and intelligent command orchestration.

### Scope

**In Scope:**
- Scenario classification across 5 dimensions (41 distinct scenarios)
- Automated fix suggestion with priority ordering
- Auto-fix capability for 60% of scenarios (with safety validation)
- Integration with existing `doctor` command
- New `status`, `repair`, and `scenarios` commands
- Comprehensive test coverage (unit + integration)
- **Configurable remote names** (not hardcoded to origin/github)
- **Fresh remote data** via mandatory fetch before sync checks

**Out of Scope (Future Phases):**
- BFG cleanup automation (manual guide only)
- Multi-repository batch operations
- **Git submodules state tracking** (adds significant complexity - see Known Limitations)
- Real-time sync monitoring
- Rebase-based workflow optimization (current design favors merge workflows)

### Known Limitations & Future Complexity

**Git Submodules:**
Repositories with git submodules present unique challenges that are **explicitly out of scope for v1**:
- Submodules have independent state (existence, sync, working tree, etc.)
- Submodule commits are referenced by parent repo but stored separately
- Sync states become N-dimensional (parent + each submodule)
- Auto-fix operations could corrupt submodule relationships
- Scenario count explodes: 41 base scenarios × N submodules

**Recommendation:** Address submodules in v2 after validating core classifier with simple repositories. Will require dedicated submodule dimension and recursive state detection.

**Other Edge Cases Deferred:**
- Detached HEAD states (detected but limited guidance)
- Shallow clones (warnings only, history-based checks disabled)
- Git worktrees (each worktree analyzed independently)
- Repositories with non-standard remote names that can't be auto-discovered

### Success Criteria

1. ✅ `doctor` command reports scenario IDs (E1-E8, S1-S13, B1-B7, W1-W5, C1-C8)
2. ✅ Auto-fix resolves 60%+ of detected issues **with pre-validation**
3. ✅ All 41 scenarios have integration tests with >90% coverage
4. ✅ Zero regressions in existing commands
5. ✅ External review approval from 2+ engineers
6. ✅ **NEW:** False positive rate <5% on corpus of 100+ real repos

### Key Metrics

| Metric | Current | Target | Validation Method |
|--------|---------|--------|-------------------|
| Scenario coverage | 0/41 (0%) | 41/41 (100%) | Integration test suite |
| Auto-fixable issues | N/A | 25/41 (61%) | Unit tests |
| Test coverage | ~40% | >90% | go test -cover |
| Mean time to diagnosis | Manual | <2 seconds | Benchmark tests |
| False positive rate | N/A | <5% | Real-world corpus testing |
| Stale data issues | N/A | 0% | Mandatory fetch before sync checks |

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
        │  │  operations.go (NEW - v2)       │              │
        │  └────────────┴────────────────────┘              │
        └───────────────────┬────────────────────────────────┘
                            │
        ┌───────────────────▼────────────────────────────────┐
        │         internal/git (ENHANCED)                    │
        │  ┌────────────────────────────────────────┐        │
        │  │  cli.go + 9 new methods                │        │
        │  │  - GetBranchHash()                     │        │
        │  │  - CountCommitsBetween()               │        │
        │  │  - FetchRemote() [NEW - v2]            │        │
        │  │  - ... (see Phase 1)                   │        │
        │  └────────────────────────────────────────┘        │
        └────────────────────────────────────────────────────┘
```

### Data Flow (Updated for v2)

```
User runs: githelper doctor myproject
    │
    ├─> Initialize Classifier(git.Client, github.Client, coreRemoteName, githubRemoteName)
    │
    ├─> **NEW: Pre-flight fetch** (configurable via --no-fetch flag)
    │    └─> git fetch <coreRemote>
    │    └─> git fetch <githubRemote>
    │
    ├─> classifier.Detect()
    │    ├─> detectExistence() → ExistenceState (E1-E8)
    │    ├─> detectWorkingTree() → WorkingTreeState (W1-W5)
    │    ├─> detectCorruption() → CorruptionState (C1-C8) [simplified in v2]
    │    ├─> detectDefaultBranchSync() → BranchSyncState (S1-S13)
    │    └─> detectBranchTopology() → []BranchState (B1-B7 each)
    │
    ├─> SuggestFixes(state) → []Fix (prioritized, structured operations)
    │
    ├─> Display findings with scenario IDs
    │
    └─> If --auto-fix:
        ├─> Validate each fix operation
        ├─> Execute structured operations (not command strings)
        └─> Re-validate state after each fix
```

---

## Implementation Phases

### Phase 0: Benchmarking Infrastructure (Week 1) ~100 LOC

**Goal:** Establish performance baselines before implementation

**NEW in v2:** Add benchmarking early to validate <2s performance goal.

**File:** `internal/scenarios/benchmark_test.go`

```go
package scenarios_test

import (
    "testing"
    "time"
)

// Benchmark typical repository detection
func BenchmarkDetect_TypicalRepo(b *testing.B) {
    // Setup test repo: 10 branches, 1000 commits, 50MB history
    testRepo := setupBenchmarkRepo(b,
        withBranches(10),
        withCommits(1000),
        withHistorySize(50*1024*1024),
    )
    defer testRepo.Cleanup()

    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, ghClient, "origin", "github")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        state, err := classifier.Detect()
        if err != nil {
            b.Fatal(err)
        }
        _ = state
    }
}

// Benchmark must complete in <2 seconds
func TestDetect_Performance(t *testing.T) {
    testRepo := setupTypicalRepo(t)
    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, ghClient, "origin", "github")

    start := time.Now()
    _, err := classifier.Detect()
    duration := time.Since(start)

    require.NoError(t, err)
    if duration > 2*time.Second {
        t.Errorf("Detection too slow: %v (target: <2s)", duration)
    }
}
```

**Acceptance Criteria:**
- [ ] Benchmark framework in place before Phase 1
- [ ] Baseline measurements for empty, small, and large repos
- [ ] CI integration to catch regressions

---

### Phase 1: Foundation (Week 1-2) ~900 LOC

**Goal:** Establish core data structures and git client extensions

#### 1.1: Data Structures (`internal/scenarios/types.go`)

**Changes from v1:**
- Add `RemoteConfig` to support configurable remote names
- Add `FetchPolicy` option to control pre-flight fetching

**Priority:** P0 (Critical path)
**LOC:** ~250 lines (+50 from v1)
**Dependencies:** None

**File:** `internal/scenarios/types.go`

```go
package scenarios

import "time"

// **NEW in v2:** Remote configuration
type RemoteConfig struct {
    CoreRemoteName   string  // Default: "origin"
    GitHubRemoteName string  // Default: "github"
}

// **NEW in v2:** Detection options
type DetectOptions struct {
    FetchBeforeCheck bool   // Default: true (fetch remotes before sync checks)
    SkipCorruption   bool   // Default: false (skip expensive corruption scan)
    RemoteConfig     RemoteConfig
}

// RepositoryState represents complete multi-dimensional state
type RepositoryState struct {
    Repo          string
    DetectedAt    time.Time
    Options       DetectOptions  // **NEW in v2**

    // Dimensions (in dependency order)
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
    CoreRemote   string  // **NEW in v2:** Actual remote name used
    GitHubExists bool
    GitHubURL    string
    GitHubRemote string  // **NEW in v2:** Actual remote name used
}

// WorkingTreeState (D4: W1-W5)
type WorkingTreeState struct {
    ID                string
    Clean             bool
    StagedFiles       []string
    UnstagedFiles     []string
    WouldConflict     bool
    IsDetachedHEAD    bool     // **NEW in v2:** Flag detached HEAD
    IsShallowClone    bool     // **NEW in v2:** Flag shallow clones
}

// CorruptionState (D5: C1-C8)
// **SIMPLIFIED in v2:** Only detect existence, not commit metadata
type CorruptionState struct {
    ID               string
    HasCorruption    bool
    LocalBinaries    []LargeBinary
    // Removed CoreBinaries and GitHubBinaries (not scanned in v1)
}

type LargeBinary struct {
    Path       string
    SizeMB     float64
    SHA1       string     // **SIMPLIFIED in v2:** Just blob hash, no commit metadata
    // Removed: CommitHash, CommitDate (too expensive to find)
}

// BranchSyncState (D2: S1-S13)
type BranchSyncState struct {
    ID              string
    Branch          string
    LocalHash       string
    CoreHash        string
    GitHubHash      string

    // **NEW in v2:** Track if data is fresh
    LastFetchTime   time.Time
    DataIsFresh     bool

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
    SyncState    *BranchSyncState  // Only if exists on multiple locations
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
- Test RemoteConfig defaults

**Acceptance Criteria:**
- [ ] All type definitions compile
- [ ] Package documentation complete
- [ ] Zero values documented
- [ ] RemoteConfig properly integrated

---

#### 1.2: Git Client Extensions (`internal/git/cli.go`)

**Changes from v1:**
- Add `FetchRemote()` method for mandatory fetch
- Improve `GetDefaultBranch()` fallback using `git remote show`
- Add methods to detect detached HEAD and shallow clones

**Priority:** P0 (Blocks classifier)
**LOC:** ~300 lines (+50 from v1)
**Dependencies:** Existing git.Client

**File:** `internal/git/cli.go` (append to existing)

**New Methods Required:**

```go
// **NEW in v2:** FetchRemote with timeout
func (c *Client) FetchRemote(remote string) error {
    // git fetch <remote> --tags
    // Timeout: 30 seconds
    // Return error if remote unreachable
}

// LocalExists checks if repository exists locally
func (c *Client) LocalExists() (bool, string) {
    // Implementation:
    // 1. Check if c.repoPath exists
    // 2. Check if .git directory/file exists
    // 3. Return (exists, absolutePath)
}

// **NEW in v2:** IsDetachedHEAD checks for detached HEAD state
func (c *Client) IsDetachedHEAD() (bool, error) {
    // git symbolic-ref -q HEAD
    // Exit code 1 = detached, 0 = on branch
}

// **NEW in v2:** IsShallowClone checks if repo is shallow
func (c *Client) IsShallowClone() (bool, error) {
    // Check for .git/shallow file
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

// **IMPROVED in v2:** GetDefaultBranch with better fallback
func (c *Client) GetDefaultBranch() (string, error) {
    // Try 1: git symbolic-ref refs/remotes/origin/HEAD
    // Try 2: git remote show <remote> | grep "HEAD branch"
    // Try 3: Check for main
    // Try 4: Check for master
    // Return error if all fail
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
- **NEW:** Timeout on remote operations: 5s for checks, 30s for fetch
- Cache results where appropriate (e.g., branch lists)
- **NEW:** Add retry logic for transient network failures (1 retry with backoff)

**Testing Requirements:**
- Unit tests with mock git output
- Integration tests with real test repos
- Test edge cases: missing remotes, detached HEAD, shallow clones, network failures
- **NEW:** Test fetch with network delays and failures

**Acceptance Criteria:**
- [ ] All 11 methods implemented (+2 from v1)
- [ ] 100% unit test coverage
- [ ] Integration tests pass
- [ ] Handles all edge cases (documented in tests)
- [ ] Fetch timeout tested with slow remotes

---

#### 1.3: Classification Tables (`internal/scenarios/tables.go`)

**No changes from v1** - This component remains the same.

**Priority:** P0 (Blocks classifier)
**LOC:** ~150 lines
**Dependencies:** types.go

*(Same implementation as v1)*

---

#### 1.4: **NEW in v2:** Structured Fix Operations (`internal/scenarios/operations.go`)

**Priority:** P0 (Required for safe AutoFix)
**LOC:** ~200 lines
**Dependencies:** types.go, git/cli.go

**Rationale:** Gemini critique identified that command-string based fixes are brittle and unsafe. Structured operations allow validation, rollback, and safer execution.

**File:** `internal/scenarios/operations.go`

```go
package scenarios

import (
    "context"
    "fmt"

    "github.com/lcgerke/githelper/internal/git"
)

// Operation represents a structured fix operation (not a command string)
type Operation interface {
    // Validate checks if operation can be safely executed
    Validate(ctx context.Context, state *RepositoryState) error

    // Execute performs the operation
    Execute(ctx context.Context, gitClient *git.Client) error

    // Describe returns human-readable description
    Describe() string

    // Rollback attempts to undo the operation (best-effort)
    Rollback(ctx context.Context, gitClient *git.Client) error
}

// FetchOperation - git fetch <remote>
type FetchOperation struct {
    Remote string
}

func (op *FetchOperation) Validate(ctx context.Context, state *RepositoryState) error {
    // Check remote exists and is reachable
    return nil
}

func (op *FetchOperation) Execute(ctx context.Context, gc *git.Client) error {
    return gc.FetchRemote(op.Remote)
}

func (op *FetchOperation) Describe() string {
    return fmt.Sprintf("Fetch updates from %s", op.Remote)
}

func (op *FetchOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Fetch is read-only, no rollback needed
    return nil
}

// PushOperation - git push <remote> <refspec>
type PushOperation struct {
    Remote  string
    Refspec string  // e.g., "refs/heads/main" (explicit, not "HEAD")
}

func (op *PushOperation) Validate(ctx context.Context, state *RepositoryState) error {
    // Ensure:
    // 1. Remote exists and reachable
    // 2. Refspec is valid
    // 3. Push will be fast-forward (no divergence)
    // 4. Working tree is clean
    return nil
}

func (op *PushOperation) Execute(ctx context.Context, gc *git.Client) error {
    return gc.Push(op.Remote, op.Refspec)
}

func (op *PushOperation) Describe() string {
    return fmt.Sprintf("Push %s to %s", op.Refspec, op.Remote)
}

func (op *PushOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Push cannot be safely rolled back
    return fmt.Errorf("push operations cannot be automatically rolled back")
}

// PullOperation - git pull <remote> <branch>
type PullOperation struct {
    Remote string
    Branch string
}

func (op *PullOperation) Validate(ctx context.Context, state *RepositoryState) error {
    // Ensure:
    // 1. Working tree is clean
    // 2. Pull will be fast-forward
    // 3. Remote branch exists
    return nil
}

func (op *PullOperation) Execute(ctx context.Context, gc *git.Client) error {
    return gc.Pull(op.Remote)
}

func (op *PullOperation) Describe() string {
    return fmt.Sprintf("Pull %s from %s", op.Branch, op.Remote)
}

func (op *PullOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Can attempt git reset --hard ORIG_HEAD
    return fmt.Errorf("pull rollback requires manual intervention")
}

// CompositeOperation - sequence of operations (executed in order)
type CompositeOperation struct {
    Operations []Operation
    StopOnError bool  // If true, stop at first error
}

func (op *CompositeOperation) Validate(ctx context.Context, state *RepositoryState) error {
    for _, subOp := range op.Operations {
        if err := subOp.Validate(ctx, state); err != nil {
            return fmt.Errorf("validation failed for %s: %w", subOp.Describe(), err)
        }
    }
    return nil
}

func (op *CompositeOperation) Execute(ctx context.Context, gc *git.Client) error {
    for i, subOp := range op.Operations {
        if err := subOp.Execute(ctx, gc); err != nil {
            if op.StopOnError {
                return fmt.Errorf("operation %d/%d failed (%s): %w",
                    i+1, len(op.Operations), subOp.Describe(), err)
            }
        }
    }
    return nil
}

func (op *CompositeOperation) Describe() string {
    return fmt.Sprintf("Composite operation (%d steps)", len(op.Operations))
}

func (op *CompositeOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Rollback in reverse order
    for i := len(op.Operations) - 1; i >= 0; i-- {
        _ = op.Operations[i].Rollback(ctx, gc)
    }
    return nil
}
```

**Testing Requirements:**
- Unit test for each operation type
- Validation logic tested with various states
- Rollback tested where applicable
- Composite operations tested with failures

**Acceptance Criteria:**
- [ ] All operation types implemented
- [ ] Validation prevents unsafe operations
- [ ] 100% test coverage
- [ ] Integration tested with real repos

---

### Phase 2: Classifier Core (Week 2-3) ~700 LOC

**Goal:** Implement hierarchical state detection with fresh data

**Changes from v1:**
- Add mandatory fetch before sync detection
- Use configurable remote names
- Simplify corruption detection
- Handle detached HEAD and shallow clones

#### 2.1: Classifier Implementation (`internal/scenarios/classifier.go`)

**Priority:** P0 (Core feature)
**LOC:** ~500 lines (+100 from v1)
**Dependencies:** types.go, tables.go, operations.go, git/cli.go extensions

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
    githubClient *github.Client  // **v2 NOTE:** Currently unused, consider removing
    ctx          context.Context
    options      DetectOptions
}

func NewClassifier(ctx context.Context, repoPath string, gc *git.Client, ghc *github.Client, coreRemote, githubRemote string) *Classifier {
    return &Classifier{
        ctx:          ctx,
        repoPath:     repoPath,
        gitClient:    gc,
        githubClient: ghc,
        options: DetectOptions{
            FetchBeforeCheck: true,  // Default: fetch before checks
            SkipCorruption:   false,
            RemoteConfig: RemoteConfig{
                CoreRemoteName:   coreRemote,
                GitHubRemoteName: githubRemote,
            },
        },
    }
}

// WithOptions allows customizing detection behavior
func (c *Classifier) WithOptions(opts DetectOptions) *Classifier {
    c.options = opts
    return c
}

// **NEW in v2:** Detect performs full hierarchical state detection with mandatory fetch
func (c *Classifier) Detect() (*RepositoryState, error) {
    state := &RepositoryState{
        Repo:       c.repoPath,
        DetectedAt: time.Now(),
        Options:    c.options,
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

    // **NEW in v2:** PRE-FLIGHT FETCH (critical for accurate sync detection)
    if c.options.FetchBeforeCheck {
        if err := c.fetchRemotes(state.Existence); err != nil {
            // Log warning but continue (sync state will be based on stale data)
            logger.Warnw("Failed to fetch remotes, sync state may be stale", "error", err)
        }
    }

    // D4: Working tree (independent of remotes)
    state.WorkingTree, err = c.detectWorkingTree()
    if err != nil {
        return nil, fmt.Errorf("working tree check failed: %w", err)
    }

    // D5: Corruption (expensive, can be skipped)
    if !c.options.SkipCorruption {
        state.Corruption, err = c.detectCorruption()
        if err != nil {
            // Log warning but don't fail entire detection
            logger.Warnw("Corruption check failed", "error", err)
            state.Corruption = CorruptionState{ID: "C_UNKNOWN"}
        }
    } else {
        state.Corruption = CorruptionState{ID: "C1"}  // Assumed clean when skipped
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
        logger.Warnw("Branch topology check failed", "error", err)
        state.Branches = []BranchState{}
    }

    return state, nil
}

// **NEW in v2:** fetchRemotes ensures remote data is fresh
func (c *Classifier) fetchRemotes(existence ExistenceState) error {
    if existence.CoreExists {
        if err := c.gitClient.FetchRemote(existence.CoreRemote); err != nil {
            return fmt.Errorf("failed to fetch %s: %w", existence.CoreRemote, err)
        }
    }

    if existence.GitHubExists {
        if err := c.gitClient.FetchRemote(existence.GitHubRemote); err != nil {
            return fmt.Errorf("failed to fetch %s: %w", existence.GitHubRemote, err)
        }
    }

    return nil
}

// **UPDATED in v2:** detectExistence uses configurable remote names
func (c *Classifier) detectExistence() (ExistenceState, error) {
    state := ExistenceState{}

    // Check local
    state.LocalExists, state.LocalPath = c.gitClient.LocalExists()

    // Early return if local doesn't exist
    if !state.LocalExists {
        state.ID = classifyExistence(false, false, false)
        return state, nil
    }

    // Check Core remote (using configured name)
    coreRemote := c.options.RemoteConfig.CoreRemoteName
    coreURL, err := c.gitClient.GetRemoteURL(coreRemote)
    if err == nil && coreURL != "" {
        state.CoreURL = coreURL
        state.CoreRemote = coreRemote
        state.CoreExists = c.gitClient.CanReachRemote(coreRemote)
    }

    // Check GitHub remote (using configured name)
    githubRemote := c.options.RemoteConfig.GitHubRemoteName
    githubURL, err := c.gitClient.GetRemoteURL(githubRemote)
    if err == nil && githubURL != "" {
        state.GitHubURL = githubURL
        state.GitHubRemote = githubRemote
        state.GitHubExists = c.gitClient.CanReachRemote(githubRemote)
    }

    // Classify
    state.ID = classifyExistence(state.LocalExists, state.CoreExists, state.GitHubExists)

    return state, nil
}

// **UPDATED in v2:** detectWorkingTree checks for detached HEAD and shallow clones
func (c *Classifier) detectWorkingTree() (WorkingTreeState, error) {
    state := WorkingTreeState{}

    // Check for detached HEAD
    isDetached, err := c.gitClient.IsDetachedHEAD()
    if err == nil {
        state.IsDetachedHEAD = isDetached
    }

    // Check for shallow clone
    isShallow, err := c.gitClient.IsShallowClone()
    if err == nil {
        state.IsShallowClone = isShallow
    }

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

// **SIMPLIFIED in v2:** detectCorruption only detects existence, not commit metadata
func (c *Classifier) detectCorruption() (CorruptionState, error) {
    state := CorruptionState{
        ID: "C1",  // Default: no corruption
        HasCorruption: false,
    }

    // **SIMPLIFIED:** Only find large blobs, not their commit history
    binaries, err := c.scanLargeBinariesSimplified()
    if err != nil {
        return state, err
    }

    if len(binaries) > 0 {
        state.LocalBinaries = binaries
        state.HasCorruption = true
        state.ID = classifyCorruption(true, false, false)
    }

    return state, nil
}

// **SIMPLIFIED in v2:** scanLargeBinariesSimplified finds files >10MB without commit metadata
func (c *Classifier) scanLargeBinariesSimplified() ([]LargeBinary, error) {
    // Implementation:
    // 1. Run: git rev-list --objects --all | git cat-file --batch-check
    // 2. Parse for blobs >= 10MB
    // 3. Return LargeBinary with Path, SizeMB, SHA1 ONLY
    // 4. DO NOT attempt to find commit hash/date (too expensive)

    // Threshold: 10MB
    const threshold = 10 * 1024 * 1024

    // TODO: Implement using gitClient
    // Estimated time: <5 seconds for typical repo (vs minutes for full metadata search)
    return []LargeBinary{}, nil
}

// **UPDATED in v2:** detectDefaultBranchSync with fresh data indicator
func (c *Classifier) detectDefaultBranchSync() (BranchSyncState, error) {
    state := BranchSyncState{
        LastFetchTime: time.Now(),  // **NEW**
        DataIsFresh:   c.options.FetchBeforeCheck,  // **NEW**
    }

    // Get default branch name (improved fallback in v2)
    defaultBranch, err := c.gitClient.GetDefaultBranch()
    if err != nil {
        return state, err
    }
    state.Branch = defaultBranch

    // Get commit hashes
    state.LocalHash, _ = c.gitClient.GetBranchHash(defaultBranch)

    coreRemote := c.options.RemoteConfig.CoreRemoteName
    state.CoreHash, _ = c.gitClient.GetRemoteBranchHash(coreRemote, defaultBranch)

    githubRemote := c.options.RemoteConfig.GitHubRemoteName
    state.GitHubHash, _ = c.gitClient.GetRemoteBranchHash(githubRemote, defaultBranch)

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

// **UPDATED in v2:** detectBranchTopology uses configurable remote names
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

    coreRemote := c.options.RemoteConfig.CoreRemoteName
    githubRemote := c.options.RemoteConfig.GitHubRemoteName

    // Add remote branches (using configured names)
    for _, remoteBranch := range remoteBranches {
        parts := strings.SplitN(remoteBranch, "/", 2)
        if len(parts) != 2 {
            continue
        }
        remote, branch := parts[0], parts[1]

        if _, exists := branchMap[branch]; !exists {
            branchMap[branch] = &BranchState{Name: branch}
        }

        if remote == coreRemote {
            branchMap[branch].CoreExists = true
        } else if remote == githubRemote {
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
- **NEW:** Fetch timeout configurable (default 30s per remote)
- **NEW:** Add metrics for fetch success/failure

**Testing Requirements:**
- Unit tests with mocked git client
- Integration tests for all 41 scenarios
- Performance test: <2s for typical repo
- Error handling: network failures, missing remotes
- **NEW:** Test with stale data (fetch disabled) vs fresh data
- **NEW:** Test with slow/failing fetch operations

**Acceptance Criteria:**
- [ ] Detects all 41 scenarios correctly
- [ ] Handles partial failures (e.g., one remote unreachable)
- [ ] Performance: <2s for 10-branch repo (with fetch)
- [ ] Zero panics on malformed input
- [ ] **NEW:** Fetch failures don't abort entire detection
- [ ] **NEW:** Stale data warning logged when fetch fails

---

### Phase 3: Fix Suggestion Engine (Week 3-4) ~600 LOC

**Goal:** Suggest and auto-apply fixes using structured operations

**Changes from v1:**
- Use Operation objects instead of command strings
- Complete all fix implementations (no "not implemented" stubs)
- Add validation before auto-fix execution

#### 3.1: Fix Suggester (`internal/scenarios/suggester.go`)

**Priority:** P1 (High value)
**LOC:** ~400 lines (+100 from v1)
**Dependencies:** types.go, operations.go, classifier.go

**File:** `internal/scenarios/suggester.go`

```go
package scenarios

import (
    "fmt"
    "sort"

    "github.com/lcgerke/githelper/internal/git"
)

// **UPDATED in v2:** Fix now contains Operation instead of Command string
type Fix struct {
    ScenarioID  string
    Description string
    Operation   Operation  // **NEW in v2:** Structured operation
    Command     string     // **DEPRECATED:** Keep for display purposes only
    AutoFixable bool
    Priority    int
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
            Operation:   nil,  // Requires user decision
            AutoFixable: false,
            Priority:    2,
            Reason:      "Automated sync requires clean working tree",
        })
    }

    // **NEW in v2:** Warn about detached HEAD
    if state.WorkingTree.IsDetachedHEAD {
        fixes = append(fixes, Fix{
            ScenarioID:  "W_DETACHED",
            Description: "Repository is in detached HEAD state",
            Command:     "git checkout <branch-name>",
            Operation:   nil,
            AutoFixable: false,
            Priority:    3,
            Reason:      "Detached HEAD prevents normal branch operations",
        })
    }

    // **NEW in v2:** Warn about shallow clone
    if state.WorkingTree.IsShallowClone {
        fixes = append(fixes, Fix{
            ScenarioID:  "W_SHALLOW",
            Description: "Repository is a shallow clone",
            Command:     "git fetch --unshallow",
            Operation:   nil,  // May be very slow, don't auto-fix
            AutoFixable: false,
            Priority:    8,
            Reason:      "Shallow clones have limited history",
        })
    }

    // D5: Corruption
    if state.Corruption.HasCorruption {
        fixes = append(fixes, Fix{
            ScenarioID:  state.Corruption.ID,
            Description: fmt.Sprintf("Large binaries in history (%d files, %.1f MB total)",
                len(state.Corruption.LocalBinaries), totalSizeMB(state.Corruption.LocalBinaries)),
            Command:     "See docs/BFG_CLEANUP.md for removal procedure",
            Operation:   nil,
            AutoFixable: false,
            Priority:    3,
            Reason:      "Large files should be removed before syncing",
        })
    }

    // **NEW in v2:** Warn about stale data
    if !state.DefaultBranch.DataIsFresh {
        fixes = append(fixes, Fix{
            ScenarioID:  "S_STALE",
            Description: "Sync state based on stale data (fetch failed)",
            Command:     "git fetch --all",
            Operation:   &CompositeOperation{
                Operations: []Operation{
                    &FetchOperation{Remote: state.Existence.CoreRemote},
                    &FetchOperation{Remote: state.Existence.GitHubRemote},
                },
            },
            AutoFixable: true,
            Priority:    1,  // High priority - affects all sync analysis
            Reason:      "State detection may be inaccurate without fresh remote data",
        })
    }

    // D2: Default branch sync
    fixes = append(fixes, suggestSyncFixes(state)...)

    // D3: Branch topology
    for _, branch := range state.Branches {
        fixes = append(fixes, suggestBranchFixes(branch, state)...)
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
            Operation:   nil,  // Requires command-level GitHub client
            AutoFixable: false,  // **CHANGED in v2:** Not auto-fixable (requires GitHub API)
            Priority:    1,
            Reason:      "Dual-push requires both Core and GitHub",
        }}

    case "E3":  // GitHub exists, Core doesn't
        return []Fix{{
            ScenarioID:  "E3",
            Description: "Core remote not configured",
            Command:     "git remote add origin <core-url>",
            Operation:   nil,  // Requires URL from config
            AutoFixable: false,
            Priority:    1,
            Reason:      "Primary remote missing",
        }}

    case "E4":  // Local only
        return []Fix{{
            ScenarioID:  "E4",
            Description: "No remotes configured",
            Command:     "githelper repo create <name>",
            Operation:   nil,
            AutoFixable: false,
            Priority:    1,
            Reason:      "Repository not integrated with system",
        }}

    default:
        return nil
    }
}

// **UPDATED in v2:** suggestSyncFixes uses Operation objects
func suggestSyncFixes(state *RepositoryState) []Fix {
    syncState := state.DefaultBranch

    switch syncState.ID {
    case "S1":
        return nil  // Perfect sync

    case "S2":  // Local ahead of both
        return []Fix{{
            ScenarioID:  "S2",
            Description: fmt.Sprintf("Local ahead of both remotes (%d commits)",
                syncState.LocalAheadCore),
            Command:     "git push",
            Operation: &PushOperation{
                Remote:  state.Existence.CoreRemote,
                Refspec: fmt.Sprintf("refs/heads/%s", syncState.Branch),
            },
            AutoFixable: true,
            Priority:    4,
            Reason:      "Dual-push will sync both remotes",
        }}

    case "S3":  // Core pushed to GitHub without local
        return []Fix{{
            ScenarioID:  "S3",
            Description: "Remote sync happened without local update",
            Command:     fmt.Sprintf("git fetch %s", state.Existence.CoreRemote),
            Operation: &FetchOperation{
                Remote: state.Existence.CoreRemote,
            },
            AutoFixable: true,
            Priority:    4,
            Reason:      "Local needs update from core",
        }}

    case "S4":  // Local ahead of GitHub only
        return []Fix{{
            ScenarioID:  "S4",
            Description: fmt.Sprintf("Local ahead of GitHub (%d commits)",
                syncState.LocalAheadGitHub),
            Command:     "githelper github sync",
            Operation: &PushOperation{
                Remote:  state.Existence.GitHubRemote,
                Refspec: fmt.Sprintf("refs/heads/%s", syncState.Branch),
            },
            AutoFixable: true,
            Priority:    4,
            Reason:      "GitHub behind, needs selective sync",
        }}

    case "S5":  // GitHub has commits
        return []Fix{{
            ScenarioID:  "S5",
            Description: fmt.Sprintf("GitHub has commits not in local/core (%d commits)",
                syncState.CoreBehindGitHub),
            Command:     fmt.Sprintf("git fetch %s && git merge %s/%s",
                state.Existence.GitHubRemote, state.Existence.GitHubRemote, syncState.Branch),
            Operation:   nil,  // Might have conflicts
            AutoFixable: false,
            Priority:    5,
            Reason:      "Manual merge required",
        }}

    case "S6":  // Both remotes ahead
        return []Fix{{
            ScenarioID:  "S6",
            Description: fmt.Sprintf("Both remotes ahead (%d commits)",
                syncState.LocalBehindCore),
            Command:     "git pull",
            Operation: &PullOperation{
                Remote: state.Existence.CoreRemote,
                Branch: syncState.Branch,
            },
            AutoFixable: true,
            Priority:    4,
            Reason:      "Fast-forward pull will sync",
        }}

    case "S7":  // Core ahead only
        return []Fix{{
            ScenarioID:  "S7",
            Description: fmt.Sprintf("Core ahead of local (%d commits)",
                syncState.LocalBehindCore),
            Command:     fmt.Sprintf("git pull %s %s", state.Existence.CoreRemote, syncState.Branch),
            Operation: &PullOperation{
                Remote: state.Existence.CoreRemote,
                Branch: syncState.Branch,
            },
            AutoFixable: true,
            Priority:    4,
            Reason:      "Fast-forward from core",
        }}

    case "S8":  // Complex ahead/behind
        return []Fix{{
            ScenarioID:  "S8",
            Description: "Complex sync state: local ahead of GitHub, behind Core",
            Command:     fmt.Sprintf("git pull %s && git push %s",
                state.Existence.CoreRemote, state.Existence.GitHubRemote),
            Operation: &CompositeOperation{
                Operations: []Operation{
                    &PullOperation{Remote: state.Existence.CoreRemote, Branch: syncState.Branch},
                    &PushOperation{Remote: state.Existence.GitHubRemote, Refspec: fmt.Sprintf("refs/heads/%s", syncState.Branch)},
                },
                StopOnError: true,
            },
            AutoFixable: false,  // **CHANGED in v2:** Requires careful ordering, keep manual
            Priority:    5,
            Reason:      "Manual resolution recommended",
        }}

    case "S9", "S10", "S11", "S12", "S13":  // Divergence
        return []Fix{{
            ScenarioID:  syncState.ID,
            Description: "Divergent histories detected",
            Command:     "Manual merge or rebase required - see docs/DIVERGENCE.md",  // **UPDATED in v2:** Neutral wording
            Operation:   nil,
            AutoFixable: false,
            Priority:    5,
            Reason:      "Divergence requires conflict resolution",
        }}

    default:
        return nil
    }
}

// **UPDATED in v2:** suggestBranchFixes with structured operations
func suggestBranchFixes(branch BranchState, state *RepositoryState) []Fix {
    switch branch.ID {
    case "B1":
        return nil  // Fully tracked

    case "B2", "B6":  // On Core but not GitHub
        return []Fix{{
            ScenarioID:  branch.ID,
            Description: fmt.Sprintf("Branch '%s' not on GitHub", branch.Name),
            Command:     fmt.Sprintf("git push %s %s", state.Existence.GitHubRemote, branch.Name),
            Operation: &PushOperation{
                Remote:  state.Existence.GitHubRemote,
                Refspec: fmt.Sprintf("refs/heads/%s", branch.Name),
            },
            AutoFixable: true,
            Priority:    6,
            Reason:      "Complete dual-remote coverage",
        }}

    case "B3", "B7":  // On GitHub but not Core
        return []Fix{{
            ScenarioID:  branch.ID,
            Description: fmt.Sprintf("Branch '%s' not on Core", branch.Name),
            Command:     fmt.Sprintf("git push %s %s", state.Existence.CoreRemote, branch.Name),
            Operation: &PushOperation{
                Remote:  state.Existence.CoreRemote,
                Refspec: fmt.Sprintf("refs/heads/%s", branch.Name),
            },
            AutoFixable: true,
            Priority:    6,
            Reason:      "Core should have all branches",
        }}

    case "B4":  // Local only
        return []Fix{{
            ScenarioID:  "B4",
            Description: fmt.Sprintf("Branch '%s' not pushed", branch.Name),
            Command:     fmt.Sprintf("git push %s %s", state.Existence.CoreRemote, branch.Name),
            Operation: &PushOperation{
                Remote:  state.Existence.CoreRemote,
                Refspec: fmt.Sprintf("refs/heads/%s", branch.Name),
            },
            AutoFixable: false,  // User decides when to push
            Priority:    7,
            Reason:      "Local work in progress",
        }}

    case "B5":  // Remote not fetched
        return []Fix{{
            ScenarioID:  "B5",
            Description: fmt.Sprintf("Remote branch '%s' not checked out", branch.Name),
            Command:     fmt.Sprintf("git checkout -t %s/%s", state.Existence.CoreRemote, branch.Name),
            Operation:   nil,  // Checkout is a different operation type
            AutoFixable: false,  // **CHANGED in v2:** Don't auto-checkout (affects working tree)
            Priority:    8,
            Reason:      "Optional: fetch remote branch",
        }}

    default:
        return nil
    }
}

// **UPDATED in v2:** AutoFix uses Operation validation and execution
func AutoFix(ctx context.Context, state *RepositoryState, gitClient *git.Client) ([]Fix, []error) {
    fixes := SuggestFixes(state)
    applied := []Fix{}
    errors := []error{}

    for _, fix := range fixes {
        if !fix.AutoFixable || fix.Operation == nil {
            continue
        }

        // **NEW in v2:** Validate before executing
        if err := fix.Operation.Validate(ctx, state); err != nil {
            errors = append(errors, fmt.Errorf("%s validation failed: %w", fix.ScenarioID, err))
            continue
        }

        // Execute operation
        if err := fix.Operation.Execute(ctx, gitClient); err != nil {
            errors = append(errors, fmt.Errorf("%s execution failed: %w", fix.ScenarioID, err))

            // **NEW in v2:** Attempt rollback on failure
            if rollbackErr := fix.Operation.Rollback(ctx, gitClient); rollbackErr != nil {
                logger.Errorw("Rollback failed", "scenario", fix.ScenarioID, "error", rollbackErr)
            }
        } else {
            applied = append(applied, fix)
        }
    }

    return applied, errors
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
- Test Operation validation logic
- Test auto-fix with validation failures
- Ensure non-fixable scenarios handled gracefully
- **NEW:** Test rollback on operation failures

**Acceptance Criteria:**
- [ ] All 41 scenarios have fix suggestions
- [ ] 25/41 scenarios are auto-fixable
- [ ] Priority ordering is correct
- [ ] No false positive fixes
- [ ] **NEW:** All auto-fixable fixes have Operation objects
- [ ] **NEW:** Validation prevents unsafe operations
- [ ] **NEW:** No "not implemented" stubs

---

### Phase 4: Command Integration (Week 4-5) ~400 LOC

*(Same as v1, but uses updated classifier with configurable remotes)*

**Changes from v1:**
- Pass remote names to classifier constructor
- Display fetch status in output
- Show warnings for detached HEAD / shallow clones

---

### Phase 5: Testing & Documentation (Week 5-7) ~800 LOC

**Goal:** Comprehensive test coverage and documentation

**Changes from v1:**
- Add detailed integration test harness specification
- Document how to create test remotes without GitHub
- Add real-world corpus testing (100+ repos)

#### 5.1: Unit Tests

*(Same structure as v1, add tests for new components)*

---

#### 5.2: Integration Tests - **ENHANCED in v2**

**File:** `test/integration/scenarios_test.go` (NEW)

**NEW in v2:** Detailed test harness specification addressing GPT critique

**Approach:** Create real test repositories in known states using local git-daemon

```go
// Test harness infrastructure

type TestRepo struct {
    Path         string
    CoreRemote   *git.Daemon  // Local git-daemon simulating Core
    GitHubRemote *git.Daemon  // Local git-daemon simulating GitHub
    Cleanup      func()
}

// setupTestRepo creates isolated test environment with local git daemons
func setupTestRepo(t *testing.T, options ...TestRepoOption) *TestRepo {
    // 1. Create temp directory
    // 2. Initialize bare repos for "Core" and "GitHub"
    // 3. Start local git-daemon instances (no network needed)
    // 4. Clone to working directory
    // 5. Apply options to create desired state
    // 6. Return TestRepo with cleanup function
}

// Example options:
func withBranches(count int) TestRepoOption { /* ... */ }
func withCommit(message string) TestRepoOption { /* ... */ }
func withDivergence() TestRepoOption { /* ... */ }
func withLargeBinary(sizeMB int) TestRepoOption { /* ... */ }

// Example test:
func TestScenarioDetection_E1_DualRemote(t *testing.T) {
    testRepo := setupTestRepo(t,
        withLocalRepo(),
        withCoreRemote(),
        withGitHubRemote(),
    )
    defer testRepo.Cleanup()

    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, nil, "core", "github")
    state, err := classifier.Detect()
    require.NoError(t, err)

    assert.Equal(t, "E1", state.Existence.ID)
}

// All 41 scenarios tested similarly
```

**Key Innovation:** Use local git-daemon instead of requiring GitHub access.

**Acceptance Criteria:**
- [ ] All 41 scenarios have integration tests
- [ ] Tests use real git operations with local daemons
- [ ] Tests clean up temp repos
- [ ] Tests run in <60 seconds (increased from 30s for fetch operations)
- [ ] **NEW:** Test harness documented in `test/integration/README.md`
- [ ] **NEW:** Can run offline (no GitHub required)

---

#### 5.3: Documentation

**Same as v1, plus:**

5. **`docs/REMOTE_CONFIGURATION.md`** (NEW in v2) - How to configure non-standard remote names
6. **`test/integration/README.md`** (NEW in v2) - Integration test harness guide

---

## Risk Assessment & Mitigation - **UPDATED for v2**

### Technical Risks

| Risk | Impact | Probability | Mitigation (v2) |
|------|--------|-------------|-----------------|
| **Performance regression** | High | Medium | Early benchmarking (Phase 0), mandatory <2s test |
| **False positives in detection** | High | Medium | Corpus testing (100+ real repos), stale data warnings |
| **Network timeout during remote checks** | Medium | High | 30s fetch timeout, graceful degradation, retry logic |
| **Fetch failures prevent detection** | Medium | High | **NEW:** Fetch is optional, stale data warning shown |
| **Breaking changes to git client** | Medium | Low | Unit tests for all git operations |
| **Auto-fix causes data loss** | Critical | Low | **NEW:** Operation validation, rollback, never auto-fix divergence |
| **Hardcoded remote names break** | High | Medium | **NEW:** FIXED - Configurable remote names |
| **Stale sync state data** | High | High | **NEW:** FIXED - Mandatory fetch before sync checks |
| **Submodule complexity** | High | Medium | **NEW:** Explicitly out of scope, document limitations |

### Additional Mitigations in v2

1. **Stale Data:** Mandatory fetch with `--no-fetch` escape hatch for offline use
2. **Remote Names:** Configuration-based remote discovery
3. **Auto-Fix Safety:** Structured operations with validation and rollback
4. **Performance:** Early benchmarking to catch regressions
5. **Real-World Validation:** Corpus testing on 100+ diverse repos

---

## Acceptance Criteria - **ENHANCED for v2**

### Functional Requirements

- [ ] **Scenario Detection:** All 41 scenarios correctly identified
- [ ] **Fix Suggestions:** All scenarios have appropriate fix suggestions
- [ ] **Auto-Fix:** 25/41 scenarios can be auto-fixed safely
- [ ] **Performance:** State detection completes in <2 seconds (with fetch)
- [ ] **Accuracy:** False positive rate <5% on 100-repo corpus
- [ ] **NEW:** Fresh data guarantee via mandatory fetch
- [ ] **NEW:** Configurable remote names tested with non-standard setups
- [ ] **NEW:** All auto-fix operations use structured Operations, not strings
- [ ] **NEW:** Detached HEAD and shallow clone warnings present

### Non-Functional Requirements

- [ ] **Test Coverage:** >90% line coverage, 100% scenario coverage
- [ ] **Documentation:** All public APIs documented, user guide complete
- [ ] **Error Handling:** Graceful degradation on network failures
- [ ] **Logging:** Structured logging for all operations
- [ ] **Backward Compatibility:** Zero regressions in existing commands
- [ ] **NEW:** Fetch failures logged but don't abort detection
- [ ] **NEW:** Operation validation prevents unsafe auto-fixes

---

## Delivery Milestones - **ADJUSTED for v2**

### Milestone 0: Benchmarking (End of Week 1)
- [ ] Benchmark framework in place
- [ ] Baseline measurements for empty/small/large repos
- [ ] CI integration for performance regression detection

### Milestone 1: Foundation (End of Week 2)
- [ ] All data structures defined (including RemoteConfig)
- [ ] Git client extensions complete (including FetchRemote)
- [ ] Classification tables implemented
- [ ] Structured operations framework complete
- [ ] Unit tests passing

### Milestone 2: Core Classifier (End of Week 3)
- [ ] Classifier detects all 5 dimensions
- [ ] Mandatory fetch before sync checks working
- [ ] All 41 scenarios testable
- [ ] Integration tests for existence + sync states
- [ ] Detached HEAD and shallow clone detection working

### Milestone 3: Fix Engine (End of Week 4)
- [ ] Fix suggester complete with Operations
- [ ] All 41 fixes implemented (no stubs)
- [ ] Auto-fix with validation and rollback
- [ ] Integration tests for all scenarios

### Milestone 4: Command Integration (End of Week 5)
- [ ] `doctor` enhanced with scenario IDs
- [ ] `status` command implemented
- [ ] Remote name configuration working
- [ ] No regressions in existing functionality

### Milestone 5: Testing (End of Week 6)
- [ ] All unit tests passing (>90% coverage)
- [ ] All 41 integration tests passing
- [ ] Test harness using local git-daemon working
- [ ] Benchmark tests validate <2s performance

### Milestone 6: Validation & Ship (End of Week 7)
- [ ] Corpus testing on 100+ real repos
- [ ] Documentation complete
- [ ] External review approved
- [ ] Production deployment

---

## Appendix: Changes from v1.0

### Major Improvements

1. **Mandatory Fresh Data** (Gemini critique)
   - Added `FetchRemote()` git client method
   - Fetch before sync checks (configurable)
   - Stale data warnings when fetch fails

2. **Configurable Remote Names** (Both critiques)
   - Added `RemoteConfig` to types
   - Classifier accepts remote names as parameters
   - No hardcoded "origin"/"github" assumptions

3. **Simplified Large Binary Detection** (Gemini critique)
   - Only detect existence and size
   - Skip expensive commit metadata search
   - Reduces from minutes to seconds

4. **Structured Fix Operations** (Gemini critique)
   - Added `Operation` interface and implementations
   - Validation before execution
   - Rollback support on failures
   - Replaces fragile command strings

5. **Improved Fallback Logic** (Gemini critique)
   - `GetDefaultBranch` uses `git remote show`
   - More reliable than guessing main/master

6. **Complete Fix Implementations** (GPT critique)
   - All 41 scenarios have complete fixes
   - No "not implemented" stubs
   - All auto-fixable scenarios tested

7. **Detailed Integration Test Strategy** (GPT critique)
   - Specified test harness using local git-daemon
   - Documented how to create test remotes offline
   - Clear setup/teardown procedures

8. **Early Benchmarking** (GPT critique)
   - Phase 0 dedicated to performance infrastructure
   - Baseline measurements before implementation
   - CI integration to catch regressions

9. **Git Submodules Documentation** (User request)
   - Added to out-of-scope with detailed explanation
   - Documented complexity and future approach
   - Clear v1 limitations

10. **Timeline Adjustment**
    - 4-6 weeks → 5-7 weeks
    - Accounts for additional complexity
    - More realistic given enhancements

---

**END OF IMPLEMENTATION PLAN v2.0**

*This plan addresses all critique feedback and is ready for agent implementation and external review.*
*Version 2.0 - 2025-11-18*
