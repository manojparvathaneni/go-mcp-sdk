package mcp

import (
	"context"
	"fmt"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/events"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/server"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/tools"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// MCPServer provides a high-level, Go-idiomatic API for creating MCP servers
// Similar to how gin.Engine works in the Gin web framework
type MCPServer struct {
	server          *server.Server
	resources       map[string]ResourceFunc
	tools           map[string]ToolFunc
	toolsMetadata   map[string]ToolMetadata
	prompts         map[string]PromptFunc
	resourceTemplates map[string]types.ResourceTemplate
	eventEmitter    *events.EventEmitter
}

// ToolMetadata stores metadata about a tool including its schemas
type ToolMetadata struct {
	Name         string
	Description  string
	InputSchema  interface{}
	OutputSchema *map[string]interface{}
}

// ResourceFunc, ToolFunc, and PromptFunc are defined in common_types.go

type MCPServerOptions struct {
	Name    string
	Version string
}

// Option represents a functional option for configuring MCPServer
type Option func(*MCPServer) error

// ResourceConfig represents a resource configuration
type ResourceConfig struct {
	URI         string
	Description string
	MimeType    string
	Handler     ResourceFunc
}

// ToolConfig represents a tool configuration
type ToolConfig struct {
	Name        string
	Description string
	InputSchema interface{}
	Handler     ToolFunc
}

// PromptConfig represents a prompt configuration
type PromptConfig struct {
	Name        string
	Description string
	Arguments   []types.PromptArgument
	Handler     PromptFunc
}

// NewMCPServer creates a new MCP server builder
// This is the main entry point, similar to gin.New() or echo.New()
func NewMCPServer(opts MCPServerOptions) *MCPServer {
	s := &MCPServer{
		resources:         make(map[string]ResourceFunc),
		tools:            make(map[string]ToolFunc),
		toolsMetadata:    make(map[string]ToolMetadata),
		prompts:          make(map[string]PromptFunc),
		resourceTemplates: make(map[string]types.ResourceTemplate),
		eventEmitter:     events.NewEventEmitter(),
	}
	
	serverOpts := server.ServerOptions{
		Name:    opts.Name,
		Version: opts.Version,
		Capabilities: types.ServerCapabilities{
			Resources: &types.ResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
			Tools: &types.ToolsCapability{
				ListChanged: true,
			},
			Prompts: &types.PromptsCapability{
				ListChanged: true,
			},
			Sampling: &types.SamplingCapability{},
			Roots: &types.RootsCapability{
				ListChanged: true,
			},
		},
		ResourceHandler: s,
		ToolHandler:     s,
		PromptHandler:   s,
	}
	
	s.server = server.NewServer(serverOpts)
	return s
}

// NewMCPServerWithOptions creates a new MCP server using functional options pattern
// Example: NewMCPServerWithOptions("MyServer", "1.0.0", WithResource(...), WithTool(...))
func NewMCPServerWithOptions(name, version string, options ...Option) (*MCPServer, error) {
	s := NewMCPServer(MCPServerOptions{
		Name:    name,
		Version: version,
	})
	
	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	
	return s, nil
}

// WithResource adds a resource using functional options pattern
func WithResource(uri, description, mimeType string, handler ResourceFunc) Option {
	return func(s *MCPServer) error {
		s.Resource(uri, description, mimeType, handler)
		return nil
	}
}

// WithTool adds a tool using functional options pattern
func WithTool(name, description string, inputSchema interface{}, handler ToolFunc) Option {
	return func(s *MCPServer) error {
		s.Tool(name, description, inputSchema, handler)
		return nil
	}
}

// WithPrompt adds a prompt using functional options pattern
func WithPrompt(name, description string, arguments []types.PromptArgument, handler PromptFunc) Option {
	return func(s *MCPServer) error {
		s.Prompt(name, description, arguments, handler)
		return nil
	}
}

// WithResourceTemplate adds a resource template using functional options pattern
func WithResourceTemplate(uriTemplate, name, description, mimeType string) Option {
	return func(s *MCPServer) error {
		s.ResourceTemplate(uriTemplate, name, description, mimeType)
		return nil
	}
}

// WithEventHook adds an event hook using functional options pattern
func WithEventHook(eventType events.EventType, hook events.EventHook) Option {
	return func(s *MCPServer) error {
		if s.eventEmitter == nil {
			s.eventEmitter = events.NewEventEmitter()
		}
		s.eventEmitter.On(eventType, hook)
		return nil
	}
}

// Server returns the underlying server instance
func (s *MCPServer) Server() *server.Server {
	return s.server
}

// Events returns the event emitter for registering hooks
func (s *MCPServer) Events() *events.EventEmitter {
	return s.eventEmitter
}

// Resource registers a resource handler using Go-style method chaining
func (s *MCPServer) Resource(uri string, description string, mimeType string, handler ResourceFunc) *MCPServer {
	s.resources[uri] = handler
	return s
}

// ResourceTemplate registers a resource template
func (s *MCPServer) ResourceTemplate(uriTemplate string, name string, description string, mimeType string) *MCPServer {
	s.resourceTemplates[uriTemplate] = types.ResourceTemplate{
		URITemplate: uriTemplate,
		Name:        name,
		Description: &description,
		MimeType:    &mimeType,
	}
	return s
}

// Tool registers a tool handler using Go-style method chaining
func (s *MCPServer) Tool(name string, description string, inputSchema interface{}, handler ToolFunc) *MCPServer {
	s.tools[name] = handler
	s.toolsMetadata[name] = ToolMetadata{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
	return s
}

// StructuredTool registers a structured tool handler with output schema
func (s *MCPServer) StructuredTool(name string, description string, inputSchema interface{}, outputSchema map[string]interface{}, handler tools.StructuredToolFunc) *MCPServer {
	wrappedHandler := tools.StructuredToolWrapper(handler, &outputSchema)
	s.tools[name] = wrappedHandler
	s.toolsMetadata[name] = ToolMetadata{
		Name:         name,
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: &outputSchema,
	}
	return s
}

// Prompt registers a prompt handler using Go-style method chaining
func (s *MCPServer) Prompt(name string, description string, arguments []types.PromptArgument, handler PromptFunc) *MCPServer {
	s.prompts[name] = handler
	return s
}

// Group creates a route group (similar to gin.Group)
type RouteGroup struct {
	prefix string
	server *MCPServer
}

// Group creates a new route group with a common prefix
func (s *MCPServer) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		prefix: prefix,
		server: s,
	}
}

