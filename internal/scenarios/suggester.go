package scenarios

import (
	"fmt"
)

// ============================================================================
// PHASE 3: Fix Suggestion Engine
// ============================================================================

// SuggestFixes analyzes repository state and suggests prioritized fixes
func SuggestFixes(state *RepositoryState) []Fix {
	var fixes []Fix

	// Suggest fixes based on each dimension
	fixes = append(fixes, suggestExistenceFixes(state.Existence)...)
	fixes = append(fixes, suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)...)
	fixes = append(fixes, suggestWorkingTreeFixes(state.WorkingTree)...)
	fixes = append(fixes, suggestCorruptionFixes(state.Corruption)...)

	// Sort by priority (1=critical, 5=low)
	return fixes
}

// suggestExistenceFixes suggests fixes for existence scenarios (E1-E8)
func suggestExistenceFixes(existence ExistenceState) []Fix {
	switch existence.ID {
	case "E1":
		return nil // All good

	case "E2": // Core exists, GitHub doesn't
		return []Fix{{
			ScenarioID:  "E2",
			Description: "GitHub remote not configured",
			Command:     "githelper github setup --create",
			Operation:   nil, // Requires GitHub client setup
			AutoFixable: false,
			Priority:    2,
			Reason:      "Dual-push requires both Core and GitHub remotes",
		}}

	case "E3": // GitHub exists, Core doesn't
		return []Fix{{
			ScenarioID:  "E3",
			Description: "Core remote not configured",
			Command:     "git remote add origin <core-url>",
			Operation:   nil, // Requires URL from config
			AutoFixable: false,
			Priority:    1,
			Reason:      "Primary remote missing",
		}}

	case "E4": // Local only
		return []Fix{{
			ScenarioID:  "E4",
			Description: "No remotes configured",
			Command:     "githelper repo create <name>",
			Operation:   nil,
			AutoFixable: false,
			Priority:    2,
			Reason:      "Repository not integrated with gitDualRemote system",
		}}

	case "E5": // Core + GitHub exist, local missing
		return []Fix{{
			ScenarioID:  "E5",
			Description: "Repository not cloned locally",
			Command:     "git clone <core-url>",
			Operation:   nil,
			AutoFixable: false,
			Priority:    2,
			Reason:      "Clone from Core to create local working copy",
		}}

	case "E6": // Core exists, local + GitHub missing
		return []Fix{{
			ScenarioID:  "E6",
			Description: "Local repository and GitHub missing",
			Command:     "git clone <core-url> && githelper github setup",
			Operation:   nil,
			AutoFixable: false,
			Priority:    2,
			Reason:      "Clone from Core and set up GitHub remote",
		}}

	case "E7": // GitHub exists, local + Core missing
		return []Fix{{
			ScenarioID:  "E7",
			Description: "Local repository and Core remote missing",
			Command:     "git clone <github-url> && git remote add origin <core-url>",
			Operation:   nil,
			AutoFixable: false,
			Priority:    2,
			Reason:      "Clone from GitHub and configure Core remote",
		}}

	case "E8": // No repositories exist
		return []Fix{{
			ScenarioID:  "E8",
			Description: "No repositories exist anywhere",
			Command:     "githelper repo create <name>",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Initialize new repository with gitDualRemote",
		}}

	default:
		return nil
	}
}

