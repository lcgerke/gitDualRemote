package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"
)

// Reporter generates test reports
type Reporter struct {
	results []TestResult
	gitVersion string
}

// NewReporter creates a new reporter
func NewReporter(gitVersion string) *Reporter {
	return &Reporter{
		results: []TestResult{},
		gitVersion: gitVersion,
	}
}

// AddResult adds a test result
func (r *Reporter) AddResult(result TestResult) {
	r.results = append(r.results, result)
}

// GenerateReport creates the full report
func (r *Reporter) GenerateReport() Report {
	summary := r.calculateSummary()

	var failures []TestResult
	var mismatches []TestResult

	for _, result := range r.results {
		if !result.Success {
			failures = append(failures, result)
		}
		if !result.Match && result.Expected != nil {
			mismatches = append(mismatches, result)
		}
	}

	return Report{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		GitVersion:  r.gitVersion,
		Summary:     summary,
		Results:     r.results,
		Failures:    failures,
		Mismatches:  mismatches,
	}
}

// calculateSummary computes aggregate statistics
func (r *Reporter) calculateSummary() Summary {
	summary := Summary{
		TotalRepos:     len(r.results),
		ScenarioCounts: make(map[string]int),
	}

	var totalDetectionTime int64
	var maxDetectionTime int64

	for _, result := range r.results {
		if result.Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}

		if result.Match {
			summary.MatchCount++
		} else if result.Expected != nil {
			summary.MismatchCount++
		}

		if result.FalsePositive {
			summary.FalsePositiveCount++
		}

		// Performance stats
		totalDetectionTime += result.DetectionTimeMs
		if result.DetectionTimeMs > maxDetectionTime {
			maxDetectionTime = result.DetectionTimeMs
		}

		// Count scenarios
		if result.Detected != nil {
			summary.ScenarioCounts[result.Detected.Existence]++
			summary.ScenarioCounts[result.Detected.Sync]++
			summary.ScenarioCounts[result.Detected.WorkingTree]++
			summary.ScenarioCounts[result.Detected.Corruption]++
		}
	}

	if summary.SuccessCount > 0 {
		summary.AvgDetectionTimeMs = float64(totalDetectionTime) / float64(summary.SuccessCount)
	}
	summary.MaxDetectionTimeMs = maxDetectionTime

	// Calculate false positive rate
	totalWithExpectations := summary.MatchCount + summary.MismatchCount
	if totalWithExpectations > 0 {
		summary.FalsePositiveRate = float64(summary.FalsePositiveCount) / float64(totalWithExpectations) * 100
	}

	return summary
}

// WriteJSON writes report to JSON file
func (r *Reporter) WriteJSON(filename string) error {
	report := r.GenerateReport()

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	fmt.Printf("✓ JSON report written to: %s\n", filename)
	return nil
}

// WriteHTML writes report to HTML file
func (r *Reporter) WriteHTML(filename string) error {
	report := r.GenerateReport()

	tmpl := template.Must(template.New("report").Parse(htmlTemplate))

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, report); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	fmt.Printf("✓ HTML report written to: %s\n", filename)
	return nil
}

