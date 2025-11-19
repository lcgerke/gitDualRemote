package main

import (
	"fmt"

	"github.com/lcgerke/githelper/internal/git"
	"github.com/lcgerke/githelper/internal/remote"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Check GitHub authentication status",
	Long: `Diagnose GitHub API authentication and permissions.

This command checks:
  - GitHub token availability from multiple sources
  - Token validation
  - Repository permissions (push/admin)
  - Default branch and protection status`,
	RunE: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
}

func runAuth(cmd *cobra.Command, args []string) error {
	gitClient := git.NewClient(".")

	if !gitClient.IsRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Get github remote
	remoteURL, err := gitClient.GetRemoteURL("github")
	if err != nil {
		fmt.Println("❌ No 'github' remote found")
		fmt.Println()
		fmt.Println("This repository does not have a GitHub remote.")
		fmt.Println("Add one with: git remote add github <url>")
		return nil
	}

	fmt.Printf("GitHub Remote: %s\n", remoteURL)
	fmt.Println()

	// Check if platform is supported
	if !remote.IsPlatformSupported(remoteURL) {
		fmt.Printf("❌ Unsupported platform\n")
		fmt.Printf("URL: %s\n", remoteURL)
		fmt.Println()
		fmt.Println("Currently supported platforms:")
		fmt.Println("  - GitHub (github.com)")
		return nil
	}

	// Create client (tests token resolution)
	fmt.Println("🔍 Checking authentication...")
	client, err := remote.NewClient(remoteURL)
	if err != nil {
		fmt.Printf("❌ Authentication failed\n")
		fmt.Println()
		fmt.Printf("Error: %v\n", err)
		return nil
	}

	fmt.Printf("✓ Token found and client created\n")
	fmt.Printf("  Platform: %s\n", client.GetPlatform())
	fmt.Printf("  Owner: %s\n", client.GetOwner())
	fmt.Printf("  Repo: %s\n", client.GetRepo())

	// Check permissions
	fmt.Println()
	fmt.Println("🔍 Checking repository permissions...")

	canPush, err := client.CanPush()
	if err != nil {
		fmt.Printf("❌ Failed to check push permission: %v\n", err)
	} else {
		if canPush {
			fmt.Println("  ✓ Push: Yes")
		} else {
			fmt.Println("  ✗ Push: No")
		}
	}

	canAdmin, err := client.CanAdmin()
	if err != nil {
		fmt.Printf("❌ Failed to check admin permission: %v\n", err)
	} else {
		if canAdmin {
			fmt.Println("  ✓ Admin: Yes")
		} else {
			fmt.Println("  ✗ Admin: No")
		}
	}

	// Check default branch
	fmt.Println()
	fmt.Println("🔍 Repository information...")
	defaultBranch, err := client.GetDefaultBranch()
	if err != nil {
		fmt.Printf("❌ Failed to get default branch: %v\n", err)
	} else {
		fmt.Printf("  Default branch: %s\n", defaultBranch)

		// Check if protected
		protected, err := client.IsBranchProtected(defaultBranch)
		if err != nil {
			fmt.Printf("  Failed to check protection: %v\n", err)
		} else if protected {
			fmt.Printf("  Protection: ✓ Enabled\n")

			rules, err := client.GetBranchProtection(defaultBranch)
			if err == nil && rules != nil {
				fmt.Printf("    - Require reviews: %v\n", rules.RequireReviews)
				fmt.Printf("    - Require status checks: %v\n", rules.RequireStatusChecks)
				fmt.Printf("    - Enforce admins: %v\n", rules.EnforceAdmins)
				fmt.Printf("    - Allow force push: %v\n", rules.AllowForcePush)
			}
		} else {
			fmt.Printf("  Protection: ✗ Disabled\n")
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ GitHub authentication is working correctly")

	return nil
}
