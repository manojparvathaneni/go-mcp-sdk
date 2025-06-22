package connection

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock transport for testing
type mockTransport struct {
	closed        bool
	shouldFail    bool
	initializeErr error
	resourcesErr  error
}

func (t *mockTransport) Send(msg *types.JSONRPCMessage) error {
	if t.shouldFail {
		return errors.New("transport send failed")
	}
	return nil
}

func (t *mockTransport) Receive() (*types.JSONRPCMessage, error) {
	if t.shouldFail {
		return nil, errors.New("transport receive failed")
	}
	// Return a basic response for initialization
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		Result:  []byte(`{"protocolVersion": "1.0", "capabilities": {}, "serverInfo": {"name": "test", "version": "1.0"}}`),
	}, nil
}

func (t *mockTransport) Close() error {
	t.closed = true
	return nil
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{ConnectionStateDisconnected, "disconnected"},
		{ConnectionStateConnecting, "connecting"},
		{ConnectionStateConnected, "connected"},
		{ConnectionStateReconnecting, "reconnecting"},
		{ConnectionState(999), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}

func TestDefaultConnectionOptions(t *testing.T) {
	opts := DefaultConnectionOptions()

	if opts.MaxReconnectAttempts != 5 {
		t.Errorf("Expected MaxReconnectAttempts 5, got %d", opts.MaxReconnectAttempts)
	}
	if opts.ReconnectDelay != time.Second {
		t.Errorf("Expected ReconnectDelay 1s, got %v", opts.ReconnectDelay)
	}
	if opts.MaxReconnectDelay != 30*time.Second {
		t.Errorf("Expected MaxReconnectDelay 30s, got %v", opts.MaxReconnectDelay)
	}
	if opts.ConnectionTimeout != 10*time.Second {
		t.Errorf("Expected ConnectionTimeout 10s, got %v", opts.ConnectionTimeout)
	}
	if opts.PingInterval != 30*time.Second {
		t.Errorf("Expected PingInterval 30s, got %v", opts.PingInterval)
	}
	if opts.PingTimeout != 5*time.Second {
		t.Errorf("Expected PingTimeout 5s, got %v", opts.PingTimeout)
	}
}

func TestNewConnectionManager(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	if cm == nil {
		t.Fatal("Expected NewConnectionManager to return non-nil")
	}
	if cm.State() != ConnectionStateDisconnected {
		t.Error("Expected initial state to be disconnected")
	}
	if cm.client == nil {
		t.Error("Expected client to be initialized")
	}
	if cm.transportFactory == nil {
		t.Error("Expected transport factory to be set")
	}
}

func TestConnectionManager_Connect_Success(t *testing.T) {
	// Create a custom mock client that can handle initialization
	transport := &mockTransport{}
	transportFactory := func() (client.Transport, error) {
		return transport, nil
	}

	clientInfo := types.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	options := DefaultConnectionOptions()
	options.PingInterval = 0 // Disable ping for this test
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	// Mock the client initialization to avoid the need for a real server
	originalClient := cm.client
	mockClient := &mockClient{
		initializeResult: &types.InitializeResult{
			ProtocolVersion: "1.0",
			ServerInfo: types.Implementation{
				Name:    "test-server",
				Version: "1.0.0",
			},
		},
	}
	cm.client = mockClient

	err := cm.Connect()
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if cm.State() != ConnectionStateConnected {
		t.Errorf("Expected state connected, got %s", cm.State())
	}

	// Restore original client
	cm.client = originalClient
}

func TestConnectionManager_Connect_TransportError(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return nil, errors.New("transport creation failed")
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	err := cm.Connect()
	if err == nil {
		t.Error("Expected connect to fail with transport error")
	}

	if cm.State() != ConnectionStateDisconnected {
		t.Error("Expected state to remain disconnected on error")
	}
}

func TestConnectionManager_Connect_AlreadyConnected(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	// Manually set state to connected
	cm.setState(ConnectionStateConnected)

	err := cm.Connect()
	if err == nil {
		t.Error("Expected error when connecting while already connected")
	}
}

func TestConnectionManager_Disconnect(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	// Set state to connected
	cm.setState(ConnectionStateConnected)

	err := cm.Disconnect()
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	if cm.State() != ConnectionStateDisconnected {
		t.Error("Expected state to be disconnected")
	}
}

func TestConnectionManager_IsConnected(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	if cm.IsConnected() {
		t.Error("Expected IsConnected to be false initially")
	}

	cm.setState(ConnectionStateConnected)
	if !cm.IsConnected() {
		t.Error("Expected IsConnected to be true when connected")
	}

	cm.setState(ConnectionStateReconnecting)
	if cm.IsConnected() {
		t.Error("Expected IsConnected to be false when reconnecting")
	}
}

func TestConnectionManager_Callbacks(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	var stateChanges []ConnectionState
	var connected bool
	var disconnected bool
	var reconnectFailed bool

	cm.SetOnStateChange(func(oldState, newState ConnectionState) {
		stateChanges = append(stateChanges, newState)
	})

	cm.SetOnConnected(func() {
		connected = true
	})

	cm.SetOnDisconnected(func(err error) {
		disconnected = true
	})

	cm.SetOnReconnectFailed(func(err error, attempt int) {
		reconnectFailed = true
	})

	// Test state change callback
	cm.setState(ConnectionStateConnecting)
	cm.setState(ConnectionStateConnected)

	if len(stateChanges) != 2 {
		t.Errorf("Expected 2 state changes, got %d", len(stateChanges))
	}
	if stateChanges[0] != ConnectionStateConnecting {
		t.Error("Expected first state change to be connecting")
	}
	if stateChanges[1] != ConnectionStateConnected {
		t.Error("Expected second state change to be connected")
	}

	// Test connected callback (manually trigger)
	if cm.onConnected != nil {
		cm.onConnected()
	}
	if !connected {
		t.Error("Expected connected callback to be called")
	}

	// Test disconnected callback (manually trigger)
	if cm.onDisconnected != nil {
		cm.onDisconnected(errors.New("test error"))
	}
	if !disconnected {
		t.Error("Expected disconnected callback to be called")
	}

	// Test reconnect failed callback (manually trigger)
	if cm.onReconnectFailed != nil {
		cm.onReconnectFailed(errors.New("test error"), 1)
	}
	if !reconnectFailed {
		t.Error("Expected reconnect failed callback to be called")
	}
}

func TestConnectionManager_GetStats(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	cm.setState(ConnectionStateConnected)
	atomic.StoreInt32(&cm.reconnectAttempts, 3)
	cm.lastConnected = time.Now().Add(-5 * time.Minute)

	stats := cm.GetStats()

	if stats.State != ConnectionStateConnected {
		t.Error("Expected stats state to be connected")
	}
	if stats.ReconnectAttempts != 3 {
		t.Errorf("Expected 3 reconnect attempts, got %d", stats.ReconnectAttempts)
	}
	if stats.Uptime <= 0 {
		t.Error("Expected positive uptime")
	}
}

func TestConnectionManager_CalculateReconnectDelay(t *testing.T) {
	options := ConnectionOptions{
		ReconnectDelay:    time.Second,
		MaxReconnectDelay: 10 * time.Second,
	}

	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}
	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	cm := NewConnectionManager(transportFactory, clientInfo, options)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // Capped at max
		{6, 10 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		delay := cm.calculateReconnectDelay(tt.attempt)
		if delay != tt.expected {
			t.Errorf("Attempt %d: expected delay %v, got %v", tt.attempt, tt.expected, delay)
		}
	}
}