// Resource adds a resource to the group with the prefix
func (g *RouteGroup) Resource(uri string, description string, mimeType string, handler ResourceFunc) *RouteGroup {
	fullURI := g.prefix + uri
	g.server.Resource(fullURI, description, mimeType, handler)
	return g
}

// Tool adds a tool to the group with the prefix
func (g *RouteGroup) Tool(name string, description string, inputSchema interface{}, handler ToolFunc) *RouteGroup {
	fullName := g.prefix + name
	g.server.Tool(fullName, description, inputSchema, handler)
	return g
}

// Implement handler interfaces

func (s *MCPServer) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	resources := make([]types.Resource, 0, len(s.resources))
	
	for uri := range s.resources {
		resources = append(resources, types.Resource{
			URI:  uri,
			Name: uri,
		})
	}
	
	result := &types.ListResourcesResult{
		Resources: resources,
	}
	
	// Emit event
	if s.eventEmitter != nil {
		s.eventEmitter.Emit(events.Event{
			Type:    events.EventResourceListed,
			Context: ctx,
			Data:    events.ResourceEventData("", result),
		})
	}
	
	return result, nil
}

func (s *MCPServer) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	handler, ok := s.resources[req.URI]
	if !ok {
		err := fmt.Errorf("resource not found: %s", req.URI)
		if s.eventEmitter != nil {
			s.eventEmitter.Emit(events.Event{
				Type:    events.EventError,
				Context: ctx,
				Data:    events.ErrorEventData(err.Error(), map[string]interface{}{"uri": req.URI}),
			})
		}
		return nil, err
	}
	
	result, err := handler(ctx, req.URI)
	
	// Emit event
	if s.eventEmitter != nil {
		s.eventEmitter.Emit(events.Event{
			Type:    events.EventResourceRead,
			Context: ctx,
			Data:    events.ResourceEventData(req.URI, result),
		})
	}
	
	return result, err
}

func (s *MCPServer) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (s *MCPServer) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

func (s *MCPServer) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	tools := make([]types.Tool, 0, len(s.tools))
	
	for name := range s.tools {
		// Get metadata for this tool
		metadata, exists := s.toolsMetadata[name]
		if !exists {
			// Fallback for tools registered without metadata
			schema := map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			}
			tools = append(tools, types.Tool{
				Name:        name,
				InputSchema: schema,
			})
			continue
		}
		
		// Convert inputSchema to map[string]interface{}
		var inputSchema map[string]interface{}
		if metadata.InputSchema != nil {
			if schemaMap, ok := metadata.InputSchema.(map[string]interface{}); ok {
				inputSchema = schemaMap
			} else {
				// Fallback schema
				inputSchema = map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				}
			}
		}
		
		tool := types.Tool{
			Name:         name,
			Description:  &metadata.Description,
			InputSchema:  inputSchema,
			OutputSchema: metadata.OutputSchema,
		}
		
		tools = append(tools, tool)
	}
	
	result := &types.ListToolsResult{
		Tools: tools,
	}
	
	// Emit event
	if s.eventEmitter != nil {
		s.eventEmitter.Emit(events.Event{
			Type:    events.EventToolListed,
			Context: ctx,
			Data:    events.ToolEventData("", nil, result),
		})
	}
	
	return result, nil
}

func (s *MCPServer) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	handler, ok := s.tools[req.Name]
	if !ok {
		err := fmt.Errorf("tool not found: %s", req.Name)
		if s.eventEmitter != nil {
			s.eventEmitter.Emit(events.Event{
				Type:    events.EventError,
				Context: ctx,
				Data:    events.ErrorEventData(err.Error(), map[string]interface{}{"name": req.Name}),
			})
		}
		return nil, err
	}
	
	result, err := handler(ctx, req.Arguments)
	
	// Emit event
	if s.eventEmitter != nil {
		s.eventEmitter.Emit(events.Event{
			Type:    events.EventToolCalled,
			Context: ctx,
			Data:    events.ToolEventData(req.Name, req.Arguments, result),
		})
	}
	
	return result, err
}

func (s *MCPServer) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	prompts := make([]types.Prompt, 0, len(s.prompts))
	
	for name := range s.prompts {
		prompts = append(prompts, types.Prompt{
			Name: name,
		})
	}
	
	return &types.ListPromptsResult{
		Prompts: prompts,
	}, nil
}

func (s *MCPServer) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	handler, ok := s.prompts[req.Name]
	if !ok {
		return nil, fmt.Errorf("prompt not found: %s", req.Name)
	}
	
	return handler(ctx, req.Arguments)
}
