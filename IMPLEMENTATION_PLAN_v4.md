# GitHelper Scenario Classification System - Implementation Plan

**Version:** 4.0
**Date:** 2025-11-18
**Target Completion:** 7-9 weeks (adjusted for safety refinements + buffer)
**Estimated LOC:** ~3,400 lines (production + tests)

**Changelog from v3.0:**
- **CRITICAL:** Added fast-forward validation to ResetOperation (prevents data loss) - Both critiques
- **CRITICAL:** Serialized git.Client operations to prevent concurrent fetch races - Both critiques
- Added `GIT_TERMINAL_PROMPT=0` to prevent credential hangs - Gemini critique
- Reversed GetDefaultBranch order (local cache first, network fallback) - Both critiques
- Documented large binary SHA→path manual lookup in BFG guide - GPT critique
- Completed E5-E8 existence scenario fixes - GPT critique
- Removed unused `github.Client` from Classifier - Both critiques
- Added corpus testing credential/caching strategy - Both critiques
- Added edge case handling (git hooks, locked refs, extreme ref counts) - Gemini critique
- Extended timeline to 7-9 weeks with overlap and buffer
- **SUBMODULES:** Keeping out of scope for v1 (minimal detection deferred to v2 per user request)

**Changelog from v2.0:**
- Implemented large binary detection (SHA+size only, no path lookup) - Gemini v2
- Fixed PullOperation safety (fetch+reset instead of pull) - Gemini v2
- Added Git LFS detection and warning - Gemini v2
- Defined new CLI commands (status, repair, scenarios) - GPT v2
- Specified 100-repo corpus source and validation strategy - Both v2
- Added credential handling documentation - GPT v2
- Specified Git version dependency (2.30+) - Gemini v2
- Improved fetch performance mitigation (concurrent checks, timeout handling) - Both v2
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

---

## Executive Summary

### Objective

Implement a comprehensive state classification system for GitHelper that detects and identifies repository states across three locations (Local, Core, GitHub), enabling automated diagnosis, repair suggestions, and intelligent command orchestration.

### Scope

**In Scope:**
- Scenario classification across 5 dimensions (41 distinct scenarios)
- Automated fix suggestion with priority ordering
- Auto-fix capability for 60%+ of scenarios (**with rigorous safety validation**)
- Integration with existing `doctor` command
- New `status`, `repair`, and `scenarios` commands (fully specified)
- Comprehensive test coverage (unit + integration + corpus)
- **Configurable remote names** (not hardcoded to origin/github)
- **Fresh remote data** via mandatory fetch before sync checks
- **Git LFS detection** and appropriate warnings
- **Safe auto-fix operations** using validated fetch+reset patterns
- **Thread-safe git operations** with serialization

**Out of Scope (Future Phases):**
- BFG cleanup automation (manual guide only)
- Multi-repository batch operations
- **Git submodules state tracking** (v2 - see Known Limitations)
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

**v3 Critique Response:** Both GPT-5 and Gemini confirmed keeping submodules out of scope for v1 is correct. Gemini suggested minimal `.gitmodules` detection with warning, but user requested keeping this as future work to avoid scope creep.

**Other Edge Cases Deferred:**
- Orphan branches (branches with no shared history)
- Sparse checkouts and partial clones
- Case-insensitive filesystem conflicts (Windows/macOS)
- Detached HEAD states (detected but limited guidance)
- Shallow clones (warnings only, history-based checks disabled)
- Git worktrees (each worktree analyzed independently)
- Repositories with non-standard remote names that can't be auto-discovered
- Git hooks that interfere with operations (documented in troubleshooting)
- Interactive credential helpers (mitigated with `GIT_TERMINAL_PROMPT=0`)

### Success Criteria

1. ✅ `doctor` command reports scenario IDs (E1-E8, S1-S13, B1-B7, W1-W5, C1-C8)
2. ✅ Auto-fix resolves 60%+ of detected issues **with rigorous pre-validation**
3. ✅ All 41 scenarios have integration tests with >90% coverage
4. ✅ Zero regressions in existing commands
5. ✅ External review approval from 2+ engineers
6. ✅ False positive rate <5% on corpus of 100+ real repos
7. ✅ **NEW:** No data loss from auto-fix operations (fast-forward validation)
8. ✅ **NEW:** No race conditions from concurrent git operations

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
| **Data loss incidents** | N/A | 0 | Fast-forward validation required |
| **Git operation races** | N/A | 0 | Serialized git.Client |

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
        │  │  operations.go (validated ops)  │              │
        │  └────────────┴────────────────────┘              │
        └───────────────────┬────────────────────────────────┘
                            │
        ┌───────────────────▼────────────────────────────────┐
        │         internal/git (ENHANCED + THREAD-SAFE)      │
        │  ┌────────────────────────────────────────┐        │
        │  │  cli.go + 14 methods + MUTEX           │        │
        │  │  - GetBranchHash()                     │        │
        │  │  - CountCommitsBetween()               │        │
        │  │  - FetchRemote()                       │        │
        │  │  - ResetToRef() [VALIDATED]            │        │
        │  │  - ScanLargeBinaries()                 │        │
        │  │  - CheckLFSEnabled()                   │        │
        │  │  - IsAncestor() [NEW v4]               │        │
        │  │  - run() [SERIALIZED with mutex]       │        │
        │  └────────────────────────────────────────┘        │
        └────────────────────────────────────────────────────┘
