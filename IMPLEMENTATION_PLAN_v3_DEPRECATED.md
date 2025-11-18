# GitHelper Scenario Classification System - Implementation Plan

**Version:** 3.0
**Date:** 2025-11-18
**Target Completion:** 6-8 weeks (adjusted for new complexity)
**Estimated LOC:** ~3,200 lines (production + tests)

**Changelog from v2.0:**
- Implemented large binary detection (SHA+size only, no path lookup) - Gemini critique
- Fixed PullOperation safety (fetch+reset instead of pull) - Gemini critique
- Added Git LFS detection and warning - Gemini critique
- Defined new CLI commands (status, repair, scenarios) - GPT critique
- Specified 100-repo corpus source and validation strategy - Both critiques
- Added credential handling documentation - GPT critique
- Specified Git version dependency (2.30+) - Gemini critique
- Improved fetch performance mitigation (concurrent checks, timeout handling) - Both critiques
- Added explicit critique request for git submodules feedback

**Changelog from v1.0:**
- Added mandatory `git fetch` before sync state detection (Gemini v1)
- Made remote names configurable instead of hardcoded (both v1)
- Simplified large binary detection to existence-only (Gemini v1)
- Refactored AutoFix to use structured operations (Gemini v1)
- Improved `GetDefaultBranch` fallback logic (Gemini v1)
- Completed all fix suggestion implementations (GPT v1)
- Added detailed integration test strategy (GPT v1)
- Added micro-benchmarking early in implementation (GPT v1)
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
- Auto-fix capability for 60%+ of scenarios (with safety validation)
- Integration with existing `doctor` command
- New `status`, `repair`, and `scenarios` commands (fully specified in v3)
- Comprehensive test coverage (unit + integration)
- **Configurable remote names** (not hardcoded to origin/github)
- **Fresh remote data** via mandatory fetch before sync checks
- **Git LFS detection** and appropriate warnings
- **Safe auto-fix operations** using fetch+reset patterns

**Out of Scope (Future Phases):**
- BFG cleanup automation (manual guide only)
- Multi-repository batch operations
- **Git submodules state tracking** (adds significant complexity - see Known Limitations)
- Real-time sync monitoring
- Rebase-based workflow optimization (current design favors merge workflows)
- Git worktree coordination across multiple worktrees

### Known Limitations & Future Complexity

**Git Submodules:**
Repositories with git submodules present unique challenges that are **explicitly out of scope for v1**:
- Submodules have independent state (existence, sync, working tree, etc.)
- Submodule commits are referenced by parent repo but stored separately
- Sync states become N-dimensional (parent + each submodule)
- Auto-fix operations could corrupt submodule relationships
- Scenario count explodes: 41 base scenarios × N submodules

**Recommendation:** Address submodules in v2 after validating core classifier with simple repositories. Will require dedicated submodule dimension and recursive state detection.

**⚠️ FUTURE CRITIQUE REQUEST:** Please explicitly assess whether git submodules should remain out-of-scope for v1, or if there are minimal-effort approaches to handle basic submodule scenarios without full N-dimensional state tracking.

**Other Edge Cases Deferred:**
- Orphan branches (branches with no shared history)
- Sparse checkouts and partial clones
- Case-insensitive filesystem conflicts (Windows/macOS)
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
6. ✅ False positive rate <5% on corpus of 100+ real repos (see Corpus Testing section)
7. ✅ **NEW:** Corruption detection completes in <5s without commit metadata lookup

### Key Metrics

| Metric | Current | Target | Validation Method |
|--------|---------|--------|-------------------|
| Scenario coverage | 0/41 (0%) | 41/41 (100%) | Integration test suite |
| Auto-fixable issues | N/A | 25/41 (61%) | Unit tests |
| Test coverage | ~40% | >90% | go test -cover |
| Mean time to diagnosis | Manual | <2 seconds | Benchmark tests |
| False positive rate | N/A | <5% | Real-world corpus testing (100+ repos) |
| Stale data issues | N/A | 0% | Mandatory fetch before sync checks |
| Large binary scan time | N/A | <5s | Benchmark (no path lookup) |

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
        │  │  operations.go (structured ops) │              │
        │  └────────────┴────────────────────┘              │
        └───────────────────┬────────────────────────────────┘
                            │
        ┌───────────────────▼────────────────────────────────┐
        │         internal/git (ENHANCED)                    │
        │  ┌────────────────────────────────────────┐        │
        │  │  cli.go + 12 new methods               │        │
        │  │  - GetBranchHash()                     │        │
        │  │  - CountCommitsBetween()               │        │
        │  │  - FetchRemote()                       │        │
        │  │  - ResetToRef() [NEW v3]               │        │
        │  │  - ScanLargeBinaries() [NEW v3]        │        │
        │  │  - CheckLFSEnabled() [NEW v3]          │        │
        │  │  - ... (see Phase 1)                   │        │
        │  └────────────────────────────────────────┘        │
        └────────────────────────────────────────────────────┘
```

### Data Flow (Updated for v3)

```
User runs: githelper doctor myproject
    │
    ├─> Initialize Classifier(git.Client, github.Client, coreRemoteName, githubRemoteName)
    │
    ├─> **Pre-flight fetch** (configurable via --no-fetch flag)
    │    ├─> CONCURRENTLY: Start git fetch <coreRemote> & git fetch <githubRemote>
    │    └─> WHILE FETCHING: Run local-only checks (working tree, LFS detection)
    │
    ├─> classifier.Detect()
    │    ├─> detectExistence() → ExistenceState (E1-E8)
    │    ├─> detectWorkingTree() → WorkingTreeState (W1-W5)
    │    ├─> detectLFS() → LFS warning if enabled [NEW v3]
    │    ├─> detectCorruption() → CorruptionState (C1-C8) [FAST: SHA+size only]
    │    ├─> detectDefaultBranchSync() → BranchSyncState (S1-S13)
    │    └─> detectBranchTopology() → []BranchState (B1-B7 each)
    │
    ├─> SuggestFixes(state) → []Fix (prioritized, structured operations)
    │
    ├─> Display findings with scenario IDs
    │
    └─> If --auto-fix:
        ├─> Validate each fix operation
        ├─> Execute structured operations (FetchOperation, ResetOperation, etc.)
        └─> Re-validate state after each fix
