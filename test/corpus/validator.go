package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Validator orchestrates corpus testing
type Validator struct {
	manifest      RepoManifest
	cache         *Cache
	reporter      *Reporter
	maxConcurrent int
}

// NewValidator creates a new validator
func NewValidator(manifestPath string) (*Validator, error) {
	// Load manifest
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest RepoManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Parse TTL
	ttl, err := parseDuration(manifest.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid cache_ttl: %w", err)
	}

	// Create cache
	cache, err := NewCache(manifest.CacheEnabled, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Get git version
	gitVersion, err := getGitVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get git version: %w", err)
	}

	maxConcurrent := manifest.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // Default
	}

	return &Validator{
		manifest:      manifest,
		cache:         cache,
		reporter:      NewReporter(gitVersion),
		maxConcurrent: maxConcurrent,
	}, nil
}

// Run executes the corpus validation
func (v *Validator) Run(ctx context.Context) error {
	startTime := time.Now()

	fmt.Printf("GitHelper Corpus Validator\n")
	fmt.Printf("Version: %s\n", v.manifest.Version)
	fmt.Printf("Description: %s\n", v.manifest.Description)
	fmt.Printf("Total Repos: %d\n", len(v.manifest.Repos))
	fmt.Printf("Concurrency: %d\n", v.maxConcurrent)
	fmt.Printf("Cache: %v (TTL: %s)\n", v.manifest.CacheEnabled, v.manifest.CacheTTL)
	fmt.Println()

	// Show cache stats
	if v.manifest.CacheEnabled {
		count, size, _ := v.cache.Stats()
		fmt.Printf("Cache: %d repos (%.2f MB)\n", count, float64(size)/(1024*1024))
	}

	// Process repos with concurrency control
	var wg sync.WaitGroup
	sem := make(chan struct{}, v.maxConcurrent)

	for i, repo := range v.manifest.Repos {
		if repo.Skip {
			fmt.Printf("[%d/%d] ⊘ Skipping %s: %s\n", i+1, len(v.manifest.Repos), repo.Name, repo.SkipReason)
			continue
		}

		wg.Add(1)
		go func(index int, r RepoConfig) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result := v.testRepo(ctx, r, index+1, len(v.manifest.Repos))
			v.reporter.AddResult(result)
		}(i, repo)
	}

	wg.Wait()

	// Update summary with total duration
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Generate report
	report := v.reporter.GenerateReport()
	report.Summary.TotalDuration = duration.Round(time.Second).String()
	v.reporter = NewReporter(v.reporter.gitVersion)
	for _, r := range report.Results {
		v.reporter.AddResult(r)
	}

	fmt.Printf("\nCompleted in %s\n", duration.Round(time.Second))

	return nil
}