// suggestSyncFixes suggests fixes for sync scenarios (S1-S13)
func suggestSyncFixes(sync SyncState, coreRemote, githubRemote string) []Fix {
	switch sync.ID {
	case "S1":
		return nil // Perfect sync

	case "S2": // Local ahead (full or partial)
		var remoteName string
		var aheadCount int
		var description string

		// Determine which remote to suggest pushing to
		if sync.PartialSync {
			remoteName = sync.AvailableRemote
			if sync.LocalAheadOfCore > 0 {
				aheadCount = sync.LocalAheadOfCore
			} else {
				aheadCount = sync.LocalAheadOfGitHub
			}
			description = fmt.Sprintf("Local has %d unpushed commits to %s", aheadCount, remoteName)
		} else {
			// Full sync - push to both remotes
			remoteName = coreRemote
			aheadCount = sync.LocalAheadOfCore
			description = "Local has unpushed commits"
		}

		return []Fix{{
			ScenarioID:  "S2",
			Description: description,
			Command:     fmt.Sprintf("git push %s %s", remoteName, sync.Branch),
			Operation: &PushOperation{
				Remote:  remoteName,
				Refspec: sync.Branch,
			},
			AutoFixable: true,
			Priority:    4,
			Reason:      "Push local commits to remote",
		}}

	case "S3": // Local behind (full or partial)
		var remoteName string
		var behindCount int
		var description string

		if sync.PartialSync {
			remoteName = sync.AvailableRemote
			if sync.LocalBehindCore > 0 {
				behindCount = sync.LocalBehindCore
			} else {
				behindCount = sync.LocalBehindGitHub
			}
			description = fmt.Sprintf("Remote %s has %d commits not in local", remoteName, behindCount)
		} else {
			remoteName = coreRemote
			behindCount = sync.LocalBehindCore
			description = "Remotes have commits local doesn't have"
		}

		return []Fix{{
			ScenarioID:  "S3",
			Description: description,
			Command:     fmt.Sprintf("git pull %s %s", remoteName, sync.Branch),
			Operation: &PullOperation{
				Remote: remoteName,
				Branch: sync.Branch,
			},
			AutoFixable: true,
			Priority:    4,
			Reason:      "Pull updates from remote",
		}}

	case "S4": // Diverged or Local ahead of GitHub only
		// Check if this is partial sync (two-way divergence)
		if sync.PartialSync {
			var remoteName string
			if sync.AvailableRemote != "" {
				remoteName = sync.AvailableRemote
			} else {
				remoteName = "remote"
			}
			return []Fix{{
				ScenarioID:  "S4",
				Description: fmt.Sprintf("Local diverged from %s - manual merge required", remoteName),
				Command:     fmt.Sprintf("git pull %s %s --rebase", remoteName, sync.Branch),
				Operation:   nil,
				AutoFixable: false,
				Priority:    2,
				Reason:      "Branch has diverged - manual intervention needed",
			}}
		}

		// Full sync: Local ahead of GitHub only
		return []Fix{{
			ScenarioID:  "S4",
			Description: "GitHub is behind (partial push failure)",
			Command:     fmt.Sprintf("git push %s %s", githubRemote, sync.Branch),
			Operation: &PushOperation{
				Remote:  githubRemote,
				Refspec: sync.Branch,
			},
			AutoFixable: true,
			Priority:    2,
			Reason:      "Sync GitHub with local and Core",
		}}

	case "S5": // Local ahead of Core only
		return []Fix{{
			ScenarioID:  "S5",
			Description: "Core is behind (partial push failure)",
			Command:     fmt.Sprintf("git push %s %s", coreRemote, sync.Branch),
			Operation: &PushOperation{
				Remote:  coreRemote,
				Refspec: sync.Branch,
			},
			AutoFixable: true,
			Priority:    2,
			Reason:      "Sync Core with local and GitHub",
		}}

	case "S6": // Local behind GitHub only
		return []Fix{{
			ScenarioID:  "S6",
			Description: "Local behind GitHub",
			Command:     fmt.Sprintf("git pull %s %s", githubRemote, sync.Branch),
			Operation: &PullOperation{
				Remote: githubRemote,
				Branch: sync.Branch,
			},
			AutoFixable: true,
			Priority:    3,
			Reason:      "Pull updates from GitHub",
		}}

	case "S7": // Local behind Core only
		return []Fix{{
			ScenarioID:  "S7",
			Description: "Local behind Core",
			Command:     fmt.Sprintf("git pull %s %s", coreRemote, sync.Branch),
			Operation: &PullOperation{
				Remote: coreRemote,
				Branch: sync.Branch,
			},
			AutoFixable: true,
			Priority:    3,
			Reason:      "Pull updates from Core",
		}}

	case "S8": // GitHub ahead of Core and local
		return []Fix{{
			ScenarioID:  "S8",
			Description: "GitHub has commits not in Core or local",
			Command:     fmt.Sprintf("git pull %s %s && git push %s %s", githubRemote, sync.Branch, coreRemote, sync.Branch),
			Operation: &CompositeOperation{
				Operations: []Operation{
					&PullOperation{Remote: githubRemote, Branch: sync.Branch},
					&PushOperation{Remote: coreRemote, Refspec: sync.Branch},
				},
				StopOnError: true,
			},
			AutoFixable: false,
			Priority:    2,
			Reason:      "Manual intervention: pull from GitHub, then push to Core",
		}}

	case "S9": // Core ahead of GitHub and local
		return []Fix{{
			ScenarioID:  "S9",
			Description: "Core has commits not in GitHub or local",
			Command:     fmt.Sprintf("git pull %s %s && git push %s %s", coreRemote, sync.Branch, githubRemote, sync.Branch),
			Operation: &CompositeOperation{
				Operations: []Operation{
					&PullOperation{Remote: coreRemote, Branch: sync.Branch},
					&PushOperation{Remote: githubRemote, Refspec: sync.Branch},
				},
				StopOnError: true,
			},
			AutoFixable: false,
			Priority:    2,
			Reason:      "Manual intervention: pull from Core, then push to GitHub",
		}}

	case "S10", "S11", "S12", "S13": // Divergence scenarios
		return []Fix{{
			ScenarioID:  sync.ID,
			Description: "Remotes have diverged - manual merge required",
			Command:     "See docs/DIVERGENCE.md for resolution steps",
			Operation:   nil,
			AutoFixable: false,
			Priority:    1,
			Reason:      "Three-way merge required - cannot auto-fix divergence",
		}}

	case "S_UNAVAILABLE":
		return []Fix{{
			ScenarioID:  "S_UNAVAILABLE",
			Description: "Remote unavailable - check network/credentials",
			Command:     "Check remote configuration: git remote -v",
			Operation:   nil,
			AutoFixable: false,
			Priority:    1,
			Reason:      "Cannot determine sync status without remote access",
		}}

	case "S_NA_DETACHED":
		return []Fix{{
			ScenarioID:  "S_NA_DETACHED",
			Description: "Detached HEAD - checkout a branch",
			Command:     "git checkout main",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Sync detection requires being on a branch",
		}}

	default:
		return nil
	}
}

