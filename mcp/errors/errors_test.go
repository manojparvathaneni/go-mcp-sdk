package errors

import (
	"errors"
	"testing"
)

func TestProtocolError_Error(t *testing.T) {
	err := NewProtocolError(ErrorCodeInvalidRequest, "Test error", nil)
	
	expected := "mcp: Test error (invalid_request)"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestProtocolError_ErrorWithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewProtocolError(ErrorCodeInternalError, "Test error", cause)
	
	expected := "mcp: Test error (internal_error): underlying error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestProtocolError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewProtocolError(ErrorCodeInternalError, "Test error", cause)
	
	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Expected unwrapped error to be the cause")
	}
}

func TestProtocolError_Is(t *testing.T) {
	err1 := NewProtocolError(ErrorCodeInvalidRequest, "Test error 1", nil)
	err2 := NewProtocolError(ErrorCodeInvalidRequest, "Test error 2", nil)
	err3 := NewProtocolError(ErrorCodeInternalError, "Test error 3", nil)
	
	if !err1.Is(err2) {
		t.Error("Expected errors with same code to be equal")
	}
	
	if err1.Is(err3) {
		t.Error("Expected errors with different codes to not be equal")
	}
}

func TestIsProtocolError(t *testing.T) {
	protocolErr := NewProtocolError(ErrorCodeInvalidRequest, "Test error", nil)
	regularErr := errors.New("regular error")
	
	if !IsProtocolError(protocolErr) {
		t.Error("Expected IsProtocolError to return true for ProtocolError")
	}
	
	if IsProtocolError(regularErr) {
		t.Error("Expected IsProtocolError to return false for regular error")
	}
}

func TestGetProtocolError(t *testing.T) {
	protocolErr := NewProtocolError(ErrorCodeInvalidRequest, "Test error", nil)
	regularErr := errors.New("regular error")
	
	result := GetProtocolError(protocolErr)
	if result != protocolErr {
		t.Error("Expected GetProtocolError to return the ProtocolError")
	}
	
	result = GetProtocolError(regularErr)
	if result != nil {
		t.Error("Expected GetProtocolError to return nil for regular error")
	}
}

func TestIsErrorCode(t *testing.T) {
	err := NewProtocolError(ErrorCodeInvalidRequest, "Test error", nil)
	
	if !IsErrorCode(err, ErrorCodeInvalidRequest) {
		t.Error("Expected IsErrorCode to return true for matching code")
	}
	
	if IsErrorCode(err, ErrorCodeInternalError) {
		t.Error("Expected IsErrorCode to return false for non-matching code")
	}
	
	regularErr := errors.New("regular error")
	if IsErrorCode(regularErr, ErrorCodeInvalidRequest) {
		t.Error("Expected IsErrorCode to return false for regular error")
	}
}

func TestNewProtocolErrorWithData(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	err := NewProtocolErrorWithData(ErrorCodeInvalidRequest, "Test error", data, nil)
	
	if err.Data == nil {
		t.Error("Expected Data field to be set")
	}
	
	if dataMap, ok := err.Data.(map[string]interface{}); !ok {
		t.Error("Expected Data to be a map")
	} else if dataMap["key"] != "value" {
		t.Error("Expected Data to contain correct values")
	}
	
	if err.Code != ErrorCodeInvalidRequest {
		t.Errorf("Expected code %s, got %s", ErrorCodeInvalidRequest, err.Code)
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants have expected values
	if ErrorCodeInvalidRequest != "invalid_request" {
		t.Errorf("Expected ErrorCodeInvalidRequest to be 'invalid_request', got %s", ErrorCodeInvalidRequest)
	}
	
	if ErrorCodeResourceNotFound != "resource_not_found" {
		t.Errorf("Expected ErrorCodeResourceNotFound to be 'resource_not_found', got %s", ErrorCodeResourceNotFound)
	}
	
	if ErrorCodeToolNotFound != "tool_not_found" {
		t.Errorf("Expected ErrorCodeToolNotFound to be 'tool_not_found', got %s", ErrorCodeToolNotFound)
	}
}