// testRepo tests a single repository
func (v *Validator) testRepo(ctx context.Context, repo RepoConfig, current, total int) TestResult {
	result := TestResult{
		RepoName:  repo.Name,
		RepoType:  repo.Type,
		RepoURL:   repo.URL,
		StartTime: time.Now(),
		Expected:  repo.Expected,
		Tags:      repo.Tags,
	}

	fmt.Printf("[%d/%d] Testing %s (%s)...\n", current, total, repo.Name, repo.Type)

	// Clone or get from cache
	cloneStart := time.Now()
	localPath, fromCache, err := v.cache.Get(repo.URL)
	if err != nil {
		result.Error = fmt.Sprintf("Clone failed: %v", err)
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime).String()
		fmt.Printf("[%d/%d] ✗ %s: %s\n", current, total, repo.Name, result.Error)
		return result
	}
	cloneEnd := time.Now()
	result.CloneTimeMs = cloneEnd.Sub(cloneStart).Milliseconds()
	result.LocalPath = localPath

	if fromCache {
		fmt.Printf("[%d/%d]   → Using cached repo\n", current, total)
	} else {
		fmt.Printf("[%d/%d]   → Cloned fresh (took %d ms)\n", current, total, result.CloneTimeMs)
	}

	// Run classifier (placeholder - will be implemented in Phase 2)
	detectionStart := time.Now()
	detected, err := v.runClassifier(ctx, localPath)
	detectionEnd := time.Now()
	result.DetectionTimeMs = detectionEnd.Sub(detectionStart).Milliseconds()

	if err != nil {
		result.Error = fmt.Sprintf("Classification failed: %v", err)
		result.Success = false
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime).String()
		fmt.Printf("[%d/%d] ✗ %s: %s\n", current, total, repo.Name, result.Error)
		return result
	}

	result.Detected = detected
	result.Success = true

	// Validate against expected scenarios
	if repo.Expected != nil {
		result.Match, result.Mismatches = v.validateExpectations(detected, repo.Expected)
		if !result.Match {
			result.FalsePositive = true
			fmt.Printf("[%d/%d] ⚠ %s: Detected scenarios don't match expectations\n", current, total, repo.Name)
		} else {
			fmt.Printf("[%d/%d] ✓ %s: Matches expectations (took %d ms)\n", current, total, repo.Name, result.DetectionTimeMs)
		}
	} else {
		result.Match = true // No expectations = always matches
		fmt.Printf("[%d/%d] ✓ %s: Classified successfully (took %d ms)\n", current, total, repo.Name, result.DetectionTimeMs)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime).String()

	return result
}

// runClassifier runs the scenario classifier on a repo
// PLACEHOLDER: This will be implemented once the classifier is built
func (v *Validator) runClassifier(ctx context.Context, repoPath string) (*DetectedScenarios, error) {
	// TODO: Once internal/scenarios package is implemented, this will call:
	// classifier := scenarios.NewClassifier(gitClient, "origin", "github")
	// state, err := classifier.Detect(ctx)
	// return convertToDetectedScenarios(state), err

	// For now, return stub data so the validator framework can be tested
	return &DetectedScenarios{
		Existence:   "E1", // Placeholder
		Sync:        "S1", // Placeholder
		WorkingTree: "W1", // Placeholder
		Corruption:  "C1", // Placeholder
		LFSEnabled:  false,
	}, nil
}

// validateExpectations compares detected vs expected scenarios
func (v *Validator) validateExpectations(detected *DetectedScenarios, expected *ExpectedScenario) (bool, []string) {
	var mismatches []string

	if detected.Existence != expected.Existence {
		mismatches = append(mismatches, fmt.Sprintf("Existence: expected %s, got %s", expected.Existence, detected.Existence))
	}

	if detected.Sync != expected.Sync {
		mismatches = append(mismatches, fmt.Sprintf("Sync: expected %s, got %s", expected.Sync, detected.Sync))
	}

	if detected.WorkingTree != expected.WorkingTree {
		mismatches = append(mismatches, fmt.Sprintf("WorkingTree: expected %s, got %s", expected.WorkingTree, detected.WorkingTree))
	}

	if detected.Corruption != expected.Corruption {
		mismatches = append(mismatches, fmt.Sprintf("Corruption: expected %s, got %s", expected.Corruption, detected.Corruption))
	}

	if detected.LFSEnabled != expected.LFSEnabled {
		mismatches = append(mismatches, fmt.Sprintf("LFS: expected %v, got %v", expected.LFSEnabled, detected.LFSEnabled))
	}

	return len(mismatches) == 0, mismatches
}

// GetReporter returns the reporter for external use
func (v *Validator) GetReporter() *Reporter {
	return v.reporter
}

// GetCache returns the cache for external use
func (v *Validator) GetCache() *Cache {
	return v.cache
}

// Helper functions

func getGitVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 7 * 24 * time.Hour, nil // Default 7 days
	}

	// Simple parser for formats like "7d", "24h", "30m"
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	default:
		return time.ParseDuration(s) // Fallback to standard parser
	}

	var count int
	if _, err := fmt.Sscanf(value, "%d", &count); err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", value)
	}

	return time.Duration(count) * multiplier, nil
}
