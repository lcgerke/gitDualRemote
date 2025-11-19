package constants

import "time"

// Remote names
const (
	DefaultCoreRemote   = "origin"
	DefaultGitHubRemote = "github"
)

// Branch names
const (
	DefaultBranch = "main"
	MasterBranch  = "master"
)

// Timeouts
const (
	DefaultFetchTimeout     = 30 * time.Second
	DefaultOperationTimeout = 10 * time.Second
	QuickOperationTimeout   = 5 * time.Second
	BranchOperationTimeout  = 2 * time.Second
)
