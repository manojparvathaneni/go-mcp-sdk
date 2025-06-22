package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

func TestStdioTransport_Send(t *testing.T) {
	var output bytes.Buffer
	transport := NewStdioTransportWithStreams(nil, &output)

	id := json.RawMessage("1")
	msg := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test",
	}

	err := transport.Send(msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Check that output contains the JSON message
	result := output.String()
	if !strings.Contains(result, `"jsonrpc":"2.0"`) {
		t.Errorf("Output missing jsonrpc field: %s", result)
	}
	if !strings.Contains(result, `"method":"test"`) {
		t.Errorf("Output missing method field: %s", result)
	}
}

func TestStdioTransport_Receive(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}` + "\n"
	transport := NewStdioTransportWithStreams(strings.NewReader(input), nil)

	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if msg.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", msg.JSONRPC)
	}
	if msg.Method != "test" {
		t.Errorf("Expected method test, got %s", msg.Method)
	}
}

func TestStdioTransport_ReceiveInvalidJSON(t *testing.T) {
	input := `{invalid json}`
	transport := NewStdioTransportWithStreams(strings.NewReader(input), nil)

	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestStdioTransport_ReceiveEOF(t *testing.T) {
	transport := NewStdioTransportWithStreams(strings.NewReader(""), nil)

	_, err := transport.Receive()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestStdioTransport_Close(t *testing.T) {
	transport := NewStdioTransport()
	
	err := transport.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

func TestStdioTransport_ConnectionID(t *testing.T) {
	transport := NewStdioTransport()
	
	id := transport.ConnectionID()
	if id == "" {
		t.Error("Connection ID should not be empty")
	}
	if len(id) != 36 { // UUID length
		t.Errorf("Expected UUID format connection ID, got %s (length %d)", id, len(id))
	}
}