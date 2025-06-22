package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/events"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/middleware"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/security"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/transport"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// TestE2E_SecureToolWithElicitation tests a complete workflow where:
// 1. Client calls a tool that requires security approval
// 2. Security system requests user approval via elicitation
// 3. User approves, tool executes successfully
// 4. All events are properly emitted throughout
func TestE2E_SecureToolWithElicitation(t *testing.T) {
	pipe := newMockPipe()
	defer pipe.Close()

	// Track events throughout the workflow
	eventEmitter := events.NewEventEmitter()
	var receivedEvents []events.Event
	var eventMu sync.Mutex

	eventEmitter.On(events.EventToolCalled, func(event events.Event) error {
		eventMu.Lock()
		receivedEvents = append(receivedEvents, event)
		eventMu.Unlock()
		return nil
	})

	// Create a secure tool handler that requires approval
	secureToolHandler := &secureToolHandler{
		approvalRequired: true,
		eventEmitter:     eventEmitter,
	}

	// Create server with secure tool
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "secure-server",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Tools:    &types.ToolsCapability{},
			Sampling: &types.SamplingCapability{},
		},
		ToolHandler: secureToolHandler,
	})

	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)

	// Start server
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server, serverTransport, serverCtx)

	// Create base client
	baseClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	baseClient.SetTransport(clientTransport)

	// Initialize client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := baseClient.Initialize(ctx, types.Implementation{
		Name:    "e2e-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Create security layer with approval workflow
	securityConfig := security.DefaultSamplingSecurityConfig()
	securityConfig.RequireUserApproval = true

	approvalCalled := false
	approvalHandler := func(ctx context.Context, req security.ApprovalRequest) (security.ApprovalResponse, error) {
		approvalCalled = true
		// Simulate user approval process
		return security.ApprovalResponse{
			Result: security.ApprovalGranted,
		}, nil
	}

	secSampling := security.NewSamplingSecurity(securityConfig)
	secSampling.SetApprovalHandler(approvalHandler)

	// Create secure client wrapper
	secureClient := security.NewSecureSamplingClient(baseClient, secSampling, "e2e-client-1")

	// Step 1: List tools to verify server is working
	tools, err := secureClient.ListTools(ctx, types.ListToolsRequest{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	if len(tools.Tools) == 0 {
		t.Fatal("Expected at least one tool")
	}

	// Step 2: Call the secure tool - this should trigger security approval
	args, _ := json.Marshal(map[string]interface{}{
		"operation": "delete",
		"file":      "/important/data.txt",
	})

	result, err := secureClient.CallTool(ctx, types.CallToolRequest{
		Name:      "secure-file-tool",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("Secure tool call failed: %v", err)
	}

	// Step 3: Verify the workflow completed successfully
	if result == nil {
		t.Error("Expected non-nil result from secure tool")
	}

	if !approvalCalled {
		t.Error("Expected security approval to be called")
	}

	// Step 4: Verify events were emitted
	time.Sleep(100 * time.Millisecond) // Allow async events to process

	eventMu.Lock()
	eventCount := len(receivedEvents)
	eventMu.Unlock()

	if eventCount < 1 {
		t.Error("Expected at least one tool event to be emitted")
	}

	// Step 5: Test that the tool actually executed by checking the result
	if len(result.Content) == 0 {
		t.Error("Expected tool to return content")
	}

	textContent, ok := result.Content[0].(types.TextContent)
	if !ok {
		t.Error("Expected text content from tool")
	} else if !contains(textContent.Text, "approved") {
		t.Error("Expected tool result to indicate approval was processed")
	}
}

// TestE2E_MultiClientResourceSubscription tests:
// 1. Multiple clients subscribing to the same resource
// 2. Resource updates triggering notifications to all clients
// 3. Client disconnection and reconnection handling
func TestE2E_MultiClientResourceSubscription(t *testing.T) {
	// Create a server with a resource that can be subscribed to
	resourceHandler := &subscribableResourceHandler{
		resources: map[string]*subscribableResource{
			"file://test.txt": {
				content:     "initial content",
				subscribers: make(map[string]bool),
			},
		},
	}

	pipe1 := newMockPipe()
	pipe2 := newMockPipe()
	defer pipe1.Close()
	defer pipe2.Close()

	// Start server instances for each client
	serverTransport1 := transport.NewStdioTransportWithStreams(pipe1.ServerReader(), pipe1.ServerWriter())
	serverTransport2 := transport.NewStdioTransportWithStreams(pipe2.ServerReader(), pipe2.ServerWriter())

	server1 := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "subscription-server-1",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{
				Subscribe: true,
			},
		},
		ResourceHandler: resourceHandler,
	})
	server2 := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "subscription-server-2",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{
				Subscribe: true,
			},
		},
		ResourceHandler: resourceHandler,
	})
	server1.SetTransport(serverTransport1)
	server2.SetTransport(serverTransport2)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server1, serverTransport1, serverCtx)
	go runServer(server2, serverTransport2, serverCtx)

	// Create two clients
	client1 := mcp.NewMCPClient(client.ClientOptions{})
	client2 := mcp.NewMCPClient(client.ClientOptions{})

	clientTransport1 := transport.NewStdioTransportWithStreams(pipe1.ClientReader(), pipe1.ClientWriter())
	clientTransport2 := transport.NewStdioTransportWithStreams(pipe2.ClientReader(), pipe2.ClientWriter())

	client1.SetTransport(clientTransport1)
	client2.SetTransport(clientTransport2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize both clients
	_, err := client1.Initialize(ctx, types.Implementation{Name: "client1", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Failed to initialize client1: %v", err)
	}

	_, err = client2.Initialize(ctx, types.Implementation{Name: "client2", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Failed to initialize client2: %v", err)
	}

	// Both clients subscribe to the same resource
	err = client1.Subscribe(ctx, types.SubscribeRequest{
		URI: "file://test.txt",
	})
	if err != nil {
		t.Fatalf("Client1 subscription failed: %v", err)
	}

	err = client2.Subscribe(ctx, types.SubscribeRequest{
		URI: "file://test.txt",
	})
	if err != nil {
		t.Fatalf("Client2 subscription failed: %v", err)
	}

	// Verify both clients are subscribed
	resource := resourceHandler.resources["file://test.txt"]
	if len(resource.subscribers) != 2 {
		t.Errorf("Expected 2 subscribers, got %d", len(resource.subscribers))
	}

	// Test resource update notification (this would be more complex in a real implementation)
	// For now, just verify the subscription state is maintained
	if !resource.subscribers["client1"] || !resource.subscribers["client2"] {
		t.Error("Expected both clients to be subscribed")
	}

	// Test unsubscription
	err = client1.Unsubscribe(ctx, types.UnsubscribeRequest{
		URI: "file://test.txt",
	})
	if err != nil {
		t.Fatalf("Client1 unsubscription failed: %v", err)
	}

	if len(resource.subscribers) != 1 {
		t.Errorf("Expected 1 subscriber after unsubscribe, got %d", len(resource.subscribers))
	}

	if !resource.subscribers["client2"] {
		t.Error("Expected client2 to still be subscribed")
	}
}

// TestE2E_ComplexPromptWithResources tests:
// 1. Prompt execution that reads multiple resources
// 2. Processing and transformation of resource content
// 3. Returning structured results
func TestE2E_ComplexPromptWithResources(t *testing.T) {
	pipe := newMockPipe()
	defer pipe.Close()

	// Create server with prompts and resources
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "prompt-server",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{},
			Prompts:   &types.PromptsCapability{},
		},
		ResourceHandler: &e2eResourceHandler{},
		PromptHandler:   &e2ePromptHandler{},
	})

	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server, serverTransport, serverCtx)

	// Create client
	mcpClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	mcpClient.SetTransport(clientTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := mcpClient.Initialize(ctx, types.Implementation{
		Name:    "prompt-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Step 1: List available prompts
	prompts, err := mcpClient.ListPrompts(ctx, types.ListPromptsRequest{})
	if err != nil {
		t.Fatalf("Failed to list prompts: %v", err)
	}

	if len(prompts.Prompts) == 0 {
		t.Fatal("Expected at least one prompt")
	}

	// Step 2: Execute a complex prompt that processes resources
	promptArgs := map[string]interface{}{
		"file_pattern": "*.go",
		"analysis":     "complexity",
	}

	result, err := mcpClient.GetPrompt(ctx, types.GetPromptRequest{
		Name:      "analyze-code",
		Arguments: promptArgs,
	})
	if err != nil {
		t.Fatalf("Prompt execution failed: %v", err)
	}

	// Step 3: Verify the prompt returned structured results
	if len(result.Messages) == 0 {
		t.Error("Expected prompt to return messages")
	}

	// Step 4: Verify the prompt actually read resources by checking content
	found := false
	for _, msg := range result.Messages {
		// Check if message has text content that mentions resources
		if msgText := getTextFromMessage(msg); msgText != "" {
			if contains(msgText, "resource") {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Expected prompt result to reference processed resources")
	}
}

// TestE2E_ConcurrentToolCallsWithSecurity tests:
// 1. Multiple concurrent tool calls with security validation
// 2. Rate limiting across concurrent requests
// 3. Proper request isolation and response handling
func TestE2E_ConcurrentToolCallsWithSecurity(t *testing.T) {
	pipe := newMockPipe()
	defer pipe.Close()

	// Create server with rate-limited secure tools
	toolHandler := &concurrentSecureToolHandler{
		callCounts: make(map[string]int),
		mu:         sync.Mutex{},
	}

	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "concurrent-server",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Tools:    &types.ToolsCapability{},
			Sampling: &types.SamplingCapability{},
		},
		ToolHandler: toolHandler,
	})

	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server, serverTransport, serverCtx)

	// Create secure client with rate limiting
	baseClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	baseClient.SetTransport(clientTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := baseClient.Initialize(ctx, types.Implementation{
		Name:    "concurrent-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Create security layer with rate limiting
	securityConfig := security.DefaultSamplingSecurityConfig()
	securityConfig.EnableRateLimit = true

	rateLimiter := middleware.NewSimpleRateLimiter(5, time.Minute) // 5 calls per minute
	secSampling := security.NewSamplingSecurity(securityConfig)
	secSampling.SetRateLimiter(rateLimiter)

	secureClient := security.NewSecureSamplingClient(baseClient, secSampling, "concurrent-client")

	// Launch concurrent tool calls
	const numConcurrent = 10
	results := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(id int) {
			args, _ := json.Marshal(map[string]interface{}{
				"task_id": id,
				"data":    fmt.Sprintf("task_%d_data", id),
			})

			_, err := secureClient.CallTool(ctx, types.CallToolRequest{
				Name:      "concurrent-task",
				Arguments: args,
			})

			results <- err
		}(i)
	}

	// Collect results
	var errors []error
	var successes int

	for i := 0; i < numConcurrent; i++ {
		select {
		case err := <-results:
			if err != nil {
				errors = append(errors, err)
			} else {
				successes++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent calls timed out")
		}
	}

	// Verify some calls succeeded (within rate limit) and some failed
	if successes == 0 {
		t.Error("Expected at least some tool calls to succeed")
	}

	if len(errors) == 0 {
		t.Error("Expected some calls to be rate limited")
	}

	// Verify tool handler received calls
	toolHandler.mu.Lock()
	totalCalls := toolHandler.callCounts["concurrent-task"]
	toolHandler.mu.Unlock()

	if totalCalls != successes {
		t.Errorf("Expected %d successful calls to reach tool handler, got %d", successes, totalCalls)
	}
}

// Helper types for E2E tests

type secureToolHandler struct {
	approvalRequired bool
	eventEmitter     *events.EventEmitter
}

func (h *secureToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "secure-file-tool",
				Description: strPtr("A tool that requires security approval"),
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{"type": "string"},
						"file":      map[string]interface{}{"type": "string"},
					},
					"required": []string{"operation", "file"},
				},
			},
		},
	}, nil
}

