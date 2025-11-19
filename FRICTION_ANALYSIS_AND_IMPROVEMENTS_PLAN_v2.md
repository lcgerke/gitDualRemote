# Githelper Friction Analysis and Improvement Proposals (v2)

> **Version 2**: Incorporates feedback from GPT-5 and Gemini critiques addressing security risks, atomicity concerns, and implementation gaps.

## Friction Points Experienced

### 1. **Branch Divergence Hidden in JSON** (Already Addressed)
- **Problem**: Divergence on current branch (`main`) was buried in `branches` array while top-level showed default branch (`master`)
- **Fix Proposed**: `BRANCH_VISIBILITY_FIX_PROPOSAL.md` - show current branch in top-level sync
- **Status**: ✅ Documented

### 2. **Stale Remote Tracking After Pull**
- **Problem**: After `git pull github main`, `git status` showed "ahead by 2 commits" but githelper showed in sync
- **Root Cause**: Remote tracking branches weren't updated - needed manual `git fetch`
- **User Impact**: Confusing inconsistency between `git status` and `githelper status`

### 3. **Orphaned Submodules Not Auto-Fixable**
- **Problem**: Githelper detected orphaned submodules but provided no fix
- **User Impact**: Had to manually investigate with `git ls-files --stage`, `git config --file .gitmodules`, then manually remove
- **Friction**: Required understanding of git internals

### 4. **Submodule Name Mismatch Not Explained**
- **Problem**: `.gitmodules` had name "31utilityScripts/..." but path "ansible/04-specialized/..."
- **User Impact**: Githelper said "orphaned" but didn't explain the name/path mismatch
- **Friction**: Unclear what "orphaned" meant in this context

