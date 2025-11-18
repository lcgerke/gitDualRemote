package github

import (
	"context"
	"os/exec"
	"testing"
)

func TestCreateRepositoryViaGH(t *testing.T) {
	// Skip if gh is not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed")
	}

	tests := []struct {
		name        string
		repoName    string
		description string
		private     bool
		wantErr     bool
	}{
		{
			name:        "create public repo",
			repoName:    "test-repo-public",
			description: "Test public repository",
			private:     false,
			wantErr:     false,
		},
		{
			name:        "create private repo",
			repoName:    "test-repo-private",
			description: "Test private repository",
			private:     true,
			wantErr:     false,
		},
		{
			name:        "empty name should error",
			repoName:    "",
			description: "Invalid",
			private:     false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client (note: doesn't need token for gh CLI approach)
			ctx := context.Background()
			client := NewClient(ctx, "")

			err := client.CreateRepositoryViaGH(tt.repoName, tt.description, tt.private)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRepositoryViaGH() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Cleanup: delete the test repo if it was created
			if err == nil && tt.repoName != "" {
				// Use gh CLI to delete the repo
				cmd := exec.Command("gh", "repo", "delete", tt.repoName, "--yes")
				_ = cmd.Run() // Best effort cleanup
			}
		})
	}
}

func TestCheckGHCLIAvailable(t *testing.T) {
	available := CheckGHCLIAvailable()

	// Just verify it returns a boolean without error
	t.Logf("gh CLI available: %v", available)
}

func TestCheckGHAuthenticated(t *testing.T) {
	if !CheckGHCLIAvailable() {
		t.Skip("gh CLI not installed")
	}

	authenticated := CheckGHAuthenticated()
	t.Logf("gh CLI authenticated: %v", authenticated)
}