// suggestWorkingTreeFixes suggests fixes for working tree scenarios (W1-W5)
func suggestWorkingTreeFixes(wt WorkingTreeState) []Fix {
	switch wt.ID {
	case "W1":
		return nil // Clean

	case "W2": // Staged changes
		return []Fix{{
			ScenarioID:  "W2",
			Description: "Uncommitted staged changes",
			Command:     "git commit -m 'Your commit message'",
			Operation:   nil, // Requires commit message from user
			AutoFixable: false,
			Priority:    3,
			Reason:      "Commit staged changes before pushing",
		}}

	case "W3": // Unstaged changes
		return []Fix{{
			ScenarioID:  "W3",
			Description: "Uncommitted unstaged changes",
			Command:     "git add . && git commit -m 'Your commit message'",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Stage and commit changes before pushing",
		}}

	case "W4": // Merge conflicts
		return []Fix{{
			ScenarioID:  "W4",
			Description: "Unresolved merge conflicts",
			Command:     "Resolve conflicts manually, then: git add <files> && git commit",
			Operation:   nil,
			AutoFixable: false,
			Priority:    1,
			Reason:      "Manual conflict resolution required",
		}}

	case "W5": // Untracked files
		return []Fix{{
			ScenarioID:  "W5",
			Description: "Untracked files present",
			Command:     "git add <files> or add to .gitignore",
			Operation:   nil,
			AutoFixable: false,
			Priority:    5,
			Reason:      "Decide whether to track or ignore files",
		}}

	default:
		return nil
	}
}

// suggestCorruptionFixes suggests fixes for corruption scenarios (C1-C8)
func suggestCorruptionFixes(corr CorruptionState) []Fix {
	switch corr.ID {
	case "C1":
		return nil // Healthy

	case "C2": // Broken references
		return []Fix{{
			ScenarioID:  "C2",
			Description: "Broken or dangling refs detected",
			Command:     "git fsck --full && git gc --prune=now",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Clean up broken references",
		}}

	case "C3": // Large binaries
		largeFileCount := len(corr.LargeBinaries)
		return []Fix{{
			ScenarioID:  "C3",
			Description: fmt.Sprintf("Large binaries detected (%d files)", largeFileCount),
			Command:     "See docs/BFG_CLEANUP.md for removal instructions",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Large binaries slow down cloning and fetching",
		}}

	case "C4": // Missing objects
		return []Fix{{
			ScenarioID:  "C4",
			Description: "Missing git objects - repository corruption",
			Command:     "git fetch --all or re-clone repository",
			Operation: &FetchOperation{
				Remote: "origin",
			},
			AutoFixable: false,
			Priority:    1,
			Reason:      "Critical: repository may be corrupted",
		}}

	case "C5": // Dangling commits
		return []Fix{{
			ScenarioID:  "C5",
			Description: "Dangling commits detected",
			Command:     "git gc --prune=now (safe to clean up)",
			Operation:   nil,
			AutoFixable: false,
			Priority:    4,
			Reason:      "Orphaned commits can be garbage collected",
		}}

	case "C6": // Git LFS
		return []Fix{{
			ScenarioID:  "C6",
			Description: "Repository uses Git LFS",
			Command:     "git lfs install (if not already installed)",
			Operation:   nil,
			AutoFixable: false,
			Priority:    5,
			Reason:      "Ensure LFS is properly configured",
		}}

	case "C7": // Detached HEAD
		return []Fix{{
			ScenarioID:  "C7",
			Description: "Repository in detached HEAD state",
			Command:     "git checkout <branch-name> or git checkout -b <new-branch>",
			Operation:   nil,
			AutoFixable: false,
			Priority:    3,
			Reason:      "Attach HEAD to a branch to avoid losing work",
		}}

	case "C8": // Shallow clone
		return []Fix{{
			ScenarioID:  "C8",
			Description: "Repository is a shallow clone",
			Command:     "git fetch --unshallow (to get full history)",
			Operation:   nil,
			AutoFixable: false,
			Priority:    4,
			Reason:      "Some operations require full history",
		}}

	default:
		return nil
	}
}

