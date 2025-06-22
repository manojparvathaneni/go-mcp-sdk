package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// RequestInterceptor can modify requests before they are handled
type RequestInterceptor func(ctx context.Context, req *types.JSONRPCMessage) (context.Context, error)

// ResponseInterceptor can modify responses before they are sent
type ResponseInterceptor func(ctx context.Context, req *types.JSONRPCMessage, resp *types.JSONRPCMessage, err error) error

type ResourceHandler interface {
	ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error)
	ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error)
	Subscribe(ctx context.Context, req types.SubscribeRequest) error
	Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error
}

type ToolHandler interface {
	ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error)
	CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error)
}

type PromptHandler interface {
	ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error)
	GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error)
}

// SamplingHandler handles message creation and sampling
type SamplingHandler interface {
	CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error)
}

// RootsHandler handles filesystem roots
type RootsHandler interface {
	ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error)
}

type Server struct {
	mu              sync.RWMutex
	info            types.Implementation
	capabilities    types.ServerCapabilities
	resourceHandler ResourceHandler
	toolHandler     ToolHandler
	promptHandler   PromptHandler
	samplingHandler SamplingHandler
	rootsHandler    RootsHandler
	initialized     bool
	transport       Transport
	logger          interface {
		Debug(msg string, fields ...interface{})
		Info(msg string, fields ...interface{})
		Warn(msg string, fields ...interface{})
		Error(msg string, fields ...interface{})
	}
	
	subscribers     map[string]map[string]bool
	subscribersMu   sync.RWMutex
	
	requestInterceptors  []RequestInterceptor
	responseInterceptors []ResponseInterceptor
	
	// Metrics
	totalRequests   uint64
	totalErrors     uint64
	activeSessions  int32
}

type ServerOptions struct {
	Name            string
	Version         string
	Capabilities    types.ServerCapabilities
	ResourceHandler ResourceHandler
	ToolHandler     ToolHandler
	PromptHandler   PromptHandler
	SamplingHandler SamplingHandler
	RootsHandler    RootsHandler
	Logger          interface {
		Debug(msg string, fields ...interface{})
		Info(msg string, fields ...interface{})
		Warn(msg string, fields ...interface{})
		Error(msg string, fields ...interface{})
	}
}

func NewServer(opts ServerOptions) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = &noOpLogger{}
	}
	
	return &Server{
		info: types.Implementation{
			Name:    opts.Name,
			Version: opts.Version,
		},
		capabilities:    opts.Capabilities,
		resourceHandler: opts.ResourceHandler,
		toolHandler:     opts.ToolHandler,
		promptHandler:   opts.PromptHandler,
		samplingHandler: opts.SamplingHandler,
		rootsHandler:    opts.RootsHandler,
		subscribers:     make(map[string]map[string]bool),
		logger:          logger,
	}
}

func (s *Server) SetTransport(transport Transport) {
	s.transport = transport
}

func (s *Server) SetCapabilities(capabilities types.ServerCapabilities) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.capabilities = capabilities
}

func (s *Server) HandleRequest(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	atomic.AddUint64(&s.totalRequests, 1)
	
	// Apply request interceptors
	var err error
	for _, interceptor := range s.requestInterceptors {
		ctx, err = interceptor(ctx, msg)
		if err != nil {
			s.logger.Error("Request interceptor failed", "error", err, "method", msg.Method)
			return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
		}
	}
	
	var resp *types.JSONRPCMessage
	switch msg.Method {
	case "initialize":
		resp, err = s.handleInitialize(ctx, msg)
	case "resources/list":
		resp, err = s.handleListResources(ctx, msg)
	case "resources/read":
		resp, err = s.handleReadResource(ctx, msg)
	case "resources/subscribe":
		resp, err = s.handleSubscribe(ctx, msg)
	case "resources/unsubscribe":
		resp, err = s.handleUnsubscribe(ctx, msg)
	case "tools/list":
		resp, err = s.handleListTools(ctx, msg)
	case "tools/call":
		resp, err = s.handleCallTool(ctx, msg)
	case "prompts/list":
		resp, err = s.handleListPrompts(ctx, msg)
	case "prompts/get":
		resp, err = s.handleGetPrompt(ctx, msg)
	case "logging/setLevel":
		resp, err = s.handleSetLogLevel(ctx, msg)
	case "completion/complete":
		resp, err = s.handleComplete(ctx, msg)
	case "sampling/createMessage":
		resp, err = s.handleCreateMessage(ctx, msg)
	case "ping":
		resp, err = s.handlePing(ctx, msg)
	case "roots/list":
		resp, err = s.handleListRoots(ctx, msg)
	default:
		resp = s.createErrorResponse(msg.ID, types.MethodNotFound, "Method not found", nil)
	}
	
	// Apply response interceptors
	for _, interceptor := range s.responseInterceptors {
		if err := interceptor(ctx, msg, resp, nil); err != nil {
			s.logger.Error("Response interceptor failed", "error", err, "method", msg.Method)
		}
	}
	
	if resp.Error != nil {
		atomic.AddUint64(&s.totalErrors, 1)
	}
	
	return resp, nil
}

