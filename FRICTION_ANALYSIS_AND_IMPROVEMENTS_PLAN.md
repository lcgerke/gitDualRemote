# Githelper Friction Analysis and Improvement Proposals

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

**Implementation**:

**File**: `cmd/githelper/status.go` (lines 41-45)

```go
// Current (BEFORE):
statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false, "Skip fetching from remotes")

// Proposed (AFTER):
statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false, "Skip fetching from remotes (default: auto-fetch)")

// In runStatus() function (line 78):
// Change default
options := scenarios.DefaultDetectionOptions()
options.SkipFetch = statusNoFetch  // Already correct - just change default messaging
```

**File**: `internal/scenarios/types.go` (line 195)

```go
// Current default:
SkipFetch: false,  // Already correct

// But add logging in classifier.go:
if !options.SkipFetch {
    fmt.Fprintln(os.Stderr, "Fetching from remotes...")
}
```

**User Experience**:
```bash
$ githelper status
Fetching from remotes...
🔍 Analyzing repository state...
# Always shows fresh data

$ githelper status --no-fetch  # Opt-out for speed
🔍 Analyzing repository state...
# Uses cached data (fast but potentially stale)
```

**Benefit**: Eliminates "git status says X, githelper says Y" inconsistencies

---

### Improvement 2: Auto-Fix for Orphaned Submodules

**Problem**: Orphaned submodules require manual intervention

**Solution**: Implement auto-fix operation for orphaned submodules

**Implementation**:

**File**: `internal/scenarios/operations.go` (create if doesn't exist)

```go
package scenarios

import (
    "fmt"
    "github.com/lcgerke/githelper/internal/git"
)

// RemoveOrphanedSubmoduleOp removes a submodule from git index
type RemoveOrphanedSubmoduleOp struct {
    SubmodulePath string
}

func (op *RemoveOrphanedSubmoduleOp) Validate(state *RepositoryState, gitClient interface{}) error {
    // Check that working tree is clean (no uncommitted changes in submodule)
    if !state.WorkingTree.Clean {
        return fmt.Errorf("working tree must be clean before removing submodule")
    }
    return nil
}

func (op *RemoveOrphanedSubmoduleOp) Execute(gitClient interface{}) error {
    client := gitClient.(*git.Client)

    // Remove from index only (--cached)
    _, err := client.Run("rm", "--cached", op.SubmodulePath)
    if err != nil {
        return fmt.Errorf("failed to remove submodule from index: %w", err)
    }

    return nil
}

func (op *RemoveOrphanedSubmoduleOp) Describe() string {
    return fmt.Sprintf("Remove orphaned submodule '%s' from git index", op.SubmodulePath)
}

func (op *RemoveOrphanedSubmoduleOp) Rollback(gitClient interface{}) error {
    // Cannot safely rollback - would need to know original commit hash
    return fmt.Errorf("rollback not supported for submodule removal")
}

// FixSubmoduleNameOp fixes .gitmodules name to match path
type FixSubmoduleNameOp struct {
    OldName string
    NewName string // Should match path
    Path    string
}

func (op *FixSubmoduleNameOp) Execute(gitClient interface{}) error {
    client := gitClient.(*git.Client)

    // Use git config to rename
    _, err := client.Run("config", "--file", ".gitmodules",
                        "--rename-section",
                        fmt.Sprintf("submodule.%s", op.OldName),
                        fmt.Sprintf("submodule.%s", op.NewName))
    if err != nil {
        return fmt.Errorf("failed to rename submodule in .gitmodules: %w", err)
    }

    return nil
}

func (op *FixSubmoduleNameOp) Describe() string {
    return fmt.Sprintf("Rename submodule '%s' to '%s' in .gitmodules", op.OldName, op.NewName)
}
```

**File**: `internal/scenarios/suggester.go` (add to existing)

```go
// Add to SuggestFixes function:
func SuggestFixes(state *RepositoryState) []Fix {
    var fixes []Fix

    // ... existing fixes ...

    // NEW: Orphaned submodules
    if len(state.WorkingTree.OrphanedSubmodules) > 0 {
        for _, submodule := range state.WorkingTree.OrphanedSubmodules {
            fixes = append(fixes, Fix{
                ScenarioID:  "W2",
                Description: fmt.Sprintf("Remove orphaned submodule '%s'", submodule),
                Command:     fmt.Sprintf("git rm --cached %s", submodule),
                Operation:   &RemoveOrphanedSubmoduleOp{SubmodulePath: submodule},
                AutoFixable: true,
                Priority:    3,
                Reason:      "Submodule is in git index but not in .gitmodules",
            })
        }
    }

    return fixes
}
```

**User Experience**:
```bash
$ githelper status
⚠️  Warnings:
  W2: Orphaned submodule detected
    Hint: Run 'githelper repair --auto' to remove

$ githelper repair --auto
Applying fix: Remove orphaned submodule 'ansible/06-testing-reference/...'
✓ Removed orphaned submodule
Changes staged. Run 'git commit' to finalize.

$ git commit -m "fix: Remove orphaned submodule"
```

**Benefit**: One-command fix instead of manual git internals wrangling

---

### Improvement 3: Enhanced Orphaned Submodule Detection

**Problem**: "Orphaned" doesn't explain name/path mismatch

**Solution**: Add detailed diagnostics for submodule issues

**Implementation**:

**File**: `internal/git/submodules.go` (create new)

```go
package git

import (
    "fmt"
    "strings"
)

type SubmoduleIssue struct {
    Type        string // "orphaned", "name_mismatch", "missing_in_worktree"
    Path        string
    Name        string
    Description string
    Suggestion  string
}

// DiagnoseSubmodules performs comprehensive submodule analysis
func (c *Client) DiagnoseSubmodules() ([]SubmoduleIssue, error) {
    var issues []SubmoduleIssue

    // Get submodules from .gitmodules
    gitmodulesMap := make(map[string]string) // path -> name
    output, err := c.Run("config", "--file", ".gitmodules", "--get-regexp", "path")
    if err == nil {
        lines := strings.Split(output, "\n")
        for _, line := range lines {
            parts := strings.Fields(line)
            if len(parts) == 2 {
                // submodule.NAME.path PATH
                name := strings.TrimPrefix(parts[0], "submodule.")
                name = strings.TrimSuffix(name, ".path")
                path := parts[1]
                gitmodulesMap[path] = name
            }
        }
    }

    // Get submodules from git index
    output, err = c.Run("ls-files", "--stage")
    if err != nil {
        return nil, fmt.Errorf("failed to list files: %w", err)
    }

    lines := strings.Split(output, "\n")
    for _, line := range lines {
        if !strings.HasPrefix(line, "160000") {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }
        path := fields[3]

        // Check if in .gitmodules
        name, inGitmodules := gitmodulesMap[path]

        if !inGitmodules {
            issues = append(issues, SubmoduleIssue{
                Type:        "orphaned",
                Path:        path,
                Description: fmt.Sprintf("Submodule '%s' exists in git index but not in .gitmodules", path),
                Suggestion:  fmt.Sprintf("Run: git rm --cached %s", path),
            })
        } else if name != path {
            // Name doesn't match path
            issues = append(issues, SubmoduleIssue{
                Type:        "name_mismatch",
                Path:        path,
                Name:        name,
                Description: fmt.Sprintf("Submodule name '%s' doesn't match path '%s'", name, path),
                Suggestion:  fmt.Sprintf("Consider renaming submodule to match path for clarity"),
            })
        }
    }

    return issues, nil
}
```

**File**: `internal/scenarios/types.go` (update WorkingTreeState)

```go
type WorkingTreeState struct {
    ID          string `json:"id"`
    Description string `json:"description"`

    Clean              bool     `json:"clean"`
    StagedFiles        []string `json:"staged_files,omitempty"`
    UnstagedFiles      []string `json:"unstaged_files,omitempty"`
    UntrackedFiles     []string `json:"untracked_files,omitempty"`
    ConflictFiles      []string `json:"conflict_files,omitempty"`
    OrphanedSubmodules []string `json:"orphaned_submodules,omitempty"`

    // NEW: Detailed submodule diagnostics
    SubmoduleIssues []SubmoduleIssue `json:"submodule_issues,omitempty"`
}

type SubmoduleIssue struct {
    Type        string `json:"type"`        // "orphaned", "name_mismatch", etc.
    Path        string `json:"path"`
    Name        string `json:"name,omitempty"`
    Description string `json:"description"`
    Suggestion  string `json:"suggestion"`
}
```

**User Experience**:
```bash
$ githelper status --format=json | jq '.working_tree.submodule_issues'
[
  {
    "type": "orphaned",
    "path": "ansible/06-testing-reference/ansible-test/build_context/prezto",
    "description": "Submodule exists in git index but not in .gitmodules",
    "suggestion": "Run: git rm --cached ansible/06-testing-reference/..."
  },
  {
    "type": "name_mismatch",
    "path": "ansible/04-specialized/jenkins-ci/external/awx",
    "name": "31utilityScripts/jenkinsSetup2/external/awx",
    "description": "Submodule name '31utilityScripts/...' doesn't match path 'ansible/04-specialized/...'",
    "suggestion": "Consider renaming submodule to match path for clarity"
  }
]
```

**Benefit**: Clear explanation of what's wrong and how to fix it

---

### Improvement 4: Include Fixes in Default JSON Output

**Problem**: Fixes are hidden behind `--show-fixes` flag

**Solution**: Always include fixes in JSON output (for programmatic consumption)

**Implementation**:

**File**: `cmd/githelper/status.go` (line 96-106)

```go
// Current (BEFORE):
if out.IsJSON() {
    // JSON output
    jsonBytes, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal state: %w", err)
    }
    fmt.Println(string(jsonBytes))
}

// Proposed (AFTER):
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

**User Experience**:
```bash
$ githelper status --format=json
{
  "repo_path": "/home/lcgerke/wk",
  "existence": { ... },
  "sync": { ... },
  "working_tree": {
    "orphaned_submodules": ["ansible/06-testing-reference/..."]
  },
  "suggested_fixes": [
    {
      "scenario_id": "W2",
      "description": "Remove orphaned submodule 'ansible/06-testing-reference/...'",
      "command": "git rm --cached ansible/06-testing-reference/...",
      "auto_fixable": true,
      "priority": 3,
      "reason": "Submodule is in git index but not in .gitmodules"
    }
  ]
}
```

**Benefit**: Programmatic access to fixes without separate flag

---

### Improvement 5: Add `githelper status --porcelain`

**Problem**: Need machine-readable summary for scripts

**Solution**: Add `--porcelain` mode similar to `git status --porcelain`

**Implementation**:

**File**: `cmd/githelper/status.go`

```go
var statusPorcelain bool

func init() {
    statusCmd.Flags().BoolVar(&statusNoFetch, "no-fetch", false, "Skip fetching from remotes")
    statusCmd.Flags().BoolVar(&statusQuick, "quick", false, "Skip corruption checks")
    statusCmd.Flags().BoolVar(&statusShowFixes, "show-fixes", false, "Show suggested fixes")
    statusCmd.Flags().BoolVar(&statusPorcelain, "porcelain", false, "Machine-readable output")
    statusCmd.Flags().StringVar(&statusCoreRemote, "core-remote", "origin", "Name of Core remote")
    statusCmd.Flags().StringVar(&statusGitHubRemote, "github-remote", "github", "Name of GitHub remote")
}

func runStatus(cmd *cobra.Command, args []string) error {
    // ... existing code ...

    if statusPorcelain {
        printPorcelainOutput(state)
        return nil
    }

    // ... existing output code ...
}

func printPorcelainOutput(state *scenarios.RepositoryState) {
    // Format: STATUS ITEM DETAILS

    // Existence
    fmt.Printf("E %s\n", state.Existence.ID)

    // Sync
    fmt.Printf("S %s %s\n", state.Sync.ID, state.Sync.Branch)
    if state.Sync.LocalAheadOfCore > 0 {
        fmt.Printf("A core %d\n", state.Sync.LocalAheadOfCore)
    }
    if state.Sync.LocalBehindCore > 0 {
        fmt.Printf("B core %d\n", state.Sync.LocalBehindCore)
    }
    if state.Sync.LocalAheadOfGitHub > 0 {
        fmt.Printf("A github %d\n", state.Sync.LocalAheadOfGitHub)
    }
    if state.Sync.LocalBehindGitHub > 0 {
        fmt.Printf("B github %d\n", state.Sync.LocalBehindGitHub)
    }
    if state.Sync.Diverged {
        fmt.Printf("D diverged\n")
    }

    // Working tree
    fmt.Printf("W %s\n", state.WorkingTree.ID)
    for _, file := range state.WorkingTree.OrphanedSubmodules {
        fmt.Printf("O submodule %s\n", file)
    }

    // Corruption
    fmt.Printf("C %s\n", state.Corruption.ID)
}
```

**User Experience**:
```bash
$ githelper status --porcelain
E E1
S S1 master
W W1
O submodule ansible/06-testing-reference/ansible-test/build_context/prezto
C C1

# Easy to parse in scripts:
if githelper status --porcelain | grep -q "^O submodule"; then
    echo "Orphaned submodules detected!"
fi
```

**Benefit**: Easy scripting and automation

---

### Improvement 6: Add `githelper doctor` Command

**Problem**: Multiple issues need to be fixed - tedious one at a time

**Solution**: Add interactive `doctor` command that walks through all issues

**Implementation**:

**File**: `cmd/githelper/doctor.go` (appears to exist already based on earlier find)

Enhance existing doctor command to:
1. Run comprehensive diagnosis
2. Show all issues with priority
3. Ask user to confirm each fix
4. Apply fixes in safe order
5. Commit changes with descriptive message

**User Experience**:
```bash
$ githelper doctor
🔍 Diagnosing repository...

Found 3 issues:

  1. [W2] Orphaned submodule 'ansible/06-testing-reference/...'
     Fix: git rm --cached ansible/06-testing-reference/...

  2. [W2] Submodule name mismatch '31utilityScripts/...' vs 'ansible/04-specialized/...'
     Fix: Rename submodule in .gitmodules

  3. [S3] Branch 'main' ahead of github by 4 commits
     Fix: git push github main

Apply all fixes? (y/n): y

Applying fixes...
✓ Removed orphaned submodule
✓ Fixed submodule name mismatch
✓ Pushed to github

All issues resolved! Run 'git commit' to finalize changes.
```

**Benefit**: One command to fix all issues instead of manual investigation

---

## Summary of Improvements

| # | Improvement | Friction Eliminated | Auto-Fixable | Priority |
|---|-------------|---------------------|--------------|----------|
| 1 | Auto-fetch before status | Stale remote data | N/A | High |
| 2 | Auto-fix orphaned submodules | Manual git internals | ✅ Yes | High |
| 3 | Enhanced submodule diagnostics | Unclear error messages | N/A | Medium |
| 4 | Fixes in JSON output | Hidden features | N/A | Medium |
| 5 | Porcelain output mode | Hard to script | N/A | Low |
| 6 | Interactive doctor command | Multiple manual fixes | ✅ Yes | High |
| 7 | Migrate to 'main' branch | Mixed master/main usage | ✅ Yes | High |

---

## Implementation Order

1. **Improvement 1** (Auto-fetch) - Prevents future confusion
2. **Improvement 7** (Migrate to main) - One-time cleanup, forces consistency
3. **Improvement 2** (Auto-fix submodules) - Solves immediate pain point
4. **Improvement 3** (Enhanced diagnostics) - Better error messages
5. **Improvement 6** (Doctor command) - Comprehensive fix workflow
6. **Improvement 4** (Fixes in JSON) - API enhancement
7. **Improvement 5** (Porcelain mode) - Nice to have

---

### Improvement 7: Enforce `main` as Default Branch with Auto-Migration

**Problem**: Mixed usage of `master` and `main` causes confusion and inconsistency

**Current Behavior**:
- Githelper works with both `master` and `main`
- User has repos with different default branch names
- Industry has moved to `main` but old repos still use `master`

**Friction**:
- Mental overhead tracking which repos use which default branch
- Inconsistent commands (sometimes `git push origin main`, sometimes `master`)
- Need to check default branch before running commands

**Solution**: Detect `master` usage and provide automated migration to `main`

**Implementation**:

**File**: `cmd/githelper/migrate.go` (new)

```go
package main

import (
	"fmt"
	"os"

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
1. Rename local 'master' branch to 'main'
2. Push 'main' to all configured remotes (origin, github)
3. Set 'main' as the default branch on GitHub (if github remote exists)
4. Optionally delete 'master' branch on remotes (use --delete-old)

The command is idempotent - safe to run multiple times.`,
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateForce, "force", false, "Force migration even if 'main' already exists")
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show what would be done without executing")
	migrateCmd.Flags().BoolVar(&migrateDeleteOld, "delete-old", false, "Delete 'master' branch from remotes after migration")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	gitClient := git.NewClient(".")

	// Check if it's a git repository
	if !gitClient.IsRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Get current branch
	currentBranch, err := gitClient.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if default branch is 'master'
	defaultBranch, err := gitClient.GetDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to get default branch: %w", err)
	}

	if defaultBranch != "master" {
		fmt.Printf("✓ Repository already uses '%s' as default branch\n", defaultBranch)
		return nil
	}

	fmt.Println("🔄 Migrating from 'master' to 'main'...")
	fmt.Println()

	// Step 1: Check if 'main' already exists
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

	if mainExists && !migrateForce {
		return fmt.Errorf("'main' branch already exists - use --force to overwrite")
	}

	// Step 2: Rename master to main (locally)
	fmt.Println("1. Renaming local 'master' branch to 'main'...")
	if migrateDryRun {
		fmt.Println("   [DRY RUN] git branch -m master main")
	} else {
		if err := gitClient.RenameBranch("master", "main"); err != nil {
			return fmt.Errorf("failed to rename branch: %w", err)
		}
		fmt.Println("   ✓ Local branch renamed")
	}

	// Step 3: Push 'main' to origin
	fmt.Println()
	fmt.Println("2. Pushing 'main' to origin...")
	if migrateDryRun {
		fmt.Println("   [DRY RUN] git push -u origin main")
	} else {
		if err := gitClient.Push("origin", "main", true); err != nil {
			return fmt.Errorf("failed to push to origin: %w", err)
		}
		fmt.Println("   ✓ Pushed to origin")
	}

	// Step 4: Push 'main' to github (if exists)
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
			if err := gitClient.Push("github", "main", true); err != nil {
				return fmt.Errorf("failed to push to github: %w", err)
			}
			fmt.Println("   ✓ Pushed to github")
		}

		// Step 5: Set default branch on GitHub
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

	// Step 6: Delete old 'master' branch from remotes (optional)
	if migrateDeleteOld {
		fmt.Println()
		fmt.Println("5. Deleting 'master' branch from remotes...")

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

func setGitHubDefaultBranch(branch string) error {
	// Use gh CLI to set default branch
	cmd := exec.Command("gh", "repo", "edit", "--default-branch", branch)
	return cmd.Run()
}
```

**File**: `cmd/githelper/status.go` (add warning detection)

```go
// In runStatus(), after detection:
if state.Sync.Branch == "master" {
	fmt.Println()
	fmt.Println("⚠️  WARNING: This repository uses 'master' as default branch")
	fmt.Println("   Modern standard is 'main' - run 'githelper migrate-to-main' to migrate")
	fmt.Println()
}
```

**File**: `internal/git/cli.go` (add helper methods)

```go
// RenameBranch renames a local branch
func (c *Client) RenameBranch(oldName, newName string) error {
	_, err := c.run("branch", "-m", oldName, newName)
	return err
}

// Push pushes a branch to a remote
func (c *Client) Push(remote, branch string, setUpstream bool) error {
	args := []string{"push"}
	if setUpstream {
		args = append(args, "-u")
	}
	args = append(args, remote, branch)

	_, err := c.run(args...)
	return err
}

// DeleteRemoteBranch deletes a branch from a remote
func (c *Client) DeleteRemoteBranch(remote, branch string) error {
	_, err := c.run("push", remote, "--delete", branch)
	return err
}

// GetCurrentBranch returns the current branch name
func (c *Client) GetCurrentBranch() (string, error) {
	output, err := c.run("branch", "--show-current")
	if err != nil {
		return "", err
	}
	return output, nil
}
```

**User Experience**:

```bash
# Check status on repo using master
$ githelper status
⚠️  WARNING: This repository uses 'master' as default branch
   Modern standard is 'main' - run 'githelper migrate-to-main' to migrate

📦 Existence: E1 - Fully configured
🔄 Sync Status: S1 - Perfect sync
  Branch: master  ← Notice this

# Migrate to main
$ githelper migrate-to-main
🔄 Migrating from 'master' to 'main'...

1. Renaming local 'master' branch to 'main'...
   ✓ Local branch renamed

2. Pushing 'main' to origin...
   ✓ Pushed to origin

3. Pushing 'main' to github...
   ✓ Pushed to github

4. Setting default branch on GitHub...
   ✓ GitHub default branch updated

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Migration complete!

Your repository now uses 'main' as the default branch.

The old 'master' branch still exists on remotes.
To delete it, run: githelper migrate-to-main --delete-old

# Delete old master branch
$ githelper migrate-to-main --delete-old
✓ Repository already uses 'main' as default branch

5. Deleting 'master' branch from remotes...
   ✓ Deleted from origin
   ✓ Deleted from github

# Verify
$ githelper status
📦 Existence: E1 - Fully configured
🔄 Sync Status: S1 - Perfect sync
  Branch: main  ← Now using main!
```

**Dry Run Mode**:
```bash
$ githelper migrate-to-main --dry-run
🔄 Migrating from 'master' to 'main'...

1. Renaming local 'master' branch to 'main'...
   [DRY RUN] git branch -m master main

2. Pushing 'main' to origin...
   [DRY RUN] git push -u origin main

3. Pushing 'main' to github...
   [DRY RUN] git push -u github main

4. Setting default branch on GitHub...
   [DRY RUN] gh repo edit --default-branch main

# Shows exactly what would happen without changing anything
```

**Benefits**:
- ✅ One command to migrate entire repository
- ✅ Works with dual-remote setup (origin + github)
- ✅ Updates GitHub default branch automatically (via `gh` CLI)
- ✅ Safe with `--dry-run` mode
- ✅ Idempotent - can run multiple times safely
- ✅ Optional `--delete-old` to clean up after migration
- ✅ Warning on status encourages migration
- ✅ Forces consistency across all repos

**Edge Cases Handled**:
- 'main' already exists → requires `--force` flag
- Not on 'master' branch → still migrates
- GitHub remote missing → skips GitHub steps
- `gh` CLI not installed → shows manual instructions
- Already migrated → reports success, does nothing

**Configuration Option** (future enhancement):

```yaml
# ~/.githelper/config.yaml
branch_naming:
  default: main
  enforce: true  # Reject operations on repos using 'master'
  auto_migrate: false  # Set to true to auto-migrate on first status check
```

---

## Testing Checklist

For each improvement:
- [ ] Unit tests for new operations
- [ ] Integration tests with real repos
- [ ] Test with edge cases (empty repos, detached HEAD, etc.)
- [ ] Verify backward compatibility
- [ ] Update documentation
- [ ] Add examples to README