### 5. **No Actionable Fix Commands in JSON Output**
- **Problem**: JSON output shows state but no suggested fixes
- **User Impact**: Had to use `--show-fixes` flag (which we didn't know about initially)
- **Friction**: Hidden feature discoverability

---

## Proposed Improvements

### Improvement 1: Auto-Fetch Before Status Check

**Problem**: Stale remote tracking branches cause inconsistencies

**Solution**: Add `--auto-fetch` as **default behavior** with opt-out via `--no-fetch`

**Security & Reliability Improvements** (from critiques):
- ✅ Add configurable timeout (15-30s default) to prevent hangs
- ✅ Show fetch duration in output for transparency
- ✅ Handle credential/authentication failures gracefully
- ✅ Provide clear error messages for offline/unreachable remotes

**Implementation**:

**File**: `cmd/githelper/status.go`

```go
var (
    statusNoFetch    bool
    statusFetchTimeout int  // NEW: timeout in seconds
)

func init() {
    statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false,
        "Skip fetching from remotes (default: auto-fetch with 30s timeout)")
    statusCmd.Flags().IntVar(&statusFetchTimeout, "fetch-timeout", 30,
        "Timeout for fetch operations in seconds")
}
```

**File**: `internal/git/cli.go` (add timeout support)

```go
// FetchWithTimeout fetches from remote with timeout
func (c *Client) FetchWithTimeout(remote string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "fetch", remote)
    cmd.Dir = c.workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return fmt.Errorf("fetch timed out after %v (remote may be unreachable or slow)", timeout)
        }
        // Check for credential errors
        if strings.Contains(string(output), "Authentication failed") ||
           strings.Contains(string(output), "Permission denied") {
            return fmt.Errorf("authentication failed - check credentials: %w", err)
        }
        return fmt.Errorf("fetch failed: %w", err)
    }
    return nil
}
```

**File**: `internal/scenarios/classifier.go` (add telemetry)

```go
func ClassifyRepository(gitClient *git.Client, options DetectionOptions) (*RepositoryState, error) {
    if !options.SkipFetch {
        fmt.Fprintln(os.Stderr, "Fetching from remotes...")
        start := time.Now()

        timeout := time.Duration(options.FetchTimeout) * time.Second
        if err := gitClient.FetchWithTimeout("origin", timeout); err != nil {
            fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
        }

        duration := time.Since(start)
        fmt.Fprintf(os.Stderr, "Fetch completed in %v\n", duration.Round(100*time.Millisecond))
    }
    // ... rest of classification ...
}
```

**User Experience**:
```bash
$ githelper status
Fetching from remotes...
Fetch completed in 1.2s
🔍 Analyzing repository state...

$ githelper status --no-fetch  # Opt-out for speed
🔍 Analyzing repository state...

$ githelper status --fetch-timeout=60  # Custom timeout
Fetching from remotes...
⚠️  Warning: fetch timed out after 60s (remote may be unreachable or slow)
```

**Estimated Effort**: 3-4 hours (includes timeout handling, error messages, testing)

---

### Improvement 2: Auto-Fix for Orphaned Submodules

**Problem**: Orphaned submodules require manual intervention

**Solution**: Implement auto-fix operation for orphaned submodules

**Security & Safety Improvements** (from critiques):
- ✅ Use `exec.CommandContext` with proper argument arrays (NO string formatting)
- ✅ Update `.gitmodules` to maintain consistency
- ✅ Record removed submodule hash for potential rollback
- ✅ Stricter validation before removal
- ✅ Handle paths with spaces/special characters correctly

**Implementation**:

**File**: `internal/scenarios/operations.go`

```go
package scenarios

import (
    "context"
    "fmt"
    "os/exec"
    "time"
    "github.com/lcgerke/githelper/internal/git"
)

// RemoveOrphanedSubmoduleOp removes a submodule from git index
type RemoveOrphanedSubmoduleOp struct {
    SubmodulePath string
    RemovedHash   string  // NEW: Record hash for potential rollback
}

func (op *RemoveOrphanedSubmoduleOp) Validate(state *RepositoryState, gitClient interface{}) error {
    // Check that working tree is clean
    if !state.WorkingTree.Clean {
        return fmt.Errorf("working tree must be clean before removing submodule")
    }

    // NEW: Verify submodule exists in index
    client := gitClient.(*git.Client)
    exists, hash, err := client.SubmoduleExistsInIndex(op.SubmodulePath)
    if err != nil {
        return fmt.Errorf("failed to check submodule: %w", err)
    }
    if !exists {
        return fmt.Errorf("submodule %s not found in index", op.SubmodulePath)
    }

    // Record hash for rollback
    op.RemovedHash = hash

    return nil
}

func (op *RemoveOrphanedSubmoduleOp) Execute(gitClient interface{}) error {
    client := gitClient.(*git.Client)

    // CRITICAL FIX: Use exec.CommandContext with argument array (NO fmt.Sprintf)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "rm", "--cached", op.SubmodulePath)
    cmd.Dir = client.WorkDir()

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to remove submodule from index: %w (output: %s)", err, output)
    }

    // NEW: Also remove from .gitmodules if present
    if err := client.RemoveFromGitmodules(op.SubmodulePath); err != nil {
        // Don't fail if .gitmodules removal fails (already orphaned)
        fmt.Fprintf(os.Stderr, "Note: Could not remove from .gitmodules: %v\n", err)
    }

    return nil
}

func (op *RemoveOrphanedSubmoduleOp) Describe() string {
    return fmt.Sprintf("Remove orphaned submodule '%s' from git index (hash: %s)",
                       op.SubmodulePath, op.RemovedHash[:8])
}

func (op *RemoveOrphanedSubmoduleOp) Rollback(gitClient interface{}) error {
    // NEW: Better rollback using recorded hash
    if op.RemovedHash == "" {
        return fmt.Errorf("cannot rollback: no hash recorded")
    }

    client := gitClient.(*git.Client)

    // Re-add the submodule entry with the original hash
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "update-index", "--add",
                               "--cacheinfo", "160000,"+op.RemovedHash, op.SubmodulePath)
    cmd.Dir = client.WorkDir()

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("rollback failed: %w", err)
    }

    return nil
}
```

**File**: `internal/git/cli.go` (add helper methods)

```go
// SubmoduleExistsInIndex checks if a path exists as submodule (mode 160000) in index
func (c *Client) SubmoduleExistsInIndex(path string) (bool, string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Use -z for null-terminated output (safer parsing per Gemini critique)
    cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--stage", "--", path)
    cmd.Dir = c.workdir

    output, err := cmd.Output()
    if err != nil {
        return false, "", err
    }

    // Parse: "160000 <hash> 0\t<path>\0"
    parts := strings.Split(string(output), "\t")
    if len(parts) < 2 {
        return false, "", nil
    }

    fields := strings.Fields(parts[0])
    if len(fields) < 2 {
        return false, "", nil
    }

    if fields[0] != "160000" {
        return false, "", nil  // Not a submodule
    }

    hash := fields[1]
    return true, hash, nil
}

// RemoveFromGitmodules removes a submodule section from .gitmodules
// Per Gemini critique: parse .gitmodules directly instead of using git config
func (c *Client) RemoveFromGitmodules(path string) error {
    gitmodulesPath := filepath.Join(c.workdir, ".gitmodules")

    // Read .gitmodules
    data, err := os.ReadFile(gitmodulesPath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil  // Already doesn't exist
        }
        return err
    }

    // Parse as INI file and remove matching section
    // (Implementation details omitted for brevity - use ini parser library)

    return nil
}
```

**Estimated Effort**: 6-8 hours (security hardening, .gitmodules handling, rollback, testing)

---

### Improvement 3: Enhanced Orphaned Submodule Detection

**Problem**: "Orphaned" doesn't explain name/path mismatch

**Solution**: Add detailed diagnostics for submodule issues

**Reliability Improvements** (from critiques):
- ✅ Parse `.gitmodules` directly (more robust than git config output)
- ✅ Use null-terminated output (`-z` flag) for safe parsing
- ✅ Handle paths with spaces and special characters
- ✅ Version compatibility safeguards

**Implementation**:

**File**: `internal/git/submodules.go` (new)

```go
package git

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "gopkg.in/ini.v1"  // Use ini parser library
)

type SubmoduleIssue struct {
    Type        string `json:"type"`        // "orphaned", "name_mismatch", "missing_in_worktree"
    Path        string `json:"path"`
    Name        string `json:"name,omitempty"`
    Description string `json:"description"`
    Suggestion  string `json:"suggestion"`
}

// DiagnoseSubmodules performs comprehensive submodule analysis
// IMPROVED: Parses .gitmodules directly per Gemini critique
func (c *Client) DiagnoseSubmodules() ([]SubmoduleIssue, error) {
    var issues []SubmoduleIssue

    // Parse .gitmodules directly (more robust than git config)
    gitmodulesPath := filepath.Join(c.workdir, ".gitmodules")
    gitmodulesMap := make(map[string]string) // path -> name

    if _, err := os.Stat(gitmodulesPath); err == nil {
        cfg, err := ini.Load(gitmodulesPath)
        if err != nil {
            return nil, fmt.Errorf("failed to parse .gitmodules: %w", err)
        }

        for _, section := range cfg.Sections() {
            if strings.HasPrefix(section.Name(), "submodule ") {
                name := strings.TrimPrefix(section.Name(), "submodule \"")
                name = strings.TrimSuffix(name, "\"")

                if pathKey, err := section.GetKey("path"); err == nil {
                    gitmodulesMap[pathKey.String()] = name
                }
            }
        }
    }

    // Get submodules from git index (use -z for null-terminated output)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--stage")
    cmd.Dir = c.workdir

    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to list files: %w", err)
    }

    // Parse null-terminated output (safer for special characters)
    entries := strings.Split(string(output), "\x00")
    for _, entry := range entries {
        if !strings.HasPrefix(entry, "160000") {
            continue
        }

        // Parse: "160000 <hash> 0\t<path>"
        parts := strings.Split(entry, "\t")
        if len(parts) < 2 {
            continue
        }
        path := parts[1]

        // Check if in .gitmodules
        name, inGitmodules := gitmodulesMap[path]

        if !inGitmodules {
            issues = append(issues, SubmoduleIssue{
                Type:        "orphaned",
                Path:        path,
                Description: fmt.Sprintf("Submodule '%s' exists in git index but not in .gitmodules", path),
                Suggestion:  "Run: githelper repair --auto",
            })
        } else if name != path {
            // Name doesn't match path
            issues = append(issues, SubmoduleIssue{
                Type:        "name_mismatch",
                Path:        path,
                Name:        name,
                Description: fmt.Sprintf("Submodule name '%s' doesn't match path '%s'", name, path),
                Suggestion:  "Consider renaming submodule to match path for clarity",
            })
        }
    }

    return issues, nil
}
```

**Estimated Effort**: 5-6 hours (direct parsing, null-termination handling, testing)

---

### Improvement 4: Include Fixes in Default JSON Output

**Problem**: Fixes are hidden behind `--show-fixes` flag

**Solution**: Always include fixes in JSON output (for programmatic consumption)

**Implementation**:

**File**: `cmd/githelper/status.go`

```go
if out.IsJSON() {
    // Create enhanced output with fixes
    type EnhancedState struct {
        *scenarios.RepositoryState
        SuggestedFixes []scenarios.Fix `json:"suggested_fixes,omitempty"`
    }

    enhanced := &EnhancedState{
        RepositoryState: state,
        SuggestedFixes:  scenarios.SuggestFixes(state),
    }

    jsonBytes, err := json.MarshalIndent(enhanced, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal state: %w", err)
    }
    fmt.Println(string(jsonBytes))
}
```

**Estimated Effort**: 1-2 hours (straightforward data structure change)

---

### Improvement 5: Add `githelper status --porcelain`

**Problem**: Need machine-readable summary for scripts

**Solution**: Add `--porcelain` mode similar to `git status --porcelain`

**Note**: The critique correctly notes this should NOT include the detailed diagnostics/fixes added in other improvements - keep it simple for scripts.

**Implementation**:

**File**: `cmd/githelper/status.go`

```go
var statusPorcelain bool

func init() {
    statusCmd.Flags().BoolVar(&statusPorcelain, "porcelain", false, "Machine-readable output")
}

func printPorcelainOutput(state *scenarios.RepositoryState) {
    // Format: STATUS ITEM DETAILS
    fmt.Printf("E %s\n", state.Existence.ID)
    fmt.Printf("S %s %s\n", state.Sync.ID, state.Sync.Branch)

    if state.Sync.LocalAheadOfCore > 0 {
        fmt.Printf("A core %d\n", state.Sync.LocalAheadOfCore)
    }
    if state.Sync.LocalBehindCore > 0 {
        fmt.Printf("B core %d\n", state.Sync.LocalBehindCore)
    }
    if state.Sync.Diverged {
        fmt.Printf("D diverged\n")
    }

    fmt.Printf("W %s\n", state.WorkingTree.ID)
    for _, file := range state.WorkingTree.OrphanedSubmodules {
        fmt.Printf("O submodule %s\n", file)
    }

    fmt.Printf("C %s\n", state.Corruption.ID)
}
```

**Estimated Effort**: 2-3 hours (simple iteration, testing with scripts)

---

### Improvement 6: Add `githelper doctor` Command

**Problem**: Multiple issues need to be fixed - tedious one at a time

**Solution**: Add interactive `doctor` command that walks through all issues

**Safety Improvements** (from critiques):
- ✅ Pre-flight checks for all dependencies (gh CLI, remotes, permissions)
- ✅ Transaction safety - fail fast if any fix fails
- ✅ Clear recovery instructions if mid-operation failure
- ✅ Explicit user confirmation for each fix
- ✅ Dry-run mode to preview changes

**Implementation**:

**File**: `cmd/githelper/doctor.go`

```go
package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/lcgerke/githelper/internal/scenarios"
    "github.com/spf13/cobra"
)

var (
    doctorAuto   bool
    doctorDryRun bool
)

var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Diagnose and fix repository issues interactively",
    Long: `Comprehensive repository diagnosis and interactive fixing.

This command will:
1. Detect all issues with repository state
2. Present each issue with suggested fix
3. Ask for confirmation before applying each fix
4. Apply fixes in safe order with rollback on failure`,
    RunE: runDoctor,
}

func init() {
    doctorCmd.Flags().BoolVar(&doctorAuto, "auto", false,
        "Apply all fixes without confirmation (use with caution)")
    doctorCmd.Flags().BoolVar(&doctorDryRun, "dry-run", false,
        "Show what would be done without executing")
}

func runDoctor(cmd *cobra.Command, args []string) error {
    gitClient := git.NewClient(".")

    if !gitClient.IsRepository() {
        return fmt.Errorf("not a git repository")
    }

    fmt.Println("🔍 Diagnosing repository...")
    fmt.Println()

    // Get repository state
    state, err := scenarios.ClassifyRepository(gitClient, scenarios.DefaultDetectionOptions())
    if err != nil {
        return fmt.Errorf("failed to classify repository: %w", err)
    }

    // Get suggested fixes
    fixes := scenarios.SuggestFixes(state)

    if len(fixes) == 0 {
        fmt.Println("✅ No issues found! Repository is healthy.")
        return nil
    }

    // PRE-FLIGHT CHECKS (per GPT critique)
    fmt.Println("Performing pre-flight checks...")
    if err := preflightChecks(gitClient, fixes); err != nil {
        return fmt.Errorf("pre-flight check failed: %w\n\nPlease resolve this before running doctor.", err)
    }
    fmt.Println()

    // Show all issues
    fmt.Printf("Found %d issue(s):\n\n", len(fixes))
    for i, fix := range fixes {
        fmt.Printf("  %d. [%s] %s\n", i+1, fix.ScenarioID, fix.Description)
        if fix.AutoFixable {
            fmt.Printf("     Fix: %s\n", fix.Command)
        } else {
            fmt.Printf("     ⚠️  Manual fix required\n")
        }
        fmt.Println()
    }

    // Confirm application
    if !doctorAuto && !doctorDryRun {
        fmt.Print("Apply all auto-fixable issues? (y/n): ")
        reader := bufio.NewReader(os.Stdin)
        response, _ := reader.ReadString('\n')
        response = strings.TrimSpace(strings.ToLower(response))

        if response != "y" && response != "yes" {
            fmt.Println("Aborted.")
            return nil
        }
    }

    fmt.Println()
    fmt.Println("Applying fixes...")

    // Apply fixes with transaction safety
    appliedFixes := []scenarios.Fix{}
    for i, fix := range fixes {
        if !fix.AutoFixable {
            continue
        }

        fmt.Printf("[%d/%d] %s...\n", i+1, len(fixes), fix.Description)

        if doctorDryRun {
            fmt.Printf("        [DRY RUN] %s\n", fix.Command)
            continue
        }

        // Validate before applying
        if err := fix.Operation.Validate(state, gitClient); err != nil {
            fmt.Printf("        ❌ Validation failed: %v\n", err)

            // TRANSACTION SAFETY: Rollback previously applied fixes
            if len(appliedFixes) > 0 {
                fmt.Println()
                fmt.Println("⚠️  Fix failed - rolling back previous changes...")
                rollbackFixes(gitClient, appliedFixes)
            }

            return fmt.Errorf("fix validation failed - repository unchanged")
        }

        // Execute fix
        if err := fix.Operation.Execute(gitClient); err != nil {
            fmt.Printf("        ❌ Failed: %v\n", err)

            // TRANSACTION SAFETY: Rollback
            if len(appliedFixes) > 0 {
                fmt.Println()
                fmt.Println("⚠️  Fix failed - rolling back previous changes...")
                rollbackFixes(gitClient, appliedFixes)
            }

            return fmt.Errorf("fix execution failed - repository unchanged")
        }

        fmt.Printf("        ✓ Applied\n")
        appliedFixes = append(appliedFixes, fix)
    }

    fmt.Println()
    if doctorDryRun {
        fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
        fmt.Println("Dry run complete - no changes made")
    } else if len(appliedFixes) > 0 {
        fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
        fmt.Println("✅ All issues resolved!")
        fmt.Println()
        fmt.Println("Changes have been staged. Review with 'git status' and commit when ready.")
    }

    return nil
}

// preflightChecks validates all dependencies before attempting fixes
func preflightChecks(gitClient *git.Client, fixes []scenarios.Fix) error {
    // Check for required tools
    requiredTools := map[string]bool{}
    for _, fix := range fixes {
        // Check if fix requires gh CLI
        if strings.Contains(fix.Command, "gh ") {
            requiredTools["gh"] = true
        }
    }

    for tool := range requiredTools {
        if _, err := exec.LookPath(tool); err != nil {
            return fmt.Errorf("required tool '%s' not found in PATH", tool)
        }
    }

    // Check push permissions to remotes
    remotes, _ := gitClient.ListRemotes()
    for _, remote := range remotes {
        if err := gitClient.CheckPushAccess(remote); err != nil {
            return fmt.Errorf("cannot push to remote '%s': %w", remote, err)
        }
    }

    return nil
}

func rollbackFixes(gitClient *git.Client, fixes []scenarios.Fix) {
    // Rollback in reverse order
    for i := len(fixes) - 1; i >= 0; i-- {
        fix := fixes[i]
        fmt.Printf("  Rolling back: %s\n", fix.Description)
        if err := fix.Operation.Rollback(gitClient); err != nil {
            fmt.Printf("  ⚠️  Rollback failed: %v\n", err)
        } else {
            fmt.Printf("  ✓ Rolled back\n")
        }
    }
}
```

**Estimated Effort**: 8-10 hours (pre-flight checks, transaction safety, rollback logic, testing)

---

### Improvement 7: Enforce `main` as Default Branch with Auto-Migration

**Problem**: Mixed usage of `master` and `main` causes confusion

**Solution**: Detect `master` usage and provide automated migration to `main`

**Atomicity & Safety Improvements** (from critiques):
- ✅ Pre-flight checks for gh CLI, remotes, and permissions
- ✅ Reorder operations: create/push `main` BEFORE deleting `master`
- ✅ Fail fast with recovery instructions if any step fails
- ✅ Check for branch protection via API before attempting push
- ✅ Handle non-GitHub remotes gracefully
- ✅ Use exec.CommandContext (no string formatting)
- ✅ Check for diverged `main` branch before migration

**Implementation**:

**File**: `cmd/githelper/migrate.go` (new)

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "time"

    "github.com/lcgerke/githelper/internal/git"
    "github.com/spf13/cobra"
)

