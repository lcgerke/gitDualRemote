package remote

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorType categorizes API errors for better handling
type ErrorType string

const (
	ErrorTypeAuth       ErrorType = "authentication"
	ErrorTypePermission ErrorType = "permission"
	ErrorTypeNotFound   ErrorType = "not_found"
	ErrorTypeRateLimit  ErrorType = "rate_limit"
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeUnknown    ErrorType = "unknown"
)

// APIError wraps errors with additional context for better error handling
// and user feedback.
type APIError struct {
	Type       ErrorType
	Message    string
	StatusCode int
	Err        error
	Retryable  bool
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error for errors.Is and errors.As support
func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a structured API error with automatic retry determination
func NewAPIError(errType ErrorType, statusCode int, message string, err error) *APIError {
	return &APIError{
		Type:       errType,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
		Retryable:  isRetryable(statusCode),
	}
}

// isRetryable determines if an error can be retried based on HTTP status code
func isRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode >= 500
}

// ClassifyGitHubError determines error type from HTTP status code
// and provides user-friendly error messages
func ClassifyGitHubError(statusCode int, err error) *APIError {
	switch statusCode {
	case http.StatusUnauthorized:
		return NewAPIError(ErrorTypeAuth, statusCode,
			"Invalid or expired token. Please re-authenticate.", err)
	case http.StatusForbidden:
		return NewAPIError(ErrorTypePermission, statusCode,
			"Insufficient permissions. Admin access required.", err)
	case http.StatusNotFound:
		return NewAPIError(ErrorTypeNotFound, statusCode,
			"Resource not found. Check repository name and access.", err)
	case http.StatusTooManyRequests:
		return NewAPIError(ErrorTypeRateLimit, statusCode,
			"GitHub API rate limit exceeded. Please wait.", err)
	default:
		if statusCode >= 500 {
			return NewAPIError(ErrorTypeNetwork, statusCode,
				"GitHub API temporary error. Retrying may help.", err)
		}
		return NewAPIError(ErrorTypeUnknown, statusCode,
			"Unexpected error occurred.", err)
	}
}

// IsAuthError checks if error is authentication-related
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Type == ErrorTypeAuth
	}
	return false
}

// IsPermissionError checks if error is permission-related
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Type == ErrorTypePermission
	}
	return false
}

// IsRetryable checks if operation can be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable
	}
	return false
}
