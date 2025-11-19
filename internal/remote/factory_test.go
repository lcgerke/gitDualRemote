package remote

import (
	"os"
	"strings"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "github https url",
			url:  "https://github.com/owner/repo.git",
			want: "github",
		},
		{
			name: "github ssh url",
			url:  "git@github.com:owner/repo.git",
			want: "github",
		},
		{
			name: "gitlab https url",
			url:  "https://gitlab.com/owner/repo.git",
			want: "gitlab",
		},
		{
			name: "gitlab ssh url",
			url:  "git@gitlab.com:owner/repo.git",
			want: "gitlab",
		},
		{
			name: "bitbucket https url",
			url:  "https://bitbucket.org/owner/repo.git",
			want: "bitbucket",
		},
		{
			name: "bitbucket ssh url",
			url:  "git@bitbucket.org:owner/repo.git",
			want: "bitbucket",
		},
		{
			name: "unknown url",
			url:  "https://example.com/owner/repo.git",
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectPlatform(tt.url)
			if got != tt.want {
				t.Errorf("detectPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPlatformSupported(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "github is supported",
			url:  "https://github.com/owner/repo.git",
			want: true,
		},
		{
			name: "gitlab not yet supported",
			url:  "https://gitlab.com/owner/repo.git",
			want: false,
		},
		{
			name: "bitbucket not yet supported",
			url:  "https://bitbucket.org/owner/repo.git",
			want: false,
		},
		{
			name: "unknown not supported",
			url:  "https://example.com/owner/repo.git",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPlatformSupported(tt.url)
			if got != tt.want {
				t.Errorf("IsPlatformSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	// Set up test token
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
		name       string
		remoteURL  string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:      "github url creates client",
			remoteURL: "https://github.com/owner/repo.git",
			wantErr:   false,
		},
		{
			name:       "gitlab url returns error",
			remoteURL:  "https://gitlab.com/owner/repo.git",
			wantErr:    true,
			wantErrMsg: "GitLab support not yet implemented",
		},
		{
			name:       "bitbucket url returns error",
			remoteURL:  "https://bitbucket.org/owner/repo.git",
			wantErr:    true,
			wantErrMsg: "Bitbucket support not yet implemented",
		},
		{
			name:       "unknown platform returns error",
			remoteURL:  "https://example.com/owner/repo.git",
			wantErr:    true,
			wantErrMsg: "unsupported platform",
		},
		{
			name:      "invalid url returns error",
			remoteURL: "not-a-url",
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

			if tt.wantErr && tt.wantErrMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("NewClient() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
			}

			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client when error was not expected")
			}

			// Verify the platform interface is implemented
			if !tt.wantErr && client != nil {
				if client.GetPlatform() != "github" {
					t.Errorf("NewClient() platform = %v, want github", client.GetPlatform())
				}
			}
		})
	}
}

func TestNewClient_NoToken(t *testing.T) {
	// Clear all token sources
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

	_, err := NewClient("https://github.com/owner/repo.git")

	if err == nil {
		t.Error("NewClient() should error when no token is available")
	}
}
