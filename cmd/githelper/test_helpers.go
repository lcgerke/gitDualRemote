package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/lcgerke/githelper/internal/ui"
)

// Helper function to run git commands
func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %v, output: %s", err, output)
	}
	return nil
}

// Helper to create test output
func newTestOutput(buf *bytes.Buffer) *ui.Output {
	return ui.NewOutput(buf)
}