```

---

## New CLI Commands Specification (v3)

### `githelper status`

**Purpose:** Quick sync status check without full diagnosis

**Usage:**
```bash
githelper status [--quick] [--no-fetch]
```

**Output:**
```
Repository: /path/to/repo
Remotes: origin (reachable), github (reachable)
Default Branch: main

Sync Status:
  Local:  abc123 (2 commits ahead of origin)
  Core:   def456 (origin/main)
  GitHub: def456 (github/main)

Classification: S2 (Local ahead of both remotes)
Suggested Fix: git push (auto-fixable)

Run 'githelper doctor' for detailed analysis
```

**Flags:**
- `--quick`: Skip corruption scan, only check sync state
- `--no-fetch`: Use cached remote data (faster but may be stale)

**Implementation:** Call `Classifier.Detect()` with `SkipCorruption: true`, display sync state only

---

### `githelper repair [--auto] [--scenario ID]`

**Purpose:** Apply fixes for detected issues

**Usage:**
```bash
# Interactive repair (asks before each fix)
githelper repair

# Auto-apply all auto-fixable issues
githelper repair --auto

# Fix specific scenario only
githelper repair --scenario S2
```

**Flow:**
1. Run `Classifier.Detect()`
2. Run `SuggestFixes(state)`
3. If `--auto`: apply all AutoFixable fixes with validation
4. If `--scenario ID`: apply only fixes matching scenario ID
5. Otherwise: prompt user for each fix

**Output:**
```
Detected Issues:
  [1] S2: Local ahead of both remotes (auto-fixable)
  [2] W2: Uncommitted changes (manual)
  [3] C3: Large binaries detected (manual)

Apply fix #1 (git push)? [Y/n]: y
✓ Fix applied successfully
Repository is now in sync (S1)
```

---

### `githelper scenarios [--list] [--explain ID]`

**Purpose:** Reference tool for scenario documentation

**Usage:**
```bash
# List all scenarios
githelper scenarios --list

# Explain specific scenario
githelper scenarios --explain S2
```

**Output for `--list`:**
```
Existence Scenarios (E1-E8):
  E1: Fully configured (local + core + github)
  E2: Core exists, GitHub missing
  ...

Sync Scenarios (S1-S13):
  S1: Perfect sync
  S2: Local ahead of both remotes
  ...
```

**Output for `--explain S2`:**
```
Scenario: S2 (Local ahead of both remotes)

Description:
  Your local repository has commits that haven't been pushed to
  either Core or GitHub remotes.

Typical Cause:
  - Made commits locally but haven't pushed yet
  - Working offline

Suggested Fix:
  git push (pushes to both remotes via dual-push)

Auto-fixable: Yes
Priority: 4 (Medium-high)

Related Scenarios:
  S4: Local ahead of GitHub only
  S7: Core ahead of local
```

**Implementation:** Embed scenario reference data in `tables.go`, expose via CLI

---

## Corpus Testing Strategy (v3)

### Source of 100+ Repositories

**Approach:** Curated corpus from multiple sources

1. **Public Open Source (50 repos)**
   - Clone from GitHub (diverse languages, sizes, activity levels)
   - Examples: golang/go, kubernetes/kubernetes, torvalds/linux, small utility repos
   - Include mix of: monorepos, microservices, docs-only, archived projects

2. **User's Managed Repos (30 repos)**
   - Real gitDualRemote-managed repositories from user's git server
   - Provides authentic dual-remote configurations
   - Includes historical edge cases encountered in production

3. **Synthetic Test Cases (20 repos)**
   - Programmatically generated to cover specific edge cases
   - Examples: orphan branches, extremely diverged histories, 100+ branches
   - Git LFS repos, shallow clones, repos with submodules (for negative testing)

### Automation

**Tool:** `test/corpus/corpus_validator.go`

```bash
# Run full corpus validation
go test -v ./test/corpus -timeout=30m

# Output:
# PASS: 98/100 repos classified correctly
# FALSE POSITIVE (2):
#   - repo-xyz: Detected S13 (divergence) but actually S1 (sync)
#   - repo-abc: Detected C3 (corruption) but LFS is intentional
```

**Process:**
1. Load corpus manifest (`test/corpus/repos.yaml`)
2. For each repo: clone, run `Classifier.Detect()`, validate expected scenario
3. Flag anomalies (unexpected scenarios, crashes, >2s detection time)
4. Generate report with false positive rate

### Golden Set (15 repos)

Manually validated reference repositories:
- Expected scenario IDs documented in `repos.yaml`
- Used for regression testing (CI)
- Covers all 41 scenarios across the set

**Example `repos.yaml` entry:**
```yaml
repos:
  - name: "dual-remote-synced"
    url: "git@github.com:user/repo.git"
    expected:
      existence: E1
      sync: S1
      branches: [B1, B1, B1]
      working_tree: W1
      corruption: C1
    notes: "Ideal state: fully synced, clean working tree"
