# Repository State Scenario Enumeration & Maintenance Plan

**Version:** 1.1
**Date:** 2025-11-18
**Status:** Living Document
**Purpose:** Comprehensive enumeration of all possible repository states across three locations (Local, Core, GitHub) to guide GitHelper implementation, testing, and help system.

---

## Document Structure

1. [Overview](#overview)
2. [Dimension Analysis](#dimension-analysis)
3. [Scenario Enumeration](#scenario-enumeration)
4. [Logical Reductions](#logical-reductions)
5. [Decision Flow](#decision-flow)
6. [Architecture: Scenario Classification System](#architecture-scenario-classification-system)
7. [Implementation Mapping](#implementation-mapping)
8. [Test Coverage](#test-coverage)
9. [Known Gaps](#known-gaps)
10. [Maintenance Log](#maintenance-log)

---

## Overview

### The Three-Location Problem

GitHelper manages repositories across three distinct locations:
- **Local**: Working copy on current machine (`~/repos/myproject`)
- **Core**: Bare repository on git server (`core.lcgerke.com:/srv/git/myproject.git`)
- **GitHub**: Remote backup (`github.com:lcgerke/myproject.git`)

### State Space Complexity

The theoretical state space is astronomical:
- Repository existence: 2³ = 8 states
- Default branch sync: 4³ = 64 states (reduced to ~13 valid)
- Per-branch topology: 2³ = 8 states per branch
- Working tree: 5 states
- Data corruption: 2³ = 8 states

**Total theoretical combinations:** 8 × 64 × 8^N × 5 × 8 where N = number of branches

**Practical reduction:** ~100-200 distinct actionable scenarios through logical constraints and hierarchical decision-making.

---

## Dimension Analysis

### D1: Repository Existence

**States:** 8 combinations (2³)

| ID | Local | Core | GitHub | Scenario Name | Valid? | Priority |
|----|-------|------|--------|---------------|--------|----------|
| E1 | ✓ | ✓ | ✓ | **Dual-remote managed** | ✅ | P0 - Primary use case |
| E2 | ✓ | ✓ | ✗ | **Core-only** | ✅ | P1 - Common migration path |
| E3 | ✓ | ✗ | ✓ | **GitHub-only clone** | ✅ | P1 - External clone scenario |
| E4 | ✓ | ✗ | ✗ | **Local-only** | ✅ | P2 - New project |
| E5 | ✗ | ✓ | ✓ | **Missing local copy** | ✅ | P1 - Clone needed |
| E6 | ✗ | ✓ | ✗ | **Core orphan** | ✅ | P3 - Rare |
| E7 | ✗ | ✗ | ✓ | **GitHub orphan** | ✅ | P3 - Rare |
| E8 | ✗ | ✗ | ✗ | **Doesn't exist** | ❌ | N/A - Out of scope |

**Resolution Actions:**
- **E1**: Normal workflow, proceed to sync state analysis
- **E2**: `githelper github setup <repo> --create [--private]`
- **E3**: Add core remote, push, then `github setup`
- **E4**: `githelper repo create <repo>` + `github setup --create`
- **E5**: `git clone core:<repo>` or `git clone github:<repo>`
- **E6**: `git clone core:<repo>`
- **E7**: `git clone github:<repo>`

---

### D2: Default Branch Sync States

**Theoretical states:** 64 (4³)
**Valid states after logical reduction:** ~13

#### Sync State Definitions

For any pair of repositories:
- **synced**: Same commit hash
- **ahead**: Has commits the other doesn't (can fast-forward)
- **behind**: Missing commits the other has (can fast-forward)
- **diverged**: Both have unique commits (requires merge/rebase)

#### Valid Sync State Matrix

| ID | Local ↔ Core | Local ↔ GitHub | Core ↔ GitHub | Scenario Description | Action Required |
|----|--------------|----------------|---------------|----------------------|-----------------|
| S1 | synced | synced | synced | **Perfect sync** | None |
| S2 | ahead | ahead | synced | Local has new commits | `git push` (dual-push) |
| S3 | synced | ahead | behind | Core pushed to GitHub without local | `git fetch core` |
| S4 | ahead | synced | behind | Local pushed to Core only | `githelper github sync` |
| S5 | synced | behind | ahead | GitHub has commits | `git fetch github && merge` |
| S6 | behind | behind | synced | Both remotes ahead | `git pull` |
| S7 | behind | synced | ahead | Core has commits | `git pull origin main` |
| S8 | ahead | behind | ahead | Local ahead of GitHub, behind Core | Fetch core, then push |
| S9 | diverged | diverged | synced | Local diverged from synced remotes | Merge, then push both |
| S10 | synced | diverged | diverged | Remotes diverged, local matches one | Sync remotes first |
| S11 | diverged | synced | diverged | Remotes diverged, local matches other | Sync remotes first |
| S12 | ahead | diverged | diverged | All three have unique commits | **Manual resolution** |
| S13 | diverged | diverged | diverged | **Three-way divergence** | **Manual resolution** |

#### Impossible States (Examples)

- Local synced with Core, Core ahead of GitHub, Local behind GitHub ← Violates transitivity
- Local ahead and behind simultaneously ← Contradiction
- Synced but diverged ← Contradiction

**Reduction principle:** Transitivity of the `synced` relation eliminates most invalid combinations.

---

### D3: Branch Topology

**States:** 8 per branch (2³)
**Treatment:** Independent analysis per branch

| ID | Local | Core | GitHub | Scenario | Action | Auto-fixable? |
|----|-------|------|--------|----------|--------|---------------|
| B1 | ✓ | ✓ | ✓ | Fully tracked | Normal sync | ✅ |
| B2 | ✓ | ✓ | ✗ | Core-tracked only | Push to GitHub | ✅ |
| B3 | ✓ | ✗ | ✓ | GitHub-tracked only | Push to Core | ✅ |
| B4 | ✓ | ✗ | ✗ | **Local feature branch** | Push when ready | ⚠️ User decision |
| B5 | ✗ | ✓ | ✓ | Remote branch not fetched | `git checkout -t origin/<branch>` | ✅ |
| B6 | ✗ | ✓ | ✗ | Core orphan branch | `git checkout -t origin/<branch>` | ✅ |
| B7 | ✗ | ✗ | ✓ | GitHub orphan branch | `git checkout -t github/<branch>` | ✅ |
| B8 | ✗ | ✗ | ✗ | Doesn't exist | N/A | N/A |

**Key Insight:** Branches are orthogonal. Don't multiply state space—handle iteratively.

**Orphan Branch Detection:**
```bash
# Branches on Core but not GitHub
comm -23 <(git ls-remote origin | grep refs/heads | cut -f2) \
         <(git ls-remote github | grep refs/heads | cut -f2)
```

---

### D4: Working Tree State

**States:** 5 theoretical, reduced to 2 actionable

| ID | State | Staged? | Unstaged? | Conflicts? | Action |
|----|-------|---------|-----------|------------|--------|
| W1 | **Clean** | ✗ | ✗ | N/A | Proceed with sync |
| W2 | Staged changes | ✓ | ✗ | Unknown | Require commit or stash |
| W3 | Unstaged changes | ✗ | ✓ | Unknown | Require commit or stash |
| W4 | Mixed | ✓ | ✓ | Unknown | Require commit or stash |
| W5 | Dirty + will conflict | Either | Either | ✓ | **Block until clean** |

**Practical Reduction:**
- Clean (W1) → Proceed
- Dirty (W2-W5) → Require `git stash` or `git commit` before automated operations

**Detection:**
```bash
# Check if working tree is clean
git diff-index --quiet HEAD --
echo $?  # 0 = clean, 1 = dirty
```

---

### D5: Data Corruption (Large Binaries)

**States:** 8 (2³)

| ID | Local | Core | GitHub | Scenario | Fix Strategy | Complexity |
|----|-------|------|--------|----------|--------------|------------|
| C1 | ✗ | ✗ | ✗ | **No corruption** | N/A | None |
| C2 | ✓ | ✗ | ✗ | Local only | BFG local, force push | Low |
| C3 | ✗ | ✓ | ✗ | Core only | BFG core, force pull | Medium |
| C4 | ✗ | ✗ | ✓ | GitHub only | BFG, force push to GitHub | Medium |
| C5 | ✓ | ✓ | ✗ | Local + Core | BFG local, force push both | High |
| C6 | ✓ | ✗ | ✓ | Local + GitHub | BFG local, force push both | High |
| C7 | ✗ | ✓ | ✓ | Core + GitHub | BFG anywhere, force push all | High |
| C8 | ✓ | ✓ | ✓ | **All infected** | BFG, force push, team re-clone | **Critical** |

**Critical Constraints:**
- Force push breaks dual-push assumptions (no fast-forward)
- Requires disabling hooks temporarily
- Must coordinate with all collaborators
- Must fix **all** locations or corruption returns

**Detection:**
```bash
# Find large objects in history
git rev-list --objects --all | \
  git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' | \
  awk '$1 == "blob" && $3 >= 1048576 {print $3/1048576 " MB:", $4}' | \
  sort -nr | head -20
```

**BFG Workflow:**
```bash
# 1. Backup
git clone --mirror core:<repo> /tmp/backup.git

# 2. BFG cleanup
git clone --mirror core:<repo> /tmp/<repo>.git
java -jar bfg.jar --strip-blobs-bigger-than 10M /tmp/<repo>.git
cd /tmp/<repo>.git && git reflog expire --expire=now --all && git gc --prune=now --aggressive

# 3. Force push (DESTRUCTIVE)
git push --force

# 4. All users must re-clone
```

---

## Logical Reductions

### Impossibilities

These combinations cannot exist due to logical constraints:

1. **Local doesn't exist but has uncommitted changes** ← Contradiction
2. **Synced and ahead simultaneously** ← Contradiction
3. **Transitive violations:**
   - Local = Core, Core = GitHub, but Local ≠ GitHub ← Impossible
   - Local ahead of Core, Core ahead of GitHub, Local synced with GitHub ← Violates ordering

### Hierarchical Dependencies

```
Existence (D1)
  └─> Working Tree (D4)
      └─> Data Integrity (D5)
          └─> Default Branch Sync (D2)
              └─> Per-Branch Topology (D3)
```

**Implication:** Check in order. Don't analyze sync state if repo doesn't exist.

### Independent vs Dependent Dimensions

**Independent:**
- Branch topology (each branch analyzed separately)
- Working tree state (orthogonal to sync state)

**Dependent:**
- Sync state depends on existence
- Data corruption blocks sync operations

---

## Decision Flow

### Master Decision Tree

```
START: User runs `githelper doctor <repo>` or `githelper github sync <repo>`

├─ [Check D1: Existence]
│  ├─ Local doesn't exist (E5-E7)
│  │  └─> ACTION: Offer to clone from Core or GitHub
│  │      └─> EXIT (cannot proceed with local operations)
│  │
│  ├─ Core doesn't exist (E3, E4)
│  │  └─> ACTION: Offer to create and push to Core
│  │      └─> CONTINUE
│  │
│  ├─ GitHub doesn't exist (E2, E4)
│  │  └─> ACTION: Offer `github setup --create`
│  │      └─> CONTINUE
│  │
│  └─ All exist (E1)
│      └─> CONTINUE
│
├─ [Check D4: Working Tree]
│  ├─ Dirty (W2-W5)
│  │  └─> ACTION: Require `git stash` or `git commit`
│  │      └─> EXIT (cannot auto-sync with dirty tree)
│  │
│  └─ Clean (W1)
│      └─> CONTINUE
│
├─ [Check D5: Data Corruption]
│  ├─ Large binaries detected (C2-C8)
│  │  └─> ACTION: Offer BFG cleanup workflow
│  │      └─> EXIT (must resolve before sync)
│  │
│  └─ No corruption (C1)
│      └─> CONTINUE
│
├─ [Check D2: Default Branch Sync]
│  ├─ Perfect sync (S1)
│  │  └─> ACTION: Report "All synced ✓"
│  │      └─> CONTINUE to branches
│  │
│  ├─ Local ahead of both (S2)
│  │  └─> ACTION: Auto-fix with `git push`
│  │      └─> CONTINUE to branches
│  │
│  ├─ Local behind both (S6)
│  │  └─> ACTION: Auto-fix with `git pull`
│  │      └─> CONTINUE to branches
│  │
│  ├─ Simple ahead/behind (S3-S8)
│  │  └─> ACTION: Offer auto-sync (fast-forward)
│  │      └─> CONTINUE to branches
│  │
│  └─ Diverged (S9-S13)
│      └─> ACTION: Require manual merge
│          └─> EXIT (cannot auto-resolve)
│
└─ [Check D3: Per-Branch Topology]
   ├─ For each branch:
   │  ├─ Fully tracked (B1)
   │  │  └─> Check sync state (same logic as D2)
   │  │
   │  ├─ Orphan on Core (B2, B6)
   │  │  └─> ACTION: Offer to push to GitHub
   │  │
   │  ├─ Orphan on GitHub (B3, B7)
   │  │  └─> ACTION: Offer to push to Core
   │  │
   │  └─ Local only (B4)
   │      └─> ACTION: Report (user decides when to push)
   │
   └─> DONE: Report summary
```

### Quick Diagnostic Decision Path

For `githelper doctor <repo> --quick`:

```
1. Existence?     E1=✓  E2-E7=✗
2. Clean tree?    W1=✓  W2-W5=✗
3. No binaries?   C1=✓  C2-C8=✗
4. Main synced?   S1=✓  S2-S13=✗
5. Branches OK?   All B1=✓  Any orphan=✗

All ✓ = Healthy
Any ✗ = Report issue + fix suggestion
```

---

## Architecture: Scenario Classification System

This section details the architecture for detecting and classifying repository states into scenario IDs.

### Design Goals

1. **Hierarchical detection** following dependency order (existence → working tree → corruption → sync → branches)
2. **Single pass analysis** with caching to avoid redundant git operations
3. **Composable state representation** allowing commands to consume only needed dimensions
4. **Deterministic classification** mapping detected states to scenario IDs
5. **Actionable output** providing fixes alongside diagnosis

---

### Core Data Structures

#### State Representation

**File:** `internal/scenarios/types.go`

```go
package scenarios

import "time"

// RepositoryState represents the complete state across all dimensions
type RepositoryState struct {
    Repo          string
    DetectedAt    time.Time

    // Dimensions (in dependency order)
    Existence     ExistenceState
    WorkingTree   WorkingTreeState
    Corruption    CorruptionState
    DefaultBranch BranchSyncState
    Branches      []BranchState
}

// ExistenceState tracks where the repository exists (D1: E1-E8)
type ExistenceState struct {
    ID           string  // "E1", "E2", ..., "E8"
    LocalExists  bool
    LocalPath    string
    CoreExists   bool
    CoreURL      string
    GitHubExists bool
    GitHubURL    string
}

// WorkingTreeState tracks uncommitted changes (D4: W1-W5)
type WorkingTreeState struct {
    ID                string  // "W1", "W2", ..., "W5"
    Clean             bool
    StagedFiles       []string
    UnstagedFiles     []string
    WouldConflict     bool  // Would pull/merge cause conflicts?
}

// CorruptionState tracks data integrity issues (D5: C1-C8)
type CorruptionState struct {
    ID               string  // "C1", "C2", ..., "C8"
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

// BranchSyncState tracks sync state for a branch (D2: S1-S13)
type BranchSyncState struct {
    ID              string  // "S1", "S2", ..., "S13"
    Branch          string
    LocalHash       string
    CoreHash        string
    GitHubHash      string

    // Pairwise comparisons
    LocalVsCore     SyncStatus
    LocalVsGitHub   SyncStatus
    CoreVsGitHub    SyncStatus

    // Quantitative deltas
    LocalAheadCore    int  // Commits local has that core doesn't
    LocalBehindCore   int  // Commits core has that local doesn't
    LocalAheadGitHub  int
    LocalBehindGitHub int
    CoreAheadGitHub   int
    CoreBehindGitHub  int
}

// BranchState tracks per-branch topology (D3: B1-B7)
type BranchState struct {
    ID           string  // "B1", "B2", ..., "B7"
    Name         string
    LocalExists  bool
    CoreExists   bool
    GitHubExists bool
    SyncState    *BranchSyncState  // Only if exists on multiple locations
}

// SyncStatus represents relationship between two refs
type SyncStatus int

const (
    StatusUnknown SyncStatus = iota
    StatusSynced     // Same commit hash
    StatusAhead      // First ref ahead of second
    StatusBehind     // First ref behind second
    StatusDiverged   // Both have unique commits
)

func (s SyncStatus) String() string {
    return [...]string{"unknown", "synced", "ahead", "behind", "diverged"}[s]
}
```

---

### Classifier Implementation

#### Main Classifier

**File:** `internal/scenarios/classifier.go`

```go
package scenarios

import (
    "context"
    "fmt"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/lcgerke/githelper/internal/github"
)

// Classifier detects repository state across all dimensions
type Classifier struct {
    repoPath     string
    gitClient    *git.Client
    githubClient *github.Client
    ctx          context.Context
}

// NewClassifier creates a new state classifier
func NewClassifier(ctx context.Context, repoPath string, gc *git.Client, ghc *github.Client) *Classifier {
    return &Classifier{
        ctx:          ctx,
        repoPath:     repoPath,
        gitClient:    gc,
        githubClient: ghc,
    }
}

// Detect performs full state classification
// Returns early if dependencies fail (e.g., local doesn't exist)
func (c *Classifier) Detect() (*RepositoryState, error) {
    state := &RepositoryState{
        Repo:       c.repoPath,
        DetectedAt: time.Now(),
    }

    // D1: Existence (required for all other checks)
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

    // D5: Data corruption (can check even if remotes missing)
    state.Corruption, err = c.detectCorruption()
    if err != nil {
        return nil, fmt.Errorf("corruption check failed: %w", err)
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
        return nil, fmt.Errorf("branch topology check failed: %w", err)
    }

    return state, nil
}

// detectExistence checks where repository exists (E1-E8)
func (c *Classifier) detectExistence() (ExistenceState, error) {
    state := ExistenceState{}

    // Check local
    localExists, localPath := c.gitClient.LocalExists()
    state.LocalExists = localExists
    state.LocalPath = localPath

    // Check core remote
    if localExists {
        coreURL, err := c.gitClient.GetRemoteURL("origin")
        if err == nil && coreURL != "" {
            state.CoreExists = c.gitClient.CanReachRemote("origin")
            state.CoreURL = coreURL
        }

        // Check GitHub remote
        githubURL, err := c.gitClient.GetRemoteURL("github")
        if err == nil && githubURL != "" {
            state.GitHubExists = c.gitClient.CanReachRemote("github")
            state.GitHubURL = githubURL
        }
    }

    // Classify into E1-E8
    state.ID = classifyExistence(state.LocalExists, state.CoreExists, state.GitHubExists)

    return state, nil
}

// detectWorkingTree checks for uncommitted changes (W1-W5)
func (c *Classifier) detectWorkingTree() (WorkingTreeState, error) {
    state := WorkingTreeState{}

    // Check staged files
    staged, err := c.gitClient.GetStagedFiles()
    if err != nil {
        return state, err
    }
    state.StagedFiles = staged

    // Check unstaged files
    unstaged, err := c.gitClient.GetUnstagedFiles()
    if err != nil {
        return state, err
    }
    state.UnstagedFiles = unstaged

    // Determine if clean
    state.Clean = len(staged) == 0 && len(unstaged) == 0

    // Classify into W1-W5
    state.ID = classifyWorkingTree(state.Clean, len(staged) > 0, len(unstaged) > 0)

    return state, nil
}

// detectDefaultBranchSync determines sync state for main/master (S1-S13)
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

    // Classify into S1-S13 using lookup table
    state.ID = classifySyncState(state.LocalVsCore, state.LocalVsGitHub, state.CoreVsGitHub)

    return state, nil
}

// compareBranches determines relationship between two refs
func (c *Classifier) compareBranches(localRef, remoteRef string) (SyncStatus, int, int) {
    if localRef == "" || remoteRef == "" {
        return StatusUnknown, 0, 0
    }

    if localRef == remoteRef {
        return StatusSynced, 0, 0
    }

    // Use git rev-list to count commits
    ahead, err := c.gitClient.CountCommitsBetween(localRef, remoteRef)
    if err != nil {
        return StatusUnknown, 0, 0
    }

    behind, err := c.gitClient.CountCommitsBetween(remoteRef, localRef)
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
```

---

### Classification Lookup Tables

#### Existence Classification

**File:** `internal/scenarios/tables.go`

```go
package scenarios

// classifyExistence maps existence booleans to E1-E8
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
    return "W4"  // Mixed (both staged and unstaged)
    // W5 (conflicts) detected later during pull attempts
}
```

#### Sync State Classification Table

```go
// syncStateKey represents the three pairwise comparisons
type syncStateKey struct {
    localVsCore   SyncStatus
    localVsGitHub SyncStatus
    coreVsGitHub  SyncStatus
}

// syncStateTable maps valid pairwise combinations to scenario IDs
var syncStateTable = map[syncStateKey]string{
    // Perfect sync
    {StatusSynced, StatusSynced, StatusSynced}: "S1",

    // Local ahead scenarios
    {StatusAhead, StatusAhead, StatusSynced}:   "S2",  // Local ahead of both
    {StatusAhead, StatusSynced, StatusBehind}:  "S4",  // Local ahead of Core only
    {StatusAhead, StatusBehind, StatusAhead}:   "S8",  // Complex: local ahead of GitHub, behind Core

    // Local behind scenarios
    {StatusBehind, StatusBehind, StatusSynced}: "S6",  // Both remotes ahead
    {StatusBehind, StatusSynced, StatusAhead}:  "S7",  // Core ahead only

    // Remote-only sync scenarios
    {StatusSynced, StatusAhead, StatusBehind}:  "S3",  // Core pushed to GitHub without local
    {StatusSynced, StatusBehind, StatusAhead}:  "S5",  // GitHub has commits

    // Divergence scenarios
    {StatusDiverged, StatusDiverged, StatusSynced}:   "S9",  // Local diverged, remotes synced
    {StatusSynced, StatusDiverged, StatusDiverged}:   "S10", // Remotes diverged, local = core
    {StatusDiverged, StatusSynced, StatusDiverged}:   "S11", // Remotes diverged, local = github
    {StatusAhead, StatusDiverged, StatusDiverged}:    "S12", // All diverged (local ahead variant)
    {StatusDiverged, StatusDiverged, StatusDiverged}: "S13", // Three-way divergence
}

// classifySyncState looks up scenario ID from pairwise comparisons
func classifySyncState(localVsCore, localVsGitHub, coreVsGitHub SyncStatus) string {
    key := syncStateKey{localVsCore, localVsGitHub, coreVsGitHub}

    if id, exists := syncStateTable[key]; exists {
        return id
    }

    // Fallback for unknown/invalid states
    return "S_UNKNOWN"
}
```

---

### Fix Suggestion Engine

**File:** `internal/scenarios/suggester.go`

```go
package scenarios

// Fix represents a suggested action to resolve a state issue
type Fix struct {
    ScenarioID  string
    Description string
    Command     string
    AutoFixable bool
    Priority    int  // Lower = more urgent (1 = critical, 10 = low)
    Reason      string
}

// SuggestFixes analyzes state and returns prioritized fix suggestions
func SuggestFixes(state *RepositoryState) []Fix {
    fixes := []Fix{}

    // D1: Existence issues (highest priority after local exists)
    if state.Existence.ID == "E2" {
        fixes = append(fixes, Fix{
            ScenarioID:  "E2",
            Description: "GitHub remote not configured",
            Command:     "githelper github setup --create",
            AutoFixable: true,
            Priority:    1,
            Reason:      "Dual-push requires both Core and GitHub remotes",
        })
    }

    // D4: Working tree must be clean for auto-operations
    if state.WorkingTree.ID != "W1" {
        fixes = append(fixes, Fix{
            ScenarioID:  state.WorkingTree.ID,
            Description: "Working tree has uncommitted changes",
            Command:     "git stash  # or: git commit",
            AutoFixable: false,  // Requires user decision
            Priority:    2,
            Reason:      "Automated sync requires clean working tree",
        })
    }

    // D5: Data corruption blocks all sync
    if state.Corruption.HasCorruption {
        fixes = append(fixes, Fix{
            ScenarioID:  state.Corruption.ID,
            Description: fmt.Sprintf("Large binaries detected in history (%d files)",
                len(state.Corruption.LocalBinaries)),
            Command:     "See docs/BFG_CLEANUP.md for removal procedure",
            AutoFixable: false,
            Priority:    3,
            Reason:      "Large files should be removed before syncing",
        })
    }

    // D2: Default branch sync issues
    switch state.DefaultBranch.ID {
    case "S1":
        // Perfect sync, no action

    case "S2":
        fixes = append(fixes, Fix{
            ScenarioID:  "S2",
            Description: fmt.Sprintf("Local ahead of both remotes (%d commits)",
                state.DefaultBranch.LocalAheadCore),
            Command:     "git push",
            AutoFixable: true,
            Priority:    4,
            Reason:      "Dual-push will sync both remotes",
        })

    case "S6":
        fixes = append(fixes, Fix{
            ScenarioID:  "S6",
            Description: fmt.Sprintf("Both remotes ahead (%d commits)",
                state.DefaultBranch.LocalBehindCore),
            Command:     "git pull",
            AutoFixable: true,
            Priority:    4,
            Reason:      "Fast-forward pull will sync local",
        })

    case "S4":
        fixes = append(fixes, Fix{
            ScenarioID:  "S4",
            Description: fmt.Sprintf("Local ahead of GitHub (%d commits)",
                state.DefaultBranch.LocalAheadGitHub),
            Command:     "githelper github sync",
            AutoFixable: true,
            Priority:    4,
            Reason:      "GitHub is behind Core, needs selective sync",
        })

    case "S9", "S10", "S11", "S12", "S13":
        fixes = append(fixes, Fix{
            ScenarioID:  state.DefaultBranch.ID,
            Description: "Divergent histories detected",
            Command:     "Manual merge required - see docs/DIVERGENCE.md",
            AutoFixable: false,
            Priority:    5,
            Reason:      "Divergence requires manual conflict resolution",
        })
    }

    // D3: Orphan branches
    for _, branch := range state.Branches {
        switch branch.ID {
        case "B2", "B6":  // On Core but not GitHub
            fixes = append(fixes, Fix{
                ScenarioID:  branch.ID,
                Description: fmt.Sprintf("Branch '%s' not on GitHub", branch.Name),
                Command:     fmt.Sprintf("git push github %s", branch.Name),
                AutoFixable: true,
                Priority:    6,
                Reason:      "Complete dual-remote coverage",
            })

        case "B3", "B7":  // On GitHub but not Core
            fixes = append(fixes, Fix{
                ScenarioID:  branch.ID,
                Description: fmt.Sprintf("Branch '%s' not on Core", branch.Name),
                Command:     fmt.Sprintf("git push origin %s", branch.Name),
                AutoFixable: true,
                Priority:    6,
                Reason:      "Core should have all branches",
            })
        }
    }

    // Sort by priority
    sort.Slice(fixes, func(i, j int) bool {
        return fixes[i].Priority < fixes[j].Priority
    })

    return fixes
}

// AutoFix applies all auto-fixable fixes
// Returns list of applied fixes and any errors
func AutoFix(state *RepositoryState, gitClient *git.Client) ([]Fix, []error) {
    fixes := SuggestFixes(state)
    applied := []Fix{}
    errors := []error{}

    for _, fix := range fixes {
        if !fix.AutoFixable {
            continue
        }

        // Execute fix based on scenario
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
```

---

### Command Integration Pattern

Commands should use the classifier in a consistent pattern:

```go
// Example: cmd/githelper/doctor.go
func runDoctor(cmd *cobra.Command, args []string) error {
    repoName := args[0]

    // Initialize clients
    gitClient := git.NewClient(repoPath)
    ghClient := github.NewClient(ctx, pat)

    // Create classifier
    classifier := scenarios.NewClassifier(ctx, repoPath, gitClient, ghClient)

    // Detect state (single pass)
    state, err := classifier.Detect()
    if err != nil {
        return fmt.Errorf("state detection failed: %w", err)
    }

    // Report findings
    out.Header("Repository State Analysis")
    out.Infof("Existence:    %s (%s)", state.Existence.ID,
              scenarios.ExplainExistence(state.Existence.ID))
    out.Infof("Working Tree: %s (%s)", state.WorkingTree.ID,
              scenarios.ExplainWorkingTree(state.WorkingTree.ID))
    out.Infof("Integrity:    %s (%s)", state.Corruption.ID,
              scenarios.ExplainCorruption(state.Corruption.ID))
    out.Infof("Sync State:   %s (%s)", state.DefaultBranch.ID,
              scenarios.ExplainSync(state.DefaultBranch.ID))

    // Get fix suggestions
    fixes := scenarios.SuggestFixes(state)

    if len(fixes) == 0 {
        out.Success("All systems healthy ✓")
        return nil
    }

    // Report issues
    out.Separator()
    out.Warningf("Found %d issue(s):", len(fixes))
    for i, fix := range fixes {
        icon := "⚠"
        if fix.AutoFixable {
            icon = "✓"
        }
        out.Infof("  %d. [%s] %s: %s", i+1, icon, fix.ScenarioID, fix.Description)
        out.Infof("     Fix: %s", fix.Command)
    }

    // Auto-fix if requested
    if autoFix {
        out.Separator()
        out.Info("Applying auto-fixes...")
        applied, errors := scenarios.AutoFix(state, gitClient)

        out.Successf("Applied %d fix(es)", len(applied))
        if len(errors) > 0 {
            out.Warningf("%d fix(es) failed:", len(errors))
            for _, err := range errors {
                out.Errorf("  - %v", err)
            }
        }
    }

    return nil
}
```

---

### Performance Considerations

1. **Caching:** `Detect()` runs once; commands reuse the `RepositoryState` object
2. **Lazy loading:** Branch topology only scanned if needed
3. **Parallelization:** Corruption check (expensive) can run concurrently with sync detection
4. **Early exit:** If existence check fails, skip expensive operations

**Optimization opportunities:**
```go
// Detect with options
func (c *Classifier) DetectWithOptions(opts DetectOptions) (*RepositoryState, error) {
    if opts.SkipCorruption {
        // Skip expensive large file scan
    }
    if opts.OnlyDefaultBranch {
        // Skip per-branch topology
    }
}
```

---

### Testing Strategy

**Unit tests:**
- `tables_test.go`: Verify all 13 valid sync states map correctly
- `classifier_test.go`: Mock git operations, test state detection

**Integration tests:**
- `scenarios_test.go`: Create repos in known states (E1-E8, S1-S13), verify detection
- `suggester_test.go`: Verify fix suggestions for each scenario

**Test data:**
```go
// scenarios_test.go
func TestSyncStateDetection(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(*git.TestRepo)
        expected string
    }{
        {
            name: "S1: Perfect sync",
            setup: func(r *git.TestRepo) {
                r.CommitToAll("sync commit")
            },
            expected: "S1",
        },
        {
            name: "S2: Local ahead of both",
            setup: func(r *git.TestRepo) {
                r.CommitLocal("local only")
            },
            expected: "S2",
        },
        // ... test all 13 states
    }
}
```

---

## Implementation Mapping

### Command: `doctor`

**Purpose:** Diagnostic health check across all dimensions

**Flags:**
- `--repo <name>`: Check specific repo
- `--all`: Check all repos in state file
- `--credentials`: Show SSH keys and PATs
- `--auto-fix`: Attempt automatic resolution
- `--verbose`: Show all checks, even passing

**Check Order:**
1. D1: Existence (report missing remotes)
2. D4: Working tree (warn if dirty)
3. D5: Data corruption (detect large binaries)
4. D2: Default branch sync (show state ID: S1-S13)
5. D3: Branch topology (list orphan branches)

**Output Format:**
```
🔍 GitHelper Diagnostic Report: myproject
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Existence:
  ✓ Local:  /home/user/repos/myproject
  ✓ Core:   core.lcgerke.com:/srv/git/myproject.git
  ✓ GitHub: github.com:lcgerke/myproject.git

Working Tree:
  ✓ Clean (no uncommitted changes)

Data Integrity:
  ✓ No large binaries detected in history

Default Branch (main):
  ⚠ Sync State: S4 (Local ahead, Core synced with GitHub)
  → Fix: githelper github sync myproject

Branches:
  ✓ main: Tracked on all remotes
  ⚠ feature/auth: B2 (On Core but not GitHub)
  ⚠ feature/ui: B4 (Local only, not pushed)

Summary:
  2 issue(s) found, 1 auto-fixable
  Run with --auto-fix to resolve automatically
```

**Implementation:** `cmd/githelper/doctor.go`

---

### Command: `github sync`

**Purpose:** Synchronize GitHub with Core for specific branch

**Flags:**
- `--branch <name>`: Branch to sync (default: current)
- `--dry-run`: Show what would happen
- `--force`: Proceed even with warnings

**Pre-conditions:**
- Existence: Must be E1 (all three exist)
- Working tree: Must be W1 (clean)
- Data corruption: Must be C1 (none)

**Logic:**
1. Determine sync state (S1-S13)
2. If S1: Report "already synced"
3. If S2-S8: Calculate diff, push to GitHub
4. If S9-S13: Require manual resolution

**Implementation:** `cmd/githelper/github_sync.go` (already exists)

---

### Command: `github setup`

**Purpose:** Configure dual-push for existing repo

**Handles existence states:**
- E2: Core exists, GitHub doesn't → Create GitHub repo
- E3: GitHub exists, Core doesn't → Error (must add Core first)
- E4: Neither exists → Error (must run `repo create` first)

**Implementation:** `cmd/githelper/github_setup.go` (already exists)

---

### New Command: `repair`

**Purpose:** Interactive repair wizard for problematic states

**Workflow:**
```bash
githelper repair myproject

# Walks through decision tree:
# 1. Detect state
# 2. Explain issue
# 3. Offer fix
# 4. Apply if confirmed
# 5. Re-check
```

**Example Session:**
```
🔧 GitHelper Repair Wizard: myproject
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[1/5] Checking existence...
  ✓ All remotes exist

[2/5] Checking working tree...
  ✗ Working tree is dirty (3 modified files)

  Fix: Commit or stash changes

  Options:
    1. Stash changes (git stash)
    2. Show modified files
    3. Skip this check

  Choice: 1

  ✓ Stashed 3 files

[3/5] Checking data integrity...
  ⚠ Found 2 large binaries in history:
    - data/large.bin (50 MB, commit a1b2c3d)
    - backup.zip (120 MB, commit e4f5g6h)

  This requires BFG cleanup (advanced, destructive)

  Options:
    1. Show BFG cleanup guide
    2. Skip (fix manually later)
    3. Abort repair

  Choice: 2

[4/5] Checking default branch sync...
  ⚠ State S4: Local ahead of GitHub

  Fix: Push 3 commits to GitHub

  Commits to push:
    a1b2c3d Add feature X
    e4f5g6h Fix bug Y
    h7i8j9k Update docs

  Proceed? (y/N): y

  ✓ Pushed to GitHub

[5/5] Checking branches...
  ⚠ feature/auth exists on Core but not GitHub

  Push to GitHub? (y/N): y

  ✓ Pushed feature/auth to GitHub

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Repair complete!

  Fixed: 3 issues
  Remaining: 1 (large binaries - manual fix required)

  Next steps:
    1. Review BFG cleanup guide: docs/BFG_CLEANUP.md
    2. Run 'githelper doctor myproject' to verify
```

**Implementation:** `cmd/githelper/repair.go` (NEW)

---

### New Command: `status`

**Purpose:** Quick state summary using scenario IDs

**Output:**
```bash
githelper status myproject

Repository: myproject
  Existence:  E1 (✓ Dual-remote managed)
  Sync State: S4 (⚠ Local ahead of GitHub)
  Branches:   2 tracked, 1 orphan (Core-only)
  Tree:       W1 (✓ Clean)
  Integrity:  C1 (✓ No issues)

Quick fix: githelper github sync myproject
```

**Implementation:** `cmd/githelper/status.go` (NEW)

---

## Test Coverage

### Test Matrix by Scenario ID

| Scenario ID | Test Type | Test Location | Status |
|-------------|-----------|---------------|--------|
| E1 | Integration | `test/integration/dual_remote_test.go` | ✅ Exists |
| E2 | Integration | `test/integration/github_setup_test.go` | ✅ Exists |
| E3 | Unit | `test/unit/existence_test.go` | ⚠️ TODO |
| E4 | Integration | `test/integration/repo_create_test.go` | ✅ Exists |
| E5-E7 | Unit | `test/unit/existence_test.go` | ⚠️ TODO |
| S1 | Unit | `test/unit/sync_state_test.go` | ⚠️ TODO |
| S2-S13 | Unit | `test/unit/sync_state_test.go` | ⚠️ TODO |
| B1-B7 | Integration | `test/integration/branch_topology_test.go` | ⚠️ TODO |
| W1-W5 | Unit | `internal/git/worktree_test.go` | ⚠️ TODO |
| C1-C8 | Integration | `test/integration/corruption_test.go` | ⚠️ TODO |

### Priority Test Additions

**P0 (Critical):**
1. Sync state detection (S1-S13) - Unit tests
2. Working tree checks (W1-W5) - Unit tests
3. Orphan branch detection (B4-B7) - Integration tests

**P1 (High):**
4. Existence state handling (E3, E5) - Integration tests
5. Data corruption detection (C2-C8) - Integration tests

**P2 (Medium):**
6. Decision tree navigation - Integration test
7. Auto-fix validation - Integration test

---

## Known Gaps

### Missing Functionality

**Status Update (v1.1):** Architecture is now fully documented. All gaps below are implementation-only.

1. **Scenario Classification System** (D1-D5: All dimensions)
   - Current: None
   - Documented: ✅ Full architecture in "Architecture" section
   - Needed: Implementation of classifier package
   - Files:
     - `internal/scenarios/types.go` (NEW - documented)
     - `internal/scenarios/classifier.go` (NEW - documented)
     - `internal/scenarios/tables.go` (NEW - documented)
     - `internal/scenarios/suggester.go` (NEW - documented)

2. **Git Client Extensions** (Supporting classifier)
   - Current: Basic git operations exist
   - Needed: Additional methods for state detection
   - Methods to add:
     - `LocalExists() (bool, string)`
     - `GetStagedFiles() ([]string, error)`
     - `GetUnstagedFiles() ([]string, error)`
     - `GetBranchHash(branch string) (string, error)`
     - `GetRemoteBranchHash(remote, branch string) (string, error)`
     - `CountCommitsBetween(ref1, ref2 string) (int, error)`
   - File: `internal/git/cli.go` (enhancement)

3. **Data corruption detection** (D5: C1-C8)
   - Current: None
   - Documented: ✅ Detection logic in classifier
   - Needed: Large binary scanning implementation
   - File: `internal/git/integrity.go` (NEW)

4. **Interactive repair wizard**
   - Current: None
   - Documented: ✅ Full UX flow in "Implementation Mapping"
   - Needed: Implementation
   - File: `cmd/githelper/repair.go` (NEW)

5. **Command Integration**
   - Current: `doctor` exists but no state classification
   - Documented: ✅ Integration pattern provided
   - Needed: Update existing commands to use classifier
   - Files:
     - `cmd/githelper/doctor.go` (enhance with classifier)
     - `cmd/githelper/github_sync.go` (add pre-condition checks)
     - `cmd/githelper/status.go` (NEW - quick state summary)
     - `cmd/githelper/scenarios.go` (NEW - help system)

### Documentation Gaps

1. BFG cleanup workflow guide
2. Manual divergence resolution guide
3. Force-push safety procedures
4. Multi-collaborator sync coordination

---

## Help System Integration

### Future State: `--help` Navigation

```bash
githelper doctor --help

# Should include:
# "For detailed scenario explanations, see:"
# "  - Existence states: githelper scenarios existence"
# "  - Sync states: githelper scenarios sync"
# "  - Common issues: githelper scenarios common"
```

### New Command: `scenarios`

```bash
# List all scenarios
githelper scenarios list

# Explain specific scenario
githelper scenarios explain S4
# Output:
#   Scenario S4: Local Ahead, Core Synced with GitHub
#
#   Situation:
#     - You pushed commits to Core (bare repo)
#     - Dual-push to GitHub failed (network issue, etc.)
#     - Core and GitHub are now synced, but local is ahead
#
#   Detection:
#     Local: 3 commits ahead of Core
#     Local: Synced with GitHub
#     Core: 3 commits behind GitHub
#
#   Fix:
#     githelper github sync myproject
#
#   Manual alternative:
#     git push github main

# Show decision tree
githelper scenarios tree
# Output: ASCII art of master decision tree

# Diagnose current state
githelper scenarios detect myproject
# Output: E1, S4, B2, W1, C1
```

**Implementation:** `cmd/githelper/scenarios.go` (NEW)

---

## Maintenance Log

### Version 1.1 (2025-11-18)

**Added Architecture Section:**
- Complete data structure definitions (`internal/scenarios/types.go`)
- Classifier implementation with hierarchical detection (`internal/scenarios/classifier.go`)
- Classification lookup tables for all dimensions (`internal/scenarios/tables.go`)
- Fix suggestion engine with auto-fix capability (`internal/scenarios/suggester.go`)
- Command integration patterns for `doctor`, `sync`, `repair`
- Performance considerations and optimization strategies
- Testing strategy with unit and integration test templates

**Affected commands:**
- `doctor`: Now has architectural blueprint for state detection
- `sync`: Clear pre-conditions and state classification flow
- `repair` (new): Complete interaction design
- `status` (new): Quick state summary using scenario IDs
- `scenarios` (new): Help system integration

**Implementation files specified:**
- `internal/scenarios/types.go` (NEW)
- `internal/scenarios/classifier.go` (NEW)
- `internal/scenarios/tables.go` (NEW)
- `internal/scenarios/suggester.go` (NEW)

**Next steps:** Implement classifier package and integrate into existing `doctor` command

---

### Version 1.0 (2025-11-18)

**Initial enumeration covering:**
- 8 existence states (E1-E8)
- 13 sync states (S1-S13)
- 7 branch topology states (B1-B7)
- 5 working tree states (W1-W5)
- 8 data corruption states (C1-C8)

**Total scenarios documented:** 41

**Logical reductions identified:**
- Transitivity constraint on sync states (64 → 13)
- Hierarchical checking order
- Branch independence principle

**Next review:** When first deficiency is found during implementation

---

### Maintenance Procedure

**When to update this document:**

1. **New scenario discovered** during real-world usage
   - Add to appropriate dimension table
   - Assign new ID
   - Update decision tree if needed
   - Add test case

2. **Logical reduction found** (invalid state identified)
   - Document in "Impossibilities" section
   - Remove from test matrix
   - Update state count

3. **Implementation reveals edge case**
   - Add to "Known Gaps"
   - Update priority
   - Create GitHub issue

4. **User reports unexpected behavior**
   - Diagnose which scenario(s) involved
   - Check if documented
   - If new: follow procedure #1
   - If documented: check implementation gap

**Update format:**
```markdown
### Version X.Y (YYYY-MM-DD)

**Changes:**
- Added scenario SX: <description>
- Removed impossible state SY
- Updated decision tree: <change>

**Affected commands:**
- `doctor`: <impact>
- `sync`: <impact>

**Test updates:**
- Added: <test file>
- Modified: <test file>
```

---

## Usage Examples

### Scenario Reference Lookup

**User asks:** "I have commits on GitHub that aren't on Core, and local is clean. What state am I in?"

**Answer:**
1. Existence: E1 (all exist)
2. Working tree: W1 (clean)
3. Sync state: S5 (synced locally, behind GitHub, ahead on Core)
4. Action: Fetch from GitHub, then push to Core

### Implementation Validation

**Developer asks:** "I'm implementing `sync` command. What states must I handle?"

**Answer:**
- Pre-conditions: E1, W1, C1
- Must handle: S1-S13 (all sync states)
- Can auto-resolve: S2-S8
- Require manual: S9-S13
- See: Decision Flow → Default Branch Sync

### Test Case Generation

**QA asks:** "What test cases do we need for branch sync?"

**Answer:**
- Existence: E1 (prereq)
- Branch topology: B1-B7 (7 test cases)
- For each: Cross with S1-S13 (91 combinations, reduce to ~20 meaningful)
- See: Test Coverage → Branch Topology

---

## Future Enhancements

### Phase 1: Core Detection (v1.1)
- Implement scenario classification in `internal/scenarios/`
- Add state detection to `doctor` command
- Report scenario IDs in output

### Phase 2: Auto-Repair (v1.2)
- Implement `repair` command
- Auto-fix for S2-S8, B2-B3, B5-B7
- Interactive wizard for S9-S13

### Phase 3: Corruption Handling (v1.3)
- Add integrity checks (D5)
- BFG workflow automation
- Collaboration lock during force-push

### Phase 4: Help Integration (v1.4)
- `scenarios` command
- Interactive help navigation
- State-specific documentation

### Phase 5: Advanced Scenarios (v2.0)
- Multi-branch bulk operations
- Tag sync analysis
- Submodule state tracking
- Worktree support

---

## Appendix: Quick Reference

### Scenario ID Cheat Sheet

```
Existence (E):        Sync State (S):           Branch (B):
E1 = All exist        S1 = Perfect sync         B1 = Fully tracked
E2 = Core only        S2 = Local ahead both     B2 = Core, not GitHub
E3 = GitHub only      S3 = Core→GitHub only     B3 = GitHub, not Core
E4 = Local only       S4 = Local→Core only      B4 = Local only
E5 = Missing local    S5 = GitHub ahead         B5 = Not fetched
E6 = Core orphan      S6 = Both ahead           B6 = Core orphan
E7 = GitHub orphan    S7 = Core ahead           B7 = GitHub orphan
                      S8 = Complex ahead
Working Tree (W):     S9 = Local diverged       Corruption (C):
W1 = Clean            S10-S13 = Multi-diverge   C1 = None
W2 = Staged                                     C2-C8 = Various
W3 = Unstaged                                   (see D5 table)
W4 = Mixed
W5 = Conflicting
```

### Command Quick Reference

```bash
# Diagnosis
githelper doctor <repo>                    # Full health check
githelper doctor <repo> --auto-fix         # Attempt automatic repair
githelper status <repo>                    # Quick state summary

# Sync Operations
githelper github sync <repo>               # Sync default branch
githelper github sync <repo> --branch X    # Sync specific branch
githelper github status <repo>             # Show sync state

# Repair
githelper repair <repo>                    # Interactive wizard (future)
githelper scenarios detect <repo>          # Show scenario IDs (future)
githelper scenarios explain S4             # Explain scenario (future)
```

---

**END OF DOCUMENT**

*This is a living document. Update version number and maintenance log when modified.*
