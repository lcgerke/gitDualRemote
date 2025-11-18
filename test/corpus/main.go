package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// Command-line flags
	reposFile := flag.String("repos", "repos.yaml", "Path to repos.yaml manifest")
	outputJSON := flag.String("output", "", "Output JSON report file (e.g., spike1.json)")
	outputHTML := flag.String("html", "", "Output HTML report file (e.g., report.html)")
	compareFile := flag.String("compare", "", "Compare with baseline JSON report")
	clearCache := flag.Bool("clear-cache", false, "Clear the corpus cache before running")
	clearExpired := flag.Bool("clear-expired", false, "Clear expired cache entries before running")
	cacheStats := flag.Bool("cache-stats", false, "Show cache statistics and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GitHelper Corpus Validator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Run Spike 1 (before implementation)\n")
		fmt.Fprintf(os.Stderr, "  %s --repos repos.yaml --output spike1.json --html spike1.html\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Run Spike 2 (after implementation) and compare\n")
		fmt.Fprintf(os.Stderr, "  %s --repos repos.yaml --output spike2.json --compare spike1.json\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Clear cache\n")
		fmt.Fprintf(os.Stderr, "  %s --clear-cache\n\n", os.Args[0])
	}

	flag.Parse()

	// Find repos file (check multiple locations)
	reposPath, err := findReposFile(*reposFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nHint: Create repos.yaml from template:\n")
		fmt.Fprintf(os.Stderr, "  cp repos.yaml.template repos.yaml\n")
		fmt.Fprintf(os.Stderr, "  # Edit repos.yaml to add your managed repos\n")
		os.Exit(1)
	}

	// Create validator
	validator, err := NewValidator(reposPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating validator: %v\n", err)
		os.Exit(1)
	}

	cache := validator.GetCache()

	// Handle cache operations
	if *cacheStats {
		count, size, err := cache.Stats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting cache stats: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cache Statistics:\n")
		fmt.Printf("  Repos cached: %d\n", count)
		fmt.Printf("  Total size:   %.2f MB\n", float64(size)/(1024*1024))
		return
	}

	if *clearCache {
		fmt.Println("Clearing cache...")
		if err := cache.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Cache cleared")
		return
	}

	if *clearExpired {
		fmt.Println("Clearing expired cache entries...")
		if err := cache.ClearExpired(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing expired cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Expired cache entries cleared")
	}

	// Run validation
	ctx := context.Background()
	if err := validator.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	// Get reporter
	reporter := validator.GetReporter()

	// Print summary
	reporter.PrintSummary()

	// Write JSON report
	if *outputJSON != "" {
		if err := reporter.WriteJSON(*outputJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON report: %v\n", err)
			os.Exit(1)
		}
	}

	// Write HTML report
	if *outputHTML != "" {
		if err := reporter.WriteHTML(*outputHTML); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML report: %v\n", err)
			os.Exit(1)
		}
	}

	// Compare with baseline
	if *compareFile != "" {
		if err := reporter.Compare(*compareFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error comparing with baseline: %v\n", err)
			os.Exit(1)
		}
	}

	// Exit with appropriate code
	report := reporter.GenerateReport()
	if report.Summary.FailureCount > 0 {
		os.Exit(1)
	}
	if report.Summary.FalsePositiveRate >= 5.0 {
		fmt.Fprintf(os.Stderr, "\n⚠ WARNING: False positive rate (%.2f%%) exceeds 5%% threshold\n", report.Summary.FalsePositiveRate)
		os.Exit(1)
	}
}

// findReposFile searches for repos.yaml in common locations
func findReposFile(filename string) (string, error) {
	// Try exact path first
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}

	// Try in current directory
	cwd, _ := os.Getwd()
	paths := []string{
		filepath.Join(cwd, filename),
		filepath.Join(cwd, "test", "corpus", filename),
		filepath.Join(cwd, "..", "..", filename),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("repos file not found: %s (searched: %v)", filename, paths)
}
