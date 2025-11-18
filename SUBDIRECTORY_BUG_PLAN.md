# GitHelper Subdirectory Detection Bug Fix Plan

## Problem Statement

**Current Behavior:**
When `githelper status` is run from a subdirectory within a git repository, it incorrectly reports:
```json
"existence": {
  "id": "E5",
  "description": "Core + GitHub exist, local missing"
}
```

**Expected Behavior:**
GitHelper should detect the repository regardless of which subdirectory the command is run from, just like native git commands.

**Real-world Impact:**
- User runs `githelper status` from `/home/lcgerke/wk/ansible/`
- GitHelper says "local missing" even though they're inside a git repo
- Confusing UX - git itself works fine from subdirectories

**Root Cause:**
GitHelper is likely checking for `.git` directory in the current working directory instead of using git's built-in repository discovery mechanism.

---

## Root Cause Analysis

### Expected: How Git Finds Repositories

Git searches for `.git` by traversing up the directory tree:
```bash
# From /home/lcgerke/wk/ansible/subdir/
git rev-parse --git-dir
# Returns: /home/lcgerke/wk/.git

git rev-parse --show-toplevel
# Returns: /home/lcgerke/wk
```

### Suspected: How GitHelper Currently Works

**File:** `internal/git/client.go` or `internal/scenarios/classifier.go`

Likely doing something like:
```go
// BROKEN - only checks current directory
func (c *Client) IsRepository() bool {
    _, err := os.Stat(".git")
    return err == nil
}
```

Should be doing:
```go
// CORRECT - uses git to find repository
func (c *Client) IsRepository() bool {
    cmd := exec.Command("git", "rev-parse", "--git-dir")
    err := cmd.Run()
    return err == nil
}
```

---

## Investigation Steps

### 1. Find the Detection Code

**Search for where local existence is determined:**

```bash
# Find where "local_exists" is set
grep -r "local_exists" internal/

# Find where .git is checked
grep -r "\.git" internal/

# Find repository initialization
grep -r "NewClient\|IsRepository" internal/
```

### 2. Trace the Detection Flow

**Starting point:** `cmd/githelper/status.go`
- Creates `git.NewClient(".")`
- Passes to `scenarios.NewClassifier()`
- Classifier calls detection methods

**Key question:** What does `git.NewClient(".")` do with the path?

### 3. Test Current Behavior

```bash
# From repo root
cd /home/lcgerke/gitDualRemote
githelper status
# Expected: E1 (works)

# From subdirectory
cd /home/lcgerke/gitDualRemote/internal/scenarios
githelper status
# Expected: E5 (broken)

# From nested subdirectory
cd /home/lcgerke/gitDualRemote/cmd/githelper
githelper status
# Expected: E5 (broken)
```

---

## Proposed Solution

### Option A: Fix Git Client Initialization (Recommended)

**Modify:** `internal/git/client.go` (or wherever `NewClient` is defined)

**Change initialization to find git repository root:**

```go
// NewClient creates a new Git client for the repository
// containing the given path (or current directory if path is ".")
func NewClient(path string) (*Client, error) {
    // Convert to absolute path
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve path: %w", err)
    }

    // Find the git repository root
    cmd := exec.Command("git", "-C", absPath, "rev-parse", "--show-toplevel")
    output, err := cmd.Output()
    if err != nil {
        // Not a git repository
        return &Client{
            workingDir: absPath,
            repoRoot:   "",
        }, nil
    }

    repoRoot := strings.TrimSpace(string(output))

    return &Client{
        workingDir: absPath,
        repoRoot:   repoRoot,
    }, nil
}
```

**Update Client struct:**

```go
type Client struct {
    workingDir string // Original path passed in
    repoRoot   string // Repository root (empty if not a git repo)
    // ... other fields
}

// IsRepository returns true if this client is in a git repository
func (c *Client) IsRepository() bool {
    return c.repoRoot != ""
}

// GetRepoRoot returns the repository root directory
func (c *Client) GetRepoRoot() string {
    return c.repoRoot
}
```