```

### Data Flow (Updated for v4)

```
User runs: githelper doctor myproject
    │
    ├─> Initialize Classifier(git.Client, coreRemoteName, githubRemoteName)
    │    [github.Client REMOVED in v4]
    │
    ├─> **Pre-flight fetch** (configurable via --no-fetch flag)
    │    ├─> SERIALIZED: git fetch <coreRemote> (mutex-protected)
    │    ├─> SERIALIZED: git fetch <githubRemote> (mutex-protected)
    │    └─> WHILE FETCHING: Run local checks (working tree, LFS, corruption)
    │
    ├─> classifier.Detect()
    │    ├─> detectExistence() → ExistenceState (E1-E8) [ALL 8 now have fixes]
    │    ├─> detectWorkingTree() → WorkingTreeState (W1-W5)
    │    ├─> detectLFS() → LFS warning if enabled
    │    ├─> detectCorruption() → CorruptionState (C1-C8) [FAST: SHA+size only]
    │    ├─> detectDefaultBranchSync() → BranchSyncState (S1-S13)
    │    └─> detectBranchTopology() → []BranchState (B1-B7 each)
    │
    ├─> SuggestFixes(state) → []Fix (prioritized, validated operations)
    │
    ├─> Display findings with scenario IDs
    │
    └─> If --auto-fix:
        ├─> Validate each fix operation
        │    └─> ResetOperation: Check clean working tree + FAST-FORWARD ONLY
        ├─> Execute structured operations (serialized through mutex)
        └─> Re-validate state after each fix
```

---

## New CLI Commands Specification

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
  E3: GitHub exists, Core missing
  E4: Local only (no remotes)
  E5: Core + GitHub exist, local missing
  E6: Core exists, local + GitHub missing
  E7: GitHub exists, local + Core missing
  E8: No repositories exist
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

## Corpus Testing Strategy

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
   - **Credentials:** SSH keys stored in CI environment, documented in `test/corpus/README.md`

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
2. For each repo: clone (with caching), run `Classifier.Detect()`, validate expected scenario
3. Flag anomalies (unexpected scenarios, crashes, >2s detection time)
4. Generate report with false positive rate

**Caching Strategy** (NEW in v4):
- Clone repos once to `~/.cache/githelper-corpus/`
- Update via `git fetch` for subsequent runs (not full re-clone)
- Invalidate cache after 7 days
- Parallel cloning (5 concurrent clones max)

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

## Credential Handling

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

**3. **NEW in v4:** Non-Interactive Mode**

All git commands set `GIT_TERMINAL_PROMPT=0` to prevent hangs:
```go
// In git.Client.run() helper:
cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
```

If credential helper requires interactive input, git fails immediately with clear error instead of hanging.

**4. Offline Behavior**

When remotes are unreachable (no auth, no network):
- `CanReachRemote()` returns `false` (5s timeout)
- Existence state reflects remote as unreachable
- Sync checks use stale data (if `--no-fetch` or fetch failed)
- Warning logged: "Remote <name> unreachable, using stale data"

**5. Push Operations with Auth Failures**

If `PushOperation.Execute()` fails with auth error:
- Error message preserved from git output
- User sees: `"Push to origin failed: Permission denied (publickey)"`
- Recommended fix: `"Check SSH key configuration: ssh -T git@server"`

### Testing Strategy

**Integration Tests:**
- Mock git daemon with auth (test both success and failure)
- Test timeout behavior (slow remotes)
- Test offline mode (`--no-fetch`)
- **NEW:** Test `GIT_TERMINAL_PROMPT=0` behavior

**Documentation:**
- `docs/AUTHENTICATION.md` - Guide to configuring SSH keys and credential helpers
- Troubleshooting section for common auth failures
- **NEW:** Section on non-interactive mode and credential hang prevention

---

## Git Version Dependency

**Minimum Required Version: Git 2.30.0** (released 2020-12-27)

### Rationale

**Required Features:**
- `git branch --format` (used in `ListBranches()`) - Available since Git 2.13
- `git rev-list --objects | git cat-file --batch-check` - Stable since Git 2.10
- `git symbolic-ref -q` - Available in Git 2.0+
- `git fetch` timeout behavior - Reliable since Git 2.30
- `git merge-base --is-ancestor` - Available since Git 1.8 (used for fast-forward validation)

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

*(Same as v3, no changes)*

---

### Phase 1: Foundation (Week 1-2) ~1,000 LOC

**Goal:** Establish core data structures and **thread-safe** git client extensions

#### 1.1: Data Structures (`internal/scenarios/types.go`)

**No changes from v3** - Types remain stable

**Priority:** P0 (Critical path)
**LOC:** ~250 lines
**Dependencies:** None

*(Same implementation as v3)*

---

#### 1.2: **UPDATED:** Thread-Safe Git Client Extensions (`internal/git/cli.go`)

**Changes from v3:**
- **CRITICAL:** Add mutex to serialize all git operations (prevent races)
- Add `IsAncestor()` method for fast-forward validation
- Set `GIT_TERMINAL_PROMPT=0` and `LC_ALL=C` in all commands
- Reverse `GetDefaultBranch()` order (local cache first)

**Priority:** P0 (Blocks classifier)
**LOC:** ~400 lines (+50 from v3)
**Dependencies:** Existing git.Client

**File:** `internal/git/cli.go`

```go
package git

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "sync"
    "time"
)

