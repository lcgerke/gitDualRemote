package scenarios

import (
	"fmt"
	"time"

	"github.com/lcgerke/githelper/internal/git"
)

// ============================================================================
// PHASE 2: Classifier Core - State Detection Engine
// ============================================================================

// NewClassifier creates a new scenario classifier
func NewClassifier(gitClient *git.Client, coreRemote, githubRemote string, options DetectionOptions) *Classifier {
	return &Classifier{
		gitClient:    gitClient,
		coreRemote:   coreRemote,
		githubRemote: githubRemote,
		options:      options,
	}
}

// Detect performs full state detection and returns classified repository state
func (c *Classifier) Detect() (*RepositoryState, error) {
	start := time.Now()

	state := &RepositoryState{
		DetectedAt:   start,
		CoreRemote:   c.coreRemote,
		GitHubRemote: c.githubRemote,
		Warnings:     []Warning{},
	}

	gc := c.gitClient.(*git.Client)

	// Pre-flight fetch (unless disabled)
	if !c.options.SkipFetch {
		// Fetch both remotes concurrently (but serialized through mutex)
		if err := gc.FetchRemote(c.coreRemote); err != nil {
			state.Warnings = append(state.Warnings, Warning{
				Code:    WarnStaleRemoteData,
				Message: fmt.Sprintf("Failed to fetch from %s: %v", c.coreRemote, err),
				Hint:    "Using stale remote data, run with --no-fetch to suppress",
			})
		}

		if err := gc.FetchRemote(c.githubRemote); err != nil {
			state.Warnings = append(state.Warnings, Warning{
				Code:    WarnStaleRemoteData,
				Message: fmt.Sprintf("Failed to fetch from %s: %v", c.githubRemote, err),
				Hint:    "Using stale remote data",
			})
		}
	}

	// Detect existence (E1-E8)
	existence, err := c.detectExistence(gc)
	if err != nil {
		return nil, fmt.Errorf("existence detection failed: %w", err)
	}
	state.Existence = existence

	// Detect working tree (W1-W5)
	if existence.LocalExists {
		workingTree, err := c.detectWorkingTree(gc)
		if err != nil {
			return nil, fmt.Errorf("working tree detection failed: %w", err)
		}
		state.WorkingTree = workingTree

		// Check for edge cases
		if detached, _ := gc.IsDetachedHEAD(); detached {
			state.DetachedHEAD = true
			state.Warnings = append(state.Warnings, Warning{
				Code:    WarnDetachedHEAD,
				Message: "Repository is in detached HEAD state",
				Hint:    "Create a branch or checkout an existing branch",
			})
		}

		if shallow, _ := gc.IsShallowClone(); shallow {
			state.ShallowClone = true
			state.Warnings = append(state.Warnings, Warning{
				Code:    WarnShallowClone,
				Message: "Repository is a shallow clone",
				Hint:    "Some operations may be limited, run 'git fetch --unshallow' to get full history",
			})
		}

		if lfsEnabled, _ := gc.CheckLFSEnabled(); lfsEnabled {
			state.LFSEnabled = true
			state.Warnings = append(state.Warnings, Warning{
				Code:    WarnLFSEnabled,
				Message: "Repository uses Git LFS",
				Hint:    "Ensure git-lfs is installed: git lfs install",
			})
		}
	}

	// Detect corruption (C1-C8) - unless skipped
	if !c.options.SkipCorruption && existence.LocalExists {
		corruption, err := c.detectCorruption(gc)
		if err != nil {
			return nil, fmt.Errorf("corruption detection failed: %w", err)
		}
		state.Corruption = corruption
	} else {
		state.Corruption = CorruptionState{
			ID:          "C1",
			Description: "Healthy Repository (not checked)",
			Healthy:     true,
		}
	}

	// Detect sync state - expanded to handle E1, E2, E3 scenarios
	if existence.LocalExists {
		// Get default branch (try from available remote, fallback to "main")
		var defaultBranch string
		if existence.CoreExists {
			defaultBranch, _ = gc.GetDefaultBranch(c.coreRemote)
		} else if existence.GitHubExists {
			defaultBranch, _ = gc.GetDefaultBranch(c.githubRemote)
		}
		if defaultBranch == "" {
			defaultBranch = "main" // fallback
		}
		state.DefaultBranch = defaultBranch

		// Detect sync based on which remotes exist
		switch state.Existence.ID {
		case "E1":
			// Full three-way sync detection
			sync, err := c.detectDefaultBranchSync(gc, defaultBranch)
			if err != nil {
				return nil, fmt.Errorf("sync detection failed: %w", err)
			}
			state.Sync = sync

			// Detect per-branch topology (B1-B7) - unless skipped
			if !c.options.SkipBranches {
				branches, err := c.detectBranchTopology(gc)
				if err != nil {
					return nil, fmt.Errorf("branch topology detection failed: %w", err)
				}
				state.Branches = branches
			}

		case "E2": // Local + Core exist, GitHub missing
			state.Sync = c.detectTwoWaySync(gc, defaultBranch, c.coreRemote, "", "GitHub")

		case "E3": // Local + GitHub exist, Core missing
			state.Sync = c.detectTwoWaySync(gc, defaultBranch, "", c.githubRemote, "Core")

		default:
			// E4-E8: Not enough locations for sync detection
			state.Sync = SyncState{
				ID:          "S1",
				Description: "N/A (not all locations exist)",
			}
		}
	} else {
		// No local repository - can't detect sync
		state.Sync = SyncState{
			ID:          "S1",
			Description: "N/A (not all locations exist)",
		}
	}

	state.DetectionTime = Duration{time.Since(start)}
	return state, nil
}