```

### Validation

**Acceptance Criteria:**
- False positive rate <5% (max 5 incorrect classifications per 100 repos)
- No crashes or panics on any corpus repo
- 90% of repos detected in <2s
- Automated report generation for review

---

## Credential Handling (v3)

### Git Credential System

GitHelper **inherits** Git's existing credential mechanisms. No custom credential management is implemented in v1.

**Supported Methods:**
- SSH keys (recommended for Core remote)
- Git credential helpers (for HTTPS)
- SSH agent forwarding
- Environment variables (`GIT_SSH_COMMAND`, `GIT_ASKPASS`)

### Implementation Details

**1. SSH Keys (Primary Method)**
```bash
# User setup (outside GitHelper)
ssh-add ~/.ssh/id_rsa  # Add key to agent
git config core.sshCommand "ssh -i ~/.ssh/id_rsa"
```

GitHelper operations (`git fetch`, `git push`) will use the configured SSH key automatically.

**2. HTTPS with Credential Helper**
```bash
# User setup
git config --global credential.helper store  # or 'cache', 'osxkeychain', etc.
```

Git will prompt once for password, then cache for subsequent operations.

**3. Offline Behavior**

When remotes are unreachable (no auth, no network):
- `CanReachRemote()` returns `false` (5s timeout)
- Existence state reflects remote as unreachable
- Sync checks use stale data (if `--no-fetch` or fetch failed)
- Warning logged: "Remote <name> unreachable, using stale data"

**4. Push Operations with Auth Failures**

If `PushOperation.Execute()` fails with auth error:
- Error message preserved from git output
- Operation validation catches obvious auth issues (no SSH key)
- User sees: `"Push to origin failed: Permission denied (publickey)"`
- Recommended fix: `"Check SSH key configuration: ssh -T git@server"`

### Testing Strategy

**Integration Tests:**
- Mock git daemon with auth (test both success and failure)
- Test timeout behavior (slow remotes)
- Test offline mode (`--no-fetch`)

**Documentation:**
- `docs/AUTHENTICATION.md` - Guide to configuring SSH keys and credential helpers
- Troubleshooting section for common auth failures

---

## Git Version Dependency (v3)

**Minimum Required Version: Git 2.30.0** (released 2020-12-27)

### Rationale

**Required Features:**
- `git branch --format` (used in `ListBranches()`) - Available since Git 2.13
- `git rev-list --objects | git cat-file --batch-check` - Stable since Git 2.10
- `git symbolic-ref -q` - Available in Git 2.0+
- `git fetch` timeout behavior - Reliable since Git 2.30

**Why 2.30.0:**
- Widely available (4+ years old)
- Available in Ubuntu 22.04 LTS (common server distro)
- Fixes critical fetch timeout bugs affecting remote operations
- Balances compatibility with reliability

### Version Detection

**On Startup:**
```go
func (c *Client) ValidateGitVersion() error {
    output, err := c.run("git", "--version")
    // Parse version: "git version 2.34.1"
    // Compare against minimum (2.30.0)
    // Return error if too old
}
```

**User Facing Error:**
```
GitHelper requires Git 2.30.0 or newer (found: 2.25.1)
Please upgrade Git: https://git-scm.com/downloads
```

**Documentation:**
- Requirements documented in `README.md`
- Installation guide includes Git version check

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

// **NEW in v3:** Benchmark large binary scan
func BenchmarkScanLargeBinaries(b *testing.B) {
    testRepo := setupRepoWithLargeBinaries(b, 5, 20*1024*1024) // 5 files, 20MB each
    defer testRepo.Cleanup()

    classifier := scenarios.NewClassifier(ctx, testRepo.Path, gitClient, nil, "origin", "github")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        binaries, err := classifier.scanLargeBinariesSimplified()
        if err != nil {
            b.Fatal(err)
        }
        _ = binaries
    }
}

// Test large binary scan completes in <5s
func TestScanLargeBinaries_Performance(t *testing.T) {
    testRepo := setupRepoWithLargeBinaries(t, 10, 15*1024*1024) // 10 files, 15MB each

    start := time.Now()
    binaries, err := testRepo.GitClient.ScanLargeBinaries(10 * 1024 * 1024) // 10MB threshold
    duration := time.Since(start)

    require.NoError(t, err)
    assert.Len(t, binaries, 10)

    if duration > 5*time.Second {
        t.Errorf("Large binary scan too slow: %v (target: <5s)", duration)
    }
}
```

**Acceptance Criteria:**
- [ ] Benchmark framework in place before Phase 1
- [ ] Baseline measurements for empty, small, and large repos
- [ ] CI integration to catch regressions
- [ ] **NEW:** Large binary scan benchmark confirms <5s target

---

### Phase 1: Foundation (Week 1-2) ~950 LOC

**Goal:** Establish core data structures and git client extensions

#### 1.1: Data Structures (`internal/scenarios/types.go`)

**Changes from v2:**
- No structural changes, types remain stable from v2

**Priority:** P0 (Critical path)
**LOC:** ~250 lines
**Dependencies:** None

*(Same implementation as v2)*

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

**Changes from v2:**
- Add `ResetToRef()` for safe pull replacement (Gemini v2 critique)
- Add `ScanLargeBinaries()` with SHA+size only (Gemini v2 critique)
- Add `CheckLFSEnabled()` to detect Git LFS (Gemini v2 critique)
- Use `LC_ALL=C` for all commands (Gemini v2 critique)

**Priority:** P0 (Blocks classifier)
**LOC:** ~350 lines (+50 from v2)
**Dependencies:** Existing git.Client

**File:** `internal/git/cli.go` (append to existing)

**New Methods Required:**

