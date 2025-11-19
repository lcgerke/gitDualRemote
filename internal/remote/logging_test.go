package remote

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	// Test without env var
	os.Unsetenv("GITHELPER_LOG")
	logger := NewLogger()
	if logger.enabled {
		t.Error("Logger should not be enabled without GITHELPER_LOG")
	}

	// Test with env var
	os.Setenv("GITHELPER_LOG", "1")
	defer os.Unsetenv("GITHELPER_LOG")
	logger = NewLogger()
	if !logger.enabled {
		t.Error("Logger should be enabled with GITHELPER_LOG")
	}
}

func TestLogOperation(t *testing.T) {
	os.Setenv("GITHELPER_LOG", "1")
	defer os.Unsetenv("GITHELPER_LOG")

	logger := NewLogger()

	// Test successful operation
	err := logger.LogOperation("test_operation", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("LogOperation should return nil for successful operation, got %v", err)
	}

	// Test failed operation
	expectedErr := errors.New("test error")
	err = logger.LogOperation("test_operation", func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("LogOperation should return error, got %v, want %v", err, expectedErr)
	}
}

func TestMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()

	// Record successful calls
	collector.RecordCall(200, 100*time.Millisecond)
	collector.RecordCall(201, 150*time.Millisecond)

	// Record failed call
	collector.RecordCall(500, 50*time.Millisecond)

	// Record rate limit
	collector.RecordCall(429, 10*time.Millisecond)

	if collector.TotalCalls != 4 {
		t.Errorf("TotalCalls = %d, want 4", collector.TotalCalls)
	}

	if collector.SuccessfulCalls != 2 {
		t.Errorf("SuccessfulCalls = %d, want 2", collector.SuccessfulCalls)
	}

	if collector.FailedCalls != 2 {
		t.Errorf("FailedCalls = %d, want 2", collector.FailedCalls)
	}

	if collector.RateLimitHits != 1 {
		t.Errorf("RateLimitHits = %d, want 1", collector.RateLimitHits)
	}

	report := collector.Report()
	if !strings.Contains(report, "Total calls: 4") {
		t.Errorf("Report should contain total calls, got: %s", report)
	}
}

func TestMetricsCollector_EmptyReport(t *testing.T) {
	collector := NewMetricsCollector()
	report := collector.Report()

	if report != "No API calls made" {
		t.Errorf("Empty collector should report no calls, got: %s", report)
	}
}

func TestLogTokenResolution(t *testing.T) {
	os.Setenv("GITHELPER_LOG", "1")
	defer os.Unsetenv("GITHELPER_LOG")

	// Just verify it doesn't panic
	LogTokenResolution("GITHUB_TOKEN")
	LogTokenResolution("gh config")
}

func TestLogAPICall(t *testing.T) {
	os.Setenv("GITHELPER_LOG", "1")
	defer os.Unsetenv("GITHELPER_LOG")

	// Just verify it doesn't panic
	LogAPICall("GET", "/repos/owner/repo", 200, 100*time.Millisecond)
	LogAPICall("POST", "/repos", 201, 200*time.Millisecond)
	LogAPICall("GET", "/repos/owner/repo", 404, 50*time.Millisecond)
	LogAPICall("GET", "/repos/owner/repo", 500, 1000*time.Millisecond)
}

func TestLogRetry(t *testing.T) {
	os.Setenv("GITHELPER_LOG", "1")
	defer os.Unsetenv("GITHELPER_LOG")

	// Just verify it doesn't panic
	LogRetry("fetch_repo", 1, 3, errors.New("timeout"))
	LogRetry("fetch_repo", 2, 3, errors.New("timeout"))
}
