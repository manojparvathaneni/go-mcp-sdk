package types

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCMessage_Marshal(t *testing.T) {
	id := json.RawMessage(`"test-id"`)
	msg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
		Params:  json.RawMessage(`{"param": "value"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal JSONRPCMessage: %v", err)
	}

	var unmarshaled JSONRPCMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSONRPCMessage: %v", err)
	}

	if unmarshaled.JSONRPC != msg.JSONRPC {
		t.Errorf("Expected JSONRPC %s, got %s", msg.JSONRPC, unmarshaled.JSONRPC)
	}
	if unmarshaled.Method != msg.Method {
		t.Errorf("Expected Method %s, got %s", msg.Method, unmarshaled.Method)
	}
}

func TestJSONRPCMessage_WithError(t *testing.T) {
	id := json.RawMessage(`"test-id"`)
	msg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Error: &JSONRPCError{
			Code:    InvalidRequest,
			Message: "Invalid request",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal JSONRPCMessage with error: %v", err)
	}

	var unmarshaled JSONRPCMessage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSONRPCMessage with error: %v", err)
	}

	if unmarshaled.Error == nil {
		t.Fatal("Expected Error to be set")
	}
	if unmarshaled.Error.Code != InvalidRequest {
		t.Errorf("Expected error code %d, got %d", InvalidRequest, unmarshaled.Error.Code)
	}
}

func TestNewJSONRPCError(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	err := NewJSONRPCError(InternalError, "Something went wrong", data)

	if err.Code != InternalError {
		t.Errorf("Expected code %d, got %d", InternalError, err.Code)
	}
	if err.Message != "Something went wrong" {
		t.Errorf("Expected message 'Something went wrong', got %s", err.Message)
	}
	if err.Data == nil {
		t.Error("Expected Data to be set")
	}

	var unmarshaled map[string]interface{}
	jsonErr := json.Unmarshal(err.Data, &unmarshaled)
	if jsonErr != nil {
		t.Fatalf("Failed to unmarshal error data: %v", jsonErr)
	}
	if unmarshaled["key"] != "value" {
		t.Error("Expected Data to contain correct values")
	}
}

func TestNewJSONRPCError_NilData(t *testing.T) {
	err := NewJSONRPCError(ParseError, "Parse error", nil)

	if err.Code != ParseError {
		t.Errorf("Expected code %d, got %d", ParseError, err.Code)
	}
	if err.Data != nil {
		t.Error("Expected Data to be nil")
	}
}

func TestMcpError_Error(t *testing.T) {
	err := &McpError{
		Code:    "test_error",
		Message: "Test error message",
		Data:    map[string]interface{}{"detail": "extra info"},
	}

	expected := "MCP Error test_error: Test error message"
	if err.Error() != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestImplementation_Marshal(t *testing.T) {
	impl := Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	data, err := json.Marshal(impl)
	if err != nil {
		t.Fatalf("Failed to marshal Implementation: %v", err)
	}

	var unmarshaled Implementation
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Implementation: %v", err)
	}

	if unmarshaled.Name != impl.Name {
		t.Errorf("Expected name %s, got %s", impl.Name, unmarshaled.Name)
	}
	if unmarshaled.Version != impl.Version {
		t.Errorf("Expected version %s, got %s", impl.Version, unmarshaled.Version)
	}
}

func TestClientCapabilities_Marshal(t *testing.T) {
	caps := ClientCapabilities{
		Experimental: map[string]interface{}{"feature": "enabled"},
		Sampling:     &SamplingCapability{},
		Roots: &RootsCapability{
			ListChanged: true,
		},
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("Failed to marshal ClientCapabilities: %v", err)
	}

	var unmarshaled ClientCapabilities
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ClientCapabilities: %v", err)
	}

	if unmarshaled.Experimental["feature"] != "enabled" {
		t.Error("Expected Experimental feature to be preserved")
	}
	if unmarshaled.Sampling == nil {
		t.Error("Expected Sampling capability to be set")
	}
	if unmarshaled.Roots == nil || !unmarshaled.Roots.ListChanged {
		t.Error("Expected Roots capability with ListChanged=true")
	}
}

func TestServerCapabilities_Marshal(t *testing.T) {
	caps := ServerCapabilities{
		Logging: &LoggingCapability{},
		Resources: &ResourcesCapability{
			Subscribe:   true,
			ListChanged: false,
		},
		Tools: &ToolsCapability{
			ListChanged: true,
		},
	}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("Failed to marshal ServerCapabilities: %v", err)
	}

	var unmarshaled ServerCapabilities
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServerCapabilities: %v", err)
	}

	if unmarshaled.Logging == nil {
		t.Error("Expected Logging capability to be set")
	}
	if unmarshaled.Resources == nil || !unmarshaled.Resources.Subscribe {
		t.Error("Expected Resources capability with Subscribe=true")
	}
	if unmarshaled.Tools == nil || !unmarshaled.Tools.ListChanged {
		t.Error("Expected Tools capability with ListChanged=true")
	}
}

func TestInitializeRequest_Marshal(t *testing.T) {
	req := InitializeRequest{
		ProtocolVersion: "1.0",
		Capabilities: ClientCapabilities{
			Sampling: &SamplingCapability{},
		},
		ClientInfo: Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeRequest: %v", err)
	}

	var unmarshaled InitializeRequest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeRequest: %v", err)
	}

	if unmarshaled.ProtocolVersion != req.ProtocolVersion {
		t.Error("Expected ProtocolVersion to be preserved")
	}
	if unmarshaled.ClientInfo.Name != req.ClientInfo.Name {
		t.Error("Expected ClientInfo to be preserved")
	}
}

func TestInitializeResult_Marshal(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "1.0",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: true},
		},
		ServerInfo: Implementation{
			Name:    "test-server",
			Version: "2.0.0",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeResult: %v", err)
	}

	var unmarshaled InitializeResult
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if unmarshaled.ServerInfo.Name != result.ServerInfo.Name {
		t.Error("Expected ServerInfo to be preserved")
	}
}

func TestLogLevel_Constants(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "debug"},
		{LogLevelInfo, "info"},
		{LogLevelWarning, "warning"},
		{LogLevelError, "error"},
	}

	for _, tt := range tests {
		if string(tt.level) != tt.expected {
			t.Errorf("Expected LogLevel %s to equal %s", tt.level, tt.expected)
		}
	}
}

func TestRole_Constants(t *testing.T) {
	if string(RoleAssistant) != "assistant" {
		t.Errorf("Expected RoleAssistant to be 'assistant', got %s", RoleAssistant)
	}
	if string(RoleUser) != "user" {
		t.Errorf("Expected RoleUser to be 'user', got %s", RoleUser)
	}
}

func TestTextContent_Marshal(t *testing.T) {
	content := TextContent{
		Type: "text",
		Text: "Hello, world!",
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal TextContent: %v", err)
	}

	var unmarshaled TextContent
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal TextContent: %v", err)
	}

	if unmarshaled.Type != content.Type {
		t.Error("Expected Type to be preserved")
	}
	if unmarshaled.Text != content.Text {
		t.Error("Expected Text to be preserved")
	}
}

func TestImageContent_Marshal(t *testing.T) {
	content := ImageContent{
		Type:     "image",
		Data:     "base64data",
		MimeType: "image/png",
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal ImageContent: %v", err)
	}

	var unmarshaled ImageContent
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ImageContent: %v", err)
	}

	if unmarshaled.MimeType != content.MimeType {
		t.Error("Expected MimeType to be preserved")
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		constant int
		expected int
	}{
		{ParseError, -32700},
		{InvalidRequest, -32600},
		{MethodNotFound, -32601},
		{InvalidParams, -32602},
		{InternalError, -32603},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Expected constant %d, got %d", tt.expected, tt.constant)
		}
	}
}

func TestResourceReference_Marshal(t *testing.T) {
	ref := ResourceReference{
		Type: "resource",
		URI:  "file://test.txt",
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Failed to marshal ResourceReference: %v", err)
	}

	var unmarshaled ResourceReference
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ResourceReference: %v", err)
	}

	if unmarshaled.Type != ref.Type {
		t.Error("Expected Type to be preserved")
	}
	if unmarshaled.URI != ref.URI {
		t.Error("Expected URI to be preserved")
	}
}

func TestSetLogLevelRequest_Marshal(t *testing.T) {
	req := SetLogLevelRequest{
		Level: LogLevelError,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal SetLogLevelRequest: %v", err)
	}

	var unmarshaled SetLogLevelRequest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SetLogLevelRequest: %v", err)
	}

	if unmarshaled.Level != req.Level {
		t.Error("Expected Level to be preserved")
	}
}