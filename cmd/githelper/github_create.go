package main

import (
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/errors"
	ghclient "github.com/lcgerke/githelper/internal/github"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	createPrivate     bool
	createDescription string
)

var githubCreateCmd = &cobra.Command{
	Use:   "create <repo-name>",
	Short: "Create a GitHub repository using gh CLI",
	Long: `Creates a GitHub repository using the gh CLI tool.

This is a simple command that creates a repository on GitHub without
requiring Vault configuration or state management. It uses the gh CLI
directly, so you must have gh installed and authenticated.

Examples:
  githelper github create my-new-repo
  githelper github create my-private-repo --private
  githelper github create my-project --description "My awesome project"`,
	Args: cobra.ExactArgs(1),
	RunE: runGitHubCreate,
}

func init() {
	githubCreateCmd.Flags().BoolVar(&createPrivate, "private", true, "Create private repository")
	githubCreateCmd.Flags().StringVar(&createDescription, "description", "", "Repository description")
	githubCmd.AddCommand(githubCreateCmd)
}

func runGitHubCreate(cmd *cobra.Command, args []string) error {
	repoName := args[0]

	// Set up output
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	if !out.IsJSON() {
		out.Header(fmt.Sprintf("🚀 Creating GitHub Repository: %s", repoName))
		out.Separator()
	}

	// Check if gh CLI is available
	if !ghclient.CheckGHCLIAvailable() {
		return errors.WithHint(
			errors.New(errors.ErrorTypeGitHub, "gh CLI is not installed"),
			"Install gh CLI from https://cli.github.com/",
		)
	}

	if !ghclient.CheckGHAuthenticated() {
		return errors.WithHint(
			errors.New(errors.ErrorTypeGitHub, "gh CLI is not authenticated"),
			"Run 'gh auth login' to authenticate with GitHub",
		)
	}

	out.Success("gh CLI is available and authenticated ✓")

	// Create GitHub client (doesn't need token for gh CLI method)
	client := ghclient.NewClient(cmd.Context(), "")

	// Create the repository
	visibility := "private"
	if !createPrivate {
		visibility = "public"
	}

	out.Infof("Creating %s repository: %s", visibility, repoName)
	if createDescription != "" {
		out.Infof("Description: %s", createDescription)
	}

	err := client.CreateRepositoryViaGH(repoName, createDescription, createPrivate)
	if err != nil {
		return errors.Wrap(errors.ErrorTypeGitHub, "failed to create GitHub repository", err)
	}

	if !out.IsJSON() {
		fmt.Println()
		out.Separator()
		out.Success(fmt.Sprintf("✓ Repository created successfully!"))
		out.Info(fmt.Sprintf("View at: https://github.com/lcgerke/%s", repoName))
		out.Info(fmt.Sprintf("Clone with: gh repo clone %s", repoName))
	} else {
		out.JSON(map[string]interface{}{
			"status":      "success",
			"repository":  repoName,
			"visibility":  visibility,
			"description": createDescription,
		})
	}

	return nil
}