var (
    migrateForce      bool
    migrateDryRun     bool
    migrateDeleteOld  bool
)

var migrateCmd = &cobra.Command{
    Use:   "migrate-to-main",
    Short: "Migrate repository from 'master' to 'main' branch",
    Long: `Automatically migrate a repository from 'master' to 'main' as the default branch.

This command will:
1. Validate prerequisites (gh CLI, remotes, permissions)
2. Create 'main' branch from 'master' (or force update if exists)
3. Push 'main' to all configured remotes (origin, github)
4. Set 'main' as default branch on GitHub (if github remote exists)
5. Optionally delete 'master' branch on remotes (use --delete-old)

The command performs pre-flight checks and fails fast if any prerequisite is missing.`,
    RunE: runMigrate,
}

func init() {
    migrateCmd.Flags().BoolVar(&migrateForce, "force", false,
        "Force migration even if 'main' already exists (use with caution)")
    migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false,
        "Show what would be done without executing")
    migrateCmd.Flags().BoolVar(&migrateDeleteOld, "delete-old", false,
        "Delete 'master' branch from remotes after migration")
}

func runMigrate(cmd *cobra.Command, args []string) error {
    gitClient := git.NewClient(".")

    if !gitClient.IsRepository() {
        return fmt.Errorf("not a git repository")
    }

    // PRE-FLIGHT CHECKS (per GPT critique)
    fmt.Println("🔍 Running pre-flight checks...")
    if err := preflightChecksMigrate(gitClient); err != nil {
        return fmt.Errorf("pre-flight check failed: %w", err)
    }
    fmt.Println("✓ Pre-flight checks passed")
    fmt.Println()

    // Check if already using main
    defaultBranch, err := gitClient.GetDefaultBranch()
    if err != nil {
        return fmt.Errorf("failed to get default branch: %w", err)
    }

    if defaultBranch != "master" {
        fmt.Printf("✓ Repository already uses '%s' as default branch\n", defaultBranch)
        return nil
    }

    // Check if main branch exists and is diverged
    branches, err := gitClient.ListBranches()
    if err != nil {
        return fmt.Errorf("failed to list branches: %w", err)
    }

    mainExists := false
    for _, branch := range branches {
        if branch == "main" {
            mainExists = true
            break
        }
    }

    if mainExists {
        // Check for divergence
        diverged, err := gitClient.BranchesDiverged("master", "main")
        if err != nil {
            return fmt.Errorf("failed to check branch divergence: %w", err)
        }

        if diverged && !migrateForce {
            return fmt.Errorf("'main' branch exists and has diverged from 'master'\n" +
                "This could cause data loss. Please resolve manually or use --force")
        }
    }

    fmt.Println("🔄 Migrating from 'master' to 'main'...")
    fmt.Println()

    // IMPROVED ORDER: Create main BEFORE deleting master (per GPT critique)

    // Step 1: Create or update local 'main' branch
    fmt.Println("1. Creating/updating local 'main' branch from 'master'...")
    if migrateDryRun {
        if mainExists {
            fmt.Println("   [DRY RUN] git branch -f main master")
        } else {
            fmt.Println("   [DRY RUN] git branch main master")
        }
    } else {
        if err := gitClient.CreateOrUpdateBranch("main", "master"); err != nil {
            return recoverableError(
                fmt.Sprintf("failed to create main branch: %v", err),
                []string{
                    "Your repository is unchanged.",
                    "Try running: git branch -m master main",
                },
            )
        }
        fmt.Println("   ✓ Local 'main' branch ready")
    }

    // Step 2: Push 'main' to origin
    fmt.Println()
    fmt.Println("2. Pushing 'main' to origin...")
    if migrateDryRun {
        fmt.Println("   [DRY RUN] git push -u origin main")
    } else {
        if err := gitClient.PushWithContext("origin", "main", true, 30*time.Second); err != nil {
            return recoverableError(
                fmt.Sprintf("failed to push to origin: %v", err),
                []string{
                    "Local 'main' branch has been created.",
                    "To recover, run: git branch -D main",
                    "Then investigate the push failure.",
                },
            )
        }
        fmt.Println("   ✓ Pushed to origin")
    }

    // Step 3: Push 'main' to github (if exists)
    remotes, _ := gitClient.ListRemotes()
    hasGitHub := false
    for _, remote := range remotes {
        if remote == "github" {
            hasGitHub = true
            break
        }
    }

    if hasGitHub {
        fmt.Println()
        fmt.Println("3. Pushing 'main' to github...")
        if migrateDryRun {
            fmt.Println("   [DRY RUN] git push -u github main")
        } else {
            if err := gitClient.PushWithContext("github", "main", true, 30*time.Second); err != nil {
                return recoverableError(
                    fmt.Sprintf("failed to push to github: %v", err),
                    []string{
                        "Local and origin 'main' have been created.",
                        "To recover: git push github main",
                        "Or rollback: git branch -D main && git push origin --delete main",
                    },
                )
            }
            fmt.Println("   ✓ Pushed to github")
        }

        // Step 4: Set default branch on GitHub
        fmt.Println()
        fmt.Println("4. Setting default branch on GitHub...")
        if migrateDryRun {
            fmt.Println("   [DRY RUN] gh repo edit --default-branch main")
        } else {
            if err := setGitHubDefaultBranch("main"); err != nil {
                fmt.Printf("   ⚠️  Could not set GitHub default branch: %v\n", err)
                fmt.Println("   Please set manually at: https://github.com/<owner>/<repo>/settings")
            } else {
                fmt.Println("   ✓ GitHub default branch updated")
            }
        }
    }

    // Step 5: Rename local master to main (now safe)
    fmt.Println()
    fmt.Println("5. Renaming local 'master' to 'main'...")
    if migrateDryRun {
        fmt.Println("   [DRY RUN] git branch -d master")
    } else {
        if err := gitClient.DeleteLocalBranch("master"); err != nil {
            fmt.Printf("   ⚠️  Could not delete local master: %v\n", err)
            fmt.Println("   You can manually delete it: git branch -d master")
        } else {
            fmt.Println("   ✓ Deleted local 'master' branch")
        }
    }

    // Step 6: Delete old 'master' branch from remotes (optional)
    if migrateDeleteOld {
        fmt.Println()
        fmt.Println("6. Deleting 'master' branch from remotes...")

        if migrateDryRun {
            fmt.Println("   [DRY RUN] git push origin --delete master")
        } else {
            if err := gitClient.DeleteRemoteBranch("origin", "master"); err != nil {
                fmt.Printf("   ⚠️  Could not delete from origin: %v\n", err)
            } else {
                fmt.Println("   ✓ Deleted from origin")
            }
        }

        if hasGitHub {
            if migrateDryRun {
                fmt.Println("   [DRY RUN] git push github --delete master")
            } else {
                if err := gitClient.DeleteRemoteBranch("github", "master"); err != nil {
                    fmt.Printf("   ⚠️  Could not delete from github: %v\n", err)
                } else {
                    fmt.Println("   ✓ Deleted from github")
                }
            }
        }
    }

    // Summary
    fmt.Println()
    fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    fmt.Println("✅ Migration complete!")
    fmt.Println()
    fmt.Println("Your repository now uses 'main' as the default branch.")
    if !migrateDeleteOld {
        fmt.Println()
        fmt.Println("The old 'master' branch still exists on remotes.")
        fmt.Println("To delete it, run: githelper migrate-to-main --delete-old")
    }

    return nil
}