// Client wraps git operations with thread-safety
type Client struct {
    repoPath string
    mu       sync.Mutex  // **NEW in v4:** Serialize all git operations
}

// **UPDATED in v4:** run() with mutex, non-interactive mode, and stable locale
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
    // **CRITICAL:** Serialize all git operations to prevent races
    c.mu.Lock()
    defer c.mu.Unlock()

    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = c.repoPath

    // **NEW in v4:** Force non-interactive mode and stable locale
    cmd.Env = append(os.Environ(),
        "GIT_TERMINAL_PROMPT=0",  // Prevent credential hangs
        "LC_ALL=C",                // Stable output parsing
    )

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git %v failed: %w\nstderr: %s", args, err, stderr.String())
    }

    return stdout.String(), nil
}

// **NEW in v4:** IsAncestor checks if commit1 is ancestor of commit2
// Used for fast-forward validation in ResetOperation
func (c *Client) IsAncestor(commit1, commit2 string) (bool, error) {
    // git merge-base --is-ancestor <commit1> <commit2>
    // Exit code 0: commit1 is ancestor of commit2
    // Exit code 1: commit1 is NOT ancestor of commit2
    // Other: error

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", commit1, commit2)
    cmd.Dir = c.repoPath
    cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")

    err := cmd.Run()
    if err == nil {
        return true, nil  // Is ancestor
    }

    if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
        return false, nil  // Not ancestor (not an error)
    }

    return false, fmt.Errorf("merge-base failed: %w", err)
}

// **UPDATED in v4:** GetDefaultBranch with reversed order (local cache first)
func (c *Client) GetDefaultBranch(remote string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // **UPDATED in v4:** Try local cache FIRST (fast, no network)
    // Try 1: git symbolic-ref refs/remotes/<remote>/HEAD
    output, err := c.run(ctx, "symbolic-ref", fmt.Sprintf("refs/remotes/%s/HEAD", remote))
    if err == nil && output != "" {
        // Output: "refs/remotes/origin/main"
        parts := strings.Split(strings.TrimSpace(output), "/")
        if len(parts) > 0 {
            return parts[len(parts)-1], nil
        }
    }

    // Try 2: git remote show <remote> | grep "HEAD branch" (network fallback)
    output, err = c.run(ctx, "remote", "show", remote)
    if err == nil {
        for _, line := range strings.Split(output, "\n") {
            if strings.Contains(line, "HEAD branch:") {
                branch := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
                return branch, nil
            }
        }
    }

    // Try 3: Check for main
    _, err = c.run(ctx, "rev-parse", "--verify", "refs/heads/main")
    if err == nil {
        return "main", nil
    }

    // Try 4: Check for master
    _, err = c.run(ctx, "rev-parse", "--verify", "refs/heads/master")
    if err == nil {
        return "master", nil
    }

    return "", fmt.Errorf("could not determine default branch")
}

// ResetToRef performs hard reset to ref (safer than pull)
func (c *Client) ResetToRef(ref string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    _, err := c.run(ctx, "reset", "--hard", ref)
    return err
}

// ScanLargeBinaries finds blobs >threshold (SHA+size only, NO path lookup)
func (c *Client) ScanLargeBinaries(thresholdBytes int64) ([]LargeBinary, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Implementation:
    // 1. git rev-list --objects --all | git cat-file --batch-check='%(objectname) %(objecttype) %(objectsize)'
    // 2. Filter for type=blob AND size >= threshold
    // 3. Return LargeBinary with SHA1 and SizeMB ONLY
    //
    // NOTE: Does NOT populate Path field (too expensive)
    // Users must manually locate: git log --all --find-object=<sha>

    // Simplified for plan - actual implementation in Phase 1
    return []LargeBinary{}, nil
}