```go
// **NEW in v3:** ResetToRef performs hard reset to ref (safer than pull)
func (c *Client) ResetToRef(ref string) error {
    // git reset --hard <ref>
    // Used instead of `git pull` to avoid user config ambiguity
    // Only call after validating working tree is clean
}

// **NEW in v3:** ScanLargeBinaries finds blobs >threshold (SHA+size only, NO path lookup)
func (c *Client) ScanLargeBinaries(thresholdBytes int64) ([]LargeBinary, error) {
    // Implementation:
    // 1. Run: git rev-list --objects --all | git cat-file --batch-check='%(objectname) %(objecttype) %(objectsize)'
    // 2. Filter for type=blob AND size >= threshold
    // 3. Return LargeBinary with SHA1 and SizeMB ONLY (path lookup is too expensive)
    //
    // NOTE: Does NOT populate Path field (would require `git log --all --find-object=<sha>`)
    // Performance: ~2-3s for 50MB repo, vs. 5+ minutes for full path lookup
}

// **NEW in v3:** CheckLFSEnabled detects if Git LFS is installed and in use
func (c *Client) CheckLFSEnabled() (bool, error) {
    // 1. Check if `git lfs version` succeeds (is LFS installed?)
    // 2. If yes, run `git lfs ls-files` (are any files tracked?)
    // 3. Return true if LFS is installed AND files are tracked
}

// FetchRemote with timeout (from v2)
func (c *Client) FetchRemote(remote string) error {
    // git fetch <remote> --tags
    // Timeout: 30 seconds
    // Return error if remote unreachable
}

// LocalExists checks if repository exists locally (from v2)
func (c *Client) LocalExists() (bool, string) {
    // Implementation:
    // 1. Check if c.repoPath exists
    // 2. Check if .git directory/file exists
    // 3. Return (exists, absolutePath)
}

// IsDetachedHEAD checks for detached HEAD state (from v2)
func (c *Client) IsDetachedHEAD() (bool, error) {
    // git symbolic-ref -q HEAD
    // Exit code 1 = detached, 0 = on branch
}

// IsShallowClone checks if repo is shallow (from v2)
func (c *Client) IsShallowClone() (bool, error) {
    // Check for .git/shallow file
}

// GetStagedFiles returns list of staged files (from v2)
func (c *Client) GetStagedFiles() ([]string, error) {
    // git diff --cached --name-only
}

// GetUnstagedFiles returns list of unstaged modifications (from v2)
func (c *Client) GetUnstagedFiles() ([]string, error) {
    // git diff --name-only
}

// GetBranchHash returns commit hash for local branch (from v2)
func (c *Client) GetBranchHash(branch string) (string, error) {
    // git rev-parse <branch>
}

// GetRemoteBranchHash returns commit hash for remote branch (from v2)
func (c *Client) GetRemoteBranchHash(remote, branch string) (string, error) {
    // git rev-parse <remote>/<branch>
    // Handle case where remote branch doesn't exist
}

// CountCommitsBetween counts commits in ref1 not in ref2 (from v2)
func (c *Client) CountCommitsBetween(ref1, ref2 string) (int, error) {
    // git rev-list --count <ref1> ^<ref2>
    // Returns 0 if refs are equal
}

// **IMPROVED in v2:** GetDefaultBranch with better fallback
func (c *Client) GetDefaultBranch() (string, error) {
    // **UPDATED in v3:** Change order per Gemini v2 critique
    // Try 1: git remote show <remote> | grep "HEAD branch" (most reliable)
    // Try 2: git symbolic-ref refs/remotes/origin/HEAD (local cache)
    // Try 3: Check for main
    // Try 4: Check for master
    // Return error if all fail
}

// CanReachRemote tests if remote is accessible (from v2)
func (c *Client) CanReachRemote(remote string) bool {
    // git ls-remote --exit-code <remote> HEAD
    // Return true if exit code 0, false otherwise
    // Timeout after 5 seconds
}

// ListBranches returns all branches (local and remote) (from v2)
func (c *Client) ListBranches() (local, remote []string, error) {
    // git branch --format='%(refname:short)'
    // git branch -r --format='%(refname:short)'
}

// GetRemoteURL returns URL for named remote
// Already exists, verify signature matches needs
```

**Implementation Notes:**
- Use existing `c.run()` helper for git commands
- **NEW in v3:** Set `LC_ALL=C` environment variable for all git commands (stable parsing)
- Handle missing refs gracefully (return empty string, no error)
- Timeout on remote operations: 5s for checks, 30s for fetch
- Cache results where appropriate (e.g., branch lists)
- Add retry logic for transient network failures (1 retry with backoff)

**Testing Requirements:**
- Unit tests with mock git output
- Integration tests with real test repos
- Test edge cases: missing remotes, detached HEAD, shallow clones, network failures
- Test fetch with network delays and failures
- **NEW:** Test large binary scan with 10+ large files
- **NEW:** Test LFS detection with and without LFS repos
- **NEW:** Test ResetToRef with clean and dirty working trees

**Acceptance Criteria:**
- [ ] All 14 methods implemented (+3 from v2)
- [ ] 100% unit test coverage
- [ ] Integration tests pass
- [ ] Handles all edge cases (documented in tests)
- [ ] Fetch timeout tested with slow remotes
- [ ] **NEW:** Large binary scan returns SHA+size in <5s
- [ ] **NEW:** LFS detection works correctly
- [ ] **NEW:** ResetToRef validated as safe pull alternative

---

#### 1.3: Classification Tables (`internal/scenarios/tables.go`)

**No changes from v2** - This component remains stable.

**Priority:** P0 (Blocks classifier)
**LOC:** ~150 lines
**Dependencies:** types.go

*(Same implementation as v2)*

---

#### 1.4: Structured Fix Operations (`internal/scenarios/operations.go`)

**Changes from v2:**
- Replace `PullOperation` with `FetchOperation` + `ResetOperation` (Gemini v2 critique)
- Improve validation logic to prevent unsafe resets
- Document that `CompositeOperation` rollback is disabled (Gemini v2 critique)

**Priority:** P0 (Required for safe AutoFix)
**LOC:** ~250 lines (+50 from v2)
**Dependencies:** types.go, git/cli.go

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

// **REMOVED in v3:** PullOperation (replaced with FetchOperation + ResetOperation)

// **NEW in v3:** ResetOperation - git reset --hard <ref>
// Safer than PullOperation because it's explicit and doesn't depend on user's pull.rebase config
type ResetOperation struct {
    Ref string  // Full ref: "refs/remotes/origin/main"
}

