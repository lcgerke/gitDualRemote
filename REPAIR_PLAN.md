# GitHelper Repair Command Implementation Plan

## Problem Statement

**Current Behavior:**
The `githelper status --show-fixes` command displays suggested fixes and says:
```
Run 'githelper repair --auto' to apply auto-fixable changes
```

But the `repair` command doesn't exist, causing user confusion.

**Root Cause:**
`cmd/githelper/status.go:270` references a non-existent command. No `cmd/githelper/repair.go` file exists.

**Gap:**
Users can see what needs fixing but must manually run git commands instead of having an automated repair option.

---

## Design Goals

1. **Safe automation** - Only apply fixes marked as `AutoFixable: true`
2. **User confirmation** - Interactive mode by default, `--auto` for unattended
3. **Rollback support** - Track changes for potential undo
4. **Clear feedback** - Show what's being fixed and results
5. **Consistent with suggester** - Use existing `Fix` and `Operation` types

---

## Command Design

### Usage

```bash
# Interactive mode (default) - prompt before each fix
githelper repair

# Automatic mode - apply all auto-fixable changes without prompts
githelper repair --auto

# Dry run - show what would be done without executing
githelper repair --dry-run

# Apply specific fix by ID
githelper repair --fix S2

# Skip specific fixes
githelper repair --skip S2,W3
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--auto` | bool | false | Apply all auto-fixable changes without prompting |
| `--dry-run` | bool | false | Show what would be done without executing |
| `--fix` | string | "" | Only apply specific fix by scenario ID (e.g., S2) |
| `--skip` | string | "" | Comma-separated list of fixes to skip |
| `--yes` | bool | false | Alias for `--auto` |

---

## Architecture

### Workflow

```
1. Run detection (same as status)
   ↓
2. Get suggested fixes
   ↓
3. Filter to auto-fixable only
   ↓
4. Apply filters (--fix, --skip)
   ↓
5. If interactive: prompt for each fix
   ↓
6. Execute operations via AutoFixExecutor
   ↓
7. Re-run detection to verify
   ↓
8. Display results
```

### Components

**New File:** `cmd/githelper/repair.go`
- Command registration and CLI parsing
- Interactive prompts
- Orchestration logic

**Existing Components (reuse):**
- `scenarios.NewClassifier()` - Detect current state
- `scenarios.SuggestFixes()` - Get fix recommendations
- `scenarios.NewAutoFixExecutor()` - Execute operations
- `scenarios.Fix` - Fix definition with Operation
- `scenarios.Operation` - Executable operation (Push, Pull, etc.)

---

## Detailed Implementation

### 1. Create Repair Command

**File:** `cmd/githelper/repair.go`

```go
package main

import (
	"fmt"
	"strings"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/scenarios"
	"github.com/spf13/cobra"
)

func newRepairCmd() *cobra.Command {
	var (
		autoMode bool
		dryRun   bool
		fixID    string
		skipIDs  string
	)

	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Automatically fix repository issues",
		Long: `Automatically apply auto-fixable fixes to bring repository to a healthy state.

By default, runs in interactive mode and prompts before each fix.
Use --auto to apply all fixes without prompting.

Examples:
  githelper repair              # Interactive mode
  githelper repair --auto       # Automatic mode
  githelper repair --dry-run    # Show what would be done
  githelper repair --fix S2     # Only fix S2 scenario
  githelper repair --skip W3,W5 # Skip working tree fixes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepair(autoMode, dryRun, fixID, skipIDs)
		},
	}

	cmd.Flags().BoolVar(&autoMode, "auto", false, "Apply all fixes without prompting")
	cmd.Flags().BoolVar(&autoMode, "yes", false, "Alias for --auto")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")
	cmd.Flags().StringVar(&fixID, "fix", "", "Only apply specific fix by scenario ID (e.g., S2)")
	cmd.Flags().StringVar(&skipIDs, "skip", "", "Comma-separated list of fixes to skip")

	return cmd
}

