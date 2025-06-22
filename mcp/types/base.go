// Package types provides the core type definitions for the MCP protocol
package types

import (
	"context"
	"encoding/json"
	"fmt"
)

type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

func NewJSONRPCError(code int, message string, data interface{}) *JSONRPCError {
	var dataBytes json.RawMessage
	if data != nil {
		dataBytes, _ = json.Marshal(data)
	}
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    dataBytes,
	}
}

type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  *JSONRPCError   `json:"error,omitempty"`
}

type McpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *McpError) Error() string {
	return fmt.Sprintf("MCP Error %s: %s", e.Code, e.Message)
}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
	Roots        *RootsCapability       `json:"roots,omitempty"`
	Elicitation  *ElicitationCapability `json:"elicitation,omitempty"`
}

type ServerCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *LoggingCapability     `json:"logging,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
	Roots        *RootsCapability       `json:"roots,omitempty"`
	Elicitation  *ElicitationCapability `json:"elicitation,omitempty"`
}

type SamplingCapability struct{}
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
type LoggingCapability struct{}
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

type InitializedNotification struct{}

type PaginationOptions struct {
	Cursor *string `json:"cursor,omitempty"`
}

type ResourceReference struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

type Role string

const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
)

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

type AudioContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

// ResourceContents is an interface for resource content types
type ResourceContents interface {
	isResourceContents()
}

type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

type LoggingMessageNotification struct {
	Level   LogLevel    `json:"level"`
	Logger  *string     `json:"logger,omitempty"`
	Data    interface{} `json:"data"`
}

type SetLogLevelRequest struct {
	Level LogLevel `json:"level"`
}

// Common function types used across the SDK

// ResourceFunc is a function that handles resource read requests
type ResourceFunc func(ctx context.Context, uri string) (*ReadResourceResult, error)

// ToolFunc is a function that handles tool call requests
type ToolFunc func(ctx context.Context, arguments json.RawMessage) (*CallToolResult, error)

// PromptFunc is a function that handles prompt get requests
type PromptFunc func(ctx context.Context, arguments map[string]interface{}) (*GetPromptResult, error)