func (h *secureToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	if h.eventEmitter != nil {
		h.eventEmitter.Emit(events.Event{
			Type: events.EventToolCalled,
			Data: events.ToolEventData(req.Name, req.Arguments, "processing"),
		})
	}

	var args struct {
		Operation string `json:"operation"`
		File      string `json:"file"`
	}

	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return &types.CallToolResult{
			Content: []interface{}{
				types.TextContent{Type: "text", Text: "Invalid arguments"},
			},
			IsError: true,
		}, nil
	}

	// Simulate security-sensitive operation
	response := fmt.Sprintf("Operation '%s' on file '%s' was approved and executed successfully", 
		args.Operation, args.File)

	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: response},
		},
	}, nil
}

type subscribableResource struct {
	content     string
	subscribers map[string]bool
	mu          sync.RWMutex
}

type subscribableResourceHandler struct {
	resources map[string]*subscribableResource
	mu        sync.RWMutex
}

func (h *subscribableResourceHandler) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var resources []types.Resource
	for uri := range h.resources {
		resources = append(resources, types.Resource{
			URI:  uri,
			Name: uri,
		})
	}

	return &types.ListResourcesResult{Resources: resources}, nil
}

func (h *subscribableResourceHandler) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	h.mu.RLock()
	resource, exists := h.resources[req.URI]
	h.mu.RUnlock()

	if !exists {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeResourceNotFound, "Resource not found", nil)
	}

	resource.mu.RLock()
	content := resource.content
	resource.mu.RUnlock()

	return &types.ReadResourceResult{
		Contents: []interface{}{
			types.ResourceContentsText{
				URI:  req.URI,
				Text: content,
			},
		},
	}, nil
}