func runRepair(autoMode, dryRun bool, fixID, skipIDs string) error {
	// 1. Initialize Git client
	gitClient := git.NewClient(".")

	// 2. Run detection
	out.Info("🔍 Analyzing repository state...")

	options := scenarios.DefaultDetectionOptions()
	classifier := scenarios.NewClassifier(
		gitClient,
		"origin",  // TODO: make configurable
		"github",  // TODO: make configurable
		options,
	)

	state, err := classifier.Detect()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// 3. Get suggested fixes
	allFixes := scenarios.SuggestFixes(state)

	// 4. Filter to auto-fixable only
	autoFixable := filterAutoFixable(allFixes)

	if len(autoFixable) == 0 {
		out.Success("✅ No auto-fixable issues found")
		return nil
	}

	// 5. Apply filters (--fix, --skip)
	fixes := applyFilters(autoFixable, fixID, skipIDs)

	if len(fixes) == 0 {
		out.Info("No fixes match the specified filters")
		return nil
	}

	// 6. Display plan
	fmt.Printf("\n🔧 Found %d auto-fixable issue(s):\n\n", len(fixes))
	for i, fix := range fixes {
		fmt.Printf("%d. [%s] %s\n", i+1, fix.ScenarioID, fix.Description)
		fmt.Printf("   Command: %s\n", fix.Command)
	}
	fmt.Println()

	if dryRun {
		out.Info("(Dry run - no changes made)")
		return nil
	}

	// 7. Execute fixes
	executor := scenarios.NewAutoFixExecutor(gitClient)
	results := make([]RepairResult, 0, len(fixes))

	for i, fix := range fixes {
		// Interactive prompt (unless --auto)
		if !autoMode {
			fmt.Printf("Apply fix %d/%d? [Y/n] ", i+1, len(fixes))
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))

			if response == "n" || response == "no" {
				results = append(results, RepairResult{
					Fix:     fix,
					Skipped: true,
				})
				continue
			}
		}

		// Execute
		out.Info(fmt.Sprintf("Applying [%s] %s...", fix.ScenarioID, fix.Description))

		err := executor.Execute(fix, state)
		if err != nil {
			out.Error(fmt.Sprintf("Failed: %v", err))
			results = append(results, RepairResult{
				Fix:   fix,
				Error: err,
			})

			// Stop on first error
			if !autoMode {
				fmt.Print("Continue with remaining fixes? [y/N] ")
				var response string
				fmt.Scanln(&response)
				response = strings.ToLower(strings.TrimSpace(response))

				if response != "y" && response != "yes" {
					break
				}
			}
		} else {
			out.Success(fmt.Sprintf("✓ Applied [%s]", fix.ScenarioID))
			results = append(results, RepairResult{
				Fix:     fix,
				Success: true,
			})
		}
	}

	// 8. Re-run detection to verify
	fmt.Println()
	out.Info("🔍 Re-checking repository state...")

	newState, err := classifier.Detect()
	if err != nil {
		out.Warning(fmt.Sprintf("Could not verify results: %v", err))
	} else {
		// Display new state
		displayRepairSummary(state, newState, results)
	}

	return nil
}

type RepairResult struct {
	Fix     scenarios.Fix
	Success bool
	Skipped bool
	Error   error
}

func filterAutoFixable(fixes []scenarios.Fix) []scenarios.Fix {
	result := make([]scenarios.Fix, 0, len(fixes))
	for _, fix := range fixes {
		if fix.AutoFixable {
			result = append(result, fix)
		}
	}
	return result
}

func applyFilters(fixes []scenarios.Fix, fixID, skipIDs string) []scenarios.Fix {
	// Parse skip list
	skipMap := make(map[string]bool)
	if skipIDs != "" {
		for _, id := range strings.Split(skipIDs, ",") {
			skipMap[strings.TrimSpace(id)] = true
		}
	}

	// Filter
	result := make([]scenarios.Fix, 0, len(fixes))
	for _, fix := range fixes {
		// Skip if in skip list
		if skipMap[fix.ScenarioID] {
			continue
		}

		// If --fix specified, only include that fix
		if fixID != "" && fix.ScenarioID != fixID {
			continue
		}

		result = append(result, fix)
	}

	return result
}