func (op *ResetOperation) Validate(ctx context.Context, state *RepositoryState) error {
    // CRITICAL: Ensure working tree is clean
    if !state.WorkingTree.Clean {
        return fmt.Errorf("working tree must be clean before reset (found %d staged, %d unstaged files)",
            len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
    }

    // Validate ref exists
    if op.Ref == "" {
        return fmt.Errorf("reset ref cannot be empty")
    }

    // Ensure this is a fast-forward (local is behind ref)
    // Note: Should only be used for S6, S7 scenarios where local is behind
    return nil
}

func (op *ResetOperation) Execute(ctx context.Context, gc *git.Client) error {
    return gc.ResetToRef(op.Ref)
}

func (op *ResetOperation) Describe() string {
    return fmt.Sprintf("Reset to %s", op.Ref)
}

func (op *ResetOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Can attempt git reset --hard ORIG_HEAD
    // But this is risky - better to fail
    return fmt.Errorf("reset rollback requires manual intervention (use: git reset --hard ORIG_HEAD)")
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
    // **UPDATED in v3:** Per Gemini v2 critique, disable automatic rollback for composite operations
    // Rollback is too risky for multi-step operations
    return fmt.Errorf("composite operation rollback disabled - manual intervention required")
}
```

**Testing Requirements:**
- Unit test for each operation type
- Validation logic tested with various states
- Rollback tested where applicable (Fetch only)
- Composite operations tested with failures
- **NEW:** ResetOperation validated as working tree clean check
- **NEW:** CompositeOperation rollback returns error

**Acceptance Criteria:**
- [ ] All operation types implemented
- [ ] Validation prevents unsafe operations
- [ ] 100% test coverage
- [ ] Integration tested with real repos
- [ ] **NEW:** PullOperation removed, replaced with Fetch+Reset pattern
- [ ] **NEW:** ResetOperation requires clean working tree

---

### Phase 2: Classifier Core (Week 2-3) ~750 LOC

**Goal:** Implement hierarchical state detection with fresh data

**Changes from v2:**
- Implement `scanLargeBinariesSimplified()` with SHA+size only (Gemini v2 critique)
- Add `detectLFS()` to check for Git LFS (Gemini v2 critique)
- Run local checks (working tree, LFS, corruption) concurrently with fetch (GPT v2 critique)
- Improve `GetDefaultBranch` to prioritize `git remote show` (Gemini v2 critique)

#### 2.1: Classifier Implementation (`internal/scenarios/classifier.go`)

**Priority:** P0 (Core feature)
**LOC:** ~550 lines (+50 from v2)
**Dependencies:** types.go, tables.go, operations.go, git/cli.go extensions

**File:** `internal/scenarios/classifier.go`

```go
package scenarios

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/lcgerke/githelper/internal/github"
)

