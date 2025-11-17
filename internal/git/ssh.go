package git

import (
	"fmt"
)

// ConfigureSSH sets up SSH for git operations
// This configures repository-local SSH, not global ~/.ssh/config
func (c *Client) ConfigureSSH(privateKeyPath string) error {
	// Set core.sshCommand to use specific key
	sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no", privateKeyPath)
	return c.ConfigSet("core.sshCommand", sshCmd)
}

// GetSSHCommand returns the current SSH command configuration
func (c *Client) GetSSHCommand() (string, error) {
	return c.ConfigGet("core.sshCommand")
}
