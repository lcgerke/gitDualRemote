package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcgerke/githelper/internal/autofix"
	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/lcgerke/githelper/internal/vault"
)

// handleCheckError is a helper to reduce duplication in error handling
func handleCheckError(out *ui.Output, results *DiagnosticResults, checkName, message string, err error) {
	if !out.IsJSON() {
		out.Error(message)
	}
	results.AddCheck(checkName, "error", err.Error(), nil)
}

func checkGitInstallation(out *ui.Output, results *DiagnosticResults) {
	err := git.CheckGitVersion()
	if err != nil {
		handleCheckError(out, results, "git_installation", fmt.Sprintf("  ✗ Git not found: %v", err), err)
		return
	}

	if !out.IsJSON() {
		out.Success("  ✓ Git installed and accessible")
	}
	results.AddCheck("git_installation", "ok", "Git installed and accessible", nil)
}

func checkVault(out *ui.Output, results *DiagnosticResults) *vault.Client {
	ctx := context.Background()

	// Try to create vault client
	vaultClient, err := vault.NewClient(ctx)
	if err != nil {
		handleCheckError(out, results, "vault_connectivity", fmt.Sprintf("  ✗ Vault client creation failed: %v", err), err)
		return nil
	}

	// Test connectivity by trying to get config
	if !vaultClient.IsReachable() {
		if !out.IsJSON() {
			out.Warning("  ⚠ Vault not reachable (will use cache if available)")
		}
		results.AddCheck("vault_connectivity", "warning", "Vault not reachable", nil)
		return vaultClient
	}

	// Try to get config
	cfg, err := vaultClient.GetConfig()
	if err != nil {
		if !out.IsJSON() {
			out.Warning(fmt.Sprintf("  ⚠ Vault reachable but config not found: %v", err))
		}
		results.AddCheck("vault_connectivity", "warning", "Vault reachable but config not found", nil)
		return vaultClient
	}

	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "default"
	}

	if !out.IsJSON() {
		out.Success(fmt.Sprintf("  ✓ Vault connected: %s", vaultAddr))
	}
	results.AddCheck("vault_connectivity", "ok", "Vault connected", map[string]interface{}{
		"address":           vaultAddr,
		"bare_repo_pattern": cfg.BareRepoPattern,
	})

	return vaultClient
}

func checkStateFile(out *ui.Output, results *DiagnosticResults) *state.Manager {
	stateMgr, err := state.NewManager("")
	if err != nil {
		handleCheckError(out, results, "state_file", fmt.Sprintf("  ✗ State manager failed: %v", err), err)
		return nil
	}

	st, err := stateMgr.Load()
	if err != nil {
		handleCheckError(out, results, "state_file", fmt.Sprintf("  ✗ Failed to load state: %v", err), err)
		return nil
	}

	repoCount := len(st.Repositories)
	if !out.IsJSON() {
		out.Success(fmt.Sprintf("  ✓ State file loaded (%d repositories)", repoCount))
	}
	results.AddCheck("state_file", "ok", "State file loaded", map[string]int{
		"repository_count": repoCount,
	})

	return stateMgr
}

func checkRepositories(out *ui.Output, results *DiagnosticResults, stateMgr *state.Manager, vaultClient *vault.Client) {
	if stateMgr == nil {
		if !out.IsJSON() {
			out.Warning("  ⚠ Cannot check repositories (state manager unavailable)")
		}
		return
	}

	st, err := stateMgr.Load()
	if err != nil {
		return
	}

	if len(st.Repositories) == 0 {
		if !out.IsJSON() {
			out.Info("  ℹ No repositories configured")
		}
		results.AddCheck("repositories", "ok", "No repositories configured", nil)
		return
	}

	repoResults := make(map[string]interface{})
	healthyRepos := 0

	for name, repo := range st.Repositories {
		// Skip if filtering
		if repoFilter != "" && name != repoFilter {
			continue
		}

		repoHealth := checkRepository(out, name, repo, vaultClient)
		repoResults[name] = repoHealth

		if repoHealth["status"] == "ok" {
			healthyRepos++
		}
	}

	totalRepos := len(st.Repositories)
	if repoFilter != "" {
		totalRepos = 1
	}

	if healthyRepos == totalRepos {
		results.AddCheck("repositories", "ok", fmt.Sprintf("All %d repositories healthy", totalRepos), repoResults)
	} else if healthyRepos > 0 {
		results.AddCheck("repositories", "warning", fmt.Sprintf("%d/%d repositories healthy", healthyRepos, totalRepos), repoResults)
	} else {
		results.AddCheck("repositories", "error", "No healthy repositories", repoResults)
	}
}