func (s *Server) HandleNotification(ctx context.Context, msg *types.JSONRPCMessage) error {
	switch msg.Method {
	case "initialized":
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown notification method: %s", msg.Method)
	}
}

func (s *Server) handleInitialize(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	var req types.InitializeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result := types.InitializeResult{
		ProtocolVersion: "1.0",
		Capabilities:    s.capabilities,
		ServerInfo:      s.info,
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleListResources(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.resourceHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Resources not supported", nil), nil
	}
	
	var req types.ListResourcesRequest
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
		}
	}
	
	result, err := s.resourceHandler.ListResources(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleReadResource(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.resourceHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Resources not supported", nil), nil
	}
	
	var req types.ReadResourceRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result, err := s.resourceHandler.ReadResource(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleSubscribe(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.resourceHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Resources not supported", nil), nil
	}
	
	var req types.SubscribeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	s.subscribersMu.Lock()
	if s.subscribers[req.URI] == nil {
		s.subscribers[req.URI] = make(map[string]bool)
	}
	s.subscribers[req.URI][s.transport.ConnectionID()] = true
	s.subscribersMu.Unlock()
	
	if err := s.resourceHandler.Subscribe(ctx, req); err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, types.EmptyResult{}), nil
}

func (s *Server) handleUnsubscribe(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.resourceHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Resources not supported", nil), nil
	}
	
	var req types.UnsubscribeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	s.subscribersMu.Lock()
	if s.subscribers[req.URI] != nil {
		delete(s.subscribers[req.URI], s.transport.ConnectionID())
		if len(s.subscribers[req.URI]) == 0 {
			delete(s.subscribers, req.URI)
		}
	}
	s.subscribersMu.Unlock()
	
	if err := s.resourceHandler.Unsubscribe(ctx, req); err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, types.EmptyResult{}), nil
}

func (s *Server) handleListTools(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.toolHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Tools not supported", nil), nil
	}
	
	var req types.ListToolsRequest
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
		}
	}
	
	result, err := s.toolHandler.ListTools(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleCallTool(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.toolHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Tools not supported", nil), nil
	}
	
	var req types.CallToolRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result, err := s.toolHandler.CallTool(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleListPrompts(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.promptHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Prompts not supported", nil), nil
	}
	
	var req types.ListPromptsRequest
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
		}
	}
	
	result, err := s.promptHandler.ListPrompts(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleGetPrompt(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.promptHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Prompts not supported", nil), nil
	}
	
	var req types.GetPromptRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result, err := s.promptHandler.GetPrompt(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handleSetLogLevel(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	var req types.SetLogLevelRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, types.EmptyResult{}), nil
}

func (s *Server) handleComplete(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	var req types.CompleteRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result := types.CompleteResult{
		Completion: struct {
			Values   []types.ArgumentCompletionValue `json:"values"`
			Total    *int                      `json:"total,omitempty"`
			HasMore  bool                      `json:"hasMore,omitempty"`
		}{
			Values: []types.ArgumentCompletionValue{},
		},
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) createSuccessResponse(id *json.RawMessage, result interface{}) *types.JSONRPCMessage {
	resultBytes, _ := json.Marshal(result)
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultBytes,
	}
}

func (s *Server) createErrorResponse(id *json.RawMessage, code int, message string, data interface{}) *types.JSONRPCMessage {
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   types.NewJSONRPCError(code, message, data),
	}
}