func displayRepairSummary(oldState, newState *scenarios.RepositoryState, results []RepairResult) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Repair Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Count results
	applied := 0
	skipped := 0
	failed := 0

	for _, result := range results {
		if result.Success {
			applied++
		} else if result.Skipped {
			skipped++
		} else {
			failed++
		}
	}

	fmt.Printf("\nApplied: %d\n", applied)
	if skipped > 0 {
		fmt.Printf("Skipped: %d\n", skipped)
	}
	if failed > 0 {
		fmt.Printf("Failed:  %d\n", failed)
	}

	// State comparison
	fmt.Println("\nBefore → After:")
	fmt.Printf("  Existence:   %s → %s\n", oldState.Existence.ID, newState.Existence.ID)
	fmt.Printf("  Sync:        %s → %s\n", oldState.Sync.ID, newState.Sync.ID)
	fmt.Printf("  Working Tree: %s → %s\n", oldState.WorkingTree.ID, newState.WorkingTree.ID)
	fmt.Printf("  Corruption:  %s → %s\n", oldState.Corruption.ID, newState.Corruption.ID)

	// Final verdict
	fmt.Println()
	if newState.Sync.ID == "S1" && newState.WorkingTree.Clean && newState.Corruption.Healthy {
		out.Success("✅ Repository is now healthy and in sync!")
	} else {
		remaining := len(scenarios.SuggestFixes(newState))
		if remaining > 0 {
			out.Info(fmt.Sprintf("⚠️  %d issue(s) remaining (may require manual intervention)", remaining))
			fmt.Println("   Run 'githelper status --show-fixes' to see remaining issues")
		}
	}
}
```

### 2. Register Command

**File:** `cmd/githelper/main.go`

```go
func init() {
	rootCmd.AddCommand(
		newStatusCmd(),
		newDoctorCmd(),
		newRepoCmd(),
		newGitHubCmd(),
		newRepairCmd(), // ADD THIS
	)
}
```

---

## Test Coverage

### Unit Tests

**File:** `cmd/githelper/repair_test.go`

```go
func TestFilterAutoFixable(t *testing.T) {
	fixes := []scenarios.Fix{
		{ScenarioID: "S2", AutoFixable: true},
		{ScenarioID: "S13", AutoFixable: false},
		{ScenarioID: "W2", AutoFixable: false},
	}

	result := filterAutoFixable(fixes)

	assert.Len(t, result, 1)
	assert.Equal(t, "S2", result[0].ScenarioID)
}

func TestApplyFilters_FixID(t *testing.T) {
	fixes := []scenarios.Fix{
		{ScenarioID: "S2"},
		{ScenarioID: "S3"},
		{ScenarioID: "W2"},
	}

	result := applyFilters(fixes, "S2", "")

	assert.Len(t, result, 1)
	assert.Equal(t, "S2", result[0].ScenarioID)
}

func TestApplyFilters_SkipIDs(t *testing.T) {
	fixes := []scenarios.Fix{
		{ScenarioID: "S2"},
		{ScenarioID: "S3"},
		{ScenarioID: "W2"},
	}

	result := applyFilters(fixes, "", "S3,W2")

	assert.Len(t, result, 1)
	assert.Equal(t, "S2", result[0].ScenarioID)
}
```

### Integration Tests

```bash
# Test 1: Dry run mode
cd /tmp/test-repo
git checkout -b test-branch
echo "test" > file.txt
git add file.txt
git commit -m "test"
# Now local is ahead

githelper repair --dry-run
# Expected: Shows S2 fix but doesn't execute

# Test 2: Auto mode
githelper repair --auto
# Expected: Pushes automatically

githelper status
# Expected: S1 - Perfect sync

