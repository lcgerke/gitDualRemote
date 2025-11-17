# Phase 4: Diagnostics & Polish - Implementation Notes

**Date:** 2025-11-16
**Status:** Complete

## Overview

Phase 4 implements comprehensive diagnostics, auto-fix capabilities, and enhanced error handling. This provides users with powerful troubleshooting tools and better error messages throughout the application.

## Features Implemented

### 1. Doctor Command (`cmd/githelper/doctor.go` - 574 lines)

Comprehensive health check system that validates all aspects of githelper configuration:

```bash
# Basic diagnostics
githelper doctor

# Show detailed credential inventory
githelper doctor --credentials

# Check specific repository only
githelper doctor --repo myproject

# Automatically fix common issues
githelper doctor --auto-fix
```

**Checks Performed:**

1. **Git Installation**
   - Verifies git is installed and accessible
   - Checks version compatibility

2. **Vault Configuration**
   - Tests Vault connectivity
   - Validates configuration loading
   - Reports connection status and address

3. **State Management**
   - Loads and validates state file
   - Counts configured repositories
   - Detects corruption

4. **Repository Health**
   - Verifies directory exists
   - Confirms it's a git repository
   - Lists configured remotes
   - Checks GitHub integration status
   - Validates sync status
   - Verifies hook installation

5. **Credential Inventory** (with `--credentials`)
   - Default SSH key status
   - Default PAT status
   - Repo-specific credential overrides
   - Disk locations and permissions

6. **Auto-Fix** (with `--auto-fix`)
   - Detects and fixes common issues
   - Reinstalls missing hooks
   - Reports what was fixed

**Output Formats:**

Human-readable (terminal):
```
🔍 GitHelper Diagnostic Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Git Installation:
  ✓ Git installed and accessible

Vault Configuration:
  ✓ Vault connected: http://core:8200

State Management:
  ✓ State file loaded (3 repositories)

Repositories:

  Repository: myproject
    ✓ Git repository exists
    ✓ Remotes configured: origin, github
    GitHub: lcgerke/myproject
    Sync Status: synced
    ✓ pre-push hook installed
    ✓ post-push hook installed
    ✓ Repository healthy

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 Summary:
  Total Checks: 5
  Passed: 5
  Warnings: 0
  Errors: 0

✅ All systems healthy
```

JSON format (for automation):
```json
{
  "checks": {
    "git_installation": {
      "name": "git_installation",
      "status": "ok",
      "message": "Git installed and accessible"
    },
    "vault_connectivity": {
      "name": "vault_connectivity",
      "status": "ok",
      "message": "Vault connected",
      "details": {
        "address": "http://core:8200"
      }
    }
  },
  "warnings": 0,
  "errors": 0
}
```

### 2. Auto-Fix System (`internal/autofix/autofix.go` - 150 lines)

Intelligent issue detection and automatic fixing:

**Detectable Issues:**

- `missing_hooks` - Hooks not installed (severity: low)
- `needs_sync` - GitHub needs sync (severity: medium)
- `missing_directory` - Repository directory missing (severity: high)
- `not_git_repo` - Directory exists but not a git repo (severity: high)

**Auto-Fixable Issues:**

- ✅ Missing hooks - automatically reinstalled
- ❌ Needs sync - requires manual intervention (network operation)
- ❌ Missing directory - critical, requires manual resolution
- ❌ Not a git repo - critical, requires manual resolution

**Usage:**

```go
fixer := autofix.NewFixer(stateMgr, false)
issues, _ := fixer.DetectIssues()
fixed, failed, _ := fixer.FixAll(issues)
```

**Output Example:**

```
🔧 Auto-Fix:

  Found 2 fixable issue(s):
    1. [low] myproject - Hook not installed: pre-push
    2. [low] myproject - Hook not installed: post-push

  ✓ Fixed 2 issue(s)
```

### 3. Enhanced Error System (`internal/errors/errors.go` - 164 lines)

Structured error types with user-friendly hints:

**Error Types:**
- `vault` - Vault connection/authentication issues
- `git` - Git operations failures
- `config` - Configuration problems
- `state` - State file issues
- `github` - GitHub API errors
- `network` - Network connectivity problems
- `filesystem` - File system operations
- `validation` - Input validation errors

**Features:**

1. **Structured Errors:**
   ```go
   type GitHelperError struct {
       Type    ErrorType
       Message string
       Hint    string  // User-friendly suggestion
       Err     error   // Wrapped original error
   }
   ```

2. **User-Friendly Hints:**
   ```go
   err := errors.VaultUnreachable("http://core:8200", originalErr)
   // Returns: "Vault unreachable at http://core:8200"
   // Hint: "Check that Vault is running and the address is correct.
   //        Run 'githelper doctor' for diagnostics."
   ```

3. **Common Error Constructors:**
   - `VaultUnreachable()` - Vault connection failed
   - `GitNotInstalled()` - Git binary not found
   - `RepositoryNotFound()` - Repo not in state
   - `GitHubAuthFailed()` - GitHub auth issues
   - `SSHKeyNotFound()` - SSH key missing
   - `RemoteNotConfigured()` - Git remote not set up
   - `DualPushNotConfigured()` - Dual-push not configured
   - `SyncRequired()` - GitHub behind, needs sync
   - `DivergenceDetected()` - Repos diverged
   - `StateCorrupted()` - State file invalid
   - `NetworkError()` - Network connectivity
   - `InvalidConfiguration()` - Config validation

**Example Usage:**

```go
import "github.com/lcgerke/githelper/internal/errors"

// Before:
return fmt.Errorf("repository not found: %s", name)

// After:
return errors.RepositoryNotFound(name)
// User sees: "Repository 'myproject' not found in state
//             Suggestion: Run 'githelper repo list' to see configured
//             repositories or 'githelper repo create myproject' to create it."
```

