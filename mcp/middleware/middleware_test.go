package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock handler for testing
type mockHandler struct {
	requestResponse *types.JSONRPCMessage
	requestError    error
	notificationError error
	requestCalled   bool
	notificationCalled bool
	shouldPanic     bool
}

func (h *mockHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	h.requestCalled = true
	if h.shouldPanic {
		panic("test panic")
	}
	return h.requestResponse, h.requestError
}

func (h *mockHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	h.notificationCalled = true
	if h.shouldPanic {
		panic("test panic")
	}
	return h.notificationError
}

// Mock rate limiter for testing
type mockRateLimiter struct {
	allowRequests bool
}

func (m *mockRateLimiter) Allow() bool {
	return m.allowRequests
}

func TestMiddlewareHandlerFunc_HandleRequest(t *testing.T) {
	expectedResponse := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{"success": true}`),
	}

	handler := MiddlewareHandlerFunc(func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
		return expectedResponse, nil
	})

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp != expectedResponse {
		t.Error("Expected response to be returned")
	}
}

func TestMiddlewareHandlerFunc_HandleNotification(t *testing.T) {
	handler := MiddlewareHandlerFunc(func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
		return nil, nil
	})

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := handler.HandleNotification(context.Background(), notification)
	if err != nil {
		t.Errorf("HandleNotification should always return nil, got: %v", err)
	}
}

func TestChain(t *testing.T) {
	// Create middleware that modifies the request method
	middleware1 := func(next MiddlewareHandler) MiddlewareHandler {
		return MiddlewareHandlerFunc(func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
			req.Method = "middleware1/" + req.Method
			return next.HandleRequest(ctx, req)
		})
	}

	middleware2 := func(next MiddlewareHandler) MiddlewareHandler {
		return MiddlewareHandlerFunc(func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
			req.Method = "middleware2/" + req.Method
			return next.HandleRequest(ctx, req)
		})
	}

	finalHandler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{JSONRPC: "2.0"},
	}

	// Chain middlewares
	chained := Chain(middleware1, middleware2)(finalHandler)

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "original",
	}

	_, err := chained.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Chained request failed: %v", err)
	}

	// Method should be modified by both middlewares in order
	expectedMethod := "middleware2/middleware1/original"
	if req.Method != expectedMethod {
		t.Errorf("Expected method %s, got %s", expectedMethod, req.Method)
	}

	if !finalHandler.requestCalled {
		t.Error("Expected final handler to be called")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"success": true}`),
		},
	}

	middleware := LoggingMiddleware(logger)(handler)

	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}

	_, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("LoggingMiddleware request failed: %v", err)
	}

	logStr := logOutput.String()
	if !strings.Contains(logStr, "[REQUEST]") {
		t.Error("Expected request log entry")
	}
	if !strings.Contains(logStr, "[RESPONSE]") {
		t.Error("Expected response log entry")
	}
	if !strings.Contains(logStr, "test/method") {
		t.Error("Expected method name in logs")
	}
}

func TestLoggingMiddleware_Error(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	handler := &mockHandler{
		requestError: errors.New("test error"),
	}

	middleware := LoggingMiddleware(logger)(handler)

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	_, err := middleware.HandleRequest(context.Background(), req)
	if err == nil {
		t.Error("Expected error to be propagated")
	}

	logStr := logOutput.String()
	if !strings.Contains(logStr, "[ERROR]") {
		t.Error("Expected error log entry")
	}
}

func TestLoggingMiddleware_Notification(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	handler := &mockHandler{}
	middleware := LoggingMiddleware(logger)(handler)

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := middleware.HandleNotification(context.Background(), notification)
	if err != nil {
		t.Fatalf("LoggingMiddleware notification failed: %v", err)
	}

	logStr := logOutput.String()
	if !strings.Contains(logStr, "[NOTIFICATION]") {
		t.Error("Expected notification log entry")
	}
	if !strings.Contains(logStr, "test/notification") {
		t.Error("Expected method name in logs")
	}
}

func TestRecoveryMiddleware_Request(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	handler := &mockHandler{
		shouldPanic: true,
	}

	middleware := RecoveryMiddleware(logger)(handler)

	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("RecoveryMiddleware should handle panic, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Expected response after panic recovery")
	}
	if resp.Error == nil {
		t.Error("Expected error response after panic")
	}
	if resp.Error.Code != types.InternalError {
		t.Errorf("Expected InternalError code, got %d", resp.Error.Code)
	}

	logStr := logOutput.String()
	if !strings.Contains(logStr, "[PANIC]") {
		t.Error("Expected panic log entry")
	}
}

func TestRecoveryMiddleware_Notification(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	handler := &mockHandler{
		shouldPanic: true,
	}

	middleware := RecoveryMiddleware(logger)(handler)

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := middleware.HandleNotification(context.Background(), notification)
	if err != nil {
		t.Fatalf("RecoveryMiddleware should handle panic: %v", err)
	}

	logStr := logOutput.String()
	if !strings.Contains(logStr, "[PANIC]") {
		t.Error("Expected panic log entry")
	}
}

