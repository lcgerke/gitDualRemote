package remote

import (
	"fmt"
	"strings"

	"github.com/lcgerke/githelper/internal/remote/github"
)

// githubClientWrapper wraps github.Client to adapt ProtectionRules types
type githubClientWrapper struct {
	*github.Client
}

// GetBranchProtection wraps the github client method to convert types
func (w *githubClientWrapper) GetBranchProtection(branch string) (*ProtectionRules, error) {
	ghRules, err := w.Client.GetBranchProtection(branch)
	if err != nil {
		return nil, err
	}

	// Convert github.ProtectionRules to remote.ProtectionRules
	return &ProtectionRules{
		Enabled:             ghRules.Enabled,
		RequireReviews:      ghRules.RequireReviews,
		RequireStatusChecks: ghRules.RequireStatusChecks,
		EnforceAdmins:       ghRules.EnforceAdmins,
		AllowForcePush:      ghRules.AllowForcePush,
	}, nil
}

// NewClient creates appropriate platform client based on remote URL
// Automatically detects the platform (GitHub, GitLab, Bitbucket) and returns
// the corresponding client implementation.
func NewClient(remoteURL string) (Platform, error) {
	platform := detectPlatform(remoteURL)

	switch platform {
	case "github":
		ghClient, err := github.NewClient(remoteURL)
		if err != nil {
			return nil, err
		}
		return &githubClientWrapper{Client: ghClient}, nil
	case "gitlab":
		return nil, fmt.Errorf("GitLab support not yet implemented")
	case "bitbucket":
		return nil, fmt.Errorf("Bitbucket support not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// detectPlatform identifies the platform from remote URL
// Supports GitHub, GitLab, and Bitbucket URLs
func detectPlatform(remoteURL string) string {
	switch {
	case strings.Contains(remoteURL, "github.com"):
		return "github"
	case strings.Contains(remoteURL, "gitlab.com"):
		return "gitlab"
	case strings.Contains(remoteURL, "bitbucket.org"):
		return "bitbucket"
	default:
		return "unknown"
	}
}

// IsPlatformSupported checks if a remote URL points to a supported platform
// Currently only GitHub is fully supported
func IsPlatformSupported(remoteURL string) bool {
	platform := detectPlatform(remoteURL)
	return platform == "github" // Expand as more platforms are added
}
