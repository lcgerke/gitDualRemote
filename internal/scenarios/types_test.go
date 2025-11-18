package scenarios

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDurationMarshalJSON verifies Duration marshals to milliseconds correctly
func TestDurationMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: Duration{time.Duration(0)},
			expected: "0",
		},
		{
			name:     "1 millisecond",
			duration: Duration{time.Millisecond},
			expected: "1",
		},
		{
			name:     "100 milliseconds",
			duration: Duration{100 * time.Millisecond},
			expected: "100",
		},
		{
			name:     "1 second (1000ms)",
			duration: Duration{time.Second},
			expected: "1000",
		},
		{
			name:     "2500 milliseconds",
			duration: Duration{2500 * time.Millisecond},
			expected: "2500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.duration)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			got := string(data)
			if got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

// TestRepositoryStateMarshalJSON verifies RepositoryState marshals correctly
func TestRepositoryStateMarshalJSON(t *testing.T) {
	state := &RepositoryState{
		RepoPath:      "/path/to/repo",
		DetectedAt:    time.Now(),
		DetectionTime: Duration{250 * time.Millisecond},
		CoreRemote:    "origin",
		GitHubRemote:  "github",
		LFSEnabled:    false,
		Existence: ExistenceState{
			ID:           "E1",
			Description:  "All configured",
			LocalExists:  true,
			CoreExists:   true,
			GitHubExists: true,
		},
		Sync: SyncState{
			ID:          "S1",
			Description: "Perfect sync",
			Branch:      "main",
		},
		WorkingTree: WorkingTreeState{
			ID:          "W1",
			Description: "Clean",
		},
		Corruption: CorruptionState{
			ID:          "C1",
			Description: "Healthy",
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify detection_time_ms is a number, not a string or invalid
	detectionTime, ok := unmarshaled["detection_time_ms"]
	if !ok {
		t.Error("detection_time_ms not found in JSON")
	}

	// Should be a valid number (float64 in JSON)
	_, ok = detectionTime.(float64)
	if !ok {
		t.Errorf("detection_time_ms should be a number, got type %T: %v", detectionTime, detectionTime)
	}
}
