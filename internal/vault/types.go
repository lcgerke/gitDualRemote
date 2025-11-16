package vault

// Config represents the githelper configuration from Vault
type Config struct {
	GitHubUsername         string `json:"github_username"`
	BareRepoPattern        string `json:"bare_repo_pattern"`
	DefaultVisibility      string `json:"default_visibility"`
	AutoCreateGitHub       bool   `json:"auto_create_github"`
	TestBeforePush         bool   `json:"test_before_push"`
	SyncOnSetup            bool   `json:"sync_on_setup"`
	RetryOnPartialFailure  bool   `json:"retry_on_partial_failure"`
}

// SSHKey represents an SSH key pair
type SSHKey struct {
	PrivateKey string
	PublicKey  string
}
