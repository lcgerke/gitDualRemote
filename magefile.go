//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Build variables
const (
	binaryName = "githelper"
	buildDir   = "."
	cmdDir     = "./cmd/githelper"
)

// Default target to run when none is specified
var Default = Build

// Build builds the githelper binary
func Build() error {
	mg.Deps(InstallDeps)
	fmt.Println("Building", binaryName, "...")

	output := filepath.Join(buildDir, binaryName)
	if err := sh.Run("go", "build", "-o", output, cmdDir); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("Build complete:", output)
	return nil
}

// Clean removes build artifacts and test directories
func Clean() error {
	fmt.Println("Cleaning...")

	// Remove binary
	if err := sh.Rm(filepath.Join(buildDir, binaryName)); err != nil {
		fmt.Printf("Warning: could not remove binary: %v\n", err)
	}

	// Remove test directories
	testDirs := []string{
		"/tmp/test-*",
		"/tmp/bare-repos",
	}

	for _, pattern := range testDirs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if err := os.RemoveAll(match); err != nil {
				fmt.Printf("Warning: could not remove %s: %v\n", match, err)
			}
		}
	}

	fmt.Println("Clean complete")
	return nil
}

// Test runs unit tests (short mode)
func Test() error {
	fmt.Println("Running unit tests...")
	return sh.RunV("go", "test", "-v", "-short", "./...")
}

// TestIntegration runs integration tests
func TestIntegration() error {
	fmt.Println("Running integration tests...")
	return sh.RunV("go", "test", "-v", "./test/integration/...")
}

// TestAll runs all tests
func TestAll() error {
	fmt.Println("Running all tests...")
	return sh.RunV("go", "test", "-v", "./...")
}

// Install installs githelper to GOPATH/bin using go install
func Install() error {
	mg.Deps(InstallDeps)
	fmt.Println("Installing", binaryName, "...")

	if err := sh.Run("go", "install", cmdDir); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	// Determine where it was installed
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}
		gobin = filepath.Join(gopath, "bin")
	}

	fmt.Printf("Install complete: %s/%s\n", gobin, binaryName)
	return nil
}

// InstallLocal installs to ~/.local/bin (alternative to go install)
func InstallLocal() error {
	mg.Deps(Build)
	fmt.Println("Installing to ~/.local/bin ...")

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	localBin := filepath.Join(home, ".local", "bin")
	target := filepath.Join(localBin, binaryName)
	source := filepath.Join(buildDir, binaryName)

	// Ensure ~/.local/bin exists
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", localBin, err)
	}

	// Copy binary
	if err := sh.Copy(target, source); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(target, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	fmt.Printf("Installed to: %s\n", target)
	return nil
}

// Fmt formats the code
func Fmt() error {
	fmt.Println("Formatting code...")
	return sh.RunV("go", "fmt", "./...")
}

// Lint runs the linter
func Lint() error {
	fmt.Println("Running linter...")

	// Check if golangci-lint is installed
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return fmt.Errorf("golangci-lint not found in PATH\nInstall: https://golangci-lint.run/usage/install/")
	}

	return sh.RunV("golangci-lint", "run", "./...")
}

// InstallDeps ensures go.mod dependencies are downloaded
func InstallDeps() error {
	fmt.Println("Downloading dependencies...")
	return sh.Run("go", "mod", "download")
}

// Tidy cleans up go.mod and go.sum
func Tidy() error {
	fmt.Println("Tidying go.mod...")
	return sh.Run("go", "mod", "tidy")
}

// Vet runs go vet
func Vet() error {
	fmt.Println("Running go vet...")
	return sh.RunV("go", "vet", "./...")
}

// Check runs all checks (fmt, vet, lint, test)
func Check() error {
	mg.Deps(Fmt, Vet, Test)

	fmt.Println("Running lint (optional)...")
	if err := Lint(); err != nil {
		fmt.Printf("Warning: Linting failed (non-fatal): %v\n", err)
	}

	fmt.Println("All checks passed!")
	return nil
}

// BuildAndInstall builds and installs in one step
func BuildAndInstall() error {
	mg.Deps(Build)
	return Install()
}

// CI runs all CI checks
func CI() error {
	mg.SerialDeps(InstallDeps, Fmt, Vet, TestAll)
	return nil
}
