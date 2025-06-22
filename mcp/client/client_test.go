package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock transport for testing
type mockTransport struct {
	sent        []*types.JSONRPCMessage
	closed      bool
	closeChan   chan struct{}
	receiveChan chan *types.JSONRPCMessage
	mu          sync.Mutex
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		closeChan:   make(chan struct{}),
		receiveChan: make(chan *types.JSONRPCMessage, 10), // Buffered to prevent blocking
	}
}

func (m *mockTransport) Send(msg *types.JSONRPCMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockTransport) Receive() (*types.JSONRPCMessage, error) {
	select {
	case <-m.closeChan:
		return nil, fmt.Errorf("transport closed")
	case msg := <-m.receiveChan:
		return msg, nil
	}
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.closeChan)
	}
	return nil
}

func (m *mockTransport) ConnectionID() string {
	return "test-connection"
}

// Helper method to queue messages for receiving
func (m *mockTransport) QueueMessage(msg *types.JSONRPCMessage) {
	select {
	case m.receiveChan <- msg:
	case <-m.closeChan:
		// Transport is closed, ignore
	}
}

func TestClient_Initialize(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(ClientOptions{})
	
	client.SetTransport(transport)
	defer client.Close()
	
	// Wait for receive loop to start
	client.WaitForReceiveLoop()

	impl := types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	// Start the Initialize call in a goroutine since we need to queue the response
	resultChan := make(chan *types.InitializeResult)
	errChan := make(chan error)
	
	go func() {
		result, err := client.Initialize(context.Background(), impl)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()
	
	// Give time for the request to be sent
	time.Sleep(10 * time.Millisecond)
	
	// Mock response
	id := json.RawMessage(`1`)
	response := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"serverInfo": {
				"name": "test-server",
				"version": "1.0.0"
			}
		}`),
	}
	transport.QueueMessage(response)
	
	// Wait for result
	select {
	case result := <-resultChan:
		if result.ServerInfo.Name != "test-server" {
			t.Errorf("Expected server name 'test-server', got %s", result.ServerInfo.Name)
		}
	case err := <-errChan:
		t.Fatalf("Initialize failed: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Initialize timed out")
	}

	// Check that initialize request and initialized notification were sent
	if len(transport.sent) != 2 {
		t.Errorf("Expected 2 sent messages, got %d", len(transport.sent))
	}
	if transport.sent[0].Method != "initialize" {
		t.Errorf("Expected first method 'initialize', got %s", transport.sent[0].Method)
	}
	if transport.sent[1].Method != "initialized" {
		t.Errorf("Expected second method 'initialized', got %s", transport.sent[1].Method)
	}
}

func TestClient_ListResources(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(ClientOptions{})
	client.SetTransport(transport)
	defer client.Close()
	
	// Wait for receive loop to start
	client.WaitForReceiveLoop()

	// Start the ListResources call in a goroutine
	resultChan := make(chan *types.ListResourcesResult)
	errChan := make(chan error)
	
	go func() {
		result, err := client.ListResources(context.Background(), types.ListResourcesRequest{})
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()
	
	// Give time for the request to be sent
	time.Sleep(10 * time.Millisecond)
	
	// Mock response
	id := json.RawMessage(`1`)
	response := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result: json.RawMessage(`{
			"resources": [
				{
					"uri": "test://resource",
					"name": "Test Resource"
				}
			]
		}`),
	}
	transport.QueueMessage(response)

	// Wait for result
	select {
	case result := <-resultChan:
		if len(result.Resources) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(result.Resources))
		}
		if result.Resources[0].URI != "test://resource" {
			t.Errorf("Expected URI 'test://resource', got %s", result.Resources[0].URI)
		}
	case err := <-errChan:
		t.Fatalf("ListResources failed: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("ListResources timed out")
	}
}

func TestClient_CallTool(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(ClientOptions{})
	client.SetTransport(transport)
	defer client.Close()
	
	// Wait for receive loop to start
	client.WaitForReceiveLoop()

	req := types.CallToolRequest{
		Name:      "test-tool",
		Arguments: json.RawMessage(`{"param": "value"}`),
	}

	// Start the CallTool call in a goroutine
	resultChan := make(chan *types.CallToolResult)
	errChan := make(chan error)
	
	go func() {
		result, err := client.CallTool(context.Background(), req)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()
	
	// Give time for the request to be sent
	time.Sleep(10 * time.Millisecond)
	
	// Mock response
	id := json.RawMessage(`1`)
	response := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result: json.RawMessage(`{
			"content": [
				{
					"type": "text",
					"text": "Tool result"
				}
			]
		}`),
	}
	transport.QueueMessage(response)

	// Wait for result
	select {
	case result := <-resultChan:
		if len(result.Content) != 1 {
			t.Errorf("Expected 1 content item, got %d", len(result.Content))
		}
	case err := <-errChan:
		t.Fatalf("CallTool failed: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("CallTool timed out")
	}

	// Check that tool call request was sent
	if len(transport.sent) != 1 {
		t.Errorf("Expected 1 sent message, got %d", len(transport.sent))
	}
	if transport.sent[0].Method != "tools/call" {
		t.Errorf("Expected method 'tools/call', got %s", transport.sent[0].Method)
	}
}

func TestClient_NotInitialized(t *testing.T) {
	client := NewClient(ClientOptions{})
	
	_, err := client.ListResources(context.Background(), types.ListResourcesRequest{})
	if err == nil {
		t.Error("Expected error for non-initialized client")
	}
}