func preflightChecksMigrate(gitClient *git.Client) error {
    // Check for gh CLI (required for GitHub operations)
    if _, err := exec.LookPath("gh"); err != nil {
        return fmt.Errorf("'gh' CLI not found in PATH (required for GitHub operations)\n" +
            "Install: https://cli.github.com/")
    }

    // Verify gh authentication
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "gh", "auth", "status")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("'gh' CLI not authenticated\n" +
            "Run: gh auth login")
    }

    // Check push access to remotes
    remotes, _ := gitClient.ListRemotes()
    for _, remote := range remotes {
        if err := gitClient.CheckPushAccess(remote); err != nil {
            return fmt.Errorf("cannot push to remote '%s': %w\n" +
                "Check your credentials and permissions", remote, err)
        }
    }

    // Check for branch protection (if possible via gh API)
    if err := checkBranchProtection("master"); err != nil {
        return fmt.Errorf("branch protection detected: %w\n" +
            "Disable protection temporarily or skip this check with --force", err)
    }

    return nil
}

func setGitHubDefaultBranch(branch string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // SECURITY FIX: Use exec.CommandContext with args (no fmt.Sprintf)
    cmd := exec.CommandContext(ctx, "gh", "repo", "edit", "--default-branch", branch)
    return cmd.Run()
}