func (s *Server) SendNotification(method string, params interface{}) error {
	if s.transport == nil {
		return fmt.Errorf("no transport set")
	}
	
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	
	msg := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	
	return s.transport.Send(msg)
}

func (s *Server) NotifyResourceUpdated(uri string) error {
	return s.SendNotification("notifications/resources/updated", types.ResourceUpdatedNotification{URI: uri})
}

func (s *Server) NotifyResourceListChanged() error {
	return s.SendNotification("notifications/resources/list_changed", types.ResourceListChangedNotification{})
}

func (s *Server) NotifyToolListChanged() error {
	return s.SendNotification("notifications/tools/list_changed", types.ToolListChangedNotification{})
}

func (s *Server) NotifyPromptListChanged() error {
	return s.SendNotification("notifications/prompts/list_changed", types.PromptListChangedNotification{})
}

type Transport interface {
	Send(msg *types.JSONRPCMessage) error
	Receive() (*types.JSONRPCMessage, error)
	Close() error
	ConnectionID() string
}

// noOpLogger is a logger that does nothing
type noOpLogger struct{}

func (l *noOpLogger) Debug(msg string, fields ...interface{}) {}
func (l *noOpLogger) Info(msg string, fields ...interface{}) {}
func (l *noOpLogger) Warn(msg string, fields ...interface{}) {}
func (l *noOpLogger) Error(msg string, fields ...interface{}) {}

// AddRequestInterceptor adds a request interceptor
func (s *Server) AddRequestInterceptor(interceptor RequestInterceptor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestInterceptors = append(s.requestInterceptors, interceptor)
}

// AddResponseInterceptor adds a response interceptor
func (s *Server) AddResponseInterceptor(interceptor ResponseInterceptor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responseInterceptors = append(s.responseInterceptors, interceptor)
}

// GetMetrics returns server metrics
func (s *Server) GetMetrics() ServerMetrics {
	return ServerMetrics{
		TotalRequests:  atomic.LoadUint64(&s.totalRequests),
		TotalErrors:    atomic.LoadUint64(&s.totalErrors),
		ActiveSessions: atomic.LoadInt32(&s.activeSessions),
	}
}

// ServerMetrics contains server performance metrics
type ServerMetrics struct {
	TotalRequests  uint64
	TotalErrors    uint64
	ActiveSessions int32
}

// Additional required MCP methods

func (s *Server) handleCreateMessage(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.samplingHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Sampling not supported", nil), nil
	}
	
	var req types.CreateMessageRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
	}
	
	result, err := s.samplingHandler.CreateMessage(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

func (s *Server) handlePing(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	// Ping is a simple health check that returns empty result
	return s.createSuccessResponse(msg.ID, types.EmptyResult{}), nil
}

func (s *Server) handleListRoots(ctx context.Context, msg *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if s.rootsHandler == nil {
		return s.createErrorResponse(msg.ID, types.MethodNotFound, "Roots not supported", nil), nil
	}
	
	var req types.ListRootsRequest
	if msg.Params != nil {
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return s.createErrorResponse(msg.ID, types.InvalidParams, "Invalid params", nil), nil
		}
	}
	
	result, err := s.rootsHandler.ListRoots(ctx, req)
	if err != nil {
		return s.createErrorResponse(msg.ID, types.InternalError, err.Error(), nil), nil
	}
	
	return s.createSuccessResponse(msg.ID, result), nil
}

// SendProgress sends a progress notification
func (s *Server) SendProgress(token types.ProgressToken, progress float64, total *float64) error {
	return s.SendNotification("notifications/progress", types.ProgressNotification{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
	})
}

// SendCancelled sends a cancellation notification
func (s *Server) SendCancelled(requestId json.RawMessage, reason *string) error {
	return s.SendNotification("notifications/cancelled", types.CancelledNotification{
		RequestId: requestId,
		Reason:    reason,
	})
}