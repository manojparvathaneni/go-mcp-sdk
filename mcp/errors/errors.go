package errors

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrNotInitialized is returned when operations are attempted before initialization
	ErrNotInitialized = errors.New("mcp: client not initialized")
	
	// ErrTransportNotSet is returned when no transport is configured
	ErrTransportNotSet = errors.New("mcp: transport not set")
	
	// ErrResourceNotFound is returned when a requested resource doesn't exist
	ErrResourceNotFound = errors.New("mcp: resource not found")
	
	// ErrToolNotFound is returned when a requested tool doesn't exist
	ErrToolNotFound = errors.New("mcp: tool not found")
	
	// ErrPromptNotFound is returned when a requested prompt doesn't exist
	ErrPromptNotFound = errors.New("mcp: prompt not found")
	
	// ErrInvalidArguments is returned when tool/prompt arguments are invalid
	ErrInvalidArguments = errors.New("mcp: invalid arguments")
	
	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("mcp: operation timed out")
	
	// ErrClosed is returned when operations are attempted on a closed connection
	ErrClosed = errors.New("mcp: connection closed")
)

// ErrorCode represents MCP-specific error codes
type ErrorCode string

const (
	// ErrorCodeInvalidRequest indicates the request was invalid
	ErrorCodeInvalidRequest ErrorCode = "invalid_request"
	
	// ErrorCodeResourceNotFound indicates the resource was not found
	ErrorCodeResourceNotFound ErrorCode = "resource_not_found"
	
	// ErrorCodeToolNotFound indicates the tool was not found
	ErrorCodeToolNotFound ErrorCode = "tool_not_found"
	
	// ErrorCodePromptNotFound indicates the prompt was not found
	ErrorCodePromptNotFound ErrorCode = "prompt_not_found"
	
	// ErrorCodeInvalidArguments indicates invalid arguments were provided
	ErrorCodeInvalidArguments ErrorCode = "invalid_arguments"
	
	// ErrorCodeInternalError indicates an internal server error
	ErrorCodeInternalError ErrorCode = "internal_error"
	
	// ErrorCodeUnauthorized indicates the request was unauthorized
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	
	// ErrorCodeRateLimited indicates the request was rate limited
	ErrorCodeRateLimited ErrorCode = "rate_limited"
)

// ProtocolError represents an MCP protocol error with additional context
type ProtocolError struct {
	Code    ErrorCode
	Message string
	Data    interface{}
	Cause   error
}

// Error implements the error interface
func (e *ProtocolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("mcp: %s (%s): %v", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("mcp: %s (%s)", e.Message, e.Code)
}

// Unwrap returns the underlying error
func (e *ProtocolError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target
func (e *ProtocolError) Is(target error) bool {
	t, ok := target.(*ProtocolError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// NewProtocolError creates a new protocol error
func NewProtocolError(code ErrorCode, message string, cause error) *ProtocolError {
	return &ProtocolError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewProtocolErrorWithData creates a new protocol error with additional data
func NewProtocolErrorWithData(code ErrorCode, message string, data interface{}, cause error) *ProtocolError {
	return &ProtocolError{
		Code:    code,
		Message: message,
		Data:    data,
		Cause:   cause,
	}
}

// IsProtocolError checks if an error is a ProtocolError
func IsProtocolError(err error) bool {
	var perr *ProtocolError
	return errors.As(err, &perr)
}

// GetProtocolError extracts the ProtocolError from an error chain
func GetProtocolError(err error) *ProtocolError {
	var perr *ProtocolError
	if errors.As(err, &perr) {
		return perr
	}
	return nil
}

// IsErrorCode checks if an error has a specific error code
func IsErrorCode(err error, code ErrorCode) bool {
	perr := GetProtocolError(err)
	return perr != nil && perr.Code == code
}