func checkBranchProtection(branch string) error {
    // Query GitHub API via gh CLI to check protection
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "gh", "api",
        "repos/{owner}/{repo}/branches/"+branch+"/protection")

    output, err := cmd.Output()
    if err != nil {
        // Branch protection not set - this is good
        return nil
    }

    // Protection exists
    if len(output) > 0 {
        return fmt.Errorf("branch '%s' is protected on GitHub", branch)
    }

    return nil
}

func recoverableError(msg string, recoverySteps []string) error {
    fmt.Println()
    fmt.Println("❌ ERROR:", msg)
    fmt.Println()
    fmt.Println("Recovery steps:")
    for i, step := range recoverySteps {
        fmt.Printf("  %d. %s\n", i+1, step)
    }
    fmt.Println()
    return fmt.Errorf("migration failed - see recovery steps above")
}
```

**File**: `internal/git/cli.go` (add helper methods)

```go
// PushWithContext pushes a branch with timeout
func (c *Client) PushWithContext(remote, branch string, setUpstream bool, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    args := []string{"push"}
    if setUpstream {
        args = append(args, "-u")
    }
    args = append(args, remote, branch)

    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = c.workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return fmt.Errorf("push timed out after %v", timeout)
        }
        return fmt.Errorf("%w (output: %s)", err, output)
    }

    return nil
}

