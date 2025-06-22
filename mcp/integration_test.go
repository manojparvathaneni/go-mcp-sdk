package mcp_test

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/transport"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// mockPipe simulates a bidirectional pipe for testing
type mockPipe struct {
	serverToClient chan []byte
	clientToServer chan []byte
	closed         bool
	mu             sync.Mutex
}

func newMockPipe() *mockPipe {
	return &mockPipe{
		serverToClient: make(chan []byte, 100),
		clientToServer: make(chan []byte, 100),
	}
}

func (p *mockPipe) ServerReader() io.Reader {
	return &mockReader{ch: p.clientToServer, pipe: p}
}

func (p *mockPipe) ServerWriter() io.Writer {
	return &mockWriter{ch: p.serverToClient}
}

func (p *mockPipe) ClientReader() io.Reader {
	return &mockReader{ch: p.serverToClient, pipe: p}
}

func (p *mockPipe) ClientWriter() io.Writer {
	return &mockWriter{ch: p.clientToServer}
}

func (p *mockPipe) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		close(p.serverToClient)
		close(p.clientToServer)
	}
}

func (p *mockPipe) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

type mockReader struct {
	ch     chan []byte
	buffer []byte
	pipe   *mockPipe
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if len(r.buffer) > 0 {
		n = copy(p, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}
	
	if r.pipe.isClosed() {
		return 0, io.EOF
	}
	
	data, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}
	
	n = copy(p, data)
	if n < len(data) {
		r.buffer = data[n:]
	}
	
	return n, nil
}

type mockWriter struct {
	ch chan []byte
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	data := make([]byte, len(p))
	copy(data, p)
	w.ch <- data
	return len(p), nil
}

// Test handler implementations
type testResourceHandler struct {
	resources map[string]string
	delay     time.Duration // Add delay for timeout testing
}

func (h *testResourceHandler) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	var resources []types.Resource
	for uri, name := range h.resources {
		resources = append(resources, types.Resource{
			URI:  uri,
			Name: name,
		})
	}
	return &types.ListResourcesResult{Resources: resources}, nil
}

func (h *testResourceHandler) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	// Add delay if configured (for timeout testing)
	if h.delay > 0 {
		select {
		case <-time.After(h.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	content, ok := h.resources[req.URI]
	if !ok {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeResourceNotFound, "Resource not found", nil)
	}
	
	return &types.ReadResourceResult{
		Contents: []interface{}{
			types.ResourceContentsText{
				URI:  req.URI,
				Text: content,
			},
		},
	}, nil
}

func (h *testResourceHandler) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (h *testResourceHandler) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

type testToolHandler struct{}

func (h *testToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "echo",
				Description: strPtr("Echo the input"),
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}, nil
}

func (h *testToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	if req.Name != "echo" {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeToolNotFound, "Tool not found", nil)
	}
	
	var args struct {
		Message string `json:"message"`
	}
	
	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return &types.CallToolResult{
			Content: []interface{}{
				types.TextContent{Type: "text", Text: "Invalid arguments"},
			},
			IsError: true,
		}, nil
	}
	
	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: args.Message},
		},
	}, nil
}

func TestIntegration_BasicFlow(t *testing.T) {
	// Create mock pipe
	pipe := newMockPipe()
	
	// Start server
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:    "test-server",
		Version: "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{},
			Tools:     &types.ToolsCapability{},
		},
		ResourceHandler: &testResourceHandler{
			resources: map[string]string{
				"test://resource1": "Hello, World!",
				"test://resource2": "Goodbye, World!",
			},
		},
		ToolHandler: &testToolHandler{},
	})
	
	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)
	
	// Start server in goroutine
	serverCtx, serverCancel := context.WithCancel(context.Background())
	
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		for {
			select {
			case <-serverCtx.Done():
				return
			default:
			}
			
			msg, err := serverTransport.Receive()
			if err != nil {
				return
			}
			
			if msg.Method != "" && msg.ID == nil {
				server.HandleNotification(serverCtx, msg)
			} else if msg.ID != nil {
				response, err := server.HandleRequest(serverCtx, msg)
				if err == nil {
					select {
					case <-serverCtx.Done():
						return
					default:
						serverTransport.Send(response)
					}
				}
			}
		}
	}()
	
	// Create client
	mcpClient := mcp.NewMCPClient(client.ClientOptions{
		Capabilities: types.ClientCapabilities{},
	})
	
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	mcpClient.SetTransport(clientTransport)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Test initialization
	initResult, err := mcpClient.Initialize(ctx, types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	
	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", initResult.ServerInfo.Name)
	}
	
	// Test list resources
	resources, err := mcpClient.ListResources(ctx, types.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	
	if len(resources.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources.Resources))
	}
	
	// Test read resource
	resource, err := mcpClient.ReadResource(ctx, types.ReadResourceRequest{
		URI: "test://resource1",
	})
	if err != nil {
		t.Fatalf("Failed to read resource: %v", err)
	}
	
	if len(resource.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(resource.Contents))
	}
	
	// Test list tools
	tools, err := mcpClient.ListTools(ctx, types.ListToolsRequest{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}
	
	if len(tools.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools.Tools))
	}
	
	// Test call tool
	args, _ := json.Marshal(map[string]interface{}{
		"message": "Hello from test!",
	})
	
	result, err := mcpClient.CallTool(ctx, types.CallToolRequest{
		Name:      "echo",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("Failed to call tool: %v", err)
	}
	
	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}
	
	textContent, ok := result.Content[0].(types.TextContent)
	if !ok {
		t.Errorf("Expected TextContent, got %T", result.Content[0])
	} else if textContent.Text != "Hello from test!" {
		t.Errorf("Expected 'Hello from test!', got '%s'", textContent.Text)
	}
	
	// Clean shutdown
	serverCancel()
	pipe.Close() // Close pipe first to unblock Receive()
	wg.Wait()
}

