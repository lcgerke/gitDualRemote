package scenarios

import (
	"encoding/json"
	"time"
)

// RepositoryState represents the complete classified state of a repository
type RepositoryState struct {
	// Core identification
	RepoPath      string    `json:"repo_path"`
	DetectedAt    time.Time `json:"detected_at"`
	DetectionTime Duration  `json:"detection_time_ms"`

	// Remote configuration
	CoreRemote   string `json:"core_remote"`   // e.g., "origin"
	GitHubRemote string `json:"github_remote"` // e.g., "github"

	// Classified states (5 dimensions)
	Existence   ExistenceState   `json:"existence"`
	Sync        SyncState        `json:"sync"`
	WorkingTree WorkingTreeState `json:"working_tree"`
	Corruption  CorruptionState  `json:"corruption"`
	Branches    []BranchState    `json:"branches"`

	// Warnings and metadata
	Warnings      []Warning `json:"warnings,omitempty"`
	LFSEnabled    bool      `json:"lfs_enabled"`
	DetachedHEAD  bool      `json:"detached_head"`
	ShallowClone  bool      `json:"shallow_clone"`
	DefaultBranch string    `json:"default_branch"`
}

// Duration wraps time.Duration for JSON marshaling
type Duration struct {
	time.Duration
}

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Milliseconds())
}

// ExistenceState describes which repository locations exist
type ExistenceState struct {
	ID          string `json:"id"`           // E1-E8
	Description string `json:"description"`  // Human-readable
	LocalExists bool   `json:"local_exists"`
	LocalPath   string `json:"local_path,omitempty"`

	CoreExists     bool   `json:"core_exists"`
	CoreReachable  bool   `json:"core_reachable"`
	CoreURL        string `json:"core_url,omitempty"`

	GitHubExists    bool   `json:"github_exists"`
	GitHubReachable bool   `json:"github_reachable"`
	GitHubURL       string `json:"github_url,omitempty"`
}

// SyncState describes synchronization status of default branch
type SyncState struct {
	ID          string `json:"id"`          // S1-S13
	Description string `json:"description"` // Human-readable

	Branch string `json:"branch"` // Default branch name

	LocalHash  string `json:"local_hash,omitempty"`
	CoreHash   string `json:"core_hash,omitempty"`
	GitHubHash string `json:"github_hash,omitempty"`

	LocalAheadOfCore   int `json:"local_ahead_of_core"`
	LocalBehindCore    int `json:"local_behind_core"`
	LocalAheadOfGitHub int `json:"local_ahead_of_github"`
	LocalBehindGitHub  int `json:"local_behind_github"`
	CoreAheadOfGitHub  int `json:"core_ahead_of_github"`
	CoreBehindGitHub   int `json:"core_behind_github"`

	Diverged bool `json:"diverged"` // True if manual merge needed
}

// BranchState describes sync status of a single branch
type BranchState struct {
	ID          string `json:"id"`          // B1-B7
	Description string `json:"description"` // Human-readable
	Branch      string `json:"branch"`      // Branch name

	LocalHash  string `json:"local_hash,omitempty"`
	CoreHash   string `json:"core_hash,omitempty"`
	GitHubHash string `json:"github_hash,omitempty"`

	LocalAheadOfCore   int `json:"local_ahead_of_core"`
	LocalBehindCore    int `json:"local_behind_core"`
	LocalAheadOfGitHub int `json:"local_ahead_of_github"`
	LocalBehindGitHub  int `json:"local_behind_github"`

	Diverged bool `json:"diverged"`
}