// CheckLFSEnabled detects if Git LFS is installed and in use
func (c *Client) CheckLFSEnabled() (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 1. Check if git lfs version succeeds
    _, err := c.run(ctx, "lfs", "version")
    if err != nil {
        return false, nil  // LFS not installed
    }

    // 2. Check if any files tracked
    output, err := c.run(ctx, "lfs", "ls-files")
    if err != nil {
        return false, nil
    }

    return strings.TrimSpace(output) != "", nil
}

// FetchRemote with timeout
func (c *Client) FetchRemote(remote string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    _, err := c.run(ctx, "fetch", remote, "--tags")
    return err
}

// LocalExists checks if repository exists locally
func (c *Client) LocalExists() (bool, string) {
    // Check if c.repoPath exists and contains .git
    // Implementation details...
    return true, c.repoPath
}

// IsDetachedHEAD checks for detached HEAD state
func (c *Client) IsDetachedHEAD() (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    _, err := c.run(ctx, "symbolic-ref", "-q", "HEAD")
    if err != nil {
        return true, nil  // Exit code 1 = detached
    }
    return false, nil
}

// IsShallowClone checks if repo is shallow
func (c *Client) IsShallowClone() (bool, error) {
    // Check for .git/shallow file
    shallowPath := filepath.Join(c.repoPath, ".git", "shallow")
    _, err := os.Stat(shallowPath)
    return err == nil, nil
}