// detectExistence determines which repository locations exist (E1-E8)
func (c *Classifier) detectExistence(gc *git.Client) (ExistenceState, error) {
	exists := ExistenceState{}

	// Check local
	localExists, localPath := gc.LocalExists()
	exists.LocalExists = localExists
	exists.LocalPath = localPath

	// Check Core remote
	remotes, err := gc.ListRemotes()
	if err != nil {
		remotes = []string{}
	}

	coreConfigured := false
	githubConfigured := false
	for _, remote := range remotes {
		if remote == c.coreRemote {
			coreConfigured = true
		}
		if remote == c.githubRemote {
			githubConfigured = true
		}
	}

	if coreConfigured {
		exists.CoreExists = true
		coreURL, _ := gc.GetRemoteURL(c.coreRemote)
		exists.CoreURL = coreURL
		exists.CoreReachable = gc.CanReachRemote(c.coreRemote)
	}

	if githubConfigured {
		exists.GitHubExists = true
		githubURL, _ := gc.GetRemoteURL(c.githubRemote)
		exists.GitHubURL = githubURL
		exists.GitHubReachable = gc.CanReachRemote(c.githubRemote)
	}

	// Classify into E1-E8
	if localExists && coreConfigured && githubConfigured {
		exists.ID = "E1"
		exists.Description = "Fully configured (local + core + github)"
	} else if localExists && coreConfigured && !githubConfigured {
		exists.ID = "E2"
		exists.Description = "Core exists, GitHub missing"
	} else if localExists && !coreConfigured && githubConfigured {
		exists.ID = "E3"
		exists.Description = "GitHub exists, Core missing"
	} else if localExists && !coreConfigured && !githubConfigured {
		exists.ID = "E4"
		exists.Description = "Local only (no remotes)"
	} else if !localExists && coreConfigured && githubConfigured {
		exists.ID = "E5"
		exists.Description = "Core + GitHub exist, local missing"
	} else if !localExists && coreConfigured && !githubConfigured {
		exists.ID = "E6"
		exists.Description = "Core exists, local + GitHub missing"
	} else if !localExists && !coreConfigured && githubConfigured {
		exists.ID = "E7"
		exists.Description = "GitHub exists, local + Core missing"
	} else {
		exists.ID = "E8"
		exists.Description = "No repositories exist"
	}

	return exists, nil
}

