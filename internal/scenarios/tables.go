package scenarios

// GetScenarioTable returns the complete scenario lookup table (all 41 scenarios)
func GetScenarioTable() ScenarioTable {
	return ScenarioTable{
		// ========== EXISTENCE SCENARIOS (E1-E8) ==========
		"E1": {
			ID:          "E1",
			Name:        "Fully Configured",
			Description: "Local repository exists with both Core and GitHub remotes configured",
			Category:    CategoryExistence,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Normal state after successful setup",
			},
			ManualSteps: []string{
				"No action needed - repository is properly configured",
			},
			RelatedIDs: []string{},
		},
		"E2": {
			ID:          "E2",
			Name:        "Core Exists, GitHub Missing",
			Description: "Local and Core exist, but GitHub remote not configured",
			Category:    CategoryExistence,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"GitHub setup not yet run",
				"Dual-push not configured",
			},
			ManualSteps: []string{
				"Run: githelper github setup --create",
			},
			RelatedIDs: []string{"E3", "E4"},
		},
		"E3": {
			ID:          "E3",
			Name:        "GitHub Exists, Core Missing",
			Description: "Local and GitHub exist, but Core remote not configured",
			Category:    CategoryExistence,
			Severity:    SeverityError,
			AutoFixable: false,
			TypicalCauses: []string{
				"Primary remote misconfigured",
				"Repository cloned from GitHub directly",
			},
			ManualSteps: []string{
				"Add Core remote: git remote add origin <core-url>",
			},
			RelatedIDs: []string{"E2", "E4"},
		},
		"E4": {
			ID:          "E4",
			Name:        "Local Only",
			Description: "Local repository exists but no remotes configured",
			Category:    CategoryExistence,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"git init run without setup",
				"Remotes removed or misconfigured",
			},
			ManualSteps: []string{
				"Run: githelper repo create <name>",
			},
			RelatedIDs: []string{"E2", "E3"},
		},
		"E5": {
			ID:          "E5",
			Name:        "Core + GitHub Exist, Local Missing",
			Description: "Remotes exist but local repository not cloned",
			Category:    CategoryExistence,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Repository not yet cloned",
				"Local directory deleted",
			},
			ManualSteps: []string{
				"Clone from Core: git clone <core-url>",
			},
			RelatedIDs: []string{"E6", "E7"},
		},
		"E6": {
			ID:          "E6",
			Name:        "Core Exists, Local + GitHub Missing",
			Description: "Only Core remote exists",
			Category:    CategoryExistence,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Bare repository exists but not cloned or backed up to GitHub",
			},
			ManualSteps: []string{
				"Clone from Core: git clone <core-url>",
				"Then run: githelper github setup",
			},
			RelatedIDs: []string{"E5", "E7"},
		},
		"E7": {
			ID:          "E7",
			Name:        "GitHub Exists, Local + Core Missing",
			Description: "Only GitHub remote exists",
			Category:    CategoryExistence,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Repository only on GitHub",
				"Core server not yet configured",
			},
			ManualSteps: []string{
				"Clone from GitHub: git clone <github-url>",
				"Add Core remote: git remote add origin <core-url>",
			},
			RelatedIDs: []string{"E5", "E6"},
		},
		"E8": {
			ID:          "E8",
			Name:        "No Repositories Exist",
			Description: "Repository does not exist anywhere",
			Category:    CategoryExistence,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"New repository to be created",
			},
			ManualSteps: []string{
				"Initialize new repository: githelper repo create <name>",
			},
			RelatedIDs: []string{},
		},

		// ========== SYNC SCENARIOS (S1-S13) ==========
		"S1": {
			ID:          "S1",
			Name:        "Perfect Sync",
			Description: "All three locations (Local, Core, GitHub) are in sync",
			Category:    CategorySync,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Normal state after successful push/pull",
			},
			ManualSteps: []string{
				"No action needed",
			},
			RelatedIDs: []string{},
		},
		"S2": {
			ID:          "S2",
			Name:        "Local Ahead of Both Remotes",
			Description: "Local has unpushed commits",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Made commits but haven't pushed",
			},
			ManualSteps: []string{
				"Push changes: git push",
			},
			RelatedIDs: []string{"S4", "S5"},
		},
		"S3": {
			ID:          "S3",
			Name:        "Local Behind Both Remotes",
			Description: "Remotes have commits local doesn't have",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Collaborator pushed changes",
				"Haven't pulled recently",
			},
			ManualSteps: []string{
				"Pull changes: git pull",
			},
			RelatedIDs: []string{"S6", "S7"},
		},
		"S4": {
			ID:          "S4",
			Name:        "Local Ahead of GitHub Only",
			Description: "Local and Core are synced, GitHub is behind",
			Category:    CategorySync,
			Severity:    SeverityError,
			AutoFixable: true,
			TypicalCauses: []string{
				"Partial push failure",
				"GitHub remote unreachable during push",
			},
			ManualSteps: []string{
				"Push to GitHub: git push github",
			},
			RelatedIDs: []string{"S2", "S5"},
		},
		"S5": {
			ID:          "S5",
			Name:        "Local Ahead of Core Only",
			Description: "Local and GitHub are synced, Core is behind",
			Category:    CategorySync,
			Severity:    SeverityError,
			AutoFixable: true,
			TypicalCauses: []string{
				"Partial push failure",
				"Core remote unreachable during push",
			},
			ManualSteps: []string{
				"Push to Core: git push origin",
			},
			RelatedIDs: []string{"S2", "S4"},
		},
		"S6": {
			ID:          "S6",
			Name:        "Local Behind GitHub Only",
			Description: "Core and GitHub are synced, local is behind",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Fetch succeeded but merge didn't happen",
			},
			ManualSteps: []string{
				"Pull from remote: git pull",
			},
			RelatedIDs: []string{"S3", "S7"},
		},
		"S7": {
			ID:          "S7",
			Name:        "Local Behind Core Only",
			Description: "Core ahead of local and GitHub",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Push to Core succeeded but GitHub failed",
			},
			ManualSteps: []string{
				"Pull from Core: git pull origin",
			},
			RelatedIDs: []string{"S3", "S6"},
		},
		"S8": {
			ID:          "S8",
			Name:        "GitHub Ahead of Core and Local",
			Description: "GitHub has commits not in Core or local",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Direct push to GitHub bypassing Core",
				"Manual GitHub editing",
			},
			ManualSteps: []string{
				"Pull from GitHub: git pull github",
				"Then push to Core: git push origin",
			},
			RelatedIDs: []string{"S9", "S10"},
		},
		"S9": {
			ID:          "S9",
			Name:        "Core Ahead of GitHub and Local",
			Description: "Core has commits not in GitHub or local",
			Category:    CategorySync,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Collaborator pushed to Core only",
			},
			ManualSteps: []string{
				"Pull from Core: git pull origin",
				"Push to GitHub: git push github",
			},
			RelatedIDs: []string{"S8", "S10"},
		},
		"S10": {
			ID:          "S10",
			Name:        "Core and GitHub Diverged (Local in Sync with One)",
			Description: "Core and GitHub have different commits, local matches one",
			Category:    CategorySync,
			Severity:    SeverityError,
			AutoFixable: false,
			TypicalCauses: []string{
				"Inconsistent pushes",
				"Manual intervention on one remote",
			},
			ManualSteps: []string{
				"Manual merge required",
				"Fetch both remotes and resolve divergence",
			},
			RelatedIDs: []string{"S11", "S12", "S13"},
		},
		"S11": {
			ID:          "S11",
			Name:        "Local Ahead of Diverged Remotes",
			Description: "Local has commits, Core and GitHub have diverged",
			Category:    CategorySync,
			Severity:    SeverityCritical,
			AutoFixable: false,
			TypicalCauses: []string{
				"Complex push failures",
				"Manual intervention required",
			},
			ManualSteps: []string{
				"Manually resolve remote divergence first",
				"Then push local changes",
			},
			RelatedIDs: []string{"S10", "S12", "S13"},
		},
		"S12": {
			ID:          "S12",
			Name:        "Local Behind Diverged Remotes",
			Description: "Core and GitHub have diverged, local is behind both",
			Category:    CategorySync,
			Severity:    SeverityCritical,
			AutoFixable: false,
			TypicalCauses: []string{
				"Haven't synced since divergence occurred",
			},
			ManualSteps: []string{
				"Resolve remote divergence first",
				"Then pull updates",
			},
			RelatedIDs: []string{"S10", "S11", "S13"},
		},
		"S13": {
			ID:          "S13",
			Name:        "Three-Way Divergence",
			Description: "All three locations have unique commits",
			Category:    CategorySync,
			Severity:    SeverityCritical,
			AutoFixable: false,
			TypicalCauses: []string{
				"Multiple concurrent modifications",
				"Network issues during sync",
			},
			ManualSteps: []string{
				"Manual three-way merge required",
				"Consult docs/DIVERGENCE.md",
			},
			RelatedIDs: []string{"S10", "S11", "S12"},
		},

		// ========== WORKING TREE SCENARIOS (W1-W5) ==========
		"W1": {
			ID:          "W1",
			Name:        "Clean Working Tree",
			Description: "No uncommitted changes",
			Category:    CategoryWorkingTree,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"All changes committed",
			},
			ManualSteps: []string{
				"No action needed",
			},
			RelatedIDs: []string{},
		},
		"W2": {
			ID:          "W2",
			Name:        "Uncommitted Changes (Staged)",
			Description: "Staged changes not yet committed",
			Category:    CategoryWorkingTree,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Files added but not committed",
			},
			ManualSteps: []string{
				"Commit changes: git commit -m 'message'",
			},
			RelatedIDs: []string{"W3", "W4"},
		},
		"W3": {
			ID:          "W3",
			Name:        "Uncommitted Changes (Unstaged)",
			Description: "Modified files not staged",
			Category:    CategoryWorkingTree,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Made changes but haven't staged",
			},
			ManualSteps: []string{
				"Stage changes: git add .",
				"Then commit: git commit -m 'message'",
			},
			RelatedIDs: []string{"W2", "W4"},
		},
		"W4": {
			ID:          "W4",
			Name:        "Merge Conflicts",
			Description: "Unresolved merge conflicts present",
			Category:    CategoryWorkingTree,
			Severity:    SeverityError,
			AutoFixable: false,
			TypicalCauses: []string{
				"Failed merge or rebase",
			},
			ManualSteps: []string{
				"Resolve conflicts manually",
				"Then: git add <resolved-files>",
				"Finally: git commit",
			},
			RelatedIDs: []string{"S10", "S11", "S12", "S13"},
		},
		"W5": {
			ID:          "W5",
			Name:        "Untracked Files",
			Description: "New files not tracked by git",
			Category:    CategoryWorkingTree,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Created new files",
				"Build artifacts",
			},
			ManualSteps: []string{
				"Add to git: git add <files>",
				"Or ignore: add to .gitignore",
			},
			RelatedIDs: []string{"W2"},
		},

		// ========== CORRUPTION/HEALTH SCENARIOS (C1-C8) ==========
		"C1": {
			ID:          "C1",
			Name:        "Healthy Repository",
			Description: "No corruption or health issues detected",
			Category:    CategoryCorruption,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Normal state",
			},
			ManualSteps: []string{
				"No action needed",
			},
			RelatedIDs: []string{},
		},
		"C2": {
			ID:          "C2",
			Name:        "Broken References",
			Description: "Dangling or broken git refs",
			Category:    CategoryCorruption,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"git gc ran and pruned refs",
				"Interrupted operations",
			},
			ManualSteps: []string{
				"Run: git fsck --full",
				"Clean up: git gc --prune=now",
			},
			RelatedIDs: []string{"C3", "C4"},
		},
		"C3": {
			ID:          "C3",
			Name:        "Large Binaries Detected",
			Description: "Large binary files in repository history",
			Category:    CategoryCorruption,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Accidentally committed large files",
				"Binary files tracked in git",
			},
			ManualSteps: []string{
				"Consider using Git LFS",
				"Or remove with BFG Repo-Cleaner",
				"See: docs/BFG_CLEANUP.md",
			},
			RelatedIDs: []string{"C6"},
		},
		"C4": {
			ID:          "C4",
			Name:        "Missing Objects",
			Description: "Git objects missing from repository",
			Category:    CategoryCorruption,
			Severity:    SeverityCritical,
			AutoFixable: false,
			TypicalCauses: []string{
				"Repository corruption",
				"Interrupted clone or fetch",
			},
			ManualSteps: []string{
				"Try fetching from remote: git fetch --all",
				"If persistent, re-clone repository",
			},
			RelatedIDs: []string{"C2"},
		},
		"C5": {
			ID:          "C5",
			Name:        "Dangling Commits",
			Description: "Orphaned commits not reachable from any ref",
			Category:    CategoryCorruption,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Branch deletion",
				"git reset --hard",
			},
			ManualSteps: []string{
				"Usually safe to ignore",
				"Clean up with: git gc --prune=now",
			},
			RelatedIDs: []string{"C2"},
		},
		"C6": {
			ID:          "C6",
			Name:        "Git LFS Repository",
			Description: "Repository uses Git LFS for large files",
			Category:    CategoryCorruption,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Intentionally using LFS",
			},
			ManualSteps: []string{
				"Ensure git-lfs is installed",
				"Run: git lfs install",
			},
			RelatedIDs: []string{"C3"},
		},
		"C7": {
			ID:          "C7",
			Name:        "Detached HEAD State",
			Description: "HEAD is not pointing to a branch",
			Category:    CategoryCorruption,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Checked out specific commit",
				"During rebase or cherry-pick",
			},
			ManualSteps: []string{
				"Create branch: git checkout -b <branch-name>",
				"Or return to branch: git checkout <branch-name>",
			},
			RelatedIDs: []string{},
		},
		"C8": {
			ID:          "C8",
			Name:        "Shallow Clone",
			Description: "Repository is a shallow clone with limited history",
			Category:    CategoryCorruption,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Cloned with --depth flag",
			},
			ManualSteps: []string{
				"Unshallow: git fetch --unshallow",
			},
			RelatedIDs: []string{},
		},

		// ========== BRANCH-SPECIFIC SCENARIOS (B1-B7) ==========
		"B1": {
			ID:          "B1",
			Name:        "Branch in Sync",
			Description: "Branch is synced across all locations",
			Category:    CategoryBranch,
			Severity:    SeverityInfo,
			AutoFixable: false,
			TypicalCauses: []string{
				"Normal state",
			},
			ManualSteps: []string{
				"No action needed",
			},
			RelatedIDs: []string{},
		},
		"B2": {
			ID:          "B2",
			Name:        "Branch Ahead of Remotes",
			Description: "Local branch has unpushed commits",
			Category:    CategoryBranch,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Made commits on branch but haven't pushed",
			},
			ManualSteps: []string{
				"Push branch: git push <remote> <branch>",
			},
			RelatedIDs: []string{"B3", "B4"},
		},
		"B3": {
			ID:          "B3",
			Name:        "Branch Behind Remotes",
			Description: "Remote branch has commits local doesn't have",
			Category:    CategoryBranch,
			Severity:    SeverityWarning,
			AutoFixable: true,
			TypicalCauses: []string{
				"Collaborator pushed to branch",
			},
			ManualSteps: []string{
				"Pull branch: git pull <remote> <branch>",
			},
			RelatedIDs: []string{"B2", "B4"},
		},
		"B4": {
			ID:          "B4",
			Name:        "Branch Diverged",
			Description: "Local and remote branch have different commits",
			Category:    CategoryBranch,
			Severity:    SeverityError,
			AutoFixable: false,
			TypicalCauses: []string{
				"Concurrent modifications",
				"Force push by collaborator",
			},
			ManualSteps: []string{
				"Merge or rebase required",
				"git pull --rebase <remote> <branch>",
			},
			RelatedIDs: []string{"B2", "B3"},
		},
		"B5": {
			ID:          "B5",
			Name:        "Branch Only Exists Locally",
			Description: "Branch exists locally but not on any remote",
			Category:    CategoryBranch,
			Severity:    SeverityInfo,
			AutoFixable: true,
			TypicalCauses: []string{
				"Created branch but haven't pushed",
			},
			ManualSteps: []string{
				"Push new branch: git push -u <remote> <branch>",
			},
			RelatedIDs: []string{"B6", "B7"},
		},
		"B6": {
			ID:          "B6",
			Name:        "Branch Only Exists on Core",
			Description: "Branch exists on Core but not locally or on GitHub",
			Category:    CategoryBranch,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Collaborator created branch on Core",
			},
			ManualSteps: []string{
				"Checkout branch: git checkout -b <branch> origin/<branch>",
			},
			RelatedIDs: []string{"B5", "B7"},
		},
		"B7": {
			ID:          "B7",
			Name:        "Branch Only Exists on GitHub",
			Description: "Branch exists on GitHub but not locally or on Core",
			Category:    CategoryBranch,
			Severity:    SeverityWarning,
			AutoFixable: false,
			TypicalCauses: []string{
				"Branch created directly on GitHub",
			},
			ManualSteps: []string{
				"Fetch and checkout: git fetch github",
				"git checkout -b <branch> github/<branch>",
			},
			RelatedIDs: []string{"B5", "B6"},
		},
	}
}

// LookupScenario returns the definition for a scenario ID
func LookupScenario(id string) (ScenarioDefinition, bool) {
	table := GetScenarioTable()
	def, ok := table[id]
	return def, ok
}

// GetScenariosByCategory returns all scenarios in a category
func GetScenariosByCategory(category string) []ScenarioDefinition {
	table := GetScenarioTable()
	var scenarios []ScenarioDefinition

	for _, def := range table {
		if def.Category == category {
			scenarios = append(scenarios, def)
		}
	}

	return scenarios
}

// GetAutoFixableScenarios returns all scenarios that can be auto-fixed
func GetAutoFixableScenarios() []ScenarioDefinition {
	table := GetScenarioTable()
	var scenarios []ScenarioDefinition

	for _, def := range table {
		if def.AutoFixable {
			scenarios = append(scenarios, def)
		}
	}

	return scenarios
}