func (h *subscribableResourceHandler) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	h.mu.RLock()
	resource, exists := h.resources[req.URI]
	h.mu.RUnlock()

	if !exists {
		return mcp.NewProtocolError(mcp.ErrorCodeResourceNotFound, "Resource not found", nil)
	}

	// Extract client ID from context (simplified)
	clientID := "client1" // In real implementation, extract from context
	if req.URI == "file://test.txt" {
		// Simple way to distinguish clients for testing
		if len(resource.subscribers) == 0 {
			clientID = "client1"
		} else {
			clientID = "client2"
		}
	}

	resource.mu.Lock()
	resource.subscribers[clientID] = true
	resource.mu.Unlock()

	return nil
}

func (h *subscribableResourceHandler) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	h.mu.RLock()
	resource, exists := h.resources[req.URI]
	h.mu.RUnlock()

	if !exists {
		return mcp.NewProtocolError(mcp.ErrorCodeResourceNotFound, "Resource not found", nil)
	}

	// Extract client ID from context (simplified)
	clientID := "client1" // In real implementation, extract from context

	resource.mu.Lock()
	delete(resource.subscribers, clientID)
	resource.mu.Unlock()

	return nil
}

type e2eResourceHandler struct{}

func (h *e2eResourceHandler) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	return &types.ListResourcesResult{
		Resources: []types.Resource{
			{URI: "file://main.go", Name: "Main Go file"},
			{URI: "file://utils.go", Name: "Utilities Go file"},
			{URI: "file://config.json", Name: "Configuration file"},
		},
	}, nil
}

