package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lcgerke/githelper/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showCredentials bool
	repoFilter      string
	autoFix         bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive diagnostics",
	Long: `Performs a comprehensive health check of githelper configuration.

Checks:
- Vault connectivity and configuration
- Git installation and version
- Repository configurations
- GitHub integration status
- SSH keys and credentials
- Sync status
- Hook installations

Use --credentials to show detailed credential inventory.
Use --repo <name> to check a specific repository.
Use --auto-fix to automatically fix common issues.`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&showCredentials, "credentials", false, "Show detailed credential inventory")
	doctorCmd.Flags().StringVar(&repoFilter, "repo", "", "Check specific repository only")
	doctorCmd.Flags().BoolVar(&autoFix, "auto-fix", false, "Automatically fix common issues")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := ui.NewOutput(os.Stdout)
	if format != "" {
		out.SetFormat(ui.OutputFormat(format))
	}
	if noColor {
		out.SetColorEnabled(false)
	}

	if !out.IsJSON() {
		out.Header("🔍 GitHelper Diagnostic Report")
		out.Separator()
		fmt.Println()
	}

	results := &DiagnosticResults{
		Checks: make(map[string]*CheckResult),
	}

	// Check 1: Git installation
	if !out.IsJSON() {
		fmt.Println("Git Installation:")
	}
	checkGitInstallation(out, results)

	// Check 2: Vault connectivity
	if !out.IsJSON() {
		fmt.Println("\nVault Configuration:")
	}
	vaultClient := checkVault(out, results)

	// Check 3: State file
	if !out.IsJSON() {
		fmt.Println("\nState Management:")
	}
	stateMgr := checkStateFile(out, results)

	// Check 4: Repositories
	if !out.IsJSON() {
		fmt.Println("\nRepositories:")
	}
	checkRepositories(out, results, stateMgr, vaultClient)

	// Check 5: Credentials (if requested)
	if showCredentials {
		if !out.IsJSON() {
			fmt.Println("\n" + strings.Repeat("━", 60))
			fmt.Println("\n📋 Credential Inventory:")
		}
		checkCredentials(out, results, vaultClient, stateMgr)
	}

	// Check 6: Auto-fix (if requested)
	if autoFix && stateMgr != nil {
		if !out.IsJSON() {
			fmt.Println("\n" + strings.Repeat("━", 60))
			fmt.Println("\n🔧 Auto-Fix:")
		}
		runAutoFix(out, results, stateMgr)
	}

	// Summary
	if !out.IsJSON() {
		fmt.Println("\n" + strings.Repeat("━", 60))
		printSummary(out, results)
	} else {
		out.JSON(results)
	}

	// Return error if any critical checks failed
	if results.HasCriticalErrors() {
		return fmt.Errorf("diagnostic checks found critical errors")
	}

	return nil
}

type DiagnosticResults struct {
	Checks    map[string]*CheckResult `json:"checks"`
	Warnings  int                     `json:"warnings"`
	Errors    int                     `json:"errors"`
	StartTime time.Time               `json:"start_time"`
	EndTime   time.Time               `json:"end_time"`
}

type CheckResult struct {
	Name    string      `json:"name"`
	Status  string      `json:"status"` // "ok", "warning", "error"
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func (r *DiagnosticResults) AddCheck(name, status, message string, details interface{}) {
	r.Checks[name] = &CheckResult{
		Name:    name,
		Status:  status,
		Message: message,
		Details: details,
	}

	if status == "warning" {
		r.Warnings++
	} else if status == "error" {
		r.Errors++
	}
}

func (r *DiagnosticResults) HasCriticalErrors() bool {
	return r.Errors > 0
}
