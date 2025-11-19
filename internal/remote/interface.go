// Package remote provides platform-agnostic interfaces for interacting with
// remote Git hosting services (GitHub, GitLab, Bitbucket, etc.).
package remote

// Platform defines the interface that all remote Git hosting platforms must implement.
// This abstraction allows GitHelper to work with multiple platforms (GitHub, GitLab, etc.)
// using a common interface.
type Platform interface {
	// Branch operations
	SetDefaultBranch(branch string) error
	GetDefaultBranch() (string, error)

	// Protection checks
	IsBranchProtected(branch string) (bool, error)
	GetBranchProtection(branch string) (*ProtectionRules, error)

	// Permission checks
	CanPush() (bool, error)
	CanAdmin() (bool, error)

	// Repository info
	GetOwner() string
	GetRepo() string
	GetPlatform() string // "github", "gitlab", "bitbucket"
}

// ProtectionRules represents branch protection settings across different platforms.
type ProtectionRules struct {
	Enabled             bool
	RequireReviews      bool
	RequireStatusChecks bool
	EnforceAdmins       bool
	AllowForcePush      bool
}