func TestNewConnectionPool(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	if pool == nil {
		t.Fatal("Expected NewConnectionPool to return non-nil")
	}
	if len(pool.connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(pool.connections))
	}
	if len(pool.healthy) != 2 {
		t.Errorf("Expected 2 health entries, got %d", len(pool.healthy))
	}
}

func TestConnectionPool_Connect(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()
	options.PingInterval = 0 // Disable ping

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	// Mock the clients to avoid real connections
	for _, mgr := range managers {
		mockClient := &mockClient{
			initializeResult: &types.InitializeResult{
				ProtocolVersion: "1.0",
				ServerInfo:      types.Implementation{Name: "test", Version: "1.0"},
			},
		}
		mgr.client = mockClient
	}

	pool := NewConnectionPool(managers)

	err := pool.Connect()
	if err != nil {
		t.Fatalf("Pool connect failed: %v", err)
	}
}

func TestConnectionPool_Connect_AllFail(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return nil, errors.New("transport failed")
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	err := pool.Connect()
	if err == nil {
		t.Error("Expected pool connect to fail when all connections fail")
	}
}

func TestConnectionPool_GetClient(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	// No healthy connections initially
	client := pool.GetClient()
	if client != nil {
		t.Error("Expected nil client when no connections are healthy")
	}

	// Mark first connection as healthy
	pool.healthy[0] = true

	client = pool.GetClient()
	if client == nil {
		t.Error("Expected non-nil client when connection is healthy")
	}
	if client != managers[0].Client() {
		t.Error("Expected client from first healthy connection")
	}

	// Mark both connections as healthy
	pool.healthy[1] = true

	// Test round-robin
	client1 := pool.GetClient()
	client2 := pool.GetClient()

	if client1 == client2 {
		t.Error("Expected different clients for round-robin selection")
	}
}