# Test 3: Interactive mode
cd /tmp/test-repo2
# Create diverged state
githelper repair  # Press 'n' when prompted
# Expected: Skips fix, no changes

# Test 4: Specific fix
githelper repair --fix S2
# Expected: Only applies S2 fix
```

---

## Error Handling

### Scenarios

| Error Condition | Handling |
|----------------|----------|
| Git command fails | Display error, ask to continue (interactive) or stop (auto) |
| No auto-fixable fixes | Success message, exit gracefully |
| Operation not supported | Skip with warning |
| Dirty working tree blocks operation | Clear error message, suggest cleanup |
| Network failure (push/pull) | Retry once, then fail with network hint |
| Permission denied | Clear error, suggest checking credentials |

### Recovery

- Each fix is independent - failure of one doesn't affect others
- No automatic rollback (git operations are not easily reversible)
- Re-detection after each batch shows current state
- User can run `git reflog` for manual recovery if needed

---

## User Experience

### Example Session (Interactive)

```bash
$ githelper repair

🔍 Analyzing repository state...

🔧 Found 2 auto-fixable issue(s):

1. [S2] Local has 3 unpushed commits to origin
   Command: git push origin main

2. [W5] Untracked files present
   Command: git add <files> or add to .gitignore

Apply fix 1/2? [Y/n] y
Applying [S2] Local has 3 unpushed commits to origin...
✓ Applied [S2]

Apply fix 2/2? [Y/n] n
Skipped [W5]

🔍 Re-checking repository state...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Repair Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Applied: 1
Skipped: 1

Before → After:
  Existence:    E1 → E1
  Sync:         S2 → S1
  Working Tree: W5 → W5
  Corruption:   C1 → C1

⚠️  1 issue(s) remaining (may require manual intervention)
   Run 'githelper status --show-fixes' to see remaining issues
```

### Example Session (Auto)

```bash
$ githelper repair --auto

🔍 Analyzing repository state...

🔧 Found 1 auto-fixable issue(s):

1. [S3] Remote origin has 2 commits not in local
   Command: git pull origin main

Applying [S3] Remote origin has 2 commits not in local...
✓ Applied [S3]

🔍 Re-checking repository state...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Repair Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Applied: 1

Before → After:
  Existence:    E1 → E1
  Sync:         S3 → S1
  Working Tree: W1 → W1
  Corruption:   C1 → C1

✅ Repository is now healthy and in sync!
```

---

## Files to Create/Modify

| File | Action | Lines | Complexity |
|------|--------|-------|------------|
| `cmd/githelper/repair.go` | Create new | ~400 | High |
| `cmd/githelper/repair_test.go` | Create new | ~200 | Medium |
| `cmd/githelper/main.go` | Add registration | +1 | Low |
| `cmd/githelper/status.go` | Fix line 270 (already correct) | 0 | Low |

**Total:** ~600 lines of new code

---

## Implementation Phases

### Phase 1: Core Repair Logic (TDD)

**Duration: 60 minutes**

1. **Write tests** for filter functions
   - `TestFilterAutoFixable`
   - `TestApplyFilters_FixID`
   - `TestApplyFilters_SkipIDs`

2. **Implement filters**
   - `filterAutoFixable()`
   - `applyFilters()`

3. **Write test** for dry-run mode
4. **Implement** dry-run display logic

### Phase 2: Execution Engine

**Duration: 45 minutes**

1. **Integrate with AutoFixExecutor**
   - Create executor instance
   - Call `Execute()` for each fix
   - Handle errors

2. **Write test** for execution results
3. **Implement** result tracking and display

### Phase 3: Interactive Mode

**Duration: 30 minutes**

1. **Implement** prompt logic
   - Before each fix
   - After errors
   - Skip/continue logic

2. **Test** interactive flow manually

### Phase 4: Summary Display

**Duration: 30 minutes**

1. **Implement** `displayRepairSummary()`
   - Before/after comparison
   - Results breakdown
   - Final verdict

2. **Add** re-detection logic
3. **Test** various outcomes (all success, partial, all failed)

### Phase 5: Integration & Testing

**Duration: 45 minutes**

1. **Integration tests** on real repos
   - S2 scenario (local ahead)
   - S3 scenario (local behind)
   - Multiple fixes
   - Error scenarios

2. **Edge case testing**
   - No auto-fixable fixes
   - All fixes skipped
   - Network failures

3. **Documentation** in README

---

## Validation Criteria

**Success means:**

1. **Dry run works:**
```bash
$ githelper repair --dry-run
🔧 Found 1 auto-fixable issue(s):
1. [S2] Local has 3 unpushed commits
(Dry run - no changes made)
```

2. **Auto mode works:**
```bash
$ githelper repair --auto
✓ Applied [S2]
✅ Repository is now healthy and in sync!
```

3. **Interactive mode works:**
```bash
$ githelper repair
Apply fix 1/1? [Y/n] y
✓ Applied [S2]
```

4. **Filters work:**
```bash
$ githelper repair --fix S2
# Only applies S2

