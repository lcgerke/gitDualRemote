package remote

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Logger provides structured logging for remote operations
type Logger struct {
	enabled bool
	verbose bool
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{
		enabled: os.Getenv("GITHELPER_LOG") != "",
		verbose: os.Getenv("GITHELPER_VERBOSE") != "",
	}
}

// LogOperation logs a remote operation with timing
func (l *Logger) LogOperation(operation string, fn func() error) error {
	if !l.enabled {
		return fn()
	}

	start := time.Now()
	l.Infof("Starting: %s", operation)

	err := fn()
	duration := time.Since(start)

	if err != nil {
		l.Errorf("Failed: %s (took %v) - %v", operation, duration, err)
	} else {
		l.Infof("Completed: %s (took %v)", operation, duration)
	}

	return err
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	if l.enabled {
		log.Printf("[INFO] %s", msg)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.enabled {
		log.Printf("[INFO] "+format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	if l.enabled {
		log.Printf("[ERROR] %s", msg)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.enabled {
		log.Printf("[ERROR] "+format, args...)
	}
}

// Debug logs a debug message (only if verbose mode is enabled)
func (l *Logger) Debug(msg string) {
	if l.enabled && l.verbose {
		log.Printf("[DEBUG] %s", msg)
	}
}

// Debugf logs a formatted debug message (only if verbose mode is enabled)
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.enabled && l.verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// defaultLogger is the package-level logger
var defaultLogger = NewLogger()

// LogAPICall logs GitHub API calls for observability
func LogAPICall(method, endpoint string, statusCode int, duration time.Duration) {
	if !defaultLogger.enabled {
		return
	}

	if statusCode >= 200 && statusCode < 300 {
		defaultLogger.Infof("API %s %s -> %d (%v)", method, endpoint, statusCode, duration)
	} else if statusCode >= 400 {
		defaultLogger.Errorf("API %s %s -> %d (%v)", method, endpoint, statusCode, duration)
	}
}

// LogTokenResolution logs where the GitHub token was found
func LogTokenResolution(source string) {
	if !defaultLogger.enabled {
		return
	}
	defaultLogger.Infof("GitHub token resolved from: %s", source)
}

// LogRetry logs retry attempts for failed operations
func LogRetry(operation string, attempt int, maxAttempts int, err error) {
	if !defaultLogger.enabled {
		return
	}
	defaultLogger.Infof("Retry %d/%d for %s: %v", attempt, maxAttempts, operation, err)
}

// MetricsCollector collects metrics about API usage
type MetricsCollector struct {
	TotalCalls      int
	SuccessfulCalls int
	FailedCalls     int
	RateLimitHits   int
	TotalDuration   time.Duration
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// RecordCall records an API call
func (m *MetricsCollector) RecordCall(statusCode int, duration time.Duration) {
	m.TotalCalls++
	m.TotalDuration += duration

	if statusCode >= 200 && statusCode < 300 {
		m.SuccessfulCalls++
	} else {
		m.FailedCalls++
	}

	if statusCode == 429 {
		m.RateLimitHits++
	}
}

// Report returns a metrics report
func (m *MetricsCollector) Report() string {
	if m.TotalCalls == 0 {
		return "No API calls made"
	}

	avgDuration := m.TotalDuration / time.Duration(m.TotalCalls)
	successRate := float64(m.SuccessfulCalls) / float64(m.TotalCalls) * 100

	return fmt.Sprintf(
		"API Metrics:\n"+
			"  Total calls: %d\n"+
			"  Successful: %d (%.1f%%)\n"+
			"  Failed: %d\n"+
			"  Rate limit hits: %d\n"+
			"  Avg duration: %v",
		m.TotalCalls,
		m.SuccessfulCalls,
		successRate,
		m.FailedCalls,
		m.RateLimitHits,
		avgDuration,
	)
}