func TestIntegration_ErrorHandling(t *testing.T) {
	pipe := newMockPipe()
	
	// Start server with empty handlers
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:    "test-server",
		Version: "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{},
			Tools:     &types.ToolsCapability{},
		},
		ResourceHandler: &testResourceHandler{resources: make(map[string]string)},
		ToolHandler:     &testToolHandler{},
	})
	
	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)
	
	// Start server
	serverCtx, serverCancel := context.WithCancel(context.Background())
	
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		for {
			select {
			case <-serverCtx.Done():
				return
			default:
			}
			
			msg, err := serverTransport.Receive()
			if err != nil {
				return
			}
			
			if msg.ID != nil {
				response, err := server.HandleRequest(serverCtx, msg)
				if err == nil {
					select {
					case <-serverCtx.Done():
						return
					default:
						serverTransport.Send(response)
					}
				}
			}
		}
	}()
	
	// Create client
	mcpClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	mcpClient.SetTransport(clientTransport)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Initialize
	_, err := mcpClient.Initialize(ctx, types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	
	// Test resource not found
	_, err = mcpClient.ReadResource(ctx, types.ReadResourceRequest{
		URI: "test://nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent resource")
	}
	
	// Test tool not found
	_, err = mcpClient.CallTool(ctx, types.CallToolRequest{
		Name:      "nonexistent",
		Arguments: json.RawMessage("{}"),
	})
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}
	
	// Clean shutdown
	serverCancel()
	pipe.Close() // Close pipe first to unblock Receive()
	wg.Wait()
}

func TestIntegration_Timeout(t *testing.T) {
	pipe := newMockPipe()
	
	// Create slow resource handler
	slowHandler := &testResourceHandler{
		resources: map[string]string{
			"test://slow": "slow content",
		},
		delay: 100 * time.Millisecond, // Wait longer than the client timeout
	}
	
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:            "test-server",
		Version:         "1.0.0",
		Capabilities:    types.ServerCapabilities{Resources: &types.ResourcesCapability{}},
		ResourceHandler: slowHandler,
	})
	
	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)
	
	// Start server
	serverCtx, serverCancel := context.WithCancel(context.Background())
	
	// Use WaitGroup to ensure server goroutine completes before closing pipe
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		for {
			select {
			case <-serverCtx.Done():
				return
			default:
			}
			
			msg, err := serverTransport.Receive()
			if err != nil {
				return
			}
			
			if msg.ID != nil {
				response, err := server.HandleRequest(serverCtx, msg)
				if err == nil {
					select {
					case <-serverCtx.Done():
						return
					default:
						serverTransport.Send(response)
					}
				}
			}
		}
	}()
	
	// Create client
	mcpClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	mcpClient.SetTransport(clientTransport)
	
	// Initialize
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := mcpClient.Initialize(initCtx, types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}
	
	// Test with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	_, err = mcpClient.ReadResource(ctx, types.ReadResourceRequest{
		URI: "test://slow",
	})
	
	if err == nil {
		t.Error("Expected timeout error")
	}
	
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded, got %v", ctx.Err())
	}
	
	// Clean shutdown
	serverCancel()
	pipe.Close() // Close pipe first to unblock Receive()
	wg.Wait()
}

func BenchmarkClientServerRoundTrip(b *testing.B) {
	pipe := newMockPipe()
	defer pipe.Close()
	
	// Setup server
	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:            "bench-server",
		Version:         "1.0.0",
		Capabilities:    types.ServerCapabilities{Tools: &types.ToolsCapability{}},
		ToolHandler:     &testToolHandler{},
	})
	
	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)
	
	// Start server
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()
	
	go func() {
		for {
			select {
			case <-serverCtx.Done():
				return
			default:
			}
			
			msg, err := serverTransport.Receive()
			if err != nil {
				return
			}
			
			if msg.ID != nil {
				response, err := server.HandleRequest(serverCtx, msg)
				if err == nil {
					select {
					case <-serverCtx.Done():
						return
					default:
						serverTransport.Send(response)
					}
				}
			}
		}
	}()
	
	// Setup client
	mcpClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	mcpClient.SetTransport(clientTransport)
	
	// Initialize
	_, err := mcpClient.Initialize(context.Background(), types.Implementation{
		Name:    "bench-client",
		Version: "1.0.0",
	})
	if err != nil {
		b.Fatalf("Failed to initialize: %v", err)
	}
	
	args, _ := json.Marshal(map[string]interface{}{
		"message": "benchmark test",
	})
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := mcpClient.CallTool(context.Background(), types.CallToolRequest{
			Name:      "echo",
			Arguments: args,
		})
		if err != nil {
			b.Fatalf("Tool call failed: %v", err)
		}
	}
}

func strPtr(s string) *string {
	return &s
}