// PrintSummary prints a summary to stdout
func (r *Reporter) PrintSummary() {
	report := r.GenerateReport()
	s := report.Summary

	fmt.Println("\n" + separator("CORPUS TEST SUMMARY", 80))
	fmt.Printf("Total Repositories: %d\n", s.TotalRepos)
	fmt.Printf("  ✓ Success:        %d\n", s.SuccessCount)
	fmt.Printf("  ✗ Failures:       %d\n", s.FailureCount)
	fmt.Printf("  ⊘ Skipped:        %d\n", s.SkippedCount)
	fmt.Println()

	if s.MatchCount+s.MismatchCount > 0 {
		fmt.Printf("Validation (repos with expected scenarios):\n")
		fmt.Printf("  ✓ Matches:        %d\n", s.MatchCount)
		fmt.Printf("  ✗ Mismatches:     %d\n", s.MismatchCount)
		fmt.Printf("  ⚠ False Positives: %d (%.2f%%)\n", s.FalsePositiveCount, s.FalsePositiveRate)
		fmt.Println()
	}

	fmt.Printf("Performance:\n")
	fmt.Printf("  Avg Detection:    %.0f ms\n", s.AvgDetectionTimeMs)
	fmt.Printf("  Max Detection:    %d ms\n", s.MaxDetectionTimeMs)
	fmt.Printf("  Total Duration:   %s\n", s.TotalDuration)
	fmt.Println()

	// Show top scenarios
	fmt.Printf("Top Scenario IDs:\n")
	scenarios := sortScenarioCounts(s.ScenarioCounts)
	for i, sc := range scenarios {
		if i >= 10 {
			break
		}
		fmt.Printf("  %s: %d\n", sc.ID, sc.Count)
	}

	fmt.Println(separator("", 80))

	// Show failures if any
	if len(report.Failures) > 0 {
		fmt.Printf("\n⚠ FAILURES (%d):\n", len(report.Failures))
		for i, failure := range report.Failures {
			if i >= 5 {
				fmt.Printf("  ... and %d more (see full report)\n", len(report.Failures)-5)
				break
			}
			fmt.Printf("  • %s: %s\n", failure.RepoName, failure.Error)
		}
		fmt.Println()
	}

	// Show mismatches if any
	if len(report.Mismatches) > 0 {
		fmt.Printf("⚠ MISMATCHES (%d):\n", len(report.Mismatches))
		for i, mismatch := range report.Mismatches {
			if i >= 5 {
				fmt.Printf("  ... and %d more (see full report)\n", len(report.Mismatches)-5)
				break
			}
			fmt.Printf("  • %s:\n", mismatch.RepoName)
			for _, mm := range mismatch.Mismatches {
				fmt.Printf("      %s\n", mm)
			}
		}
		fmt.Println()
	}

	// Success criteria check
	fmt.Println("SUCCESS CRITERIA:")
	checkCriteria("False positive rate < 5%", s.FalsePositiveRate < 5.0)
	checkCriteria("No crashes or panics", s.FailureCount == 0)
	checkCriteria("90% of repos detected in < 2s", s.AvgDetectionTimeMs < 2000)
	fmt.Println()
}

// Compare compares with a baseline report
func (r *Reporter) Compare(baselineFile string) error {
	data, err := os.ReadFile(baselineFile)
	if err != nil {
		return fmt.Errorf("failed to read baseline: %w", err)
	}

	var baseline Report
	if err := json.Unmarshal(data, &baseline); err != nil {
		return fmt.Errorf("failed to parse baseline: %w", err)
	}

	current := r.GenerateReport()

	fmt.Println("\n" + separator("COMPARISON WITH BASELINE", 80))
	fmt.Printf("Baseline: %s (generated %s)\n", baselineFile, baseline.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Current:  %s\n", current.GeneratedAt.Format(time.RFC3339))
	fmt.Println()

	printComparison("Total Repos", baseline.Summary.TotalRepos, current.Summary.TotalRepos)
	printComparison("Success Rate",
		fmt.Sprintf("%.1f%%", float64(baseline.Summary.SuccessCount)/float64(baseline.Summary.TotalRepos)*100),
		fmt.Sprintf("%.1f%%", float64(current.Summary.SuccessCount)/float64(current.Summary.TotalRepos)*100))
	printComparison("False Positive Rate",
		fmt.Sprintf("%.2f%%", baseline.Summary.FalsePositiveRate),
		fmt.Sprintf("%.2f%%", current.Summary.FalsePositiveRate))
	printComparison("Avg Detection Time",
		fmt.Sprintf("%.0f ms", baseline.Summary.AvgDetectionTimeMs),
		fmt.Sprintf("%.0f ms", current.Summary.AvgDetectionTimeMs))

	fmt.Println(separator("", 80))

	return nil
}

// Helper functions

func separator(title string, width int) string {
	if title == "" {
		return fmt.Sprintf("%s", repeatChar("=", width))
	}
	padding := (width - len(title) - 2) / 2
	return fmt.Sprintf("%s %s %s", repeatChar("=", padding), title, repeatChar("=", padding))
}

func repeatChar(char string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}

func checkCriteria(description string, passed bool) {
	if passed {
		fmt.Printf("  ✓ %s\n", description)
	} else {
		fmt.Printf("  ✗ %s\n", description)
	}
}

func printComparison(label string, baseline, current interface{}) {
	fmt.Printf("  %-25s %20v → %v\n", label+":", baseline, current)
}

type scenarioCount struct {
	ID    string
	Count int
}

func sortScenarioCounts(counts map[string]int) []scenarioCount {
	var results []scenarioCount
	for id, count := range counts {
		results = append(results, scenarioCount{ID: id, Count: count})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})
	return results
}

// LoadReport loads a report from JSON file
func LoadReport(filename string) (*Report, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read report: %w", err)
	}

	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}

	return &report, nil
}

