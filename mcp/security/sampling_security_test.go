package security

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock implementations for testing

type mockAuditLogger struct {
	requests  []types.CreateMessageRequest
	approvals []ApprovalRequest
	responses []types.CreateMessageResult
}

func (m *mockAuditLogger) LogSamplingRequest(ctx context.Context, req types.CreateMessageRequest) error {
	m.requests = append(m.requests, req)
	return nil
}

func (m *mockAuditLogger) LogApprovalRequest(ctx context.Context, req ApprovalRequest, resp ApprovalResponse) error {
	m.approvals = append(m.approvals, req)
	return nil
}

func (m *mockAuditLogger) LogSamplingResponse(ctx context.Context, result types.CreateMessageResult) error {
	m.responses = append(m.responses, result)
	return nil
}

type mockRateLimiter struct {
	allowSampling bool
	quota         int
}

func (m *mockRateLimiter) AllowSampling(ctx context.Context, clientID string) (bool, error) {
	return m.allowSampling, nil
}

func (m *mockRateLimiter) GetRemainingQuota(ctx context.Context, clientID string) (int, error) {
	return m.quota, nil
}

func TestNewSamplingSecurity(t *testing.T) {
	config := SamplingSecurityConfig{
		RequireUserApproval: true,
		MaxTokensLimit:      1000,
	}
	
	security := NewSamplingSecurity(config)
	if security == nil {
		t.Fatal("Expected NewSamplingSecurity to return non-nil")
	}
	if security.config.MaxTokensLimit != 1000 {
		t.Error("Expected config to be preserved")
	}
}

func TestSamplingSecurity_SetApprovalHandler(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	called := false
	
	handler := func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
		called = true
		return ApprovalResponse{Result: ApprovalGranted}, nil
	}
	
	security.SetApprovalHandler(handler)
	
	// Verify handler was set by calling ValidateAndApprove
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	config := SamplingSecurityConfig{RequireUserApproval: true}
	security.config = config
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err != nil {
		t.Fatalf("ValidateAndApprove failed: %v", err)
	}
	
	if !called {
		t.Error("Expected approval handler to be called")
	}
}

func TestSamplingSecurity_SetContentValidator(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	
	validator := func(ctx context.Context, req types.CreateMessageRequest) error {
		return errors.New("validation failed")
	}
	
	security.SetContentValidator(validator)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected validation to fail")
	}
	if !strings.Contains(err.Error(), "content validation failed") {
		t.Error("Expected content validation error")
	}
}

func TestSamplingSecurity_SetAuditLogger(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{EnableAuditLogging: true})
	logger := &mockAuditLogger{}
	
	security.SetAuditLogger(logger)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err != nil {
		t.Fatalf("ValidateAndApprove failed: %v", err)
	}
	
	if len(logger.requests) != 1 {
		t.Errorf("Expected 1 logged request, got %d", len(logger.requests))
	}
}

func TestSamplingSecurity_SetRateLimiter(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{EnableRateLimit: true})
	limiter := &mockRateLimiter{allowSampling: false}
	
	security.SetRateLimiter(limiter)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected rate limit to be enforced")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Error("Expected rate limit error")
	}
}

func TestSamplingSecurity_ValidateAndApprove_Success(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "Hello"}},
		},
		MaxTokens: 100,
	}
	
	result, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err != nil {
		t.Fatalf("ValidateAndApprove failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.MaxTokens != 100 {
		t.Error("Expected request to be preserved")
	}
}

func TestSamplingSecurity_ValidateAndApprove_MaxTokensLimit(t *testing.T) {
	config := SamplingSecurityConfig{MaxTokensLimit: 50}
	security := NewSamplingSecurity(config)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected max tokens validation to fail")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Error("Expected max tokens error")
	}
}