### 4. Diagnostic Results Tracking

Structured result tracking for comprehensive reporting:

```go
type DiagnosticResults struct {
    Checks    map[string]*CheckResult
    Warnings  int
    Errors    int
    StartTime time.Time
    EndTime   time.Time
}

type CheckResult struct {
    Name    string
    Status  string  // "ok", "warning", "error"
    Message string
    Details interface{}
}
```

## Command Line Interface

### Doctor Command

```bash
# Full diagnostics
githelper doctor

# With credential inventory
githelper doctor --credentials

# Check specific repo
githelper doctor --repo myproject

# Auto-fix issues
githelper doctor --auto-fix

# Combine flags
githelper doctor --credentials --auto-fix --repo myproject

# JSON output
githelper doctor --format=json
```

### Flags

- `--credentials` - Show detailed credential inventory
- `--repo <name>` - Filter to specific repository
- `--auto-fix` - Automatically fix detected issues
- `--format json` - JSON output for automation
- `--no-color` - Disable colored output
- `-q, --quiet` - Minimal output
- `-v, --verbose` - Verbose output

## Implementation Details

### Check Functions

Each check is implemented as a separate function:

1. `checkGitInstallation()` - Validates git binary
2. `checkVault()` - Tests Vault connectivity
3. `checkStateFile()` - Loads and validates state
4. `checkRepositories()` - Iterates through all repos
5. `checkRepository()` - Deep check of single repo
6. `checkHooks()` - Validates hook installation
7. `checkCredentials()` - Credential inventory
8. `runAutoFix()` - Detect and fix issues

### Error Aggregation

Results are aggregated into a `DiagnosticResults` struct:

```go
results.AddCheck("check_name", "status", "message", details)
```

Supports three statuses:
- `ok` - Check passed
- `warning` - Non-critical issue
- `error` - Critical problem

### Auto-Fix Workflow

1. Create fixer: `fixer := autofix.NewFixer(stateMgr, dryRun)`
2. Detect issues: `issues, err := fixer.DetectIssues()`
3. Display to user
4. Fix all: `fixed, failed, err := fixer.FixAll(issues)`
5. Report results

## Testing Recommendations

### Manual Testing

1. **Healthy system:**
   ```bash
   githelper doctor
   # Should show all green
   ```

2. **Missing hooks:**
   ```bash
   rm .git/hooks/pre-push
   githelper doctor --auto-fix
   # Should detect and fix
   ```

3. **Credential inventory:**
   ```bash
   githelper doctor --credentials
   # Should list all credentials
   ```

4. **Specific repo check:**
   ```bash
   githelper doctor --repo myproject
   # Should check only that repo
   ```

5. **JSON output:**
   ```bash
   githelper doctor --format=json | jq
   # Should output valid JSON
   ```

### Edge Cases

- Vault unreachable
- State file corrupted
- Repository directory deleted
- Hooks with wrong permissions
- Mixed health states across repos
- Empty state (no repos configured)

## Success Criteria

- ✅ Comprehensive diagnostics covering all components
- ✅ Credential inventory with disk locations
- ✅ Auto-fix for common issues (hooks)
- ✅ Structured error types with helpful hints
- ✅ Human and JSON output modes
- ✅ Repository-specific filtering
- ✅ Clear summary with pass/warning/error counts

## Known Limitations

1. **Auto-fix scope:** Only fixes missing hooks automatically. Other issues require manual intervention.

2. **Credential validation:** Does not test if credentials actually work, only if they exist.

3. **Network checks:** Does not ping remotes (use `githelper github check` for that).

4. **State reconstruction:** Cannot automatically rebuild state from git repos (planned for future).

## Next Steps (Phase 5)

- Comprehensive testing (unit, integration, edge cases)
- Mock infrastructure (Vault server, GitHub API)
- Documentation (workflow guide, vault setup)
- Release automation

## Files Added/Modified

**New Files:**
- `cmd/githelper/doctor.go` (574 lines) - Doctor command
- `internal/autofix/autofix.go` (150 lines) - Auto-fix system
- `internal/errors/errors.go` (164 lines) - Enhanced error handling
- `docs/PHASE4_NOTES.md` - This file

**Modified Files:**
- `cmd/githelper/main.go` - Registered doctor command

**Total New Code:** ~888 lines

## Architecture

```
┌─────────────────────────────────────────┐
│         Doctor Command                  │
│  • Orchestrates all checks              │
│  • Aggregates results                   │
│  • Formats output                       │
└──────────┬──────────────────────────────┘
           │
           ├──> checkGitInstallation()
           ├──> checkVault()
           ├──> checkStateFile()
           ├──> checkRepositories()
           │    └──> checkRepository() (per repo)
           │         └──> checkHooks()
           ├──> checkCredentials()
           └──> runAutoFix()
                └──> AutoFix System
                     ├──> DetectIssues()
                     └──> FixAll()
                          └──> Hook Manager

┌─────────────────────────────────────────┐
│         Error System                    │
│  GitHelperError                         │
│  • Type (vault, git, github, etc)       │
│  • Message                              │
│  • Hint (user-friendly suggestion)      │
│  • Wrapped error                        │
└─────────────────────────────────────────┘
```

## Benefits

1. **Troubleshooting:** Single command to diagnose all issues
2. **Automation:** JSON output for CI/CD integration
3. **Self-healing:** Auto-fix resolves common problems
4. **User Experience:** Helpful hints guide users to solutions
5. **Maintenance:** Structured errors easier to debug
6. **Documentation:** Built-in health check documents expectations