// suggestBranchFixes suggests fixes for branch-specific scenarios (B1-B7)
func suggestBranchFixes(branch BranchState, coreRemote, githubRemote string) []Fix {
	switch branch.ID {
	case "B1":
		return nil // In sync

	case "B2": // Branch ahead of remotes
		return []Fix{{
			ScenarioID:  "B2",
			Description: fmt.Sprintf("Branch %s has unpushed commits", branch.Branch),
			Command:     fmt.Sprintf("git push %s %s", coreRemote, branch.Branch),
			Operation: &PushOperation{
				Remote:  coreRemote,
				Refspec: branch.Branch,
			},
			AutoFixable: true,
			Priority:    4,
			Reason:      "Push branch to remotes",
		}}

	case "B3": // Branch behind remotes
		return []Fix{{
			ScenarioID:  "B3",
			Description: fmt.Sprintf("Branch %s is behind remotes", branch.Branch),
			Command:     fmt.Sprintf("git checkout %s && git pull", branch.Branch),
			Operation: &PullOperation{
				Remote: coreRemote,
				Branch: branch.Branch,
			},
			AutoFixable: true,
			Priority:    4,
			Reason:      "Update branch with remote changes",
		}}

	case "B4": // Branch diverged
		return []Fix{{
			ScenarioID:  "B4",
			Description: fmt.Sprintf("Branch %s has diverged from remotes", branch.Branch),
			Command:     fmt.Sprintf("git checkout %s && git pull --rebase", branch.Branch),
			Operation:   nil,
			AutoFixable: false,
			Priority:    2,
			Reason:      "Manual merge or rebase required for diverged branch",
		}}

	case "B5": // Branch only exists locally
		return []Fix{{
			ScenarioID:  "B5",
			Description: fmt.Sprintf("Branch %s only exists locally", branch.Branch),
			Command:     fmt.Sprintf("git push -u %s %s", coreRemote, branch.Branch),
			Operation: &PushOperation{
				Remote:  coreRemote,
				Refspec: branch.Branch,
			},
			AutoFixable: true,
			Priority:    4,
			Reason:      "Push new branch to remotes",
		}}

	case "B6": // Branch only on Core
		return []Fix{{
			ScenarioID:  "B6",
			Description: fmt.Sprintf("Branch %s exists on %s but not locally", branch.Branch, coreRemote),
			Command:     fmt.Sprintf("git checkout -b %s %s/%s", branch.Branch, coreRemote, branch.Branch),
			Operation:   nil,
			AutoFixable: false,
			Priority:    4,
			Reason:      "Checkout remote branch locally",
		}}

	case "B7": // Branch only on GitHub
		return []Fix{{
			ScenarioID:  "B7",
			Description: fmt.Sprintf("Branch %s exists on %s but not locally", branch.Branch, githubRemote),
			Command:     fmt.Sprintf("git checkout -b %s %s/%s", branch.Branch, githubRemote, branch.Branch),
			Operation:   nil,
			AutoFixable: false,
			Priority:    4,
			Reason:      "Checkout remote branch locally",
		}}

	default:
		return nil
	}
}

// PrioritizeFixes sorts fixes by priority (1=critical, 5=low)
func PrioritizeFixes(fixes []Fix) []Fix {
	// Simple sort by priority
	for i := 0; i < len(fixes)-1; i++ {
		for j := i + 1; j < len(fixes); j++ {
			if fixes[i].Priority > fixes[j].Priority {
				fixes[i], fixes[j] = fixes[j], fixes[i]
			}
		}
	}
	return fixes
}
