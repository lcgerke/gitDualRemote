package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-github/v58/github"
)

// TestOperations_Integration contains integration tests that use the real GitHub API
// These tests are skipped in short mode
func TestGetDefaultBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set")
	}

	client, err := NewClient("https://github.com/lcgerke/gitDualRemote.git")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	branch, err := client.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}

	if branch == "" {
		t.Error("GetDefaultBranch() returned empty branch")
	}

	t.Logf("Default branch: %s", branch)
}

// Mock server tests for operations
func TestGetDefaultBranch_Mock(t *testing.T) {
	// Create mock server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup mock response
	mux.HandleFunc("/repos/testowner/testrepo", func(w http.ResponseWriter, r *http.Request) {
		repo := &github.Repository{
			DefaultBranch: github.String("main"),
		}
		json.NewEncoder(w).Encode(repo)
	})

	// Create client pointing to mock server
	token := "test_token"
	os.Setenv("GITHUB_TOKEN", token)
	defer os.Unsetenv("GITHUB_TOKEN")

	client := &Client{
		client: github.NewClient(nil),
		owner:  "testowner",
		repo:   "testrepo",
		ctx:    context.Background(),
	}
	baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

	branch, err := client.GetDefaultBranch()
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}

	if branch != "main" {
		t.Errorf("GetDefaultBranch() = %v, want %v", branch, "main")
	}
}

func TestSetDefaultBranch_Mock(t *testing.T) {
	// Create mock server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup mock response
	mux.HandleFunc("/repos/testowner/testrepo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Expected PATCH request, got %s", r.Method)
		}

		var repo github.Repository
		json.NewDecoder(r.Body).Decode(&repo)

		if *repo.DefaultBranch != "main" {
			t.Errorf("Expected default_branch to be 'main', got %s", *repo.DefaultBranch)
		}

		json.NewEncoder(w).Encode(&repo)
	})

	// Create client pointing to mock server
	token := "test_token"
	os.Setenv("GITHUB_TOKEN", token)
	defer os.Unsetenv("GITHUB_TOKEN")

	client := &Client{
		client: github.NewClient(nil),
		owner:  "testowner",
		repo:   "testrepo",
		ctx:    context.Background(),
	}
	baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

	err := client.SetDefaultBranch("main")
	if err != nil {
		t.Fatalf("SetDefaultBranch() error = %v", err)
	}
}