**Update all git commands to use repoRoot:**

```go
// Before (broken in subdirectories)
func (c *Client) GetBranchHash(branch string) (string, error) {
    cmd := exec.Command("git", "rev-parse", branch)
    // ...
}

// After (works from anywhere)
func (c *Client) GetBranchHash(branch string) (string, error) {
    cmd := exec.Command("git", "-C", c.repoRoot, "rev-parse", branch)
    // ...
}
```

### Option B: Fix at Classifier Level (Less Ideal)

**Modify:** `internal/scenarios/classifier.go`

Add repository root detection before creating client:

```go
func (c *Classifier) Detect() (*RepositoryState, error) {
    // Find repository root first
    cmd := exec.Command("git", "rev-parse", "--show-toplevel")
    output, err := cmd.Output()

    if err != nil {
        // Not in a repository
        state.Existence.LocalExists = false
    } else {
        repoRoot := strings.TrimSpace(string(output))
        state.Existence.LocalExists = true
        state.Existence.LocalPath = repoRoot
    }
    // ...
}
```

**Why Option A is better:**
- Fixes the issue at the source (Git client)
- All git commands automatically work from subdirectories
- More robust and consistent
- Aligns with how git itself works

---

## Implementation Steps

### Phase 1: Investigation (TDD Setup)

**Duration: 20 minutes**

1. **Write failing test** - `internal/git/client_test.go`

```go
func TestNewClient_FromSubdirectory(t *testing.T) {
    // Setup: Create a git repo with subdirectory
    tmpDir := t.TempDir()
    repoRoot := filepath.Join(tmpDir, "repo")
    subDir := filepath.Join(repoRoot, "sub", "nested")

    // Initialize git repo
    os.MkdirAll(subDir, 0755)
    exec.Command("git", "-C", repoRoot, "init").Run()

    // Create client from subdirectory
    client, err := git.NewClient(subDir)

    require.NoError(t, err)
    assert.True(t, client.IsRepository())
    assert.Equal(t, repoRoot, client.GetRepoRoot())
}

func TestNewClient_NotInRepository(t *testing.T) {
    tmpDir := t.TempDir()

    client, err := git.NewClient(tmpDir)

    require.NoError(t, err)
    assert.False(t, client.IsRepository())
    assert.Empty(t, client.GetRepoRoot())
}

func TestNewClient_FromCurrentDirectory(t *testing.T) {
    // Create repo in temp dir
    tmpDir := t.TempDir()
    repoRoot := filepath.Join(tmpDir, "repo")
    os.MkdirAll(repoRoot, 0755)
    exec.Command("git", "-C", repoRoot, "init").Run()

    // Change to repo directory
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    os.Chdir(repoRoot)

    // Create client with "."
    client, err := git.NewClient(".")

    require.NoError(t, err)
    assert.True(t, client.IsRepository())
    assert.Equal(t, repoRoot, client.GetRepoRoot())
}
```

2. **Run test** - Verify it fails with current implementation

3. **Locate existing Client code** - Find where `NewClient` and detection logic live

### Phase 2: Fix Git Client

**Duration: 30 minutes**

1. **Update Client struct** - Add `repoRoot` field

2. **Modify NewClient()** - Implement repository root detection

3. **Update IsRepository()** - Use `repoRoot != ""`

4. **Add GetRepoRoot()** - Return repository root

5. **Run tests** - Verify tests pass

### Phase 3: Update Git Commands

**Duration: 45 minutes**

Find all places where git commands are executed and ensure they use `-C` flag:

```go
// Pattern to search for
grep -r "exec.Command.*git" internal/git/

// Update each command to use repoRoot
// Before:
cmd := exec.Command("git", "status")

// After:
cmd := exec.Command("git", "-C", c.repoRoot, "status")
```