func TestSamplingSecurity_ValidateAndApprove_BlockedPatterns(t *testing.T) {
	config := SamplingSecurityConfig{
		BlockedPatterns: []string{"password", "secret"},
	}
	security := NewSamplingSecurity(config)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "What is my password?"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected blocked pattern validation to fail")
	}
	if !strings.Contains(err.Error(), "blocked pattern") {
		t.Error("Expected blocked pattern error")
	}
}

func TestSamplingSecurity_ValidateAndApprove_MimeTypeValidation(t *testing.T) {
	config := SamplingSecurityConfig{
		AllowedMimeTypes: []string{"image/jpeg"},
	}
	security := NewSamplingSecurity(config)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{
				Role: types.RoleUser,
				Content: types.ImageContent{
					Type:     "image",
					Data:     "base64data",
					MimeType: "image/png", // Not allowed
				},
			},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected MIME type validation to fail")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Error("Expected MIME type error")
	}
}

func TestSamplingSecurity_ValidateAndApprove_UserApproval(t *testing.T) {
	config := SamplingSecurityConfig{RequireUserApproval: true}
	security := NewSamplingSecurity(config)
	
	approvalCalled := false
	handler := func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
		approvalCalled = true
		return ApprovalResponse{Result: ApprovalGranted}, nil
	}
	security.SetApprovalHandler(handler)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	result, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err != nil {
		t.Fatalf("ValidateAndApprove failed: %v", err)
	}
	
	if !approvalCalled {
		t.Error("Expected approval handler to be called")
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestSamplingSecurity_ValidateAndApprove_UserDenied(t *testing.T) {
	config := SamplingSecurityConfig{RequireUserApproval: true}
	security := NewSamplingSecurity(config)
	
	handler := func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
		return ApprovalResponse{
			Result: ApprovalDenied,
			Reason: "User rejected",
		}, nil
	}
	security.SetApprovalHandler(handler)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected user denial to fail")
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Error("Expected user denial error")
	}
}

func TestSamplingSecurity_ValidateAndApprove_UserModified(t *testing.T) {
	config := SamplingSecurityConfig{RequireUserApproval: true}
	security := NewSamplingSecurity(config)
	
	modifiedPrompt := "Modified prompt"
	handler := func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
		return ApprovalResponse{
			Result:         ApprovalModified,
			ModifiedPrompt: &modifiedPrompt,
		}, nil
	}
	security.SetApprovalHandler(handler)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	result, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err != nil {
		t.Fatalf("ValidateAndApprove failed: %v", err)
	}
	
	if result.SystemPrompt == nil || *result.SystemPrompt != modifiedPrompt {
		t.Error("Expected modified prompt to be applied")
	}
}

func TestSamplingSecurity_ValidateAndApprove_ApprovalTimeout(t *testing.T) {
	config := SamplingSecurityConfig{
		RequireUserApproval: true,
		ApprovalTimeout:     10 * time.Millisecond,
	}
	security := NewSamplingSecurity(config)
	
	handler := func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return ApprovalResponse{}, ctx.Err()
		default:
		}
		
		// Simulate work that takes longer than timeout
		select {
		case <-time.After(20 * time.Millisecond):
			return ApprovalResponse{Result: ApprovalGranted}, nil
		case <-ctx.Done():
			return ApprovalResponse{}, ctx.Err()
		}
	}
	security.SetApprovalHandler(handler)
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "test"}},
		},
		MaxTokens: 100,
	}
	
	_, err := security.ValidateAndApprove(context.Background(), req, "test-client")
	if err == nil {
		t.Error("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "approval request failed") {
		t.Errorf("Expected approval request failed error, got: %v", err)
	}
}

func TestSamplingSecurity_LogSamplingResponse(t *testing.T) {
	config := SamplingSecurityConfig{EnableAuditLogging: true}
	security := NewSamplingSecurity(config)
	logger := &mockAuditLogger{}
	security.SetAuditLogger(logger)
	
	result := types.CreateMessageResult{
		Content: types.TextContent{Type: "text", Text: "response"},
		Model:   "test-model",
		Role:    types.RoleAssistant,
	}
	
	security.LogSamplingResponse(context.Background(), result)
	
	if len(logger.responses) != 1 {
		t.Errorf("Expected 1 logged response, got %d", len(logger.responses))
	}
}