// detectWorkingTree determines local modification state (W1-W5)
func (c *Classifier) detectWorkingTree(gc *git.Client) (WorkingTreeState, error) {
	wt := WorkingTreeState{}

	stagedFiles, err := gc.GetStagedFiles()
	if err != nil {
		return wt, err
	}
	wt.StagedFiles = stagedFiles

	unstagedFiles, err := gc.GetUnstagedFiles()
	if err != nil {
		return wt, err
	}
	wt.UnstagedFiles = unstagedFiles

	untrackedFiles, err := gc.GetUntrackedFiles()
	if err != nil {
		return wt, err
	}
	wt.UntrackedFiles = untrackedFiles

	conflictFiles, err := gc.GetConflictFiles()
	if err != nil {
		return wt, err
	}
	wt.ConflictFiles = conflictFiles

	// Classify into W1-W5
	if len(conflictFiles) > 0 {
		wt.ID = "W4"
		wt.Description = "Merge conflicts"
		wt.Clean = false
	} else if len(stagedFiles) > 0 {
		wt.ID = "W2"
		wt.Description = "Uncommitted changes (staged)"
		wt.Clean = false
	} else if len(unstagedFiles) > 0 {
		wt.ID = "W3"
		wt.Description = "Uncommitted changes (unstaged)"
		wt.Clean = false
	} else if len(untrackedFiles) > 0 {
		wt.ID = "W5"
		wt.Description = "Untracked files"
		wt.Clean = true // Untracked files don't make tree "dirty"
	} else {
		wt.ID = "W1"
		wt.Description = "Clean working tree"
		wt.Clean = true
	}

	return wt, nil
}

// detectCorruption checks repository health (C1-C8)
func (c *Classifier) detectCorruption(gc *git.Client) (CorruptionState, error) {
	corr := CorruptionState{
		Healthy: true,
	}

	// Check for large binaries
	thresholdBytes := int64(c.options.BinarySizeThresholdMB * 1024 * 1024)
	largeBinaries, err := gc.ScanLargeBinaries(thresholdBytes)
	if err != nil {
		return corr, fmt.Errorf("large binary scan failed: %w", err)
	}

	if len(largeBinaries) > 0 {
		corr.ID = "C3"
		corr.Description = fmt.Sprintf("Large binaries detected (%d files)", len(largeBinaries))
		corr.Healthy = false
		// Convert git.LargeBinary to scenarios.LargeBinary
		for _, lb := range largeBinaries {
			corr.LargeBinaries = append(corr.LargeBinaries, LargeBinary{
				SHA1:   lb.SHA1,
				SizeMB: lb.SizeMB,
			})
		}
		return corr, nil
	}

	// If Git LFS is enabled, mark as C6
	if lfsEnabled, _ := gc.CheckLFSEnabled(); lfsEnabled {
		corr.ID = "C6"
		corr.Description = "Git LFS repository"
		corr.Healthy = true // LFS is intentional, not corruption
		return corr, nil
	}

	// If detached HEAD, mark as C7
	if detached, _ := gc.IsDetachedHEAD(); detached {
		corr.ID = "C7"
		corr.Description = "Detached HEAD state"
		corr.Healthy = true // Not really corruption
		return corr, nil
	}

	// If shallow clone, mark as C8
	if shallow, _ := gc.IsShallowClone(); shallow {
		corr.ID = "C8"
		corr.Description = "Shallow clone"
		corr.Healthy = true // Not corruption
		return corr, nil
	}

	// Default: healthy
	corr.ID = "C1"
	corr.Description = "Healthy repository"
	return corr, nil
}

