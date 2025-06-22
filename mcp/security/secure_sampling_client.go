package security

import (
	"context"
	"fmt"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// MCPClient defines the interface for MCP client operations
type MCPClient interface {
	CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error)
	Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error)
	ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error)
	ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error)
	ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error)
	CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error)
	ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error)
	GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error)
	Subscribe(ctx context.Context, req types.SubscribeRequest) error
	Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error
	ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error)
	Ping(ctx context.Context) error
	Close() error
	ServerInfo() *types.Implementation
	ServerCapabilities() *types.ServerCapabilities
}

// SecureSamplingClient wraps a regular MCP client with sampling security controls
type SecureSamplingClient struct {
	client   MCPClient
	security *SamplingSecurity
	clientID string
}

// NewSecureSamplingClient creates a new secure sampling client
func NewSecureSamplingClient(mcpClient MCPClient, security *SamplingSecurity, clientID string) *SecureSamplingClient {
	return &SecureSamplingClient{
		client:   mcpClient,
		security: security,
		clientID: clientID,
	}
}

// NewSecureSamplingClientFromClient creates a new secure sampling client from a concrete client.Client
func NewSecureSamplingClientFromClient(mcpClient *client.Client, security *SamplingSecurity, clientID string) *SecureSamplingClient {
	return NewSecureSamplingClient(mcpClient, security, clientID)
}

// CreateMessage creates a message with security validation and user approval
func (c *SecureSamplingClient) CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error) {
	// Validate and potentially get user approval
	validatedReq, err := c.security.ValidateAndApprove(ctx, req, c.clientID)
	if err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Make the actual sampling request
	result, err := c.client.CreateMessage(ctx, *validatedReq)
	if err != nil {
		return nil, err
	}

	// Log the response for audit
	c.security.LogSamplingResponse(ctx, *result)

	return result, nil
}

// GetRemainingQuota returns the remaining sampling quota for this client
func (c *SecureSamplingClient) GetRemainingQuota(ctx context.Context) (int, error) {
	return c.security.rateLimiter.GetRemainingQuota(ctx, c.clientID)
}

// Embed other client methods
func (c *SecureSamplingClient) Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error) {
	return c.client.Initialize(ctx, clientInfo)
}

func (c *SecureSamplingClient) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	return c.client.ListResources(ctx, req)
}

func (c *SecureSamplingClient) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	return c.client.ReadResource(ctx, req)
}

func (c *SecureSamplingClient) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return c.client.ListTools(ctx, req)
}

func (c *SecureSamplingClient) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	// Apply security checks for tool calls that might be sensitive
	if err := c.security.ValidateToolCall(ctx, req, c.clientID); err != nil {
		return nil, fmt.Errorf("tool call security validation failed: %w", err)
	}

	return c.client.CallTool(ctx, req)
}

func (c *SecureSamplingClient) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	return c.client.ListPrompts(ctx, req)
}

func (c *SecureSamplingClient) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	return c.client.GetPrompt(ctx, req)
}

func (c *SecureSamplingClient) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return c.client.Subscribe(ctx, req)
}

func (c *SecureSamplingClient) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return c.client.Unsubscribe(ctx, req)
}

func (c *SecureSamplingClient) ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error) {
	return c.client.ListRoots(ctx, req)
}

func (c *SecureSamplingClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}

func (c *SecureSamplingClient) Close() error {
	return c.client.Close()
}

func (c *SecureSamplingClient) ServerInfo() *types.Implementation {
	return c.client.ServerInfo()
}

func (c *SecureSamplingClient) ServerCapabilities() *types.ServerCapabilities {
	return c.client.ServerCapabilities()
}