func checkRepository(out *ui.Output, name string, repo *state.Repository, vaultClient *vault.Client) map[string]interface{} {
	result := make(map[string]interface{})
	result["name"] = name
	result["path"] = repo.Path
	issues := []string{}

	if !out.IsJSON() {
		fmt.Printf("\n  Repository: %s\n", name)
	}

	// Check if directory exists
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		if !out.IsJSON() {
			out.Error(fmt.Sprintf("    ✗ Directory not found: %s", repo.Path))
		}
		issues = append(issues, "directory not found")
		result["status"] = "error"
		result["issues"] = issues
		return result
	}

	// Check if it's a git repository
	gitClient := git.NewClient(repo.Path)
	if !gitClient.IsRepository() {
		if !out.IsJSON() {
			out.Error("    ✗ Not a git repository")
		}
		issues = append(issues, "not a git repository")
		result["status"] = "error"
		result["issues"] = issues
		return result
	}

	if !out.IsJSON() {
		out.Success("    ✓ Git repository exists")
	}

	// Check remotes
	remotes, err := gitClient.ListRemotes()
	if err != nil {
		if !out.IsJSON() {
			out.Warning(fmt.Sprintf("    ⚠ Failed to list remotes: %v", err))
		}
		issues = append(issues, "failed to list remotes")
	} else {
		if !out.IsJSON() {
			out.Success(fmt.Sprintf("    ✓ Remotes configured: %s", strings.Join(remotes, ", ")))
		}
		result["remotes"] = remotes
	}

	// Check GitHub integration
	if repo.GitHub != nil && repo.GitHub.Enabled {
		if !out.IsJSON() {
			fmt.Printf("    GitHub: %s/%s\n", repo.GitHub.User, repo.GitHub.Repo)
			fmt.Printf("    Sync Status: %s\n", repo.GitHub.SyncStatus)
		}
		result["github_enabled"] = true
		result["github_user"] = repo.GitHub.User
		result["github_repo"] = repo.GitHub.Repo
		result["sync_status"] = repo.GitHub.SyncStatus

		if repo.GitHub.NeedsRetry {
			if !out.IsJSON() {
				out.Warning("    ⚠ Needs retry")
			}
			issues = append(issues, "needs retry")
		}

		if repo.GitHub.LastError != "" {
			if !out.IsJSON() {
				out.Error(fmt.Sprintf("    ✗ Last error: %s", repo.GitHub.LastError))
			}
			issues = append(issues, "last push failed")
		}
	}

	// Check hooks
	hookStatus := checkHooks(out, repo.Path)
	result["hooks"] = hookStatus

	if len(issues) == 0 {
		result["status"] = "ok"
		if !out.IsJSON() {
			out.Success("    ✓ Repository healthy")
		}
	} else {
		result["status"] = "warning"
		result["issues"] = issues
	}

	return result
}

func checkHooks(out *ui.Output, repoPath string) map[string]bool {
	hookDir := filepath.Join(repoPath, ".git", "hooks")
	status := make(map[string]bool)

	hooks := []string{"pre-push", "post-push"}
	for _, hook := range hooks {
		hookPath := filepath.Join(hookDir, hook)
		if _, err := os.Stat(hookPath); err == nil {
			status[hook] = true
			if !out.IsJSON() {
				out.Success(fmt.Sprintf("    ✓ %s hook installed", hook))
			}
		} else {
			status[hook] = false
			if !out.IsJSON() {
				out.Warning(fmt.Sprintf("    ⚠ %s hook not found", hook))
			}
		}
	}

	return status
}