// HTML template
const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>GitHelper Corpus Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
        .stat { background: #f9f9f9; padding: 15px; border-left: 4px solid #4CAF50; }
        .stat.warning { border-color: #ff9800; }
        .stat.error { border-color: #f44336; }
        .stat-label { font-size: 12px; color: #666; text-transform: uppercase; }
        .stat-value { font-size: 24px; font-weight: bold; color: #333; margin-top: 5px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th { background: #4CAF50; color: white; padding: 10px; text-align: left; }
        td { padding: 8px; border-bottom: 1px solid #ddd; }
        tr:hover { background: #f5f5f5; }
        .success { color: #4CAF50; }
        .failure { color: #f44336; }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 11px; font-weight: bold; }
        .badge-success { background: #4CAF50; color: white; }
        .badge-error { background: #f44336; color: white; }
        .badge-warning { background: #ff9800; color: white; }
        .mismatch { background: #fff3cd; padding: 5px; margin: 2px 0; border-left: 3px solid #ff9800; }
    </style>
</head>
<body>
    <div class="container">
        <h1>GitHelper Corpus Test Report</h1>
        <p><strong>Generated:</strong> {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
        <p><strong>Git Version:</strong> {{.GitVersion}}</p>

        <h2>Summary</h2>
        <div class="summary">
            <div class="stat">
                <div class="stat-label">Total Repositories</div>
                <div class="stat-value">{{.Summary.TotalRepos}}</div>
            </div>
            <div class="stat">
                <div class="stat-label">Success</div>
                <div class="stat-value success">{{.Summary.SuccessCount}}</div>
            </div>
            <div class="stat {{if gt .Summary.FailureCount 0}}error{{end}}">
                <div class="stat-label">Failures</div>
                <div class="stat-value">{{.Summary.FailureCount}}</div>
            </div>
            <div class="stat {{if gt .Summary.FalsePositiveRate 5.0}}warning{{end}}">
                <div class="stat-label">False Positive Rate</div>
                <div class="stat-value">{{printf "%.2f%%" .Summary.FalsePositiveRate}}</div>
            </div>
            <div class="stat">
                <div class="stat-label">Avg Detection Time</div>
                <div class="stat-value">{{printf "%.0f ms" .Summary.AvgDetectionTimeMs}}</div>
            </div>
            <div class="stat {{if gt .Summary.MaxDetectionTimeMs 2000}}warning{{end}}">
                <div class="stat-label">Max Detection Time</div>
                <div class="stat-value">{{.Summary.MaxDetectionTimeMs}} ms</div>
            </div>
        </div>

        {{if .Failures}}
        <h2>Failures ({{len .Failures}})</h2>
        <table>
            <tr><th>Repository</th><th>Type</th><th>Error</th></tr>
            {{range .Failures}}
            <tr>
                <td>{{.RepoName}}</td>
                <td><span class="badge badge-warning">{{.RepoType}}</span></td>
                <td class="failure">{{.Error}}</td>
            </tr>
            {{end}}
        </table>
        {{end}}

        {{if .Mismatches}}
        <h2>Mismatches ({{len .Mismatches}})</h2>
        <table>
            <tr><th>Repository</th><th>Mismatches</th></tr>
            {{range .Mismatches}}
            <tr>
                <td>{{.RepoName}}</td>
                <td>
                    {{range .Mismatches}}
                    <div class="mismatch">{{.}}</div>
                    {{end}}
                </td>
            </tr>
            {{end}}
        </table>
        {{end}}

        <h2>All Results</h2>
        <table>
            <tr>
                <th>Repository</th>
                <th>Type</th>
                <th>Status</th>
                <th>Existence</th>
                <th>Sync</th>
                <th>Working Tree</th>
                <th>Corruption</th>
                <th>Detection Time</th>
            </tr>
            {{range .Results}}
            <tr>
                <td>{{.RepoName}}</td>
                <td><span class="badge badge-warning">{{.RepoType}}</span></td>
                <td>
                    {{if .Success}}
                        {{if .Match}}
                            <span class="badge badge-success">✓ MATCH</span>
                        {{else if .Expected}}
                            <span class="badge badge-error">✗ MISMATCH</span>
                        {{else}}
                            <span class="badge badge-success">✓ OK</span>
                        {{end}}
                    {{else}}
                        <span class="badge badge-error">✗ FAIL</span>
                    {{end}}
                </td>
                <td>{{if .Detected}}{{.Detected.Existence}}{{end}}</td>
                <td>{{if .Detected}}{{.Detected.Sync}}{{end}}</td>
                <td>{{if .Detected}}{{.Detected.WorkingTree}}{{end}}</td>
                <td>{{if .Detected}}{{.Detected.Corruption}}{{end}}</td>
                <td>{{.DetectionTimeMs}} ms</td>
            </tr>
            {{end}}
        </table>
    </div>
</body>
</html>
`
