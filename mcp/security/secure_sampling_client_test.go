package security

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock client for testing SecureSamplingClient
type mockMCPClient struct {
	createMessageResult *types.CreateMessageResult
	createMessageError  error
	initializeResult    *types.InitializeResult
	serverInfo          *types.Implementation
	serverCapabilities  *types.ServerCapabilities
}

func (m *mockMCPClient) CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error) {
	if m.createMessageError != nil {
		return nil, m.createMessageError
	}
	return m.createMessageResult, nil
}

func (m *mockMCPClient) Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error) {
	return m.initializeResult, nil
}

func (m *mockMCPClient) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	return &types.ListResourcesResult{}, nil
}

func (m *mockMCPClient) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	return &types.ReadResourceResult{}, nil
}

func (m *mockMCPClient) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{}, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	return &types.CallToolResult{}, nil
}

func (m *mockMCPClient) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	return &types.ListPromptsResult{}, nil
}

func (m *mockMCPClient) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	return &types.GetPromptResult{}, nil
}

func (m *mockMCPClient) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (m *mockMCPClient) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

func (m *mockMCPClient) ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error) {
	return &types.ListRootsResult{}, nil
}

func (m *mockMCPClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockMCPClient) Close() error {
	return nil
}

func (m *mockMCPClient) ServerInfo() *types.Implementation {
	return m.serverInfo
}

func (m *mockMCPClient) ServerCapabilities() *types.ServerCapabilities {
	return m.serverCapabilities
}


func TestNewSecureSamplingClient(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	clientID := "test-client"
	
	secureClient := NewSecureSamplingClient(mockClient, security, clientID)
	
	if secureClient == nil {
		t.Fatal("Expected NewSecureSamplingClient to return non-nil")
	}
	if secureClient.clientID != clientID {
		t.Error("Expected client ID to be preserved")
	}
	if secureClient.client == nil {
		t.Error("Expected client to be set")
	}
	if secureClient.security != security {
		t.Error("Expected security to be preserved")
	}
}

func TestSecureSamplingClient_CreateMessage_Success(t *testing.T) {
	expectedResult := &types.CreateMessageResult{
		Content: types.TextContent{Type: "text", Text: "Hello"},
		Model:   "test-model",
		Role:    types.RoleAssistant,
	}
	
	mockClient := &mockMCPClient{
		createMessageResult: expectedResult,
	}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{EnableAuditLogging: true})
	logger := &mockAuditLogger{}
	security.SetAuditLogger(logger)
	
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "Hello"}},
		},
		MaxTokens: 100,
	}
	
	result, err := secureClient.CreateMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}
	
	if result != expectedResult {
		t.Error("Expected result to be returned")
	}
	
	// Check that response was logged
	if len(logger.responses) != 1 {
		t.Errorf("Expected 1 logged response, got %d", len(logger.responses))
	}
}

func TestSecureSamplingClient_CreateMessage_SecurityValidationFailed(t *testing.T) {
	mockClient := &mockMCPClient{}
	
	// Configure security to block requests with too many tokens
	config := SamplingSecurityConfig{MaxTokensLimit: 50}
	security := NewSamplingSecurity(config)
	
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "Hello"}},
		},
		MaxTokens: 100, // Exceeds limit
	}
	
	_, err := secureClient.CreateMessage(context.Background(), req)
	if err == nil {
		t.Error("Expected security validation to fail")
	}
	if !strings.Contains(err.Error(), "security validation failed") {
		t.Error("Expected security validation error")
	}
}

func TestSecureSamplingClient_CreateMessage_ClientError(t *testing.T) {
	mockClient := &mockMCPClient{
		createMessageError: errors.New("client error"),
	}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	req := types.CreateMessageRequest{
		Messages: []types.SamplingMessage{
			{Role: types.RoleUser, Content: types.TextContent{Type: "text", Text: "Hello"}},
		},
		MaxTokens: 100,
	}
	
	_, err := secureClient.CreateMessage(context.Background(), req)
	if err == nil {
		t.Error("Expected client error to be propagated")
	}
	if !strings.Contains(err.Error(), "client error") {
		t.Error("Expected client error message")
	}
}

func TestSecureSamplingClient_GetRemainingQuota(t *testing.T) {
	mockClient := &mockMCPClient{}
	rateLimiter := &mockRateLimiter{quota: 42}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	security.SetRateLimiter(rateLimiter)
	
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	quota, err := secureClient.GetRemainingQuota(context.Background())
	if err != nil {
		t.Fatalf("GetRemainingQuota failed: %v", err)
	}
	
	if quota != 42 {
		t.Errorf("Expected quota 42, got %d", quota)
	}
}

func TestSecureSamplingClient_Initialize(t *testing.T) {
	expectedResult := &types.InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo: types.Implementation{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}
	
	mockClient := &mockMCPClient{
		initializeResult: expectedResult,
	}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	clientInfo := types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}
	
	result, err := secureClient.Initialize(context.Background(), clientInfo)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	if result != expectedResult {
		t.Error("Expected result to be returned from underlying client")
	}
}

func TestSecureSamplingClient_ListResources(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.ListResources(context.Background(), types.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_ReadResource(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.ReadResource(context.Background(), types.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_ListTools(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.ListTools(context.Background(), types.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_CallTool(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.CallTool(context.Background(), types.CallToolRequest{})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_ListPrompts(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.ListPrompts(context.Background(), types.ListPromptsRequest{})
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_GetPrompt(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.GetPrompt(context.Background(), types.GetPromptRequest{})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_Subscribe(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	err := secureClient.Subscribe(context.Background(), types.SubscribeRequest{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
}

func TestSecureSamplingClient_Unsubscribe(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	err := secureClient.Unsubscribe(context.Background(), types.UnsubscribeRequest{})
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}

func TestSecureSamplingClient_ListRoots(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	result, err := secureClient.ListRoots(context.Background(), types.ListRootsRequest{})
	if err != nil {
		t.Fatalf("ListRoots failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestSecureSamplingClient_Ping(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	err := secureClient.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestSecureSamplingClient_Close(t *testing.T) {
	mockClient := &mockMCPClient{}
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	err := secureClient.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestSecureSamplingClient_ServerInfo(t *testing.T) {
	expectedInfo := &types.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}
	
	mockClient := &mockMCPClient{
		serverInfo: expectedInfo,
	}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	info := secureClient.ServerInfo()
	if info != expectedInfo {
		t.Error("Expected server info to be returned from underlying client")
	}
}

func TestSecureSamplingClient_ServerCapabilities(t *testing.T) {
	expectedCaps := &types.ServerCapabilities{
		Tools: &types.ToolsCapability{ListChanged: true},
	}
	
	mockClient := &mockMCPClient{
		serverCapabilities: expectedCaps,
	}
	
	security := NewSamplingSecurity(SamplingSecurityConfig{})
	secureClient := NewSecureSamplingClient(mockClient, security, "test-client")
	
	caps := secureClient.ServerCapabilities()
	if caps != expectedCaps {
		t.Error("Expected server capabilities to be returned from underlying client")
	}
}

