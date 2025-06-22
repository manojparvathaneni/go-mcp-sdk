package connection

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// ConnectionState represents the state of a connection
type ConnectionState int32

const (
	// ConnectionStateDisconnected means the connection is not active
	ConnectionStateDisconnected ConnectionState = iota
	// ConnectionStateConnecting means the connection is being established
	ConnectionStateConnecting
	// ConnectionStateConnected means the connection is active
	ConnectionStateConnected
	// ConnectionStateReconnecting means the connection is being re-established
	ConnectionStateReconnecting
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ConnectionOptions configures connection behavior
type ConnectionOptions struct {
	// MaxReconnectAttempts is the maximum number of reconnection attempts (0 = infinite)
	MaxReconnectAttempts int
	// ReconnectDelay is the base delay between reconnection attempts
	ReconnectDelay time.Duration
	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration
	// ConnectionTimeout is the timeout for establishing connections
	ConnectionTimeout time.Duration
	// PingInterval is the interval for sending ping messages (0 = disabled)
	PingInterval time.Duration
	// PingTimeout is the timeout for ping responses
	PingTimeout time.Duration
}

// DefaultConnectionOptions returns default connection options
func DefaultConnectionOptions() ConnectionOptions {
	return ConnectionOptions{
		MaxReconnectAttempts: 5,
		ReconnectDelay:       1 * time.Second,
		MaxReconnectDelay:    30 * time.Second,
		ConnectionTimeout:    10 * time.Second,
		PingInterval:         30 * time.Second,
		PingTimeout:          5 * time.Second,
	}
}

// MCPClient interface defines the methods needed from an MCP client
type MCPClient interface {
	Initialize(ctx context.Context, clientInfo types.Implementation) (*types.InitializeResult, error)
	ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error)
	Close() error
	SetTransport(transport client.Transport)
}

// ConnectionManager manages MCP client connections with automatic reconnection
type ConnectionManager struct {
	mu      sync.RWMutex
	client  MCPClient
	state   int32 // ConnectionState
	options ConnectionOptions
	
	// Connection factory
	transportFactory func() (client.Transport, error)
	clientInfo       types.Implementation
	
	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	
	// Reconnection state
	reconnectAttempts int32
	lastConnected     time.Time
	
	// Callbacks
	onStateChange      func(oldState, newState ConnectionState)
	onConnected        func()
	onDisconnected     func(err error)
	onReconnectFailed  func(err error, attempt int)
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(
	transportFactory func() (client.Transport, error),
	clientInfo types.Implementation,
	options ConnectionOptions,
) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ConnectionManager{
		client:           client.NewClient(client.ClientOptions{}),
		options:          options,
		transportFactory: transportFactory,
		clientInfo:       clientInfo,
		ctx:              ctx,
		cancel:           cancel,
		state:            int32(ConnectionStateDisconnected),
	}
}

// Connect establishes the connection
func (cm *ConnectionManager) Connect() error {
	if !cm.compareAndSwapState(ConnectionStateDisconnected, ConnectionStateConnecting) {
		return errors.New("connection already active or in progress")
	}
	
	err := cm.connect()
	if err != nil {
		cm.setState(ConnectionStateDisconnected)
		return err
	}
	
	cm.setState(ConnectionStateConnected)
	atomic.StoreInt32(&cm.reconnectAttempts, 0)
	cm.lastConnected = time.Now()
	
	if cm.onConnected != nil {
		cm.onConnected()
	}
	
	// Start background tasks
	go cm.healthCheck()
	
	return nil
}

// Disconnect closes the connection
func (cm *ConnectionManager) Disconnect() error {
	cm.setState(ConnectionStateDisconnected)
	cm.cancel()
	return cm.client.Close()
}

// Client returns the underlying MCP client
func (cm *ConnectionManager) Client() MCPClient {
	return cm.client
}

// State returns the current connection state
func (cm *ConnectionManager) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&cm.state))
}

// IsConnected returns true if the connection is active
func (cm *ConnectionManager) IsConnected() bool {
	return cm.State() == ConnectionStateConnected
}

// SetOnStateChange sets the state change callback
func (cm *ConnectionManager) SetOnStateChange(fn func(oldState, newState ConnectionState)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onStateChange = fn
}

// SetOnConnected sets the connected callback
func (cm *ConnectionManager) SetOnConnected(fn func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onConnected = fn
}

// SetOnDisconnected sets the disconnected callback
func (cm *ConnectionManager) SetOnDisconnected(fn func(err error)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onDisconnected = fn
}

// SetOnReconnectFailed sets the reconnect failed callback
func (cm *ConnectionManager) SetOnReconnectFailed(fn func(err error, attempt int)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onReconnectFailed = fn
}

// GetStats returns connection statistics
func (cm *ConnectionManager) GetStats() ConnectionStats {
	return ConnectionStats{
		State:             cm.State(),
		ReconnectAttempts: atomic.LoadInt32(&cm.reconnectAttempts),
		LastConnected:     cm.lastConnected,
		Uptime:           time.Since(cm.lastConnected),
	}
}

// ConnectionStats contains connection statistics
type ConnectionStats struct {
	State             ConnectionState
	ReconnectAttempts int32
	LastConnected     time.Time
	Uptime           time.Duration
}

func (cm *ConnectionManager) connect() error {
	ctx, cancel := context.WithTimeout(cm.ctx, cm.options.ConnectionTimeout)
	defer cancel()
	
	// Create transport
	transport, err := cm.transportFactory()
	if err != nil {
		return err
	}
	
	// Set transport on client
	cm.client.SetTransport(transport)
	
	// Initialize connection
	_, err = cm.client.Initialize(ctx, cm.clientInfo)
	if err != nil {
		transport.Close()
		return err
	}
	
	return nil
}