func TestIsBranchProtected_Mock(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		statusCode int
		want       bool
		wantErr    bool
	}{
		{
			name:       "protected branch returns true",
			branch:     "main",
			statusCode: http.StatusOK,
			want:       true,
			wantErr:    false,
		},
		{
			name:       "unprotected branch returns false",
			branch:     "develop",
			statusCode: http.StatusNotFound,
			want:       false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			defer server.Close()

			// Setup mock response
			mux.HandleFunc("/repos/testowner/testrepo/branches/"+tt.branch+"/protection", func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == http.StatusNotFound {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"message": "Branch not protected"})
					return
				}

				protection := &github.Protection{
					RequiredStatusChecks: &github.RequiredStatusChecks{
						Strict: false,
					},
				}
				json.NewEncoder(w).Encode(protection)
			})

			// Create client pointing to mock server
			token := "test_token"
			os.Setenv("GITHUB_TOKEN", token)
			defer os.Unsetenv("GITHUB_TOKEN")

			client := &Client{
				client: github.NewClient(nil),
				owner:  "testowner",
				repo:   "testrepo",
				ctx:    context.Background(),
			}
			baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

			got, err := client.IsBranchProtected(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsBranchProtected() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("IsBranchProtected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBranchProtection_Mock(t *testing.T) {
	// Create mock server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup mock response for protected branch
	mux.HandleFunc("/repos/testowner/testrepo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		protection := &github.Protection{
			RequiredStatusChecks: &github.RequiredStatusChecks{
				Strict: true,
			},
			RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
				RequiredApprovingReviewCount: 1,
			},
			EnforceAdmins: &github.AdminEnforcement{
				Enabled: true,
			},
			AllowForcePushes: &github.AllowForcePushes{
				Enabled: false,
			},
		}
		json.NewEncoder(w).Encode(protection)
	})

	// Setup mock response for unprotected branch
	mux.HandleFunc("/repos/testowner/testrepo/branches/develop/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Branch not protected"})
	})

	// Create client pointing to mock server
	token := "test_token"
	os.Setenv("GITHUB_TOKEN", token)
	defer os.Unsetenv("GITHUB_TOKEN")

	client := &Client{
		client: github.NewClient(nil),
		owner:  "testowner",
		repo:   "testrepo",
		ctx:    context.Background(),
	}
	baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

	// Test protected branch
	rules, err := client.GetBranchProtection("main")
	if err != nil {
		t.Fatalf("GetBranchProtection() error = %v", err)
	}

	if !rules.Enabled {
		t.Error("GetBranchProtection() Enabled should be true for protected branch")
	}

	if !rules.RequireStatusChecks {
		t.Error("GetBranchProtection() RequireStatusChecks should be true")
	}

	if !rules.RequireReviews {
		t.Error("GetBranchProtection() RequireReviews should be true")
	}

	if !rules.EnforceAdmins {
		t.Error("GetBranchProtection() EnforceAdmins should be true")
	}

	if rules.AllowForcePush {
		t.Error("GetBranchProtection() AllowForcePush should be false")
	}

	// Test unprotected branch
	rules, err = client.GetBranchProtection("develop")
	if err != nil {
		t.Fatalf("GetBranchProtection() error = %v", err)
	}

	if rules.Enabled {
		t.Error("GetBranchProtection() Enabled should be false for unprotected branch")
	}
}

func TestCanPush_Mock(t *testing.T) {
	// Create mock server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup mock response
	mux.HandleFunc("/repos/testowner/testrepo", func(w http.ResponseWriter, r *http.Request) {
		repo := &github.Repository{
			Permissions: map[string]bool{
				"pull":  true,
				"push":  true,
				"admin": false,
			},
		}
		json.NewEncoder(w).Encode(repo)
	})

	// Create client pointing to mock server
	token := "test_token"
	os.Setenv("GITHUB_TOKEN", token)
	defer os.Unsetenv("GITHUB_TOKEN")

	client := &Client{
		client: github.NewClient(nil),
		owner:  "testowner",
		repo:   "testrepo",
		ctx:    context.Background(),
	}
	baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

	canPush, err := client.CanPush()
	if err != nil {
		t.Fatalf("CanPush() error = %v", err)
	}

	if !canPush {
		t.Error("CanPush() should return true when user has push permission")
	}
}

func TestCanAdmin_Mock(t *testing.T) {
	tests := []struct {
		name    string
		perms   map[string]bool
		want    bool
		wantErr bool
	}{
		{
			name: "admin permission returns true",
			perms: map[string]bool{
				"pull":  true,
				"push":  true,
				"admin": true,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "no admin permission returns false",
			perms: map[string]bool{
				"pull":  true,
				"push":  true,
				"admin": false,
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			defer server.Close()

			// Setup mock response
			mux.HandleFunc("/repos/testowner/testrepo", func(w http.ResponseWriter, r *http.Request) {
				repo := &github.Repository{
					Permissions: tt.perms,
				}
				json.NewEncoder(w).Encode(repo)
			})

			// Create client pointing to mock server
			token := "test_token"
			os.Setenv("GITHUB_TOKEN", token)
			defer os.Unsetenv("GITHUB_TOKEN")

			client := &Client{
				client: github.NewClient(nil),
				owner:  "testowner",
				repo:   "testrepo",
				ctx:    context.Background(),
			}
			baseURL, _ := url.Parse(server.URL + "/")
	client.client.BaseURL = baseURL

			got, err := client.CanAdmin()
			if (err != nil) != tt.wantErr {
				t.Errorf("CanAdmin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("CanAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}
