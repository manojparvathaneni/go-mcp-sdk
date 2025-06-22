package mcp

import (
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Common function types are re-exported from the types package

// ResourceFunc is a function that handles resource read requests
type ResourceFunc = types.ResourceFunc

// ToolFunc is a function that handles tool call requests
type ToolFunc = types.ToolFunc

// PromptFunc is a function that handles prompt get requests
type PromptFunc = types.PromptFunc