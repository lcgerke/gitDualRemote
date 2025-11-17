package main

import (
	"fmt"
	"os"

	"github.com/lcgerke/githelper/internal/state"
	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed repositories",
	Long:  "Lists all repositories managed by githelper.",
	RunE:  runRepoList,
}

func runRepoList(cmd *cobra.Command, args []string) error {
	// Set up output
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	// Load state
	stateMgr, err := state.NewManager("")
	if err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	repos, err := stateMgr.ListRepositories()
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repos) == 0 {
		if out.IsJSON() {
			out.JSON(map[string]interface{}{
				"repositories": []interface{}{},
			})
		} else {
			out.Info("No repositories found.")
			out.Info("Create one with: githelper repo create <name>")
		}
		return nil
	}

	if out.IsJSON() {
		// JSON output
		repoList := make([]map[string]interface{}, 0, len(repos))
		for name, repo := range repos {
			r := map[string]interface{}{
				"name":    name,
				"path":    repo.Path,
				"remote":  repo.Remote,
				"created": repo.Created,
			}
			if repo.Type != "" {
				r["type"] = repo.Type
			}
			if repo.GitHub != nil {
				r["github"] = map[string]interface{}{
					"enabled":     repo.GitHub.Enabled,
					"user":        repo.GitHub.User,
					"repo":        repo.GitHub.Repo,
					"sync_status": repo.GitHub.SyncStatus,
				}
			}
			repoList = append(repoList, r)
		}
		out.JSON(map[string]interface{}{
			"repositories": repoList,
		})
	} else {
		// Human-readable output
		out.Header("Managed Repositories")
		fmt.Println()

		for name, repo := range repos {
			fmt.Printf("📁 %s\n", name)
			fmt.Printf("   Path:    %s\n", repo.Path)
			fmt.Printf("   Remote:  %s\n", repo.Remote)
			fmt.Printf("   Created: %s\n", repo.Created.Format("2006-01-02 15:04:05"))
			if repo.Type != "" {
				fmt.Printf("   Type:    %s\n", repo.Type)
			}
			if repo.GitHub != nil && repo.GitHub.Enabled {
				fmt.Printf("   GitHub:  %s/%s (%s)\n",
					repo.GitHub.User,
					repo.GitHub.Repo,
					repo.GitHub.SyncStatus)
			}
			fmt.Println()
		}

		out.Separator()
		out.Infof("Total: %d repositories", len(repos))
	}

	return nil
}
