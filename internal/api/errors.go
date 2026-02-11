package api

import "fmt"

// Gemini error codes from the web API.
const (
	ErrTransient      = 1013
	ErrRateLimited    = 1037
	ErrModelMismatch  = 1050
	ErrInvalidHeader  = 1052
	ErrIPBanned       = 1060
)

// GeminiError represents a structured error from the Gemini API.
type GeminiError struct {
	Code    int
	Message string
}

func (e *GeminiError) Error() string {
	return fmt.Sprintf("gemini error %d: %s", e.Code, e.Message)
}

// IsRetryable returns true if the error is transient and can be retried.
func (e *GeminiError) IsRetryable() bool {
	return e.Code == ErrTransient || e.Code == ErrRateLimited
}

var errorMessages = map[int]string{
	ErrTransient:     "transient error, please retry",
	ErrRateLimited:   "rate limited, please wait before retrying",
	ErrModelMismatch: "model mismatch - the requested model may not be available",
	ErrInvalidHeader: "invalid model header value",
	ErrIPBanned:      "IP address has been temporarily banned",
}

// NewGeminiError creates a GeminiError with a default message for known codes.
func NewGeminiError(code int) *GeminiError {
	msg, ok := errorMessages[code]
	if !ok {
		msg = "unknown error"
	}
	return &GeminiError{Code: code, Message: msg}
}