func TestConnectionPool_HealthyCount(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	// Initially no healthy connections
	count := pool.HealthyCount()
	if count != 0 {
		t.Errorf("Expected 0 healthy connections, got %d", count)
	}

	// Mark some connections as healthy
	pool.healthy[0] = true
	pool.healthy[2] = true

	count = pool.HealthyCount()
	if count != 2 {
		t.Errorf("Expected 2 healthy connections, got %d", count)
	}
}

func TestConnectionPool_GetStats(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	stats := pool.GetStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats entries, got %d", len(stats))
	}

	for _, stat := range stats {
		if stat.State != ConnectionStateDisconnected {
			t.Error("Expected initial state to be disconnected")
		}
	}
}

func TestConnectionPool_Disconnect(t *testing.T) {
	transportFactory := func() (client.Transport, error) {
		return &mockTransport{}, nil
	}

	clientInfo := types.Implementation{Name: "test", Version: "1.0"}
	options := DefaultConnectionOptions()

	managers := []*ConnectionManager{
		NewConnectionManager(transportFactory, clientInfo, options),
		NewConnectionManager(transportFactory, clientInfo, options),
	}

	pool := NewConnectionPool(managers)

	err := pool.Disconnect()
	if err != nil {
		t.Fatalf("Pool disconnect failed: %v", err)
	}

	// Verify all connections are disconnected
	for i, mgr := range managers {
		if mgr.State() != ConnectionStateDisconnected {
			t.Errorf("Expected connection %d to be disconnected", i)
		}
	}
}

// Mock client for testing
type mockClient struct {
	initializeResult *types.InitializeResult
	initializeError  error
	listResourcesErr error
}

func (c *mockClient) Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error) {
	if c.initializeError != nil {
		return nil, c.initializeError
	}
	return c.initializeResult, nil
}

func (c *mockClient) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	if c.listResourcesErr != nil {
		return nil, c.listResourcesErr
	}
	return &types.ListResourcesResult{}, nil
}

func (c *mockClient) Close() error {
	return nil
}

func (c *mockClient) SetTransport(transport client.Transport) {
	// No-op for mock
}

// Implement other required methods as no-ops for interface compliance
func (c *mockClient) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	return &types.ReadResourceResult{}, nil
}

func (c *mockClient) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (c *mockClient) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

func (c *mockClient) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{}, nil
}

func (c *mockClient) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	return &types.CallToolResult{}, nil
}

func (c *mockClient) ListPrompts(ctx context.Context, req types.ListPromptsRequest) (*types.ListPromptsResult, error) {
	return &types.ListPromptsResult{}, nil
}

func (c *mockClient) GetPrompt(ctx context.Context, req types.GetPromptRequest) (*types.GetPromptResult, error) {
	return &types.GetPromptResult{}, nil
}

func (c *mockClient) SetLogLevel(ctx context.Context, req types.SetLogLevelRequest) error {
	return nil
}

func (c *mockClient) Complete(ctx context.Context, req types.CompleteRequest) (*types.CompleteResult, error) {
	return &types.CompleteResult{}, nil
}

func (c *mockClient) Ping(ctx context.Context) error {
	return nil
}

func (c *mockClient) CreateMessage(ctx context.Context, req types.CreateMessageRequest) (*types.CreateMessageResult, error) {
	return &types.CreateMessageResult{}, nil
}

func (c *mockClient) ListRoots(ctx context.Context, req types.ListRootsRequest) (*types.ListRootsResult, error) {
	return &types.ListRootsResult{}, nil
}

func (c *mockClient) SendProgress(token types.ProgressToken, progress float64, total *float64) error {
	return nil
}

func (c *mockClient) SendCancelled(requestId []byte, reason *string) error {
	return nil
}

func (c *mockClient) ServerInfo() *types.Implementation {
	return &types.Implementation{Name: "mock", Version: "1.0"}
}

func (c *mockClient) ServerCapabilities() *types.ServerCapabilities {
	return &types.ServerCapabilities{}
}