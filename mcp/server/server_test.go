package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

type mockTransport struct {
	sent []*types.JSONRPCMessage
}

func (m *mockTransport) Send(msg *types.JSONRPCMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockTransport) Receive() (*types.JSONRPCMessage, error) {
	return nil, nil
}

func (m *mockTransport) Close() error {
	return nil
}

func (m *mockTransport) ConnectionID() string {
	return "test-connection"
}

func TestServerInitialize(t *testing.T) {
	server := NewServer(ServerOptions{
		Name:    "test-server",
		Version: "1.0.0",
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{},
		},
	})
	
	transport := &mockTransport{}
	server.SetTransport(transport)
	
	// Create initialize request
	req := types.InitializeRequest{
		ProtocolVersion: "1.0",
		Capabilities:    types.ClientCapabilities{},
		ClientInfo: types.Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	
	reqBytes, _ := json.Marshal(req)
	idBytes, _ := json.Marshal(1)
	idRaw := json.RawMessage(idBytes)
	
	msg := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "initialize",
		Params:  reqBytes,
	}
	
	// Handle request
	resp, err := server.HandleRequest(context.Background(), msg)
	if err != nil {
		t.Fatalf("Failed to handle initialize request: %v", err)
	}
	
	if resp.Error != nil {
		t.Fatalf("Initialize returned error: %v", resp.Error)
	}
	
	// Verify response
	var result types.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", result.ServerInfo.Name)
	}
	
	if result.ServerInfo.Version != "1.0.0" {
		t.Errorf("Expected server version '1.0.0', got '%s'", result.ServerInfo.Version)
	}
	
	if result.ProtocolVersion != "1.0" {
		t.Errorf("Expected protocol version '1.0', got '%s'", result.ProtocolVersion)
	}
}

type mockResourceHandler struct {
	resources []types.Resource
}

func (m *mockResourceHandler) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	return &types.ListResourcesResult{
		Resources: m.resources,
	}, nil
}

func (m *mockResourceHandler) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	return &types.ReadResourceResult{
		Contents: []interface{}{
			types.ResourceContentsText{
				URI:  req.URI,
				Text: "test content",
			},
		},
	}, nil
}

func (m *mockResourceHandler) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (m *mockResourceHandler) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

func TestServerResources(t *testing.T) {
	handler := &mockResourceHandler{
		resources: []types.Resource{
			{URI: "test://resource1", Name: "Resource 1"},
			{URI: "test://resource2", Name: "Resource 2"},
		},
	}
	
	server := NewServer(ServerOptions{
		Name:            "test-server",
		Version:         "1.0.0",
		ResourceHandler: handler,
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{},
		},
	})
	
	// Test list resources
	idBytes, _ := json.Marshal(1)
	idRaw := json.RawMessage(idBytes)
	
	msg := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "resources/list",
	}
	
	resp, err := server.HandleRequest(context.Background(), msg)
	if err != nil {
		t.Fatalf("Failed to handle list resources: %v", err)
	}
	
	var result types.ListResourcesResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	if len(result.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(result.Resources))
	}
	
	// Test read resource
	req := types.ReadResourceRequest{URI: "test://resource1"}
	reqBytes, _ := json.Marshal(req)
	
	msg = &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  "resources/read",
		Params:  reqBytes,
	}
	
	resp, err = server.HandleRequest(context.Background(), msg)
	if err != nil {
		t.Fatalf("Failed to handle read resource: %v", err)
	}
	
	var readResult types.ReadResourceResult
	if err := json.Unmarshal(resp.Result, &readResult); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	if len(readResult.Contents) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(readResult.Contents))
	}
}