// detectDefaultBranchSync determines sync state of default branch (S1-S13)
func (c *Classifier) detectDefaultBranchSync(gc *git.Client, branch string) (SyncState, error) {
	sync := SyncState{
		Branch: branch,
	}

	// Get hashes for all three locations
	localHash, err := gc.GetBranchHash(branch)
	if err != nil {
		return sync, fmt.Errorf("failed to get local hash: %w", err)
	}
	sync.LocalHash = localHash

	coreHash, err := gc.GetRemoteBranchHash(c.coreRemote, branch)
	if err != nil {
		return sync, fmt.Errorf("failed to get core hash: %w", err)
	}
	sync.CoreHash = coreHash

	githubHash, err := gc.GetRemoteBranchHash(c.githubRemote, branch)
	if err != nil {
		return sync, fmt.Errorf("failed to get github hash: %w", err)
	}
	sync.GitHubHash = githubHash

	// If all same, perfect sync (S1)
	if localHash == coreHash && localHash == githubHash {
		sync.ID = "S1"
		sync.Description = "Perfect sync"
		return sync, nil
	}

	// Calculate commit counts between each pair
	if localHash != coreHash {
		sync.LocalAheadOfCore, _ = gc.CountCommitsBetween(localHash, coreHash)
		sync.LocalBehindCore, _ = gc.CountCommitsBetween(coreHash, localHash)
	}

	if localHash != githubHash {
		sync.LocalAheadOfGitHub, _ = gc.CountCommitsBetween(localHash, githubHash)
		sync.LocalBehindGitHub, _ = gc.CountCommitsBetween(githubHash, localHash)
	}

	if coreHash != githubHash {
		sync.CoreAheadOfGitHub, _ = gc.CountCommitsBetween(coreHash, githubHash)
		sync.CoreBehindGitHub, _ = gc.CountCommitsBetween(githubHash, coreHash)
	}

	// Classify into S2-S13 based on commit counts
	localAhead := sync.LocalAheadOfCore > 0 || sync.LocalAheadOfGitHub > 0
	localBehind := sync.LocalBehindCore > 0 || sync.LocalBehindGitHub > 0
	remotesDiverged := sync.CoreAheadOfGitHub > 0 && sync.CoreBehindGitHub > 0

	if localAhead && !localBehind && !remotesDiverged {
		if sync.LocalAheadOfCore > 0 && sync.LocalAheadOfGitHub > 0 {
			sync.ID = "S2"
			sync.Description = "Local ahead of both remotes"
		} else if sync.LocalAheadOfGitHub > 0 {
			sync.ID = "S4"
			sync.Description = "Local ahead of GitHub only"
		} else {
			sync.ID = "S5"
			sync.Description = "Local ahead of Core only"
		}
	} else if localBehind && !localAhead && !remotesDiverged {
		if sync.LocalBehindCore > 0 && sync.LocalBehindGitHub > 0 {
			sync.ID = "S3"
			sync.Description = "Local behind both remotes"
		} else if sync.LocalBehindCore > 0 {
			sync.ID = "S7"
			sync.Description = "Local behind Core only"
		} else {
			sync.ID = "S6"
			sync.Description = "Local behind GitHub only"
		}
	} else if remotesDiverged {
		if localAhead && localBehind {
			sync.ID = "S13"
			sync.Description = "Three-way divergence"
			sync.Diverged = true
		} else if localAhead {
			sync.ID = "S11"
			sync.Description = "Local ahead of diverged remotes"
			sync.Diverged = true
		} else if localBehind {
			sync.ID = "S12"
			sync.Description = "Local behind diverged remotes"
			sync.Diverged = true
		} else {
			sync.ID = "S10"
			sync.Description = "Core and GitHub diverged (local in sync with one)"
			sync.Diverged = true
		}
	} else if sync.CoreAheadOfGitHub > 0 && sync.CoreBehindGitHub == 0 {
		sync.ID = "S9"
		sync.Description = "Core ahead of GitHub and local"
	} else if sync.CoreBehindGitHub > 0 && sync.CoreAheadOfGitHub == 0 {
		sync.ID = "S8"
		sync.Description = "GitHub ahead of Core and local"
	} else {
		sync.ID = "S1"
		sync.Description = "In sync"
	}

	return sync, nil
}

// gitClient defines the interface for git operations needed by detectTwoWaySync
type gitClient interface {
	FetchRemote(remote string) error
	GetBranchHash(branch string) (string, error)
	GetRemoteBranchHash(remote, branch string) (string, error)
	CountCommitsBetween(ref1, ref2 string) (int, error)
}