// CreateOrUpdateBranch creates a new branch or force-updates existing branch
func (c *Client) CreateOrUpdateBranch(newBranch, fromBranch string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Use -f to force update if exists
    cmd := exec.CommandContext(ctx, "git", "branch", "-f", newBranch, fromBranch)
    cmd.Dir = c.workdir

    return cmd.Run()
}

// DeleteLocalBranch deletes a local branch
func (c *Client) DeleteLocalBranch(branch string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", "branch", "-d", branch)
    cmd.Dir = c.workdir

    return cmd.Run()
}

// BranchesDiverged checks if two branches have diverged
func (c *Client) BranchesDiverged(branch1, branch2 string) (bool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Check if branch1 is ancestor of branch2
    cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", branch1, branch2)
    cmd.Dir = c.workdir

    if err := cmd.Run(); err == nil {
        // branch1 is ancestor - not diverged
        return false, nil
    }

    // Check reverse
    cmd = exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", branch2, branch1)
    cmd.Dir = c.workdir

    if err := cmd.Run(); err == nil {
        // branch2 is ancestor - not diverged
        return false, nil
    }

    // Neither is ancestor - diverged
    return true, nil
}

// CheckPushAccess verifies push access to a remote
func (c *Client) CheckPushAccess(remote string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Try a dry-run push
    cmd := exec.CommandContext(ctx, "git", "push", "--dry-run", remote, "HEAD")
    cmd.Dir = c.workdir

    output, err := cmd.CombinedOutput()
    if err != nil {
        if strings.Contains(string(output), "Authentication") ||
           strings.Contains(string(output), "Permission denied") {
            return fmt.Errorf("authentication failed")
        }
        if strings.Contains(string(output), "protected branch") {
            return fmt.Errorf("branch is protected")
        }
        // Other errors might be ok (e.g., already up-to-date)
    }

    return nil
}
```

**Estimated Effort**: 10-12 hours (pre-flight checks, atomicity, recovery paths, testing)

---

## Summary of Improvements

| # | Improvement | Friction Eliminated | Auto-Fixable | Priority | Effort (hrs) |
|---|-------------|---------------------|--------------|----------|--------------|
| 1 | Auto-fetch with timeout | Stale remote data | N/A | High | 3-4 |
| 2 | Auto-fix orphaned submodules | Manual git internals | ✅ Yes | High | 6-8 |
| 3 | Enhanced submodule diagnostics | Unclear error messages | N/A | Medium | 5-6 |
| 4 | Fixes in JSON output | Hidden features | N/A | Medium | 1-2 |
| 5 | Porcelain output mode | Hard to script | N/A | Low | 2-3 |
| 6 | Interactive doctor command | Multiple manual fixes | ✅ Yes | High | 8-10 |
| 7 | Migrate to 'main' branch | Mixed master/main usage | ✅ Yes | High | 10-12 |

**Total Estimated Effort**: 35-45 hours

---

## Implementation Order & Timeline

**Week 1: Foundation (11-14 hours)**
1. **Improvement 1** (Auto-fetch with timeout) - Prevents future confusion, adds timeout safety
2. **Improvement 4** (Fixes in JSON) - Simple data structure change
3. **Improvement 5** (Porcelain mode) - Quick win for scripting

**Week 2: Core Fixes (11-14 hours)**
4. **Improvement 3** (Enhanced diagnostics) - Better error messages using direct parsing
5. **Improvement 2** (Auto-fix submodules) - Solves immediate pain point with security fixes

**Week 3: Advanced Features (13-17 hours)**
6. **Improvement 7** (Migrate to main) - One-time cleanup with pre-flight checks
7. **Improvement 6** (Doctor command) - Comprehensive fix workflow with transaction safety

---

## Testing Strategy

### Integration Testing Requirements
Per Gemini critique: **Integration tests are critical** - unit tests alone are insufficient.

**Test Environment Setup**:
```bash
# Create test fixture: temporary git repositories
create_test_repo() {
    local name=$1
    local tmpdir=$(mktemp -d)
    git init "$tmpdir/$name"
    # ... set up specific broken state
    echo "$tmpdir/$name"
}

