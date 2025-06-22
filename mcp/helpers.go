package mcp

import (
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Helper functions for creating content

// TextContent creates a text content item
func TextContent(text string) types.TextContent {
	return types.TextContent{
		Type: "text",
		Text: text,
	}
}

// ResourceTextContent creates a text resource content with the given URI
func ResourceTextContent(uri string, text string) types.ResourceContentsText {
	mimeType := "text/plain"
	return types.ResourceContentsText{
		URI:      uri,
		MimeType: &mimeType,
		Text:     text,
	}
}

// ResourceBlobContent creates a blob resource content with the given URI
func ResourceBlobContent(uri string, blob string, mimeType string) types.ResourceContentsBlob {
	return types.ResourceContentsBlob{
		URI:      uri,
		MimeType: &mimeType,
		Blob:     blob,
	}
}

// ToolResult creates a successful tool result
func ToolResult(content ...interface{}) *types.CallToolResult {
	return &types.CallToolResult{
		Content: content,
		IsError: false,
	}
}

// ToolError creates an error tool result
func ToolError(message string) *types.CallToolResult {
	return &types.CallToolResult{
		Content: []interface{}{TextContent(message)},
		IsError: true,
	}
}

// ResourceResult creates a resource result with the given contents
func ResourceResult(contents ...types.ResourceContents) *types.ReadResourceResult {
	// Convert to []interface{}
	result := make([]interface{}, len(contents))
	for i, content := range contents {
		result[i] = content
	}
	return &types.ReadResourceResult{
		Contents: result,
	}
}

// PromptResult creates a prompt result with the given messages
func PromptResult(messages ...types.PromptMessage) *types.GetPromptResult {
	// Convert to []interface{}
	result := make([]interface{}, len(messages))
	for i, message := range messages {
		result[i] = message
	}
	return &types.GetPromptResult{
		Messages: result,
	}
}

// UserMessage creates a user prompt message
func UserMessage(content ...interface{}) types.PromptMessage {
	return types.PromptMessage{
		Role:    types.RoleUser,
		Content: content,
	}
}

// AssistantMessage creates an assistant prompt message
func AssistantMessage(content ...interface{}) types.PromptMessage {
	return types.PromptMessage{
		Role:    types.RoleAssistant,
		Content: content,
	}
}

// ImageContent creates an image content item
func ImageContent(data string, mimeType string) types.ImageContent {
	return types.ImageContent{
		Type:     "image",
		Data:     data,
		MimeType: mimeType,
	}
}