// detectTwoWaySync performs two-way sync detection for E2/E3 scenarios
// where only one remote is available (either core or github, but not both)
func (c *Classifier) detectTwoWaySync(
	gc gitClient,
	branch string,
	coreRemote string,
	githubRemote string,
	missingRemoteName string,
) SyncState {
	// Determine which remote is available
	remoteName := coreRemote
	if remoteName == "" {
		remoteName = githubRemote
	}

	// 1. FETCH from available remote (ensures fresh data)
	err := gc.FetchRemote(remoteName)
	if err != nil {
		// Remote unreachable - return error state
		return SyncState{
			ID:          "S_UNAVAILABLE",
			Description: fmt.Sprintf("Cannot connect to %s", remoteName),
			Branch:      branch,
			PartialSync: true,
			Error:       err.Error(),
		}
	}

	// 2. Get commit hashes
	localHash, err := gc.GetBranchHash(branch)
	if err != nil {
		return SyncState{
			ID:          "S_NA_DETACHED",
			Description: "Sync N/A (detached HEAD or missing branch)",
			PartialSync: true,
			Error:       err.Error(),
		}
	}

	remoteHash, err := gc.GetRemoteBranchHash(remoteName, branch)
	if err != nil {
		return SyncState{
			ID:          "S_UNAVAILABLE",
			Description: fmt.Sprintf("Remote branch %s/%s not found", remoteName, branch),
			Branch:      branch,
			PartialSync: true,
			Error:       err.Error(),
		}
	}

	// 3. Calculate ahead/behind counts (reuse existing helper)
	ahead, _ := gc.CountCommitsBetween(localHash, remoteHash)
	behind, _ := gc.CountCommitsBetween(remoteHash, localHash)

	// 4. Determine sync scenario and build description
	var scenarioID string
	var description string
	diverged := ahead > 0 && behind > 0

	if ahead == 0 && behind == 0 {
		scenarioID = "S1"
		description = fmt.Sprintf("In sync with %s (%s N/A)", remoteName, missingRemoteName)
	} else if ahead > 0 && behind == 0 {
		scenarioID = "S2"
		description = fmt.Sprintf("Local ahead of %s (%s N/A)", remoteName, missingRemoteName)
	} else if ahead == 0 && behind > 0 {
		scenarioID = "S3"
		description = fmt.Sprintf("Local behind %s (%s N/A)", remoteName, missingRemoteName)
	} else {
		scenarioID = "S4"
		description = fmt.Sprintf("Diverged from %s (%s N/A)", remoteName, missingRemoteName)
	}

	// 5. Build SyncState with appropriate fields populated
	state := SyncState{
		ID:          scenarioID,
		Description: description,
		Branch:      branch,
		PartialSync: true,
		LocalHash:   localHash,
		Diverged:    diverged,
	}

	// Populate remote-specific fields
	if coreRemote != "" {
		state.CoreHash = remoteHash
		state.LocalAheadOfCore = ahead
		state.LocalBehindCore = behind
		state.AvailableRemote = coreRemote
	} else {
		state.GitHubHash = remoteHash
		state.LocalAheadOfGitHub = ahead
		state.LocalBehindGitHub = behind
		state.AvailableRemote = githubRemote
	}

	return state
}

// detectBranchTopology analyzes per-branch sync state (B1-B7 for each branch)
func (c *Classifier) detectBranchTopology(gc *git.Client) ([]BranchState, error) {
	localBranches, _, err := gc.ListBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	// Limit branches if configured
	if c.options.MaxBranches > 0 && len(localBranches) > c.options.MaxBranches {
		localBranches = localBranches[:c.options.MaxBranches]
	}

	var branchStates []BranchState

	for _, branch := range localBranches {
		bs := BranchState{
			Branch: branch,
		}

		// Get local hash
		localHash, err := gc.GetBranchHash(branch)
		if err != nil {
			continue
		}
		bs.LocalHash = localHash

		// Get remote hashes
		coreHash, _ := gc.GetRemoteBranchHash(c.coreRemote, branch)
		bs.CoreHash = coreHash

		githubHash, _ := gc.GetRemoteBranchHash(c.githubRemote, branch)
		bs.GitHubHash = githubHash

		// Classify
		if coreHash == "" && githubHash == "" {
			bs.ID = "B5"
			bs.Description = "Branch only exists locally"
		} else if localHash == coreHash && localHash == githubHash {
			bs.ID = "B1"
			bs.Description = "Branch in sync"
		} else {
			// Calculate divergence
			if coreHash != "" {
				bs.LocalAheadOfCore, _ = gc.CountCommitsBetween(localHash, coreHash)
				bs.LocalBehindCore, _ = gc.CountCommitsBetween(coreHash, localHash)
			}
			if githubHash != "" {
				bs.LocalAheadOfGitHub, _ = gc.CountCommitsBetween(localHash, githubHash)
				bs.LocalBehindGitHub, _ = gc.CountCommitsBetween(githubHash, localHash)
			}

			ahead := bs.LocalAheadOfCore > 0 || bs.LocalAheadOfGitHub > 0
			behind := bs.LocalBehindCore > 0 || bs.LocalBehindGitHub > 0

			if ahead && behind {
				bs.ID = "B4"
				bs.Description = "Branch diverged"
				bs.Diverged = true
			} else if ahead {
				bs.ID = "B2"
				bs.Description = "Branch ahead of remotes"
			} else if behind {
				bs.ID = "B3"
				bs.Description = "Branch behind remotes"
			} else {
				bs.ID = "B1"
				bs.Description = "Branch in sync"
			}
		}

		branchStates = append(branchStates, bs)
	}

	return branchStates, nil
}
