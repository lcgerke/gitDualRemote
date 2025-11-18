package main

import "time"

// RepoConfig defines a single repository to test
type RepoConfig struct {
	Name     string            `yaml:"name"`
	URL      string            `yaml:"url"`
	Type     string            `yaml:"type"` // "public", "managed", "synthetic"
	Expected *ExpectedScenario `yaml:"expected,omitempty"`
	Notes    string            `yaml:"notes,omitempty"`
	Tags     []string          `yaml:"tags,omitempty"`
	Skip     bool              `yaml:"skip,omitempty"`
	SkipReason string          `yaml:"skip_reason,omitempty"`
}

// ExpectedScenario defines the expected classification
type ExpectedScenario struct {
	Existence  string   `yaml:"existence"`   // E1-E8
	Sync       string   `yaml:"sync"`        // S1-S13
	WorkingTree string  `yaml:"working_tree"` // W1-W5
	Corruption string   `yaml:"corruption"`  // C1-C8
	Branches   []string `yaml:"branches,omitempty"` // B1-B7 for each branch
	LFSEnabled bool     `yaml:"lfs_enabled,omitempty"`
}

// RepoManifest is the root configuration
type RepoManifest struct {
	Version      string       `yaml:"version"`
	Description  string       `yaml:"description"`
	CacheEnabled bool         `yaml:"cache_enabled"`
	CacheTTL     string       `yaml:"cache_ttl"` // e.g., "7d", "24h"
	MaxConcurrent int         `yaml:"max_concurrent"`
	Repos        []RepoConfig `yaml:"repos"`
}

// TestResult captures the result of testing one repo
type TestResult struct {
	RepoName    string    `json:"repo_name"`
	RepoType    string    `json:"repo_type"`
	RepoURL     string    `json:"repo_url"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    string    `json:"duration"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`

	// Detected scenarios
	Detected *DetectedScenarios `json:"detected,omitempty"`

	// Expected scenarios (if provided)
	Expected *ExpectedScenario `json:"expected,omitempty"`

	// Validation results
	Match         bool     `json:"match"`
	Mismatches    []string `json:"mismatches,omitempty"`
	FalsePositive bool     `json:"false_positive"`

	// Performance metrics
	DetectionTimeMs int64 `json:"detection_time_ms"`
	CloneTimeMs     int64 `json:"clone_time_ms,omitempty"`

	// Additional metadata
	LocalPath   string   `json:"local_path"`
	GitVersion  string   `json:"git_version,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// DetectedScenarios captures what the classifier detected
type DetectedScenarios struct {
	Existence   string   `json:"existence"`
	Sync        string   `json:"sync"`
	WorkingTree string   `json:"working_tree"`
	Corruption  string   `json:"corruption"`
	Branches    []string `json:"branches,omitempty"`
	LFSEnabled  bool     `json:"lfs_enabled"`
}

// Summary provides aggregate statistics
type Summary struct {
	TotalRepos       int     `json:"total_repos"`
	SuccessCount     int     `json:"success_count"`
	FailureCount     int     `json:"failure_count"`
	SkippedCount     int     `json:"skipped_count"`
	MatchCount       int     `json:"match_count"`
	MismatchCount    int     `json:"mismatch_count"`
	FalsePositiveCount int   `json:"false_positive_count"`
	FalsePositiveRate float64 `json:"false_positive_rate"`

	// Performance stats
	AvgDetectionTimeMs float64 `json:"avg_detection_time_ms"`
	MaxDetectionTimeMs int64   `json:"max_detection_time_ms"`
	TotalDuration      string  `json:"total_duration"`

	// Scenario distribution
	ScenarioCounts map[string]int `json:"scenario_counts"`
}

// Report is the full test report
type Report struct {
	Version     string       `json:"version"`
	GeneratedAt time.Time    `json:"generated_at"`
	GitVersion  string       `json:"git_version"`
	Summary     Summary      `json:"summary"`
	Results     []TestResult `json:"results"`
	Failures    []TestResult `json:"failures,omitempty"`
	Mismatches  []TestResult `json:"mismatches,omitempty"`
}