func TestDefaultSamplingSecurityConfig(t *testing.T) {
	config := DefaultSamplingSecurityConfig()
	
	if !config.RequireUserApproval {
		t.Error("Expected RequireUserApproval to be true")
	}
	if config.MaxTokensLimit != 4096 {
		t.Error("Expected MaxTokensLimit to be 4096")
	}
	if len(config.BlockedPatterns) == 0 {
		t.Error("Expected BlockedPatterns to be non-empty")
	}
	if len(config.AllowedMimeTypes) == 0 {
		t.Error("Expected AllowedMimeTypes to be non-empty")
	}
	if !config.EnableAuditLogging {
		t.Error("Expected EnableAuditLogging to be true")
	}
	if !config.EnableRateLimit {
		t.Error("Expected EnableRateLimit to be true")
	}
}

func TestApprovalResult_Constants(t *testing.T) {
	if ApprovalDenied != 0 {
		t.Error("Expected ApprovalDenied to be 0")
	}
	if ApprovalGranted != 1 {
		t.Error("Expected ApprovalGranted to be 1")
	}
	if ApprovalModified != 2 {
		t.Error("Expected ApprovalModified to be 2")
	}
	if ApprovalTimeout != 3 {
		t.Error("Expected ApprovalTimeout to be 3")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()
	
	if id1 == id2 {
		t.Error("Expected different request IDs")
	}
	if !strings.HasPrefix(id1, "req_") {
		t.Error("Expected request ID to have 'req_' prefix")
	}
}

func TestContainsBlockedPattern(t *testing.T) {
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	
	tests := []struct {
		message types.SamplingMessage
		pattern string
		expect  bool
	}{
		{
			types.SamplingMessage{Content: types.TextContent{Text: "What is my password?"}},
			"password",
			true,
		},
		{
			types.SamplingMessage{Content: types.TextContent{Text: "Hello world"}},
			"password",
			false,
		},
		{
			types.SamplingMessage{Content: "string content"},
			"content",
			true,
		},
		{
			types.SamplingMessage{Content: types.ImageContent{Type: "image"}},
			"password",
			false,
		},
	}
	
	for i, tt := range tests {
		result := security.containsBlockedPattern(tt.message, tt.pattern)
		if result != tt.expect {
			t.Errorf("Test %d: expected %v, got %v", i, tt.expect, result)
		}
	}
}

func TestValidateContentMimeType(t *testing.T) {
	config := SamplingSecurityConfig{
		AllowedMimeTypes: []string{"image/jpeg", "audio/mp3"},
	}
	security := NewSamplingSecurity(config)
	
	tests := []struct {
		content interface{}
		expect  bool
	}{
		{types.TextContent{Text: "hello"}, true}, // Non-media content always passes
		{types.ImageContent{MimeType: "image/jpeg"}, true},
		{types.ImageContent{MimeType: "image/png"}, false},
		{types.AudioContent{MimeType: "audio/mp3"}, true},
		{types.AudioContent{MimeType: "audio/wav"}, false},
	}
	
	for i, tt := range tests {
		err := security.validateContentMimeType(tt.content)
		hasError := err != nil
		if hasError == tt.expect {
			t.Errorf("Test %d: expected error=%v, got error=%v", i, !tt.expect, hasError)
		}
	}
}

func TestValidateContentMimeType_NoRestrictions(t *testing.T) {
	// Empty AllowedMimeTypes means no restrictions
	config := SamplingSecurityConfig{AllowedMimeTypes: []string{}}
	security := NewSamplingSecurity(config)
	
	err := security.validateContentMimeType(types.ImageContent{MimeType: "image/png"})
	if err != nil {
		t.Error("Expected no error when no MIME type restrictions")
	}
}