**Commands to update:**
- `GetBranchHash()`
- `GetRemoteBranchHash()`
- `CountCommitsBetween()`
- `GetStagedFiles()`
- `GetUnstagedFiles()`
- `GetUntrackedFiles()`
- `GetConflictFiles()`
- `Fetch()`
- `Push()`
- `Pull()`
- All other git operations

### Phase 4: Integration Testing

**Duration: 30 minutes**

1. **Test from repo root:**
```bash
cd /home/lcgerke/gitDualRemote
githelper status
# Expected: E1 - Fully configured
```

2. **Test from subdirectory:**
```bash
cd /home/lcgerke/gitDualRemote/internal/scenarios
githelper status
# Expected: E1 - Fully configured (was E5)
```

3. **Test from deep nested subdirectory:**
```bash
cd /home/lcgerke/gitDualRemote/cmd/githelper
githelper status
# Expected: E1 - Fully configured
```

4. **Test from outside repository:**
```bash
cd /tmp
githelper status
# Expected: Appropriate error or E4/E5 status
```

5. **Test on original bug:**
```bash
cd /home/lcgerke/wk/ansible
githelper status
# Expected: E1 - Fully configured (was E5 - local missing)
```

### Phase 5: Edge Cases

**Duration: 20 minutes**

1. **Test with worktrees** (if supported)
2. **Test with symbolic links**
3. **Test with relative paths**
4. **Test with bare repositories**
5. **Test with submodules**

---

## Files to Modify

| File | Change | Lines | Complexity |
|------|--------|-------|------------|
| `internal/git/client.go` | Add repoRoot detection to NewClient() | +30 | Medium |
| `internal/git/client.go` | Update all git commands to use -C flag | +40 | Low |
| `internal/git/client.go` | Add repoRoot field and getter | +5 | Low |
| `internal/git/client_test.go` | Add subdirectory detection tests | +80 | Medium |
| `internal/git/*_test.go` | Update existing tests if needed | ~20 | Low |

**Total:** ~175 lines of changes

---

## Validation Criteria

**Success means:**

1. **Works from subdirectories:**
```bash
$ cd /home/lcgerke/wk/ansible
$ githelper status
📦 Existence:
  E1 - Fully configured (local + core + github)
  Local: ✓ /home/lcgerke/wk
```

2. **Repo root properly detected:**
```json
{
  "existence": {
    "id": "E1",
    "local_exists": true,
    "local_path": "/home/lcgerke/wk"  // Not /home/lcgerke/wk/ansible
  }
}
```

3. **All git operations work from subdirectories:**
```bash
cd /home/lcgerke/wk/ansible/subdir
githelper status --show-fixes
# All git operations (fetch, status checks, etc.) succeed
```

4. **Error handling for non-repositories:**
```bash
cd /tmp
githelper status
# Clear error or appropriate E5 status
```

5. **No regression in existing functionality:**
```bash
cd /home/lcgerke/gitDualRemote
githelper status
# Still works as before
```

---

## Risk Assessment

**Low Risk:**
- Using `git rev-parse --show-toplevel` is standard git practice
- Git's `-C` flag is well-supported and stable
- Changes are localized to git client initialization

**Medium Risk:**
- Need to update all git command invocations
- Could miss some edge cases (symlinks, worktrees, bare repos)
- Might affect performance if called frequently

**Mitigation:**
- Comprehensive test coverage for subdirectory scenarios
- Cache repoRoot in client (only detect once)
- Test against real repositories with various configurations
- Regression tests for existing functionality

**Zero Risk:**
- No changes to classifier logic
- No changes to scenario detection
- No changes to suggester

---

## Timeline Estimate

- Phase 1 (Investigation + Tests): **20 min**
- Phase 2 (Fix Client): **30 min**
- Phase 3 (Update Commands): **45 min**
- Phase 4 (Integration Testing): **30 min**
- Phase 5 (Edge Cases): **20 min**