func (cm *ConnectionManager) setState(newState ConnectionState) {
	oldState := ConnectionState(atomic.SwapInt32(&cm.state, int32(newState)))
	
	if oldState != newState && cm.onStateChange != nil {
		cm.onStateChange(oldState, newState)
	}
}

func (cm *ConnectionManager) compareAndSwapState(old, new ConnectionState) bool {
	return atomic.CompareAndSwapInt32(&cm.state, int32(old), int32(new))
}

func (cm *ConnectionManager) healthCheck() {
	if cm.options.PingInterval <= 0 {
		return
	}
	
	ticker := time.NewTicker(cm.options.PingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			if cm.State() != ConnectionStateConnected {
				continue
			}
			
			// Send ping by listing resources (lightweight operation)
			ctx, cancel := context.WithTimeout(cm.ctx, cm.options.PingTimeout)
			_, err := cm.client.ListResources(ctx, types.ListResourcesRequest{})
			cancel()
			
			if err != nil {
				cm.handleConnectionError(err)
			}
		}
	}
}

func (cm *ConnectionManager) handleConnectionError(err error) {
	if cm.State() != ConnectionStateConnected {
		return
	}
	
	cm.setState(ConnectionStateReconnecting)
	
	if cm.onDisconnected != nil {
		cm.onDisconnected(err)
	}
	
	go cm.reconnect()
}

func (cm *ConnectionManager) reconnect() {
	attempts := atomic.AddInt32(&cm.reconnectAttempts, 1)
	
	// Check if we've exceeded max attempts
	if cm.options.MaxReconnectAttempts > 0 && int(attempts) > cm.options.MaxReconnectAttempts {
		cm.setState(ConnectionStateDisconnected)
		if cm.onReconnectFailed != nil {
			cm.onReconnectFailed(errors.New("max reconnect attempts exceeded"), int(attempts))
		}
		return
	}
	
	// Calculate delay with exponential backoff
	delay := cm.calculateReconnectDelay(int(attempts))
	
	select {
	case <-cm.ctx.Done():
		return
	case <-time.After(delay):
	}
	
	// Attempt reconnection
	err := cm.connect()
	if err != nil {
		if cm.onReconnectFailed != nil {
			cm.onReconnectFailed(err, int(attempts))
		}
		
		// Schedule next attempt
		go cm.reconnect()
		return
	}
	
	// Reconnection successful
	cm.setState(ConnectionStateConnected)
	cm.lastConnected = time.Now()
	
	if cm.onConnected != nil {
		cm.onConnected()
	}
	
	// Reset attempt counter
	atomic.StoreInt32(&cm.reconnectAttempts, 0)
}

func (cm *ConnectionManager) calculateReconnectDelay(attempt int) time.Duration {
	// Exponential backoff: delay = baseDelay * 2^(attempt-1)
	delay := cm.options.ReconnectDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > cm.options.MaxReconnectDelay {
			delay = cm.options.MaxReconnectDelay
			break
		}
	}
	return delay
}

// ConnectionPool manages multiple connections for load balancing and redundancy
type ConnectionPool struct {
	mu          sync.RWMutex
	connections []*ConnectionManager
	roundRobin  int32
	healthy     []bool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(managers []*ConnectionManager) *ConnectionPool {
	pool := &ConnectionPool{
		connections: managers,
		healthy:     make([]bool, len(managers)),
	}
	
	// Set up health monitoring
	for i, mgr := range managers {
		i := i // capture loop variable
		mgr.SetOnStateChange(func(oldState, newState ConnectionState) {
			pool.mu.Lock()
			pool.healthy[i] = newState == ConnectionStateConnected
			pool.mu.Unlock()
		})
	}
	
	return pool
}

// Connect connects all connections in the pool
func (p *ConnectionPool) Connect() error {
	var lastErr error
	connected := 0
	
	for _, mgr := range p.connections {
		if err := mgr.Connect(); err != nil {
			lastErr = err
		} else {
			connected++
		}
	}
	
	if connected == 0 {
		return lastErr
	}
	
	return nil
}

// Disconnect disconnects all connections in the pool
func (p *ConnectionPool) Disconnect() error {
	for _, mgr := range p.connections {
		mgr.Disconnect()
	}
	return nil
}

// GetClient returns a healthy client using round-robin selection
func (p *ConnectionPool) GetClient() MCPClient {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if len(p.connections) == 0 {
		return nil
	}
	
	// Find healthy connections
	var healthy []int
	for i, isHealthy := range p.healthy {
		if isHealthy {
			healthy = append(healthy, i)
		}
	}
	
	if len(healthy) == 0 {
		return nil
	}
	
	// Round-robin selection
	next := atomic.AddInt32(&p.roundRobin, 1)
	idx := healthy[int(next-1)%len(healthy)]
	
	return p.connections[idx].Client()
}

// GetStats returns statistics for all connections
func (p *ConnectionPool) GetStats() []ConnectionStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	stats := make([]ConnectionStats, len(p.connections))
	for i, mgr := range p.connections {
		stats[i] = mgr.GetStats()
	}
	
	return stats
}

// HealthyCount returns the number of healthy connections
func (p *ConnectionPool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	count := 0
	for _, isHealthy := range p.healthy {
		if isHealthy {
			count++
		}
	}
	
	return count
}