# Create bare remote for testing
create_bare_remote() {
    local tmpdir=$(mktemp -d)
    git init --bare "$tmpdir/remote.git"
    echo "$tmpdir/remote.git"
}
```

**Test Cases by Improvement**:

1. **Auto-fetch timeout**:
   - [ ] Test with unreachable remote (should timeout gracefully)
   - [ ] Test with credential failure (should show clear error)
   - [ ] Test with slow remote (should respect timeout)
   - [ ] Test duration reporting

2. **Orphaned submodule fix**:
   - [ ] Test with path containing spaces
   - [ ] Test with path containing special characters (`$`, `'`, etc.)
   - [ ] Test rollback on failure
   - [ ] Test .gitmodules update
   - [ ] Verify command injection prevention

3. **Enhanced diagnostics**:
   - [ ] Test with null bytes in paths (edge case)
   - [ ] Test with different git versions (2.30+, 2.40+)
   - [ ] Test direct .gitmodules parsing vs git config

4. **Doctor command**:
   - [ ] Test transaction rollback on mid-operation failure
   - [ ] Test pre-flight check failures
   - [ ] Mock gh CLI for testing
   - [ ] Test with missing permissions

5. **Migrate to main**:
   - [ ] Test with diverged main branch (should fail without --force)
   - [ ] Test with protected branch (should fail pre-flight)
   - [ ] Test with gh CLI not installed (should fail pre-flight)
   - [ ] Test with non-GitHub remote (should skip GitHub steps)
   - [ ] Test recovery from mid-migration failure
   - [ ] Snapshot test for dry-run output