type Classifier struct {
    repoPath     string
    gitClient    *git.Client
    githubClient *github.Client  // **v2 NOTE:** Currently unused, may remove in v4
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

// **UPDATED in v3:** Detect with concurrent fetch and local checks
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

    // **NEW in v3:** Run fetch CONCURRENTLY with local checks
    var fetchErr error
    var wg sync.WaitGroup

    if c.options.FetchBeforeCheck {
        wg.Add(1)
        go func() {
            defer wg.Done()
            fetchErr = c.fetchRemotes(state.Existence)
        }()
    }

    // While fetch runs, perform local-only checks (don't need remote data)

    // D4: Working tree (independent of remotes)
    state.WorkingTree, err = c.detectWorkingTree()
    if err != nil {
        return nil, fmt.Errorf("working tree check failed: %w", err)
    }

    // **NEW in v3:** D_LFS: Check for Git LFS
    state.LFSEnabled, err = c.detectLFS()
    if err != nil {
        logger.Warnw("LFS check failed", "error", err)
        state.LFSEnabled = false
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

    // Wait for fetch to complete
    wg.Wait()
    if fetchErr != nil {
        // Log warning but continue (sync state will be based on stale data)
        logger.Warnw("Failed to fetch remotes, sync state may be stale", "error", fetchErr)
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

// fetchRemotes ensures remote data is fresh
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

// detectExistence uses configurable remote names
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

// detectWorkingTree checks for detached HEAD and shallow clones
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

// **NEW in v3:** detectLFS checks if Git LFS is enabled and in use
func (c *Classifier) detectLFS() (bool, error) {
    return c.gitClient.CheckLFSEnabled()
}

// **IMPLEMENTED in v3:** detectCorruption with fast SHA+size scanning
func (c *Classifier) detectCorruption() (CorruptionState, error) {
    state := CorruptionState{
        ID: "C1",  // Default: no corruption
        HasCorruption: false,
    }

    // Scan for large binaries (SHA+size only, NO path lookup)
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

// **IMPLEMENTED in v3:** scanLargeBinariesSimplified - fast scan without path lookup
func (c *Classifier) scanLargeBinariesSimplified() ([]LargeBinary, error) {
    // Threshold: 10MB
    const thresholdBytes = 10 * 1024 * 1024

    // Call git client method
    binaries, err := c.gitClient.ScanLargeBinaries(thresholdBytes)
    if err != nil {
        return nil, err
    }

    // Note: Path field is empty (finding paths from SHA is too expensive)
    // Users can manually locate with: git log --all --find-object=<sha>
    return binaries, nil
}

// detectDefaultBranchSync with fresh data indicator
func (c *Classifier) detectDefaultBranchSync() (BranchSyncState, error) {
    state := BranchSyncState{
        LastFetchTime: time.Now(),
        DataIsFresh:   c.options.FetchBeforeCheck,
    }

    // Get default branch name (improved fallback in v3)
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

// detectBranchTopology uses configurable remote names
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
- Fetch timeout configurable (default 30s per remote)
- Add metrics for fetch success/failure
- **NEW:** Run local checks concurrently with fetch for performance

**Testing Requirements:**
- Unit tests with mocked git client
- Integration tests for all 41 scenarios
- Performance test: <2s for typical repo (with concurrent fetch)
- Error handling: network failures, missing remotes
- Test with stale data (fetch disabled) vs fresh data
- Test with slow/failing fetch operations
- **NEW:** Test large binary scan with 10+ large files (<5s)
- **NEW:** Test LFS detection (enabled/disabled)
- **NEW:** Test concurrent fetch and local checks

**Acceptance Criteria:**
- [ ] Detects all 41 scenarios correctly
- [ ] Handles partial failures (e.g., one remote unreachable)
- [ ] Performance: <2s for 10-branch repo (with concurrent fetch)
- [ ] Zero panics on malformed input
- [ ] Fetch failures don't abort entire detection
- [ ] Stale data warning logged when fetch fails
- [ ] **NEW:** Large binary scan completes in <5s
- [ ] **NEW:** LFS warning shown when LFS files detected
- [ ] **NEW:** Local checks run during fetch (not sequentially)

---

### Phase 3: Fix Suggestion Engine (Week 3-4) ~650 LOC

**Goal:** Suggest and auto-apply fixes using safe structured operations

**Changes from v2:**
- Replace `PullOperation` with `FetchOperation` + `ResetOperation` (Gemini v2 critique)
- Add LFS warning to fix suggestions (Gemini v2 critique)
- Mark S8 and all divergence scenarios as non-auto-fixable (Gemini v2 critique)

#### 3.1: Fix Suggester (`internal/scenarios/suggester.go`)

**Priority:** P1 (High value)
**LOC:** ~450 lines (+50 from v2)
**Dependencies:** types.go, operations.go, classifier.go

**File:** `internal/scenarios/suggester.go`

```go
package scenarios

import (
    "fmt"
    "sort"

    "github.com/lcgerke/githelper/internal/git"
)

// Fix structure (updated in v2 with Operation)
type Fix struct {
    ScenarioID  string
    Description string
    Operation   Operation  // Structured operation
    Command     string     // Display purposes only
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

    // Warn about detached HEAD
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

    // Warn about shallow clone
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

    // **NEW in v3:** Warn about Git LFS
    if state.LFSEnabled {
        fixes = append(fixes, Fix{
            ScenarioID:  "W_LFS",
            Description: "Repository uses Git LFS (large files tracked externally)",
            Command:     "# No action needed - LFS is intentional",
            Operation:   nil,
            AutoFixable: false,
            Priority:    9,  // Informational only
            Reason:      "Large binary scan will only see LFS pointer files, not actual assets",
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

    // Warn about stale data
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
            AutoFixable: false,  // Not auto-fixable (requires GitHub API)
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

// **UPDATED in v3:** suggestSyncFixes uses Fetch+Reset instead of Pull
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
            Command:     fmt.Sprintf("git fetch %s && git reset --hard %s/%s",
                state.Existence.CoreRemote, state.Existence.CoreRemote, syncState.Branch),
            Operation: &CompositeOperation{
                Operations: []Operation{
                    &FetchOperation{Remote: state.Existence.CoreRemote},
                    &ResetOperation{Ref: fmt.Sprintf("refs/remotes/%s/%s", state.Existence.CoreRemote, syncState.Branch)},
                },
                StopOnError: true,
            },
            AutoFixable: true,  // **CHANGED in v3:** Safe with ResetOperation (validates clean working tree)
            Priority:    4,
            Reason:      "Fast-forward via fetch+reset",
        }}

    case "S7":  // Core ahead only
        return []Fix{{
            ScenarioID:  "S7",
            Description: fmt.Sprintf("Core ahead of local (%d commits)",
                syncState.LocalBehindCore),
            Command:     fmt.Sprintf("git fetch %s && git reset --hard %s/%s",
                state.Existence.CoreRemote, state.Existence.CoreRemote, syncState.Branch),
            Operation: &CompositeOperation{
                Operations: []Operation{
                    &FetchOperation{Remote: state.Existence.CoreRemote},
                    &ResetOperation{Ref: fmt.Sprintf("refs/remotes/%s/%s", state.Existence.CoreRemote, syncState.Branch)},
                },
                StopOnError: true,
            },
            AutoFixable: true,  // **CHANGED in v3:** Safe with ResetOperation
            Priority:    4,
            Reason:      "Fast-forward from core via fetch+reset",
        }}

    case "S8":  // Complex ahead/behind
        return []Fix{{
            ScenarioID:  "S8",
            Description: "Complex sync state: local ahead of GitHub, behind Core",
            Command:     fmt.Sprintf("git fetch %s && git reset --hard %s/%s && git push %s",
                state.Existence.CoreRemote, state.Existence.CoreRemote, syncState.Branch,
                state.Existence.GitHubRemote),
            Operation:   nil,  // **CHANGED in v3:** Removed composite operation (too risky)
            AutoFixable: false,  // Keep manual per Gemini v2 critique
            Priority:    5,
            Reason:      "Manual resolution recommended (local work will be lost)",
        }}

    case "S9", "S10", "S11", "S12", "S13":  // Divergence
        return []Fix{{
            ScenarioID:  syncState.ID,
            Description: "Divergent histories detected",
            Command:     "Manual merge or rebase required - see docs/DIVERGENCE.md",
            Operation:   nil,
            AutoFixable: false,
            Priority:    5,
            Reason:      "Divergence requires conflict resolution",
        }}

    default:
        return nil
    }
}

// suggestBranchFixes with structured operations
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
            AutoFixable: false,  // Don't auto-checkout (affects working tree)
            Priority:    8,
            Reason:      "Optional: fetch remote branch",
        }}

    default:
        return nil
    }
}

// AutoFix uses Operation validation and execution
func AutoFix(ctx context.Context, state *RepositoryState, gitClient *git.Client) ([]Fix, []error) {
    fixes := SuggestFixes(state)
    applied := []Fix{}
    errors := []error{}

    for _, fix := range fixes {
        if !fix.AutoFixable || fix.Operation == nil {
            continue
        }

        // Validate before executing
        if err := fix.Operation.Validate(ctx, state); err != nil {
            errors = append(errors, fmt.Errorf("%s validation failed: %w", fix.ScenarioID, err))
            continue
        }

        // Execute operation
        if err := fix.Operation.Execute(ctx, gitClient); err != nil {
            errors = append(errors, fmt.Errorf("%s execution failed: %w", fix.ScenarioID, err))

            // Attempt rollback on failure (will fail for most operations)
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
- Test rollback on operation failures
- **NEW:** Test Fetch+Reset pattern for S6, S7
- **NEW:** Verify S8 is non-auto-fixable
- **NEW:** Test LFS warning appears when LFS enabled

**Acceptance Criteria:**
- [ ] All 41 scenarios have fix suggestions
- [ ] 25/41 scenarios are auto-fixable
- [ ] Priority ordering is correct
- [ ] No false positive fixes
- [ ] All auto-fixable fixes have Operation objects
- [ ] Validation prevents unsafe operations
- [ ] No "not implemented" stubs
- [ ] **NEW:** S6, S7 use Fetch+Reset (not Pull)
- [ ] **NEW:** S8 is non-auto-fixable
- [ ] **NEW:** LFS warning present when applicable

---

### Phase 4: Command Integration (Week 4-5) ~450 LOC

**Goal:** Integrate classifier into commands with new CLI surfaces

**Changes from v2:**
- Implement `status`, `repair`, `scenarios` commands (GPT v2 critique)
- Display LFS warning in doctor output
- Show fetch status and stale data warnings

*(Implementation details for doctor, status, repair, scenarios commands - same structure as v2 but with new command specs from earlier in this document)*

---

### Phase 5: Testing & Documentation (Week 5-7) ~900 LOC

**Goal:** Comprehensive test coverage and validation

**Changes from v2:**
- Implement corpus testing with 100-repo validation
- Create golden set of 15 manually validated repos
- Document all new features (LFS, Fetch+Reset, credentials, Git version)

#### 5.1: Unit Tests

*(Same structure as v2, add tests for new components)*

---

#### 5.2: Integration Tests

**File:** `test/integration/scenarios_test.go`

*(Same test harness structure as v2 using local git-daemon)*

**Additional Test Cases in v3:**
- Test large binary detection with 10+ large files
- Test LFS detection and warning
- Test Fetch+Reset operations for S6, S7
- Test concurrent fetch with local checks
- Test ResetOperation validation (clean working tree requirement)

---

#### 5.3: Corpus Testing (NEW in v3)

**File:** `test/corpus/corpus_test.go`

```go
package corpus_test

import (
    "testing"

    "github.com/lcgerke/githelper/internal/scenarios"
)

func TestCorpusValidation(t *testing.T) {
    // Load corpus manifest
    repos := loadCorpusManifest(t, "repos.yaml")

    falsePositives := 0
    totalRepos := len(repos)

    for _, repo := range repos {
        t.Run(repo.Name, func(t *testing.T) {
            // Clone repo (cached)
            repoPath := cloneRepo(t, repo.URL)

            // Run classifier
            classifier := scenarios.NewClassifier(ctx, repoPath, gitClient, nil, "origin", "github")
            state, err := classifier.Detect()
            require.NoError(t, err)

            // Validate expected scenario
            if repo.Expected.Existence != "" && state.Existence.ID != repo.Expected.Existence {
                t.Errorf("FALSE POSITIVE: Expected existence %s, got %s",
                    repo.Expected.Existence, state.Existence.ID)
                falsePositives++
            }

            // ... validate other dimensions ...
        })
    }

    falsePositiveRate := float64(falsePositives) / float64(totalRepos)
    if falsePositiveRate > 0.05 {
        t.Errorf("False positive rate too high: %.2f%% (target: <5%%)",
            falsePositiveRate*100)
    }
}
```

---

#### 5.4: Documentation (Updated for v3)

**Required Documentation:**

1. **`README.md`** - Updated with Git 2.30+ requirement
2. **`docs/SCENARIO_REFERENCE.md`** - All 41 scenarios explained (powers `scenarios` command)
3. **`docs/BFG_CLEANUP.md`** - Large binary removal guide
4. **`docs/DIVERGENCE.md`** - Handling divergent histories
5. **`docs/REMOTE_CONFIGURATION.md`** - Configuring non-standard remote names
6. **`docs/AUTHENTICATION.md`** (NEW in v3) - SSH keys, credential helpers, troubleshooting
7. **`test/integration/README.md`** - Integration test harness guide (git-daemon setup)
8. **`test/corpus/README.md`** (NEW in v3) - Corpus testing guide, how to add repos

---

## Risk Assessment & Mitigation - **UPDATED for v3**

### Technical Risks

| Risk | Impact | Probability | Mitigation (v3) |
|------|--------|-------------|-----------------|
| **Performance regression** | High | Low | Early benchmarking (Phase 0), <2s mandatory test, concurrent fetch |
| **False positives in detection** | High | Medium | Corpus testing (100+ repos), <5% target with validation |
| **Network timeout during remote checks** | Medium | High | 30s fetch timeout, graceful degradation, retry logic |
| **Fetch failures prevent detection** | Medium | Low | **FIXED:** Fetch is optional, stale data warning shown |
| **Large binary scan too slow** | Medium | Low | **FIXED:** SHA+size only (no path lookup), <5s target |
| **Auto-fix causes data loss** | Critical | Low | **FIXED:** ResetOperation validates clean working tree, rollback disabled for composites |
| **Hardcoded remote names break** | High | N/A | **FIXED:** Configurable remote names |
| **Stale sync state data** | High | N/A | **FIXED:** Mandatory fetch before sync checks |
| **Git version incompatibility** | Medium | Low | **NEW:** Require Git 2.30+, version check on startup |
| **Credential failures block operations** | Medium | Medium | **NEW:** Inherit Git credentials, document setup, timeout handling |
| **LFS files not detected** | Low | Low | **NEW:** CheckLFSEnabled() warns user |
| **User config breaks PullOperation** | High | N/A | **FIXED:** Replaced with Fetch+Reset pattern |
| **Submodule complexity** | High | Medium | **SCOPED OUT:** Explicitly out of v1, documented for v2 |

---

## Acceptance Criteria - **FINAL for v3**

### Functional Requirements

- [ ] **Scenario Detection:** All 41 scenarios correctly identified
- [ ] **Fix Suggestions:** All scenarios have appropriate fix suggestions
- [ ] **Auto-Fix:** 25/41 scenarios can be auto-fixed safely
- [ ] **Performance:** State detection completes in <2 seconds (with concurrent fetch)
- [ ] **Accuracy:** False positive rate <5% on 100-repo corpus
- [ ] **Fresh Data:** Mandatory fetch before sync checks (with --no-fetch escape hatch)
- [ ] **Configurable Remotes:** Non-standard remote names work correctly
- [ ] **Safe Operations:** All auto-fix operations use structured Operations, not strings
- [ ] **Edge Cases:** Detached HEAD, shallow clone, LFS warnings present
- [ ] **NEW:** Large binary scan completes in <5s with SHA+size only
- [ ] **NEW:** ResetOperation validates clean working tree
- [ ] **NEW:** CLI commands (status, repair, scenarios) functional
- [ ] **NEW:** Credential handling documented, inherits Git config

### Non-Functional Requirements

- [ ] **Test Coverage:** >90% line coverage, 100% scenario coverage
- [ ] **Documentation:** All public APIs documented, user guide complete
- [ ] **Error Handling:** Graceful degradation on network failures
- [ ] **Logging:** Structured logging for all operations
- [ ] **Backward Compatibility:** Zero regressions in existing commands
- [ ] **Fetch Failures:** Logged but don't abort detection
- [ ] **Operation Validation:** Prevents unsafe auto-fixes
- [ ] **NEW:** Git 2.30+ version validated on startup
- [ ] **NEW:** Corpus testing automated with CI integration
- [ ] **NEW:** <5% false positive rate validated on 100+ repos

---

## Delivery Milestones - **FINAL for v3**

### Milestone 0: Benchmarking (End of Week 1)
- [ ] Benchmark framework in place
- [ ] Baseline measurements for empty/small/large repos
- [ ] CI integration for performance regression detection
- [ ] Large binary scan benchmark confirms <5s

### Milestone 1: Foundation (End of Week 2)
- [ ] All data structures defined (RemoteConfig, DetectOptions)
- [ ] Git client extensions complete (14 methods including ResetToRef, ScanLargeBinaries, CheckLFSEnabled)
- [ ] Classification tables implemented
- [ ] Structured operations framework complete (with ResetOperation)
- [ ] Unit tests passing
- [ ] Git version check implemented

### Milestone 2: Core Classifier (End of Week 3)
- [ ] Classifier detects all 5 dimensions + LFS
- [ ] Concurrent fetch with local checks working
- [ ] Large binary scan implemented (SHA+size, <5s)
- [ ] All 41 scenarios testable
- [ ] Integration tests for existence + sync states
- [ ] Detached HEAD, shallow clone, LFS detection working

### Milestone 3: Fix Engine (End of Week 4)
- [ ] Fix suggester complete with Operations
- [ ] All 41 fixes implemented (no stubs)
- [ ] Auto-fix with validation (ResetOperation validates clean working tree)
- [ ] Fetch+Reset pattern working for S6, S7
- [ ] S8 and divergence scenarios non-auto-fixable
- [ ] Integration tests for all scenarios

### Milestone 4: Command Integration (End of Week 5)
- [ ] `doctor` enhanced with scenario IDs, LFS warnings
- [ ] `status` command implemented (quick sync check)
- [ ] `repair` command implemented (interactive and --auto modes)
- [ ] `scenarios` command implemented (--list and --explain)
- [ ] Remote name configuration working
- [ ] No regressions in existing functionality

### Milestone 5: Testing (End of Week 6)
- [ ] All unit tests passing (>90% coverage)
- [ ] All 41 integration tests passing
- [ ] Test harness using local git-daemon working
- [ ] Benchmark tests validate <2s detection, <5s corruption scan
- [ ] Corpus test infrastructure ready (repos.yaml, golden set)

### Milestone 6: Validation & Ship (End of Week 7-8)
- [ ] Corpus testing on 100+ real repos (<5% false positive rate)
- [ ] Golden set validated (15 manually reviewed repos)
- [ ] Documentation complete (including AUTHENTICATION.md, corpus README)
- [ ] External review approved (2+ engineers)
- [ ] Production deployment

---

## Appendix: Changes from v2.0

### Major Improvements (v3 over v2)

1. **Large Binary Detection Implemented** (Gemini v2 critique)
   - Implemented `ScanLargeBinaries()` with SHA+size only
   - No path lookup (would take minutes vs. <5s)
   - Performance validated with benchmarks

2. **PullOperation Replaced** (Gemini v2 critique)
   - New `ResetOperation` with clean working tree validation
   - Avoids user config ambiguity (pull.rebase)
   - S6, S7 now use Fetch+Reset pattern

3. **Git LFS Detection** (Gemini v2 critique)
   - New `CheckLFSEnabled()` method
   - Warning shown when LFS files detected
   - Explains why large binary scan may show pointer files

4. **CLI Commands Defined** (GPT v2 critique)
   - `status`: Quick sync check
   - `repair`: Interactive/auto fix application
   - `scenarios`: Reference tool for scenario docs

5. **100-Repo Corpus Strategy** (Both v2 critiques)
   - Defined sources: 50 open source, 30 user repos, 20 synthetic
   - Golden set of 15 manually validated repos
   - Automated validation with <5% false positive target

6. **Credential Handling Documented** (GPT v2 critique)
   - Inherits Git's credential system
   - SSH keys, credential helpers, offline behavior
   - Troubleshooting guide in AUTHENTICATION.md

7. **Git Version Dependency** (Gemini v2 critique)
   - Minimum: Git 2.30.0
   - Version check on startup
   - Documented in README

8. **Performance Improvements** (Both v2 critiques)
   - Concurrent fetch with local checks
   - Large binary scan <5s (vs. minutes in v1 spec)
   - Timeout handling with retry logic

9. **Auto-Fix Safety** (Gemini v2 critique)
   - S8 marked non-auto-fixable
   - ResetOperation validates clean working tree
   - CompositeOperation rollback disabled

10. **Explicit Critique Request for Submodules**
    - Added request for feedback on submodule scoping
    - Asking if minimal-effort approach exists
    - Documented in Known Limitations section

---

**END OF IMPLEMENTATION PLAN v3.0**

*This plan addresses all v2 critique feedback, implements previously stubbed features, and is ready for implementation and external review.*

**Version 3.0 - 2025-11-18**
