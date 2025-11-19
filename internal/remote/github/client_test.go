package github

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "https url with .git",
			url:       "https://github.com/lcgerke/gitDualRemote.git",
			wantOwner: "lcgerke",
			wantRepo:  "gitDualRemote",
			wantErr:   false,
		},
		{
			name:      "https url without .git",
			url:       "https://github.com/lcgerke/gitDualRemote",
			wantOwner: "lcgerke",
			wantRepo:  "gitDualRemote",
			wantErr:   false,
		},
		{
			name:      "ssh url with .git",
			url:       "git@github.com:lcgerke/gitDualRemote.git",
			wantOwner: "lcgerke",
			wantRepo:  "gitDualRemote",
			wantErr:   false,
		},
		{
			name:      "ssh url without .git",
			url:       "git@github.com:lcgerke/gitDualRemote",
			wantOwner: "lcgerke",
			wantRepo:  "gitDualRemote",
			wantErr:   false,
		},
		{
			name:      "gitlab url should error",
			url:       "https://gitlab.com/owner/repo.git",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "invalid url",
			url:       "not-a-url",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "invalid ssh format",
			url:       "git@github.com:owner",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
		{
			name:      "invalid https path",
			url:       "https://github.com/owner",
			wantOwner: "",
			wantRepo:  "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitHubURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("parseGitHubURL() owner = %v, want %v", owner, tt.wantOwner)
			}

			if repo != tt.wantRepo {
				t.Errorf("parseGitHubURL() repo = %v, want %v", repo, tt.wantRepo)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	// Test with a valid token set in environment
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	tests := []struct {
		name      string
		remoteURL string
		wantErr   bool
	}{
		{
			name:      "valid github https url",
			remoteURL: "https://github.com/lcgerke/gitDualRemote.git",
			wantErr:   false,
		},
		{
			name:      "valid github ssh url",
			remoteURL: "git@github.com:lcgerke/gitDualRemote.git",
			wantErr:   false,
		},
		{
			name:      "invalid url",
			remoteURL: "not-a-url",
			wantErr:   true,
		},
		{
			name:      "non-github url",
			remoteURL: "https://gitlab.com/owner/repo.git",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.remoteURL)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if client == nil {
					t.Error("NewClient() returned nil client")
				}
				if client.client == nil {
					t.Error("NewClient() returned client with nil github client")
				}
				if client.ctx == nil {
					t.Error("NewClient() returned client with nil context")
				}
			}
		})
	}
}

func TestNewClient_NoToken(t *testing.T) {
	// Save and clear all token sources
	originalGitHubToken := os.Getenv("GITHUB_TOKEN")
	originalGHToken := os.Getenv("GH_TOKEN")
	defer func() {
		if originalGitHubToken != "" {
			os.Setenv("GITHUB_TOKEN", originalGitHubToken)
		}
		if originalGHToken != "" {
			os.Setenv("GH_TOKEN", originalGHToken)
		}
	}()

	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")

	_, err := NewClient("https://github.com/lcgerke/gitDualRemote.git")

	if err == nil {
		t.Error("NewClient() should error when no token is available")
	}

	if err != nil && err.Error() == "" {
		t.Error("NewClient() error message should not be empty")
	}
}

func TestNewClientWithTimeout(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	client, cancel, err := NewClientWithTimeout(
		"https://github.com/lcgerke/gitDualRemote.git",
		30*time.Second,
	)

	if err != nil {
		t.Fatalf("NewClientWithTimeout() error = %v", err)
	}

	if client == nil {
		t.Fatal("NewClientWithTimeout() returned nil client")
	}

	if cancel == nil {
		t.Fatal("NewClientWithTimeout() returned nil cancel function")
	}

	// Test that cancel function works
	defer cancel()

	// Context should be valid initially
	select {
	case <-client.ctx.Done():
		t.Error("Context should not be done immediately")
	default:
		// Expected
	}
}

func TestClient_GetOwner(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	client, err := NewClient("https://github.com/testowner/testrepo.git")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if got := client.GetOwner(); got != "testowner" {
		t.Errorf("GetOwner() = %v, want %v", got, "testowner")
	}
}

func TestClient_GetRepo(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	client, err := NewClient("https://github.com/testowner/testrepo.git")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if got := client.GetRepo(); got != "testrepo" {
		t.Errorf("GetRepo() = %v, want %v", got, "testrepo")
	}
}

func TestClient_GetPlatform(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	client, err := NewClient("https://github.com/testowner/testrepo.git")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if got := client.GetPlatform(); got != "github" {
		t.Errorf("GetPlatform() = %v, want %v", got, "github")
	}
}

func TestClient_ImplementsPlatformInterface(t *testing.T) {
	// This test ensures Client implements the Platform interface at compile time
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	os.Setenv("GITHUB_TOKEN", "test_token")

	client, err := NewClient("https://github.com/testowner/testrepo.git")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// This line will cause a compile error if Client doesn't implement Platform
	var _ interface {
		SetDefaultBranch(branch string) error
		GetDefaultBranch() (string, error)
		IsBranchProtected(branch string) (bool, error)
		GetBranchProtection(branch string) (*ProtectionRules, error)
		CanPush() (bool, error)
		CanAdmin() (bool, error)
		GetOwner() string
		GetRepo() string
		GetPlatform() string
	} = client
}

// Helper function to set up test context
func setupTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}
