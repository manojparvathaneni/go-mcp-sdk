package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

type Client struct {
	mu              sync.RWMutex
	transport       Transport
	capabilities    types.ClientCapabilities
	serverInfo      *types.Implementation
	serverCapabilities *types.ServerCapabilities
	
	requestID       atomic.Int64
	pendingRequests map[string]chan *types.JSONRPCMessage
	pendingMu       sync.Mutex
	
	notificationHandlers map[string]NotificationHandler
	handlersMu          sync.RWMutex
	
	ctx    context.Context
	cancel context.CancelFunc
	
	// For testing synchronization
	receiveLoopStarted chan struct{}
	receiveLoopOnce    sync.Once
}

type ClientOptions struct {
	Capabilities types.ClientCapabilities
}

type NotificationHandler func(ctx context.Context, params json.RawMessage) error

func NewClient(opts ClientOptions) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		capabilities:         opts.Capabilities,
		pendingRequests:     make(map[string]chan *types.JSONRPCMessage),
		notificationHandlers: make(map[string]NotificationHandler),
		ctx:                 ctx,
		cancel:              cancel,
		receiveLoopStarted:  make(chan struct{}),
	}
}

func (c *Client) SetTransport(transport Transport) {
	c.transport = transport
	go c.receiveLoop()
}

// WaitForReceiveLoop waits for the receive loop to start (useful for testing)
func (c *Client) WaitForReceiveLoop() {
	<-c.receiveLoopStarted
}

func (c *Client) Close() error {
	c.cancel()
	if c.transport != nil {
		return c.transport.Close()
	}
	return nil
}

func (c *Client) Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error) {
	req := types.InitializeRequest{
		ProtocolVersion: "1.0",
		Capabilities:    c.capabilities,
		ClientInfo:      clientInfo,
	}
	
	var result types.InitializeResult
	if err := c.sendRequest(ctx, "initialize", req, &result); err != nil {
		return nil, err
	}
	
	c.mu.Lock()
	c.serverInfo = &result.ServerInfo
	c.serverCapabilities = &result.Capabilities
	c.mu.Unlock()
	
	if err := c.sendNotification("initialized", types.InitializedNotification{}); err != nil {
		return nil, err
	}
	
	return &result, nil
}

func (c *Client) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	var result types.ListResourcesResult
	if err := c.sendRequest(ctx, "resources/list", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	var result types.ReadResourceResult
	if err := c.sendRequest(ctx, "resources/read", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	var result types.EmptyResult
	return c.sendRequest(ctx, "resources/subscribe", req, &result)
}

func (c *Client) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	var result types.EmptyResult
	return c.sendRequest(ctx, "resources/unsubscribe", req, &result)
}

func (c *Client) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	var result types.ListToolsResult
	if err := c.sendRequest(ctx, "tools/list", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	var result types.CallToolResult
	if err := c.sendRequest(ctx, "tools/call", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	var result types.ListPromptsResult
	if err := c.sendRequest(ctx, "prompts/list", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	var result types.GetPromptResult
	if err := c.sendRequest(ctx, "prompts/get", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SetLogLevel(ctx context.Context, req types.SetLogLevelRequest) error {
	var result types.EmptyResult
	return c.sendRequest(ctx, "logging/setLevel", req, &result)
}

func (c *Client) Complete(ctx context.Context, req types.CompleteRequest) (*types.CompleteResult, error) {
	var result types.CompleteResult
	if err := c.sendRequest(ctx, "completion/complete", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Ping sends a health check request
func (c *Client) Ping(ctx context.Context) error {
	var result types.EmptyResult
	return c.sendRequest(ctx, "ping", types.PingRequest{}, &result)
}

// CreateMessage sends a message creation request for LLM sampling
func (c *Client) CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error) {
	var result types.CreateMessageResult
	if err := c.sendRequest(ctx, "sampling/createMessage", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListRoots lists available filesystem roots
func (c *Client) ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error) {
	var result types.ListRootsResult
	if err := c.sendRequest(ctx, "roots/list", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendProgress sends a progress notification
func (c *Client) SendProgress(token types.ProgressToken, progress float64, total *float64) error {
	return c.sendNotification("notifications/progress", types.ProgressNotification{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
	})
}

// SendCancelled sends a cancellation notification
func (c *Client) SendCancelled(requestId json.RawMessage, reason *string) error {
	return c.sendNotification("notifications/cancelled", types.CancelledNotification{
		RequestId: requestId,
		Reason:    reason,
	})
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.notificationHandlers[method] = handler
}

func (c *Client) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.transport == nil {
		return fmt.Errorf("no transport set")
	}
	
	id := c.requestID.Add(1)
	idBytes, _ := json.Marshal(id)
	idRaw := json.RawMessage(idBytes)
	
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	
	msg := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &idRaw,
		Method:  method,
		Params:  paramsBytes,
	}
	
	respChan := make(chan *types.JSONRPCMessage, 1)
	
	c.pendingMu.Lock()
	c.pendingRequests[fmt.Sprintf("%d", id)] = respChan
	c.pendingMu.Unlock()
	
	if err := c.transport.Send(msg); err != nil {
		c.pendingMu.Lock()
		delete(c.pendingRequests, fmt.Sprintf("%d", id))
		c.pendingMu.Unlock()
		return err
	}
	
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil && resp.Result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pendingRequests, fmt.Sprintf("%d", id))
		c.pendingMu.Unlock()
		return ctx.Err()
	}
}

func (c *Client) sendNotification(method string, params interface{}) error {
	if c.transport == nil {
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
	
	return c.transport.Send(msg)
}

func (c *Client) receiveLoop() {
	// Signal that receive loop has started
	c.receiveLoopOnce.Do(func() {
		close(c.receiveLoopStarted)
	})
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		
		msg, err := c.transport.Receive()
		if err != nil {
			// Check if context is done to avoid busy loop on transport errors
			select {
			case <-c.ctx.Done():
				return
			default:
				continue
			}
		}
		
		if msg.ID != nil {
			var id int64
			if err := json.Unmarshal(*msg.ID, &id); err != nil {
				continue
			}
			
			c.pendingMu.Lock()
			ch, ok := c.pendingRequests[fmt.Sprintf("%d", id)]
			if ok {
				delete(c.pendingRequests, fmt.Sprintf("%d", id))
			}
			c.pendingMu.Unlock()
			
			if ok {
				ch <- msg
			}
		} else if msg.Method != "" {
			c.handleNotification(msg)
		}
	}
}

func (c *Client) handleNotification(msg *types.JSONRPCMessage) {
	c.handlersMu.RLock()
	handler, ok := c.notificationHandlers[msg.Method]
	c.handlersMu.RUnlock()
	
	if ok {
		go handler(context.Background(), msg.Params)
	}
}

func (c *Client) ServerInfo() *types.Implementation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

func (c *Client) ServerCapabilities() *types.ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverCapabilities
}

type Transport interface {
	Send(msg *types.JSONRPCMessage) error
	Receive() (*types.JSONRPCMessage, error)
	Close() error
}