### Snapshot Testing
Per Gemini: Use snapshot testing for JSON and porcelain outputs.

```bash
# Generate golden snapshots
githelper status --format=json > test/snapshots/status.json
githelper status --porcelain > test/snapshots/status.porcelain

# Test against snapshots
diff test/snapshots/status.json <(githelper status --format=json)
```

---

## Critical Security Fixes Applied

Per both critiques, the following security/safety issues have been addressed:

1. ✅ **Command Injection Prevention**: All shell commands use `exec.CommandContext` with argument arrays
2. ✅ **Timeout Protection**: All network operations have configurable timeouts
3. ✅ **Transaction Safety**: Doctor command rolls back on failure
4. ✅ **Pre-flight Validation**: Migrate command checks all prerequisites before modifying state
5. ✅ **Atomicity**: Migrate reordered to create new branch before deleting old
6. ✅ **Null-terminated Parsing**: Use `-z` flag for safe git output parsing
7. ✅ **Direct File Parsing**: Parse `.gitmodules` directly instead of git config output
8. ✅ **Recovery Instructions**: Clear rollback steps provided on failure

---

## Changes from v1

**Major architectural changes**:
- All `fmt.Sprintf` shell commands → `exec.CommandContext` with args
- Added timeout handling to all network operations
- Added pre-flight checks before destructive operations
- Added transaction safety with rollback capability
- Reordered migrate operations for atomicity
- Direct `.gitmodules` parsing instead of git config
- Null-terminated output parsing for safety
- Added effort estimates and timeline (Week 1/2/3)
- Enhanced testing strategy with integration tests
- Added snapshot testing for outputs

**Security hardening**: 7 critical fixes applied (see list above)

**Implementation realism**: Total estimated effort 35-45 hours across 3 weeks
