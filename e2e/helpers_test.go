package mcp_test

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/server"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// strPtr returns a pointer to a string
func strPtr(s string) *string {
	return &s
}

// mockPipe simulates a bidirectional pipe for testing
type mockPipe struct {
	serverToClient chan []byte
	clientToServer chan []byte
	closed         bool
}

func newMockPipe() *mockPipe {
	return &mockPipe{
		serverToClient: make(chan []byte, 100),
		clientToServer: make(chan []byte, 100),
	}
}

func (p *mockPipe) ServerReader() io.Reader {
	return &mockReader{ch: p.clientToServer, closed: &p.closed}
}

func (p *mockPipe) ServerWriter() io.Writer {
	return &mockWriter{ch: p.serverToClient}
}

func (p *mockPipe) ClientReader() io.Reader {
	return &mockReader{ch: p.serverToClient, closed: &p.closed}
}

func (p *mockPipe) ClientWriter() io.Writer {
	return &mockWriter{ch: p.clientToServer}
}

func (p *mockPipe) Close() {
	p.closed = true
	close(p.serverToClient)
	close(p.clientToServer)
}

type mockReader struct {
	ch     chan []byte
	buffer []byte
	closed *bool
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if len(r.buffer) > 0 {
		n = copy(p, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}
	
	if *r.closed {
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
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	
	resources := make([]types.Resource, 0, len(h.resources))
	for uri, name := range h.resources {
		resources = append(resources, types.Resource{
			URI:         uri,
			Name:        name,
			Description: strPtr("Test resource"),
			MimeType:    strPtr("text/plain"),
		})
	}
	
	return &types.ListResourcesResult{
		Resources: resources,
	}, nil
}

func (h *testResourceHandler) ReadResource(ctx context.Context, uri string) (*types.ReadResourceResult, error) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	
	if content, exists := h.resources[uri]; exists {
		return &types.ReadResourceResult{
			Contents: []interface{}{
				types.ResourceContentsText{
					Text:     content,
					MimeType: strPtr("text/plain"),
				},
			},
		}, nil
	}
	
	return nil, &types.McpError{
		Code:    "resource_not_found",
		Message: "Resource not found",
	}
}

type testToolHandler struct {
	tools map[string]types.Tool
	delay time.Duration
}

func (h *testToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	
	tools := make([]types.Tool, 0, len(h.tools))
	for _, tool := range h.tools {
		tools = append(tools, tool)
	}
	
	return &types.ListToolsResult{
		Tools: tools,
	}, nil
}

func (h *testToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	
	if tool, exists := h.tools[req.Name]; exists {
		// Simple echo tool implementation
		if req.Name == "echo" {
			var args map[string]interface{}
			if req.Arguments != nil {
				json.Unmarshal(req.Arguments, &args)
			}
			
			return &types.CallToolResult{
				Content: []interface{}{
					types.ResourceContentsText{
						Text: args["message"].(string),
					},
				},
			}, nil
		}
		
		// Default tool response
		return &types.CallToolResult{
			Content: []interface{}{
				types.ResourceContentsText{
					Text: "Tool executed: " + tool.Name,
				},
			},
		}, nil
	}
	
	return nil, &types.McpError{
		Code:    "tool_not_found",
		Message: "Tool not found",
	}
}

// Helper function to run server
func runServer(server *server.Server, serverTransport client.Transport, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		msg, err := serverTransport.Receive()
		if err != nil {
			return
		}
		
		if msg.Method != "" && msg.ID == nil {
			server.HandleNotification(ctx, msg)
		} else if msg.ID != nil {
			response, err := server.HandleRequest(ctx, msg)
			if err == nil {
				serverTransport.Send(response)
			}
		}
	}
}