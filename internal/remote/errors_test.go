package remote

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewAPIError(t *testing.T) {
	tests := []struct {
		name       string
		errType    ErrorType
		statusCode int
		message    string
		err        error
		wantRetry  bool
	}{
		{
			name:       "auth error not retryable",
			errType:    ErrorTypeAuth,
			statusCode: http.StatusUnauthorized,
			message:    "invalid token",
			err:        errors.New("unauthorized"),
			wantRetry:  false,
		},
		{
			name:       "rate limit is retryable",
			errType:    ErrorTypeRateLimit,
			statusCode: http.StatusTooManyRequests,
			message:    "rate limit exceeded",
			err:        errors.New("too many requests"),
			wantRetry:  true,
		},
		{
			name:       "server error is retryable",
			errType:    ErrorTypeNetwork,
			statusCode: http.StatusInternalServerError,
			message:    "server error",
			err:        errors.New("internal error"),
			wantRetry:  true,
		},
		{
			name:       "permission error not retryable",
			errType:    ErrorTypePermission,
			statusCode: http.StatusForbidden,
			message:    "insufficient permissions",
			err:        errors.New("forbidden"),
			wantRetry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := NewAPIError(tt.errType, tt.statusCode, tt.message, tt.err)

			if apiErr.Type != tt.errType {
				t.Errorf("Type = %v, want %v", apiErr.Type, tt.errType)
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %v, want %v", apiErr.StatusCode, tt.statusCode)
			}
			if apiErr.Message != tt.message {
				t.Errorf("Message = %v, want %v", apiErr.Message, tt.message)
			}
			if apiErr.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", apiErr.Retryable, tt.wantRetry)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	err := NewAPIError(ErrorTypeAuth, http.StatusUnauthorized, "token invalid", errors.New("unauthorized"))
	want := "authentication error: token invalid"

	if got := err.Error(); got != want {
		t.Errorf("Error() = %v, want %v", got, want)
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	apiErr := NewAPIError(ErrorTypeAuth, http.StatusUnauthorized, "test", originalErr)

	if unwrapped := apiErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestClassifyGitHubError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
		wantType   ErrorType
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			err:        errors.New("unauthorized"),
			wantType:   ErrorTypeAuth,
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			err:        errors.New("forbidden"),
			wantType:   ErrorTypePermission,
		},
		{
			name:       "404 not found",
			statusCode: http.StatusNotFound,
			err:        errors.New("not found"),
			wantType:   ErrorTypeNotFound,
		},
		{
			name:       "429 rate limit",
			statusCode: http.StatusTooManyRequests,
			err:        errors.New("rate limit"),
			wantType:   ErrorTypeRateLimit,
		},
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			err:        errors.New("server error"),
			wantType:   ErrorTypeNetwork,
		},
		{
			name:       "503 service unavailable",
			statusCode: http.StatusServiceUnavailable,
			err:        errors.New("service unavailable"),
			wantType:   ErrorTypeNetwork,
		},
		{
			name:       "400 bad request",
			statusCode: http.StatusBadRequest,
			err:        errors.New("bad request"),
			wantType:   ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := ClassifyGitHubError(tt.statusCode, tt.err)

			if apiErr.Type != tt.wantType {
				t.Errorf("ClassifyGitHubError() Type = %v, want %v", apiErr.Type, tt.wantType)
			}
			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("ClassifyGitHubError() StatusCode = %v, want %v", apiErr.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "auth error returns true",
			err:  NewAPIError(ErrorTypeAuth, http.StatusUnauthorized, "test", nil),
			want: true,
		},
		{
			name: "permission error returns false",
			err:  NewAPIError(ErrorTypePermission, http.StatusForbidden, "test", nil),
			want: false,
		},
		{
			name: "regular error returns false",
			err:  errors.New("regular error"),
			want: false,
		},
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.want {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "permission error returns true",
			err:  NewAPIError(ErrorTypePermission, http.StatusForbidden, "test", nil),
			want: true,
		},
		{
			name: "auth error returns false",
			err:  NewAPIError(ErrorTypeAuth, http.StatusUnauthorized, "test", nil),
			want: false,
		},
		{
			name: "regular error returns false",
			err:  errors.New("regular error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermissionError(tt.err); got != tt.want {
				t.Errorf("IsPermissionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate limit error is retryable",
			err:  NewAPIError(ErrorTypeRateLimit, http.StatusTooManyRequests, "test", nil),
			want: true,
		},
		{
			name: "server error is retryable",
			err:  NewAPIError(ErrorTypeNetwork, http.StatusInternalServerError, "test", nil),
			want: true,
		},
		{
			name: "auth error is not retryable",
			err:  NewAPIError(ErrorTypeAuth, http.StatusUnauthorized, "test", nil),
			want: false,
		},
		{
			name: "permission error is not retryable",
			err:  NewAPIError(ErrorTypePermission, http.StatusForbidden, "test", nil),
			want: false,
		},
		{
			name: "regular error is not retryable",
			err:  errors.New("regular error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryable_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{
			name:       "429 is retryable",
			statusCode: http.StatusTooManyRequests,
			want:       true,
		},
		{
			name:       "500 is retryable",
			statusCode: http.StatusInternalServerError,
			want:       true,
		},
		{
			name:       "502 is retryable",
			statusCode: http.StatusBadGateway,
			want:       true,
		},
		{
			name:       "503 is retryable",
			statusCode: http.StatusServiceUnavailable,
			want:       true,
		},
		{
			name:       "401 is not retryable",
			statusCode: http.StatusUnauthorized,
			want:       false,
		},
		{
			name:       "403 is not retryable",
			statusCode: http.StatusForbidden,
			want:       false,
		},
		{
			name:       "404 is not retryable",
			statusCode: http.StatusNotFound,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := NewAPIError(ErrorTypeUnknown, tt.statusCode, "test", nil)
			if got := apiErr.Retryable; got != tt.want {
				t.Errorf("status %d: Retryable = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}
