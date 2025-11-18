package scenarios

import (
	"strings"
	"testing"
)

// Test S2 partial sync fix suggestion
func TestSuggestFixes_S2_Partial(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:               "S2",
			PartialSync:      true,
			AvailableRemote:  "origin",
			LocalAheadOfCore: 9,
			Branch:           "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S2" {
		t.Errorf("Expected ScenarioID S2, got %s", fix.ScenarioID)
	}
	if !strings.Contains(fix.Command, "git push origin main") {
		t.Errorf("Expected command to contain 'git push origin main', got %s", fix.Command)
	}
	if !fix.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
	if !strings.Contains(fix.Description, "9 unpushed commits") {
		t.Errorf("Expected description to mention 9 commits, got %s", fix.Description)
	}
}

// Test S3 partial sync fix suggestion
func TestSuggestFixes_S3_Partial(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:                "S3",
			PartialSync:       true,
			AvailableRemote:   "origin",
			LocalBehindCore:   5,
			Branch:            "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S3" {
		t.Errorf("Expected ScenarioID S3, got %s", fix.ScenarioID)
	}
	if !strings.Contains(fix.Command, "git pull origin main") {
		t.Errorf("Expected command to contain 'git pull origin main', got %s", fix.Command)
	}
	if !fix.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
	if !strings.Contains(fix.Description, "5 commits") {
		t.Errorf("Expected description to mention 5 commits, got %s", fix.Description)
	}
}

// Test S4 partial sync (diverged) fix suggestion
func TestSuggestFixes_S4_Partial_Diverged(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:                "S4",
			PartialSync:       true,
			AvailableRemote:   "origin",
			LocalAheadOfCore:  3,
			LocalBehindCore:   2,
			Diverged:          true,
			Branch:            "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S4" {
		t.Errorf("Expected ScenarioID S4, got %s", fix.ScenarioID)
	}
	if !strings.Contains(fix.Command, "git pull origin main --rebase") {
		t.Errorf("Expected command to contain rebase, got %s", fix.Command)
	}
	if fix.AutoFixable {
		t.Error("Expected AutoFixable to be false for diverged scenario")
	}
	if !strings.Contains(fix.Description, "diverged") {
		t.Errorf("Expected description to mention diverged, got %s", fix.Description)
	}
}

// Test S_UNAVAILABLE error scenario
func TestSuggestFixes_S_UNAVAILABLE(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:              "S_UNAVAILABLE",
			PartialSync:     true,
			AvailableRemote: "origin",
			Error:           "Connection refused",
			Branch:          "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S_UNAVAILABLE" {
		t.Errorf("Expected ScenarioID S_UNAVAILABLE, got %s", fix.ScenarioID)
	}
	if fix.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
	if fix.Priority != 1 {
		t.Errorf("Expected Priority 1 (critical), got %d", fix.Priority)
	}
}

// Test S_NA_DETACHED error scenario
func TestSuggestFixes_S_NA_DETACHED(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:          "S_NA_DETACHED",
			PartialSync: true,
			Error:       "not on a branch",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S_NA_DETACHED" {
		t.Errorf("Expected ScenarioID S_NA_DETACHED, got %s", fix.ScenarioID)
	}
	if fix.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
	if !strings.Contains(fix.Command, "git checkout") {
		t.Errorf("Expected command to contain 'git checkout', got %s", fix.Command)
	}
}

// Test S2 full sync (non-partial) - ensure no regression
func TestSuggestFixes_S2_Full(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:                 "S2",
			PartialSync:        false, // Full three-way sync
			LocalAheadOfCore:   5,
			LocalAheadOfGitHub: 5,
			Branch:             "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if fix.ScenarioID != "S2" {
		t.Errorf("Expected ScenarioID S2, got %s", fix.ScenarioID)
	}
	if !fix.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
	// For full sync, it should push to the core remote
	if !strings.Contains(fix.Command, "origin") {
		t.Errorf("Expected command to push to origin, got %s", fix.Command)
	}
}

// Test S3 with GitHub remote (E3 scenario)
func TestSuggestFixes_S3_GitHub(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:                  "S3",
			PartialSync:         true,
			AvailableRemote:     "github",
			LocalBehindGitHub:   3,
			Branch:              "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	if len(fixes) != 1 {
		t.Fatalf("Expected 1 fix, got %d", len(fixes))
	}

	fix := fixes[0]
	if !strings.Contains(fix.Command, "git pull github main") {
		t.Errorf("Expected command to pull from github, got %s", fix.Command)
	}
}

// Test S1 with partial sync (in sync but only with one remote)
func TestSuggestFixes_S1_Partial(t *testing.T) {
	state := &RepositoryState{
		CoreRemote:   "origin",
		GitHubRemote: "github",
		Sync: SyncState{
			ID:               "S1",
			PartialSync:      true,
			AvailableRemote:  "origin",
			LocalAheadOfCore: 0,
			LocalBehindCore:  0,
			Branch:           "main",
		},
	}

	fixes := suggestSyncFixes(state.Sync, state.CoreRemote, state.GitHubRemote)

	// S1 should return no fixes (in sync)
	if len(fixes) != 0 {
		t.Errorf("Expected 0 fixes for S1, got %d", len(fixes))
	}
}