// WorkingTreeState describes local modifications
type WorkingTreeState struct {
	ID          string `json:"id"`          // W1-W5
	Description string `json:"description"` // Human-readable

	Clean         bool     `json:"clean"`
	StagedFiles   []string `json:"staged_files,omitempty"`
	UnstagedFiles []string `json:"unstaged_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
}

// CorruptionState describes repository health issues
type CorruptionState struct {
	ID          string `json:"id"`          // C1-C8
	Description string `json:"description"` // Human-readable

	Healthy       bool           `json:"healthy"`
	LargeBinaries []LargeBinary  `json:"large_binaries,omitempty"`
	BrokenRefs    []string       `json:"broken_refs,omitempty"`
	MissingObjects []string      `json:"missing_objects,omitempty"`
	DanglingCommits []string     `json:"dangling_commits,omitempty"`
}

// LargeBinary represents a large binary file detected in history
type LargeBinary struct {
	SHA1   string  `json:"sha1"`     // Object SHA1
	SizeMB float64 `json:"size_mb"`  // Size in megabytes
	// Path is NOT populated (too expensive to lookup)
	// Users must manually run: git log --all --find-object=<sha>
}

// Warning represents a non-critical issue or edge case
type Warning struct {
	Code    string `json:"code"`    // W_LFS_ENABLED, W_DETACHED_HEAD, etc.
	Message string `json:"message"` // Human-readable warning
	Hint    string `json:"hint,omitempty"` // Suggested action
}

// Fix represents a suggested remediation with optional auto-fix capability
type Fix struct {
	ScenarioID  string    `json:"scenario_id"`  // E.g., "S2", "W2"
	Description string    `json:"description"`  // Human-readable issue
	Command     string    `json:"command"`      // Manual fix command
	Operation   Operation `json:"-"`            // Structured operation (for auto-fix)
	AutoFixable bool      `json:"auto_fixable"` // Can be automatically fixed
	Priority    int       `json:"priority"`     // 1=critical, 5=low
	Reason      string    `json:"reason"`       // Why this fix is needed
}

// Operation represents a structured fix operation (interface defined in operations.go)
type Operation interface {
	// Validate checks if operation is safe to execute
	Validate(state *RepositoryState, gitClient interface{}) error

	// Execute performs the operation
	Execute(gitClient interface{}) error

	// Describe returns human-readable description
	Describe() string

	// Rollback attempts to undo the operation (best effort)
	Rollback(gitClient interface{}) error
}

// DetectionOptions configures what gets detected
type DetectionOptions struct {
	// SkipFetch disables pre-flight fetch (use cached remote data)
	SkipFetch bool

	// SkipCorruption disables expensive corruption checks
	SkipCorruption bool

	// SkipBranches disables per-branch analysis (only check default branch)
	SkipBranches bool

	// MaxBranches limits how many branches to analyze (0 = unlimited)
	MaxBranches int

	// BinarySizeThresholdMB sets large binary detection threshold
	BinarySizeThresholdMB float64

	// FetchTimeout sets timeout for fetch operations
	FetchTimeout time.Duration

	// RemoteCheckTimeout sets timeout for remote reachability checks
	RemoteCheckTimeout time.Duration
}

// DefaultDetectionOptions returns sensible defaults
func DefaultDetectionOptions() DetectionOptions {
	return DetectionOptions{
		SkipFetch:             false,
		SkipCorruption:        false,
		SkipBranches:          false,
		MaxBranches:           100, // Prevent extreme ref counts from hanging
		BinarySizeThresholdMB: 50.0, // 50MB threshold
		FetchTimeout:          30 * time.Second,
		RemoteCheckTimeout:    5 * time.Second,
	}
}

// Classifier is the main state detection engine
type Classifier struct {
	gitClient     interface{} // git.Client (interface to avoid import cycle)
	coreRemote    string
	githubRemote  string
	options       DetectionOptions
}

// Suggester provides fix suggestions based on classified state
type Suggester struct {
	// Configuration for fix suggestion logic
}

// AutoFixResult contains results of auto-fix execution
type AutoFixResult struct {
	Applied []Fix   `json:"applied"` // Successfully applied fixes
	Failed  []Fix   `json:"failed"`  // Failed fixes
	Errors  []error `json:"-"`       // Detailed errors
}

// Scenario lookup types (used in tables.go)
type ScenarioDefinition struct {
	ID          string
	Name        string
	Description string
	Category    string // "existence", "sync", "working_tree", "corruption", "branch"
	Severity    string // "info", "warning", "error", "critical"
	AutoFixable bool
	TypicalCauses []string
	ManualSteps   []string
	RelatedIDs    []string
}

// ScenarioTable is a lookup table for all 41 scenarios
type ScenarioTable map[string]ScenarioDefinition

// Constants for scenario categories
const (
	CategoryExistence   = "existence"
	CategorySync        = "sync"
	CategoryWorkingTree = "working_tree"
	CategoryCorruption  = "corruption"
	CategoryBranch      = "branch"
)

// Constants for scenario severity
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// Constants for warning codes
const (
	WarnLFSEnabled         = "W_LFS_ENABLED"
	WarnDetachedHEAD       = "W_DETACHED_HEAD"
	WarnShallowClone       = "W_SHALLOW_CLONE"
	WarnManyBranches       = "W_MANY_BRANCHES"
	WarnStaleRemoteData    = "W_STALE_REMOTE_DATA"
	WarnNetworkUnreachable = "W_NETWORK_UNREACHABLE"
)