func TestTimeoutMiddleware_Success(t *testing.T) {
	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"success": true}`),
		},
	}

	middleware := TimeoutMiddleware(100 * time.Millisecond)(handler)

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("TimeoutMiddleware request failed: %v", err)
	}

	if resp.Error != nil {
		t.Error("Expected successful response")
	}
}

func TestTimeoutMiddleware_Timeout(t *testing.T) {
	handler := MiddlewareHandlerFunc(func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
		// Simulate slow operation
		time.Sleep(50 * time.Millisecond)
		return &types.JSONRPCMessage{JSONRPC: "2.0"}, nil
	})

	middleware := TimeoutMiddleware(10 * time.Millisecond)(handler)

	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("TimeoutMiddleware should return response, not error: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected timeout error response")
	}
	if !strings.Contains(resp.Error.Message, "timeout") {
		t.Errorf("Expected timeout message, got: %s", resp.Error.Message)
	}
}

func TestTimeoutMiddleware_NotificationTimeout(t *testing.T) {
	// Create a handler that simulates slow notification processing
	slowNotificationHandler := &slowNotificationMockHandler{}

	middleware := TimeoutMiddleware(10 * time.Millisecond)(slowNotificationHandler)

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := middleware.HandleNotification(context.Background(), notification)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// Helper type for testing notification timeouts
type slowNotificationMockHandler struct{}

func (h *slowNotificationMockHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	return &types.JSONRPCMessage{JSONRPC: "2.0"}, nil
}

func (h *slowNotificationMockHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	time.Sleep(50 * time.Millisecond)
	return nil
}

func TestRateLimitMiddleware_Allowed(t *testing.T) {
	limiter := &mockRateLimiter{allowRequests: true}
	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{JSONRPC: "2.0"},
	}

	middleware := RateLimitMiddleware(limiter)(handler)

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("RateLimitMiddleware request failed: %v", err)
	}

	if resp.Error != nil {
		t.Error("Expected successful response when rate limit allows")
	}
	if !handler.requestCalled {
		t.Error("Expected handler to be called when rate limit allows")
	}
}

func TestRateLimitMiddleware_Blocked(t *testing.T) {
	limiter := &mockRateLimiter{allowRequests: false}
	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{JSONRPC: "2.0"},
	}

	middleware := RateLimitMiddleware(limiter)(handler)

	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("RateLimitMiddleware should return response, not error: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected rate limit error response")
	}
	if !strings.Contains(resp.Error.Message, "Rate limit exceeded") {
		t.Errorf("Expected rate limit message, got: %s", resp.Error.Message)
	}
	if handler.requestCalled {
		t.Error("Expected handler to not be called when rate limited")
	}
}

func TestRateLimitMiddleware_NotificationBlocked(t *testing.T) {
	limiter := &mockRateLimiter{allowRequests: false}
	handler := &mockHandler{}

	middleware := RateLimitMiddleware(limiter)(handler)

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := middleware.HandleNotification(context.Background(), notification)
	if err == nil {
		t.Error("Expected rate limit error")
	}

	if !strings.Contains(err.Error(), "Rate limit exceeded") {
		t.Errorf("Expected rate limit error, got: %v", err)
	}
}

func TestValidationMiddleware_Valid(t *testing.T) {
	validator := func(ctx context.Context, req *types.JSONRPCMessage) error {
		return nil // Always valid
	}

	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{JSONRPC: "2.0"},
	}

	middleware := ValidationMiddleware(validator)(handler)

	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ValidationMiddleware request failed: %v", err)
	}

	if resp.Error != nil {
		t.Error("Expected successful response when validation passes")
	}
	if !handler.requestCalled {
		t.Error("Expected handler to be called when validation passes")
	}
}

func TestValidationMiddleware_Invalid(t *testing.T) {
	validator := func(ctx context.Context, req *types.JSONRPCMessage) error {
		return errors.New("validation failed")
	}

	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{JSONRPC: "2.0"},
	}

	middleware := ValidationMiddleware(validator)(handler)

	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ValidationMiddleware should return response, not error: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected validation error response")
	}
	if !strings.Contains(resp.Error.Message, "validation failed") {
		t.Errorf("Expected validation error message, got: %s", resp.Error.Message)
	}
	if handler.requestCalled {
		t.Error("Expected handler to not be called when validation fails")
	}
}

func TestValidationMiddleware_NotificationInvalid(t *testing.T) {
	validator := func(ctx context.Context, req *types.JSONRPCMessage) error {
		return errors.New("validation failed")
	}

	handler := &mockHandler{}
	middleware := ValidationMiddleware(validator)(handler)

	notification := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test/notification",
	}

	err := middleware.HandleNotification(context.Background(), notification)
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestMiddlewareChain_Multiple(t *testing.T) {
	var logOutput bytes.Buffer
	logger := log.New(&logOutput, "", 0)

	validator := func(ctx context.Context, req *types.JSONRPCMessage) error {
		if req.Method == "invalid" {
			return errors.New("invalid method")
		}
		return nil
	}

	handler := &mockHandler{
		requestResponse: &types.JSONRPCMessage{
			JSONRPC: "2.0",
			Result:  json.RawMessage(`{"success": true}`),
		},
	}

	// Chain multiple middlewares
	middleware := Chain(
		LoggingMiddleware(logger),
		ValidationMiddleware(validator),
		RecoveryMiddleware(logger),
	)(handler)

	// Test valid request
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "valid/method",
	}

	resp, err := middleware.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Chained middleware request failed: %v", err)
	}

	if resp.Error != nil {
		t.Error("Expected successful response")
	}

	// Check that logging occurred
	logStr := logOutput.String()
	if !strings.Contains(logStr, "[REQUEST]") {
		t.Error("Expected request log entry")
	}
}