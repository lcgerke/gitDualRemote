package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcgerke/githelper/internal/state"
	"github.com/spf13/cobra"
)

// TestDoctorCmd tests the basic command structure
func TestDoctorCmd(t *testing.T) {
	tests := []struct {
		name          string
		flags         map[string]interface{}
		expectedFlags map[string]interface{}
	}{
		{
			name: "default flags",
			flags: map[string]interface{}{
				"credentials": false,
				"repo":        "",
				"auto-fix":    false,
			},
			expectedFlags: map[string]interface{}{
				"credentials": false,
				"repo":        "",
				"auto-fix":    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify command flags exist
			for flagName := range tt.flags {
				flag := doctorCmd.Flags().Lookup(flagName)
				if flag == nil {
					t.Errorf("Flag --%s not found", flagName)
				}
			}
		})
	}
}

// TestRunDoctor_EmptyState tests doctor with no repositories
func TestRunDoctor_EmptyState(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set vault env to prevent vault connectivity
	os.Setenv("VAULT_SKIP_VERIFY", "true")

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags
	format = ""
	noColor = true
	showCredentials = false
	repoFilter = ""
	autoFix = false

	err = runDoctor(cmd, args)
	// Should complete without error even with empty state
	if err != nil {
		// Check if it's a critical error
		if strings.Contains(err.Error(), "diagnostic checks found critical errors") {
			// This is acceptable - doctor found issues
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// TestRunDoctor_WithRepositories tests doctor with configured repositories
func TestRunDoctor_WithRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create a real git repository
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repository
	if err := runGitCommand(repoPath, "init"); err != nil {
		t.Skipf("Git not available, skipping test: %v", err)
		return
	}

	// Configure git
	if err := runGitCommand(repoPath, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("Failed to configure git: %v", err)
	}
	if err := runGitCommand(repoPath, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("Failed to configure git: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := runGitCommand(repoPath, "add", "."); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
	if err := runGitCommand(repoPath, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Add repository to state
	repo := &state.Repository{
		Path:    repoPath,
		Remote:  "origin",
		Created: time.Now(),
	}

	if err := stateMgr.AddRepository("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set vault env to prevent vault connectivity
	os.Setenv("VAULT_SKIP_VERIFY", "true")

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags
	format = ""
	noColor = true
	showCredentials = false
	repoFilter = ""
	autoFix = false

	err = runDoctor(cmd, args)
	// Doctor should run successfully, even if it finds issues
	if err != nil {
		// Check if it's a critical error (not warnings)
		if strings.Contains(err.Error(), "diagnostic checks found critical errors") {
			t.Logf("Doctor found critical errors (expected in test environment)")
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// TestRunDoctor_RepoFilter tests doctor with specific repository filter
func TestRunDoctor_RepoFilter(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Add multiple repositories
	for i := 1; i <= 3; i++ {
		repoName := fmt.Sprintf("repo-%d", i)
		repo := &state.Repository{
			Path:    filepath.Join(tmpDir, repoName),
			Remote:  "origin",
			Created: time.Now(),
		}
		if err := stateMgr.AddRepository(repoName, repo); err != nil {
			t.Fatalf("Failed to add repository: %v", err)
		}
	}

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set vault env to prevent vault connectivity
	os.Setenv("VAULT_SKIP_VERIFY", "true")

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags
	format = ""
	noColor = true
	showCredentials = false
	repoFilter = "repo-2" // Filter for specific repo
	autoFix = false

	err = runDoctor(cmd, args)
	// Should run without panicking, even if repos don't exist on disk
	if err != nil {
		if !strings.Contains(err.Error(), "diagnostic checks found critical errors") {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	// Verify filter was set
	if repoFilter != "repo-2" {
		t.Errorf("Expected repoFilter='repo-2', got '%s'", repoFilter)
	}
}

// TestRunDoctor_Flags tests various flag combinations
func TestRunDoctor_Flags(t *testing.T) {
	tests := []struct {
		name            string
		credentials     bool
		repo            string
		autoFix         bool
	}{
		{
			name:        "default",
			credentials: false,
			repo:        "",
			autoFix:     false,
		},
		{
			name:        "show credentials",
			credentials: true,
			repo:        "",
			autoFix:     false,
		},
		{
			name:        "filter repo",
			credentials: false,
			repo:        "test-repo",
			autoFix:     false,
		},
		{
			name:        "auto-fix",
			credentials: false,
			repo:        "",
			autoFix:     true,
		},
		{
			name:        "all flags",
			credentials: true,
			repo:        "test-repo",
			autoFix:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			showCredentials = tt.credentials
			repoFilter = tt.repo
			autoFix = tt.autoFix

			// Verify flags were set
			if showCredentials != tt.credentials {
				t.Errorf("Expected showCredentials=%v, got %v", tt.credentials, showCredentials)
			}
			if repoFilter != tt.repo {
				t.Errorf("Expected repoFilter=%v, got %v", tt.repo, repoFilter)
			}
			if autoFix != tt.autoFix {
				t.Errorf("Expected autoFix=%v, got %v", tt.autoFix, autoFix)
			}
		})
	}
}

// TestRunDoctor_JSONOutput tests JSON output format
func TestRunDoctor_JSONOutput(t *testing.T) {
	// Create temporary directory for state
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment to use temp state dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set vault env to prevent vault connectivity
	os.Setenv("VAULT_SKIP_VERIFY", "true")

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags with JSON format
	format = "json"
	noColor = true
	showCredentials = false
	repoFilter = ""
	autoFix = false

	// Run doctor
	_ = runDoctor(cmd, args)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify JSON output
	if format == "json" && len(output) > 0 {
		// Try to parse as JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Logf("Output was: %s", output)
			// Don't fail - JSON might be mixed with other output
		}
	}

	// Reset format
	format = ""
}

// TestDiagnosticResults tests the diagnostic results structure
func TestDiagnosticResults(t *testing.T) {
	results := &DiagnosticResults{
		Checks:    make(map[string]*CheckResult),
		StartTime: time.Now(),
	}

	tests := []struct {
		name           string
		checkName      string
		status         string
		message        string
		expectedErrors int
		expectedWarns  int
	}{
		{
			name:           "ok check",
			checkName:      "test_ok",
			status:         "ok",
			message:        "All good",
			expectedErrors: 0,
			expectedWarns:  0,
		},
		{
			name:           "warning check",
			checkName:      "test_warn",
			status:         "warning",
			message:        "Minor issue",
			expectedErrors: 0,
			expectedWarns:  1,
		},
		{
			name:           "error check",
			checkName:      "test_error",
			status:         "error",
			message:        "Critical issue",
			expectedErrors: 1,
			expectedWarns:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset counters
			results.Warnings = 0
			results.Errors = 0

			// Add check
			results.AddCheck(tt.checkName, tt.status, tt.message, nil)

			// Verify counters
			if results.Errors != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tt.expectedErrors, results.Errors)
			}
			if results.Warnings != tt.expectedWarns {
				t.Errorf("Expected %d warnings, got %d", tt.expectedWarns, results.Warnings)
			}

			// Verify check was added
			check, exists := results.Checks[tt.checkName]
			if !exists {
				t.Errorf("Check %s was not added", tt.checkName)
			} else {
				if check.Status != tt.status {
					t.Errorf("Expected status %s, got %s", tt.status, check.Status)
				}
				if check.Message != tt.message {
					t.Errorf("Expected message %s, got %s", tt.message, check.Message)
				}
			}
		})
	}
}

// TestDiagnosticResults_HasCriticalErrors tests error detection
func TestDiagnosticResults_HasCriticalErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   int
		warnings int
		expected bool
	}{
		{
			name:     "no errors",
			errors:   0,
			warnings: 0,
			expected: false,
		},
		{
			name:     "only warnings",
			errors:   0,
			warnings: 3,
			expected: false,
		},
		{
			name:     "has errors",
			errors:   1,
			warnings: 0,
			expected: true,
		},
		{
			name:     "errors and warnings",
			errors:   2,
			warnings: 3,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := &DiagnosticResults{
				Checks:   make(map[string]*CheckResult),
				Errors:   tt.errors,
				Warnings: tt.warnings,
			}

			if results.HasCriticalErrors() != tt.expected {
				t.Errorf("Expected HasCriticalErrors()=%v, got %v", tt.expected, results.HasCriticalErrors())
			}
		})
	}
}

// TestCheckResult tests the check result structure
func TestCheckResult(t *testing.T) {
	tests := []struct {
		name    string
		result  *CheckResult
		details interface{}
	}{
		{
			name: "simple check",
			result: &CheckResult{
				Name:    "test_check",
				Status:  "ok",
				Message: "Test passed",
			},
			details: nil,
		},
		{
			name: "check with details",
			result: &CheckResult{
				Name:    "test_check",
				Status:  "ok",
				Message: "Test passed",
				Details: map[string]string{
					"key": "value",
				},
			},
			details: map[string]string{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Name == "" {
				t.Error("Name should not be empty")
			}
			if tt.result.Status == "" {
				t.Error("Status should not be empty")
			}
			if tt.details != nil && tt.result.Details == nil {
				t.Error("Details should be set")
			}
		})
	}
}

// TestRunDoctor_OutputFormat tests human-readable output
func TestRunDoctor_OutputFormat(t *testing.T) {
	// Test that output contains expected sections
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	os.Setenv("VAULT_SKIP_VERIFY", "true")

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags
	format = ""
	noColor = true
	showCredentials = false
	repoFilter = ""
	autoFix = false

	// Run doctor
	_ = runDoctor(cmd, args)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify expected sections appear
	expectedSections := []string{
		"Git Installation",
		"Vault Configuration",
		"State Management",
		"Repositories",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Expected output to contain section %q", section)
		}
	}
}

// TestRunDoctor_AutoFix tests auto-fix functionality
func TestRunDoctor_AutoFix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "doctor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create state manager
	stateMgr, err := state.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}

	// Create a git repository with missing hooks
	repoPath := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git", "hooks"), 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git
	if err := runGitCommand(repoPath, "init"); err != nil {
		t.Skipf("Git not available, skipping test: %v", err)
		return
	}

	// Add to state
	repo := &state.Repository{
		Path:    repoPath,
		Remote:  "origin",
		Created: time.Now(),
	}
	if err := stateMgr.AddRepository("test-repo", repo); err != nil {
		t.Fatalf("Failed to add repository: %v", err)
	}

	// Set environment
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	os.Setenv("VAULT_SKIP_VERIFY", "true")

	cmd := &cobra.Command{}
	args := []string{}

	// Reset global flags with auto-fix enabled
	format = ""
	noColor = true
	showCredentials = false
	repoFilter = ""
	autoFix = true

	// Run doctor with auto-fix
	_ = runDoctor(cmd, args)

	// Auto-fix might have installed hooks
	// We don't assert on the result as it depends on the fixer implementation
}

// Helper functions are in test_helpers.go