func checkCredentials(out *ui.Output, results *DiagnosticResults, vaultClient *vault.Client, stateMgr *state.Manager) {
	if vaultClient == nil {
		if !out.IsJSON() {
			out.Warning("\n  Vault unavailable - cannot check credentials")
		}
		return
	}

	credInventory := make(map[string]interface{})

	// Check default SSH key
	_, err := vaultClient.GetSSHKey("default")
	if err != nil {
		if !out.IsJSON() {
			out.Warning(fmt.Sprintf("  ⚠ Default SSH key not found: %v", err))
		}
	} else {
		if !out.IsJSON() {
			out.Success("  ✓ Default SSH key found")
		}
		credInventory["default_ssh"] = "configured"

		// Check if it's on disk
		home, _ := os.UserHomeDir()
		keyPath := filepath.Join(home, ".ssh", "github_default")
		if _, err := os.Stat(keyPath); err == nil {
			if !out.IsJSON() {
				out.Success(fmt.Sprintf("    ✓ On disk: %s", keyPath))
			}
			credInventory["default_ssh_path"] = keyPath
		}
	}

	// Check default PAT
	defaultPAT, err := vaultClient.GetPAT("default")
	if err != nil {
		if !out.IsJSON() {
			out.Warning(fmt.Sprintf("  ⚠ Default PAT not found: %v", err))
		}
	} else {
		if !out.IsJSON() {
			out.Success(fmt.Sprintf("  ✓ Default PAT found (length: %d)", len(defaultPAT)))
		}
		credInventory["default_pat"] = "configured"
	}

	// Check repo-specific credentials
	if stateMgr != nil {
		st, err := stateMgr.Load()
		if err == nil {
			repoCredentials := make(map[string]interface{})
			for name := range st.Repositories {
				repoSSH, err := vaultClient.GetSSHKey(name)
				if err == nil {
					repoCredentials[name] = map[string]string{
						"ssh": "configured",
					}
					if !out.IsJSON() {
						out.Success(fmt.Sprintf("  ✓ %s: SSH key override found", name))
					}
				} else {
					_ = repoSSH // Avoid unused variable
				}
			}
			if len(repoCredentials) > 0 {
				credInventory["repo_specific"] = repoCredentials
			}
		}
	}

	results.AddCheck("credentials", "ok", "Credential inventory complete", credInventory)
}

func runAutoFix(out *ui.Output, results *DiagnosticResults, stateMgr *state.Manager) {
	fixer := autofix.NewFixer(stateMgr, false)

	// Detect issues
	issues, err := fixer.DetectIssues()
	if err != nil {
		handleCheckError(out, results, "auto_fix", fmt.Sprintf("  ✗ Failed to detect issues: %v", err), err)
		return
	}

	if len(issues) == 0 {
		if !out.IsJSON() {
			out.Success("  ✓ No fixable issues detected")
		}
		results.AddCheck("auto_fix", "ok", "No issues detected", nil)
		return
	}

	if !out.IsJSON() {
		fmt.Printf("\n  Found %d fixable issue(s):\n", len(issues))
		for i, issue := range issues {
			fmt.Printf("    %d. [%s] %s - %s\n", i+1, issue.Severity, issue.RepoName, issue.Description)
		}
		fmt.Println()
	}

	// Attempt fixes
	fixed, failed, err := fixer.FixAll(issues)
	if err != nil {
		handleCheckError(out, results, "auto_fix", fmt.Sprintf("  ✗ Auto-fix failed: %v", err), err)
		return
	}

	if !out.IsJSON() {
		if fixed > 0 {
			out.Success(fmt.Sprintf("  ✓ Fixed %d issue(s)", fixed))
		}
		if failed > 0 {
			out.Warning(fmt.Sprintf("  ⚠ Could not fix %d issue(s) (require manual intervention)", failed))
		}
	}

	results.AddCheck("auto_fix", "ok", fmt.Sprintf("Fixed %d of %d issues", fixed, len(issues)), map[string]int{
		"detected": len(issues),
		"fixed":    fixed,
		"failed":   failed,
	})
}

func printSummary(out *ui.Output, results *DiagnosticResults) {
	totalChecks := len(results.Checks)
	passed := totalChecks - results.Warnings - results.Errors

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Checks: %d\n", totalChecks)
	fmt.Printf("  Passed: %d\n", passed)

	if results.Warnings > 0 {
		out.Warning(fmt.Sprintf("  Warnings: %d", results.Warnings))
	} else {
		fmt.Printf("  Warnings: %d\n", results.Warnings)
	}

	if results.Errors > 0 {
		out.Error(fmt.Sprintf("  Errors: %d", results.Errors))
	} else {
		fmt.Printf("  Errors: %d\n", results.Errors)
	}

	fmt.Println()
	if results.Errors == 0 && results.Warnings == 0 {
		out.Success("✅ All systems healthy")
	} else if results.Errors == 0 {
		out.Warning("⚠️  Some warnings detected")
	} else {
		out.Error("❌ Critical errors detected")
	}
}