$ githelper repair --skip W5
# Skips W5
```

5. **Error handling works:**
```bash
$ githelper repair --auto
Applying [S2]...
Failed: network error
⚠️  1 issue(s) remaining
```

---

## Risk Assessment

**Low Risk:**
- Reusing existing `AutoFixExecutor` - already tested
- Filter logic is simple
- Dry-run mode allows safe testing

**Medium Risk:**
- Interactive prompts - need careful stdin handling
- Error recovery - must not leave repo in bad state
- Multiple fixes - order matters for some scenarios

**Mitigation:**
- Comprehensive unit tests for filters
- Integration tests with real git repos
- Re-detection after execution to verify state
- Clear error messages with recovery hints

---

## Timeline Estimate

- Phase 1 (Core Logic): 60 min
- Phase 2 (Execution): 45 min
- Phase 3 (Interactive): 30 min
- Phase 4 (Summary): 30 min
- Phase 5 (Testing): 45 min

**Total: ~210 minutes (3.5 hours)**

**Buffer:** +30 minutes for unexpected issues

**Final estimate: 4 hours** for complete TDD implementation with comprehensive testing

---

## Future Enhancements

**Not in initial scope, but possible later:**

1. **Undo support** - Track operations for rollback
2. **Batch mode** - Apply multiple fixes in one transaction
3. **Fix prioritization** - Apply critical fixes first
4. **Progress bar** - For long-running operations
5. **Parallel execution** - Run independent fixes concurrently
6. **Pre-fix validation** - Check preconditions before applying
7. **Post-fix hooks** - Run custom commands after repairs
8. **Repair history** - Log all repairs for audit trail
9. **Suggested fix order** - Smart ordering based on dependencies
10. **Interactive fix selection** - Checkbox UI for selecting fixes

---

## Decision Points

### Should repair stop on first error?

**Option A:** Stop immediately (safer)
- ✓ Prevents cascading failures
- ✗ May leave some fixable issues unresolved

**Option B:** Continue with remaining fixes (more aggressive)
- ✓ Fixes as much as possible
- ✗ May cause unexpected states

**Decision:** Option A in auto mode, Option B with prompt in interactive mode

### Should repair require confirmation by default?

**Option A:** Interactive by default, `--auto` for unattended
- ✓ Safer for new users
- ✓ User sees what's happening
- ✗ Requires user interaction

**Option B:** Auto by default, `--interactive` for prompts
- ✓ Faster workflow
- ✗ Surprising behavior for new users

**Decision:** Option A (interactive by default) - safer and more transparent

### Should repair handle non-auto-fixable issues?

**Option A:** Only handle auto-fixable (current plan)
- ✓ Simple and safe
- ✓ Clear scope
- ✗ User still needs manual intervention

**Option B:** Prompt for manual fixes too
- ✓ Comprehensive solution
- ✗ More complex
- ✗ Harder to automate

**Decision:** Option A (auto-fixable only) - keep it simple for v1