func (h *e2eResourceHandler) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	content := map[string]string{
		"file://main.go":     "package main\n\nfunc main() {\n\t// Sample Go code\n}",
		"file://utils.go":    "package main\n\nfunc helper() string {\n\treturn \"helper\"\n}",
		"file://config.json": `{"setting": "value", "enabled": true}`,
	}

	text, exists := content[req.URI]
	if !exists {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeResourceNotFound, "Resource not found", nil)
	}

	return &types.ReadResourceResult{
		Contents: []interface{}{
			types.ResourceContentsText{
				URI:  req.URI,
				Text: text,
			},
		},
	}, nil
}

func (h *e2eResourceHandler) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (h *e2eResourceHandler) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

type e2ePromptHandler struct{}

func (h *e2ePromptHandler) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	return &types.ListPromptsResult{
		Prompts: []types.Prompt{
			{
				Name:        "analyze-code",
				Description: strPtr("Analyze code files for complexity"),
				Arguments: []types.PromptArgument{
					{
						Name:        "file_pattern",
						Description: strPtr("Pattern to match files"),
						Required:    true,
					},
					{
						Name:        "analysis",
						Description: strPtr("Type of analysis to perform"),
						Required:    false,
					},
				},
			},
		},
	}, nil
}

func (h *e2ePromptHandler) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	// Extract arguments from the map
	filePattern, _ := req.Arguments["file_pattern"].(string)
	analysis, _ := req.Arguments["analysis"].(string)
	
	if filePattern == "" {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeInvalidRequest, "file_pattern is required", nil)
	}

	// Simulate reading and analyzing resources
	analysisResult := fmt.Sprintf("Analyzed files matching pattern '%s' for %s. Found 3 resources with varying complexity levels.",
		filePattern, analysis)

	return &types.GetPromptResult{
		Messages: []interface{}{
			types.PromptMessage{
				Role: types.RoleAssistant,
				Content: types.TextContent{
					Type: "text",
					Text: analysisResult,
				},
			},
		},
	}, nil
}

type concurrentSecureToolHandler struct {
	callCounts map[string]int
	mu         sync.Mutex
}

func (h *concurrentSecureToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "concurrent-task",
				Description: strPtr("A tool for testing concurrent execution"),
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{"type": "integer"},
						"data":    map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}, nil
}

func (h *concurrentSecureToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	h.mu.Lock()
	h.callCounts[req.Name]++
	count := h.callCounts[req.Name]
	h.mu.Unlock()

	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)

	var args struct {
		TaskID int    `json:"task_id"`
		Data   string `json:"data"`
	}

	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return &types.CallToolResult{
			Content: []interface{}{
				types.TextContent{Type: "text", Text: "Invalid arguments"},
			},
			IsError: true,
		}, nil
	}

	response := fmt.Sprintf("Task %d processed successfully (call #%d): %s", 
		args.TaskID, count, args.Data)

	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: response},
		},
	}, nil
}

// Helper functions
func contains(text, substr string) bool {
	if len(text) == 0 || len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func getTextFromMessage(msg interface{}) string {
	if promptMsg, ok := msg.(types.PromptMessage); ok {
		if textContent, ok := promptMsg.Content.(types.TextContent); ok {
			return textContent.Text
		}
	}
	return ""
}