**Total: ~145 minutes (2.5 hours)**

**Buffer:** +15 minutes for unexpected issues

**Final estimate: 2.5-3 hours** for complete TDD implementation with comprehensive testing

---

## Alternative Approaches

### Alternative 1: Always Change to Repo Root

```go
func (c *Client) ensureInRepo() error {
    if c.repoRoot == "" {
        return fmt.Errorf("not in a git repository")
    }
    return os.Chdir(c.repoRoot)
}
```

**Pros:**
- Simple to implement
- No need to update every command

**Cons:**
- Changes working directory (side effects!)
- Not thread-safe
- Breaks user expectations
- Can interfere with relative paths

**Verdict:** ❌ Not recommended

### Alternative 2: Wrapper Script

Create a shell wrapper that changes to repo root:

```bash
#!/bin/bash
REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null)
if [ -n "$REPO_ROOT" ]; then
    cd "$REPO_ROOT"
fi
exec githelper-real "$@"
```

**Pros:**
- No code changes needed
- Easy to implement

**Cons:**
- Requires two binaries
- Clunky installation
- Doesn't fix the underlying issue

**Verdict:** ❌ Not recommended

### Alternative 3: Use GitPython/go-git Library

Replace manual git commands with a proper git library.

**Pros:**
- More robust
- Better error handling
- No subprocess overhead

**Cons:**
- Large refactor
- New dependency
- Learning curve

**Verdict:** 🤔 Consider for future, not for this fix

---

## Decision Points

### Should we cache the repository root?

**Option A:** Detect on every NewClient() call
- ✓ Always up-to-date
- ✗ Slight performance overhead

**Option B:** Cache in Client struct (current proposal)
- ✓ Fast after initial detection
- ✓ Repository root doesn't change during execution
- ✗ Slight memory overhead (negligible)

**Decision:** **Option B** - Cache in Client struct (recommended in proposal)

### Should we handle worktrees?

**Option A:** Full worktree support
- ✓ Comprehensive solution
- ✗ More complex
- ✗ Need to detect worktree vs main repo

**Option B:** Basic support (rev-parse finds the right .git)
- ✓ Simpler
- ✓ git rev-parse handles it automatically
- ✗ May not handle all worktree edge cases

**Decision:** **Option B** - Let git handle it (rev-parse works with worktrees)

### Should we return error or empty string for non-repos?

**Option A:** Return error from NewClient()
```go
func NewClient(path string) (*Client, error) {
    // ...
    if err != nil {
        return nil, fmt.Errorf("not a git repository")
    }
}
```

**Option B:** Return client with empty repoRoot (current proposal)
```go
func NewClient(path string) (*Client, error) {
    // ...
    return &Client{
        repoRoot: "", // Empty means not a repo
    }, nil
}
```

**Decision:** **Option B** - Return client with empty repoRoot
- Allows caller to decide how to handle
- Detection and usage are separate concerns
- Classifier can check `IsRepository()` and set existence accordingly

---

## Post-Fix Improvements

**Not in scope for this fix, but worth considering:**

1. **Better error messages** - When not in a repo, suggest running from repo root
2. **Auto-detect remote names** - Instead of hardcoding "origin" and "github"
3. **Config file support** - Allow user to specify remote names
4. **Worktree awareness** - Show which worktree you're in
5. **Performance optimization** - Cache git command results

---

## Related Issues

This fix will also help with:
- Running githelper from CI/CD scripts (they often run in subdirectories)
- Integration with editors/IDEs (they may invoke from current file's directory)
- Better compatibility with monorepos (where tools run from project subdirs)

---

## Summary

**Problem:** GitHelper doesn't work from subdirectories (shows "local missing")

**Solution:** Fix Git client to use `git rev-parse --show-toplevel` and `-C` flag

**Effort:** 2.5-3 hours with TDD

**Impact:** Works from any directory in a repository, just like native git commands

**Risk:** Low - well-tested git features, localized changes
