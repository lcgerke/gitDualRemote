package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// DownloadSSHKey downloads SSH key from Vault and writes it to disk
// Returns the path to the private key file
func (c *Client) DownloadSSHKey(repoName, destDir string) (string, error) {
	// Get SSH key from Vault
	sshKey, err := c.GetSSHKey(repoName)
	if err != nil {
		return "", err
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create SSH directory: %w", err)
	}

	// Determine key filename
	var keyName string
	if repoName != "" {
		keyName = fmt.Sprintf("github_%s", repoName)
	} else {
		keyName = "github_default"
	}

	// Write private key
	privateKeyPath := filepath.Join(destDir, keyName)
	if err := os.WriteFile(privateKeyPath, []byte(sshKey.PrivateKey), 0600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key if available
	if sshKey.PublicKey != "" {
		publicKeyPath := filepath.Join(destDir, keyName+".pub")
		if err := os.WriteFile(publicKeyPath, []byte(sshKey.PublicKey), 0644); err != nil {
			// Not fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to write public key: %v\n", err)
		}
	}

	return privateKeyPath, nil
}

// GetSSHKeyPath returns the path where the SSH key should be stored
func GetSSHKeyPath(repoName, sshDir string) string {
	if sshDir == "" {
		home, _ := os.UserHomeDir()
		sshDir = filepath.Join(home, ".ssh")
	}

	var keyName string
	if repoName != "" {
		keyName = fmt.Sprintf("github_%s", repoName)
	} else {
		keyName = "github_default"
	}

	return filepath.Join(sshDir, keyName)
}