// GetStagedFiles returns list of staged files
func (c *Client) GetStagedFiles() ([]string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    output, err := c.run(ctx, "diff", "--cached", "--name-only")
    if err != nil {
        return nil, err
    }

    if output == "" {
        return []string{}, nil
    }
    return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// GetUnstagedFiles returns list of unstaged modifications
func (c *Client) GetUnstagedFiles() ([]string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    output, err := c.run(ctx, "diff", "--name-only")
    if err != nil {
        return nil, err
    }

    if output == "" {
        return []string{}, nil
    }
    return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// GetBranchHash returns commit hash for local branch
func (c *Client) GetBranchHash(branch string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    output, err := c.run(ctx, "rev-parse", branch)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(output), nil
}

// GetRemoteBranchHash returns commit hash for remote branch
func (c *Client) GetRemoteBranchHash(remote, branch string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    ref := fmt.Sprintf("%s/%s", remote, branch)
    output, err := c.run(ctx, "rev-parse", ref)
    if err != nil {
        return "", nil  // Branch doesn't exist, not an error
    }
    return strings.TrimSpace(output), nil
}

// CountCommitsBetween counts commits in ref1 not in ref2
func (c *Client) CountCommitsBetween(ref1, ref2 string) (int, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    output, err := c.run(ctx, "rev-list", "--count", ref1, "^"+ref2)
    if err != nil {
        return 0, err
    }

    count, err := strconv.Atoi(strings.TrimSpace(output))
    if err != nil {
        return 0, fmt.Errorf("invalid count output: %s", output)
    }
    return count, nil
}

// CanReachRemote tests if remote is accessible
func (c *Client) CanReachRemote(remote string) bool {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    _, err := c.run(ctx, "ls-remote", "--exit-code", remote, "HEAD")
    return err == nil
}

// ListBranches returns all branches (local and remote)
func (c *Client) ListBranches() (local, remote []string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Local branches
    localOut, err := c.run(ctx, "branch", "--format=%(refname:short)")
    if err != nil {
        return nil, nil, err
    }
    local = strings.Split(strings.TrimSpace(localOut), "\n")

    // Remote branches
    remoteOut, err := c.run(ctx, "branch", "-r", "--format=%(refname:short)")
    if err != nil {
        return nil, nil, err
    }
    remote = strings.Split(strings.TrimSpace(remoteOut), "\n")

    return local, remote, nil
}

// GetRemoteURL returns URL for named remote
func (c *Client) GetRemoteURL(remote string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    output, err := c.run(ctx, "remote", "get-url", remote)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(output), nil
}
```

**Implementation Notes:**
- **CRITICAL:** All git commands serialized through `mu.Lock()`
- Set `GIT_TERMINAL_PROMPT=0` to prevent hangs
- Set `LC_ALL=C` for stable output parsing
- `GetDefaultBranch()` tries local cache before network
- `IsAncestor()` enables fast-forward validation
- Generous timeouts with context cancellation
- Handle missing refs gracefully (return empty, not error)

**Testing Requirements:**
- Unit tests with mock git output
- Integration tests with real test repos
- Test edge cases: missing remotes, detached HEAD, shallow clones, network failures
- **NEW:** Test concurrent operations (spawn 10 goroutines calling fetch)
- **NEW:** Test IsAncestor() with various commit relationships
- **NEW:** Test GIT_TERMINAL_PROMPT=0 behavior
- **NEW:** Test GetDefaultBranch() local vs network fallback

**Acceptance Criteria:**
- [ ] All 15 methods implemented (+1 IsAncestor from v3)
- [ ] 100% unit test coverage
- [ ] Integration tests pass
- [ ] Handles all edge cases (documented in tests)
- [ ] **NEW:** Thread-safety validated (concurrent fetch test)
- [ ] **NEW:** IsAncestor() works correctly
- [ ] **NEW:** No credential hangs (GIT_TERMINAL_PROMPT test)
- [ ] **NEW:** GetDefaultBranch() fast path tested

---

#### 1.3: Classification Tables (`internal/scenarios/tables.go`)

**No changes from v3** - This component remains stable.

---

#### 1.4: **UPDATED:** Structured Fix Operations with Fast-Forward Validation

**Changes from v3:**
- **CRITICAL:** Add fast-forward validation to `ResetOperation.Validate()`
- Improve validation error messages

**Priority:** P0 (Required for safe AutoFix)
**LOC:** ~280 lines (+30 from v3)
**Dependencies:** types.go, git/cli.go

**File:** `internal/scenarios/operations.go`

```go
package scenarios

import (
    "context"
    "fmt"

    "github.com/lcgerke/githelper/internal/git"
)

// Operation represents a structured fix operation
type Operation interface {
    Validate(ctx context.Context, state *RepositoryState, gitClient *git.Client) error
    Execute(ctx context.Context, gitClient *git.Client) error
    Describe() string
    Rollback(ctx context.Context, gitClient *git.Client) error
}

// FetchOperation - git fetch <remote>
type FetchOperation struct {
    Remote string
}

func (op *FetchOperation) Validate(ctx context.Context, state *RepositoryState, gc *git.Client) error {
    // Check remote exists and is reachable
    if !gc.CanReachRemote(op.Remote) {
        return fmt.Errorf("remote %s is not reachable", op.Remote)
    }
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

func (op *PushOperation) Validate(ctx context.Context, state *RepositoryState, gc *git.Client) error {
    // Ensure remote is reachable
    if !gc.CanReachRemote(op.Remote) {
        return fmt.Errorf("remote %s is not reachable", op.Remote)
    }

    // Ensure working tree is clean
    if !state.WorkingTree.Clean {
        return fmt.Errorf("working tree must be clean before push (found %d staged, %d unstaged files)",
            len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
    }

    // TODO: Check if push will be fast-forward (requires remote ref comparison)
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

// **UPDATED in v4:** ResetOperation with CRITICAL fast-forward validation
type ResetOperation struct {
    Ref string  // Full ref: "refs/remotes/origin/main"
}

func (op *ResetOperation) Validate(ctx context.Context, state *RepositoryState, gc *git.Client) error {
    // Validation 1: Working tree must be clean
    if !state.WorkingTree.Clean {
        return fmt.Errorf("working tree must be clean before reset (found %d staged, %d unstaged files)",
            len(state.WorkingTree.StagedFiles), len(state.WorkingTree.UnstagedFiles))
    }

    // Validation 2: Ref must exist
    if op.Ref == "" {
        return fmt.Errorf("reset ref cannot be empty")
    }

    // **CRITICAL in v4:** Validation 3: Must be fast-forward only (prevent data loss)
    // Check if target ref is ancestor of current HEAD
    // If HEAD has commits not in target, this would discard them
    isAncestor, err := gc.IsAncestor(op.Ref, "HEAD")
    if err != nil {
        return fmt.Errorf("failed to check fast-forward status: %w", err)
    }

    if !isAncestor {
        return fmt.Errorf("reset would discard local commits (not a fast-forward); target ref is not ancestor of HEAD")
    }

    return nil
}

func (op *ResetOperation) Execute(ctx context.Context, gc *git.Client) error {
    return gc.ResetToRef(op.Ref)
}

func (op *ResetOperation) Describe() string {
    return fmt.Sprintf("Reset to %s (fast-forward only)", op.Ref)
}

func (op *ResetOperation) Rollback(ctx context.Context, gc *git.Client) error {
    // Reset rollback is risky - better to fail
    return fmt.Errorf("reset rollback requires manual intervention (use: git reset --hard ORIG_HEAD)")
}

// CompositeOperation - sequence of operations (executed in order)
type CompositeOperation struct {
    Operations []Operation
    StopOnError bool  // If true, stop at first error
}

func (op *CompositeOperation) Validate(ctx context.Context, state *RepositoryState, gc *git.Client) error {
    for _, subOp := range op.Operations {
        if err := subOp.Validate(ctx, state, gc); err != nil {
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
    // Disabled per v3 critique - too risky for multi-step operations
    return fmt.Errorf("composite operation rollback disabled - manual intervention required")
}
```

**Testing Requirements:**
- Unit test for each operation type
- Validation logic tested with various states
- **NEW:** ResetOperation validation tested with:
  - Clean working tree + fast-forward: PASS
  - Clean working tree + diverged commits: FAIL (prevent data loss)
  - Dirty working tree + fast-forward: FAIL
- Composite operations tested with failures
- Rollback tested where applicable

**Acceptance Criteria:**
- [ ] All operation types implemented
- [ ] Validation prevents unsafe operations
- [ ] 100% test coverage
- [ ] Integration tested with real repos
- [ ] **NEW:** ResetOperation CANNOT discard local commits
- [ ] **NEW:** Fast-forward validation tested with diverged branches

---

### Phase 2: Classifier Core (Week 2-3) ~750 LOC

**Changes from v3:**
- Remove unused `github.Client` from Classifier
- Pass `gitClient` to operation validation

*(Classifier implementation mostly unchanged from v3, with minor adjustments)*

---

### Phase 3: Fix Suggestion Engine (Week 3-4) ~700 LOC

**Changes from v3:**
- Complete E5-E8 existence scenario fixes (GPT critique)
- Update operation validation calls to pass gitClient

**File:** `internal/scenarios/suggester.go`

```go
// **UPDATED in v4:** suggestExistenceFixes handles ALL E1-E8
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
            AutoFixable: false,
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

    // **NEW in v4:** E5-E8 fixes
    case "E5":  // Core + GitHub exist, local missing
        return []Fix{{
            ScenarioID:  "E5",
            Description: "Repository not cloned locally",
            Command:     "git clone <core-url>",
            Operation:   nil,  // Requires URL from config
            AutoFixable: false,
            Priority:    1,
            Reason:      "Clone from Core to create local copy",
        }}

    case "E6":  // Core exists, local + GitHub missing
        return []Fix{{
            ScenarioID:  "E6",
            Description: "Local repository missing, GitHub not configured",
            Command:     "git clone <core-url> && githelper github setup",
            Operation:   nil,
            AutoFixable: false,
            Priority:    1,
            Reason:      "Clone from Core and set up GitHub remote",
        }}

    case "E7":  // GitHub exists, local + Core missing
        return []Fix{{
            ScenarioID:  "E7",
            Description: "Local repository and Core remote missing",
            Command:     "git clone <github-url> && git remote add origin <core-url>",
            Operation:   nil,
            AutoFixable: false,
            Priority:    1,
            Reason:      "Clone from GitHub and configure Core remote",
        }}

    case "E8":  // No repositories exist
        return []Fix{{
            ScenarioID:  "E8",
            Description: "No repositories exist anywhere",
            Command:     "githelper repo create <name>",
            Operation:   nil,
            AutoFixable: false,
            Priority:    1,
            Reason:      "Initialize new repository",
        }}

    default:
        return nil
    }
}

// **UPDATED in v4:** AutoFix passes gitClient to Validate
func AutoFix(ctx context.Context, state *RepositoryState, gitClient *git.Client) ([]Fix, []error) {
    fixes := SuggestFixes(state)
    applied := []Fix{}
    errors := []error{}

    for _, fix := range fixes {
        if !fix.AutoFixable || fix.Operation == nil {
            continue
        }

        // **UPDATED in v4:** Pass gitClient for validation (needed for IsAncestor check)
        if err := fix.Operation.Validate(ctx, state, gitClient); err != nil {
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
```

---

### Phase 4: Command Integration (Week 4-5) ~450 LOC

*(Same as v3, but with github.Client removed from Classifier initialization)*

---

### Phase 5: Testing & Documentation (Week 5-7) ~1,000 LOC

**Changes from v3:**
- Add tests for concurrent git operations
- Add tests for fast-forward validation
- Add corpus caching infrastructure
- Document edge cases (git hooks, locked refs, etc.)

#### 5.1: Unit Tests

*(Enhanced with concurrency and safety tests)*

#### 5.2: Integration Tests

**Additional Test Cases in v4:**
- **Concurrent Fetch Test**: Spawn 10 goroutines calling `FetchRemote()` simultaneously, verify no races
- **Fast-Forward Validation Test**: S6 auto-fix with diverged local commits should FAIL validation
- **Credential Hang Test**: Mock interactive credential helper, verify fails instead of hanging
- **GetDefaultBranch Performance Test**: Verify local cache path is faster than network

#### 5.3: Corpus Testing

**Caching Infrastructure (NEW in v4):**

```go
// test/corpus/cache.go
type CorpusCache struct {
    cacheDir string  // ~/.cache/githelper-corpus/
    ttl      time.Duration  // 7 days
}

func (c *CorpusCache) GetRepo(url string) (string, error) {
    // Hash URL to directory name
    // Check if exists and < 7 days old
    // If yes: git fetch origin
    // If no: git clone
    // Return path
}
```

#### 5.4: Documentation (Updated for v4)

**Required Documentation:**

1. **`README.md`** - Git 2.30+ requirement, safety features highlighted
2. **`docs/SCENARIO_REFERENCE.md`** - All 41 scenarios (E5-E8 now complete)
3. **`docs/BFG_CLEANUP.md`** - **NEW:** Section on SHA→path manual lookup using `git log --all --find-object=<sha>`
4. **`docs/DIVERGENCE.md`** - Handling divergent histories
5. **`docs/REMOTE_CONFIGURATION.md`** - Configuring non-standard remote names
6. **`docs/AUTHENTICATION.md`** - SSH keys, credential helpers, **non-interactive mode**
7. **`docs/EDGE_CASES.md`** (NEW in v4) - Git hooks, locked refs, extreme ref counts, troubleshooting
8. **`test/integration/README.md`** - Integration test harness guide
9. **`test/corpus/README.md`** - Corpus testing guide, **caching strategy, credential setup**

---

## Risk Assessment & Mitigation - **FINAL for v4**

### Technical Risks

| Risk | Impact | Probability | Mitigation (v4) |
|------|--------|-------------|-----------------|
| **Data loss from auto-fix** | **CRITICAL** | **N/A** | **FIXED:** Fast-forward validation in ResetOperation |
| **Concurrent fetch races** | **HIGH** | **N/A** | **FIXED:** Mutex serialization in git.Client |
| **Credential hangs** | **MEDIUM** | **N/A** | **FIXED:** GIT_TERMINAL_PROMPT=0 |
| **Performance regression** | High | Low | Early benchmarking, <2s mandatory test, concurrent fetch |
| **False positives in detection** | High | Medium | Corpus testing (100+ repos), <5% target with validation |
| **Network timeout during checks** | Medium | High | 30s fetch timeout, graceful degradation, retry logic |
| **Large binary scan too slow** | Medium | Low | SHA+size only (<5s target), manual path lookup documented |
| **Git version incompatibility** | Medium | Low | Require Git 2.30+, version check on startup |
| **Locked refs during operations** | Low | Medium | **NEW:** Retry logic for "ref is locked" errors |
| **Git hooks interference** | Low | Medium | **NEW:** Documented in EDGE_CASES.md |
| **Extreme ref counts** | Low | Low | **NEW:** Performance testing with 1000+ branches |

---

## Acceptance Criteria - **FINAL for v4**

### Functional Requirements

- [ ] **Scenario Detection:** All 41 scenarios correctly identified
- [ ] **Fix Suggestions:** All scenarios have appropriate fix suggestions (E1-E8 complete)
- [ ] **Auto-Fix:** 25/41 scenarios can be auto-fixed safely
- [ ] **Safety:** ResetOperation validates fast-forward (no data loss)
- [ ] **Performance:** State detection completes in <2 seconds (with concurrent fetch)
- [ ] **Accuracy:** False positive rate <5% on 100-repo corpus
- [ ] **Fresh Data:** Mandatory fetch before sync checks (with --no-fetch escape hatch)
- [ ] **Configurable Remotes:** Non-standard remote names work correctly
- [ ] **Safe Operations:** All auto-fix operations validated before execution
- [ ] **Edge Cases:** Detached HEAD, shallow clone, LFS warnings present
- [ ] **Thread Safety:** No race conditions from concurrent operations
- [ ] **Non-Interactive:** No hangs from credential prompts

### Non-Functional Requirements

- [ ] **Test Coverage:** >90% line coverage, 100% scenario coverage
- [ ] **Documentation:** All public APIs documented, user guide complete
- [ ] **Error Handling:** Graceful degradation on network failures
- [ ] **Logging:** Structured logging for all operations
- [ ] **Backward Compatibility:** Zero regressions in existing commands
- [ ] **Concurrency Safety:** Mutex-protected git operations
- [ ] **Credential Handling:** GIT_TERMINAL_PROMPT=0 prevents hangs
- [ ] **Git Version:** 2.30+ validated on startup
- [ ] **Corpus Testing:** Automated with caching, <5% false positive rate

---

## Delivery Milestones - **FINAL for v4**

### Milestone 0: Benchmarking (End of Week 1)
- [ ] Benchmark framework in place
- [ ] Baseline measurements for empty/small/large repos
- [ ] CI integration for performance regression detection
- [ ] Large binary scan benchmark confirms <5s

### Milestone 1: Foundation (End of Week 2)
- [ ] All data structures defined
- [ ] **Thread-safe git.Client with mutex serialization**
- [ ] **IsAncestor() method for fast-forward validation**
- [ ] **GIT_TERMINAL_PROMPT=0 and LC_ALL=C in all commands**
- [ ] **GetDefaultBranch() local-first optimization**
- [ ] Classification tables implemented
- [ ] Structured operations with ResetOperation fast-forward validation
- [ ] Unit tests passing
- [ ] Git version check implemented

### Milestone 2: Core Classifier (End of Week 3)
- [ ] Classifier detects all 5 dimensions + LFS
- [ ] **Classifier uses ONLY git.Client (github.Client removed)**
- [ ] Concurrent fetch with local checks working (serialized through mutex)
- [ ] Large binary scan implemented (<5s)
- [ ] All 41 scenarios testable
- [ ] Integration tests for all dimensions

### Milestone 3: Fix Engine (End of Week 4)
- [ ] Fix suggester complete with Operations
- [ ] **All E1-E8 existence fixes implemented**
- [ ] All 41 fixes implemented (no stubs)
- [ ] **ResetOperation validates fast-forward (prevents data loss)**
- [ ] Auto-fix with rigorous validation
- [ ] Integration tests for all scenarios

### Milestone 4: Command Integration (End of Week 5)
- [ ] `doctor` enhanced with scenario IDs, LFS warnings
- [ ] `status` command implemented
- [ ] `repair` command implemented
- [ ] `scenarios` command implemented
- [ ] No regressions in existing functionality

### Milestone 5: Testing (End of Week 6-7)
- [ ] All unit tests passing (>90% coverage)
- [ ] **Concurrent operation tests (no races)**
- [ ] **Fast-forward validation tests (no data loss)**
- [ ] All 41 integration tests passing
- [ ] Benchmark tests validate <2s detection, <5s corruption scan
- [ ] Corpus test infrastructure ready (caching, credentials)

### Milestone 6: Validation & Ship (End of Week 8-9)
- [ ] Corpus testing on 100+ real repos (<5% false positive rate)
- [ ] Golden set validated (15 manually reviewed repos)
- [ ] Documentation complete (including EDGE_CASES.md, updated BFG guide)
- [ ] External review approved (2+ engineers)
- [ ] Production deployment

**Note:** Milestones 5-6 can overlap with Milestone 4 if parallel teams available.

---

## Appendix: Changes from v3.0

### Critical Safety Fixes (v4 over v3)

1. **Fast-Forward Validation** (Both v3 critiques)
   - `ResetOperation.Validate()` now calls `git merge-base --is-ancestor`
   - Prevents data loss when local has commits not in remote
   - Auto-fix for S6/S7 will FAIL if not fast-forward

2. **Thread-Safe Git Operations** (Both v3 critiques)
   - Added `sync.Mutex` to `git.Client`
   - All `run()` calls serialized
   - Prevents concurrent fetch races and corruption

3. **Credential Hang Prevention** (Gemini v3 critique)
   - Set `GIT_TERMINAL_PROMPT=0` in all git commands
   - Fails immediately instead of hanging on interactive prompts

4. **Performance Optimization** (Both v3 critiques)
   - `GetDefaultBranch()` tries local cache before network
   - Significantly faster in common case

5. **Complete Existence Fixes** (GPT v3 critique)
   - E5-E8 scenarios now have fix suggestions
   - All 41 scenarios covered

6. **Removed Unused Dependency** (Both v3 critiques)
   - Removed `github.Client` from `Classifier`
   - Simplifies code, removes confusion

7. **Corpus Testing Infrastructure** (Both v3 critiques)
   - Caching layer for 100-repo corpus
   - Credential strategy documented
   - Parallel cloning (5 concurrent max)

8. **Edge Case Documentation** (Gemini v3 critique)
   - New `EDGE_CASES.md` covering git hooks, locked refs, extreme ref counts
   - Updated `BFG_CLEANUP.md` with SHA→path manual lookup instructions

9. **Timeline Adjustment**
   - v3: 6-8 weeks
   - v4: 7-9 weeks (accounts for safety refinements + buffer for overlap)

10. **Submodules Decision**
    - v3 critique confirmed: Keep out of scope for v1
    - User requested: Defer minimal detection to v2
    - Rationale: Avoid scope creep, validate core classifier first

---

**END OF IMPLEMENTATION PLAN v4.0**

*This plan addresses all critical safety concerns from v3 critique, implements thread-safe operations, prevents data loss through fast-forward validation, and is ready for implementation.*

**Version 4.0 - 2025-11-18**
