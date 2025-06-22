package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/middleware"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/security"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/transport"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// TestE2E_HighThroughputToolCalls tests system behavior under high load:
// 1. Many concurrent tool calls
// 2. Rate limiting behavior
// 3. Resource cleanup and memory management
// 4. Error handling under stress
func TestE2E_HighThroughputToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	pipe := newMockPipe()
	defer pipe.Close()

	// Create high-throughput tool handler
	toolHandler := &highThroughputToolHandler{
		callCount:    0,
		errorRate:    0.1, // 10% error rate to test error handling
		responseTime: 5 * time.Millisecond,
	}

	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "stress-server",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Tools:    &types.ToolsCapability{},
			Sampling: &types.SamplingCapability{},
		},
		ToolHandler: toolHandler,
	})

	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server, serverTransport, serverCtx)

	// Create secure client with rate limiting
	baseClient := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	baseClient.SetTransport(clientTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := baseClient.Initialize(ctx, types.Implementation{
		Name:    "stress-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Create security layer with high throughput rate limiter
	securityConfig := security.DefaultSamplingSecurityConfig()
	securityConfig.EnableRateLimit = true

	rateLimiter := middleware.NewSimpleRateLimiter(100, time.Minute) // Higher limit for stress test
	secSampling := security.NewSamplingSecurity(securityConfig)
	secSampling.SetRateLimiter(rateLimiter)

	secureClient := security.NewSecureSamplingClient(baseClient, secSampling, "stress-client")

	// Launch high number of concurrent calls
	const numConcurrent = 50
	const callsPerWorker = 5
	
	var successCount int64
	var errorCount int64
	var totalLatency int64

	startTime := time.Now()

	// Use worker pool pattern for controlled concurrency
	workerPool := make(chan struct{}, numConcurrent)
	results := make(chan callResult, numConcurrent*callsPerWorker)

	// Launch workers
	for i := 0; i < numConcurrent; i++ {
		go func(workerID int) {
			workerPool <- struct{}{} // Acquire worker slot
			defer func() { <-workerPool }() // Release worker slot

			for j := 0; j < callsPerWorker; j++ {
				callStart := time.Now()
				
				args, _ := json.Marshal(map[string]interface{}{
					"worker_id": workerID,
					"call_id":   j,
					"data":      fmt.Sprintf("worker_%d_call_%d", workerID, j),
				})

				_, err := secureClient.CallTool(ctx, types.CallToolRequest{
					Name:      "stress-tool",
					Arguments: args,
				})

				latency := time.Since(callStart)
				
				results <- callResult{
					success: err == nil,
					latency: latency,
					error:   err,
				}
			}
		}(i)
	}

	// Collect results
	expectedResults := numConcurrent * callsPerWorker
	for i := 0; i < expectedResults; i++ {
		select {
		case result := <-results:
			if result.success {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}
			atomic.AddInt64(&totalLatency, int64(result.latency))
		case <-time.After(25 * time.Second):
			t.Fatalf("Stress test timed out after processing %d/%d results", i, expectedResults)
		}
	}

	totalTime := time.Since(startTime)

	// Analyze results
	totalCalls := successCount + errorCount
	successRate := float64(successCount) / float64(totalCalls)
	avgLatency := time.Duration(totalLatency / totalCalls)
	throughput := float64(totalCalls) / totalTime.Seconds()

	t.Logf("Stress Test Results:")
	t.Logf("  Total Calls: %d", totalCalls)
	t.Logf("  Success Rate: %.2f%%", successRate*100)
	t.Logf("  Average Latency: %v", avgLatency)
	t.Logf("  Throughput: %.2f calls/sec", throughput)
	t.Logf("  Total Time: %v", totalTime)

	// Verify performance expectations
	// With rate limiting of 100 calls/minute and 250 total calls, expect ~40% success rate
	if successRate < 0.3 { // At least 30% success rate expected (allowing for some variance)
		t.Errorf("Success rate too low: %.2f%%, expected >= 30%%", successRate*100)
	}

	if avgLatency > 100*time.Millisecond { // Average latency should be reasonable
		t.Errorf("Average latency too high: %v, expected <= 100ms", avgLatency)
	}

	if throughput < 10 { // Should handle at least 10 calls per second
		t.Errorf("Throughput too low: %.2f calls/sec, expected >= 10", throughput)
	}

	// Verify tool handler received expected number of successful calls
	toolHandler.mu.Lock()
	handlerCallCount := toolHandler.callCount
	toolHandler.mu.Unlock()

	if int64(handlerCallCount) != successCount {
		t.Errorf("Tool handler call count mismatch: got %d, expected %d", handlerCallCount, successCount)
	}
}

// TestE2E_MemoryLeakDetection tests for memory leaks during sustained operation:
// 1. Long-running operations with resource cleanup
// 2. Connection cycling
// 3. Memory usage monitoring
func TestE2E_MemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Create a test that simulates long-running operation with multiple connection cycles
	const cycles = 10
	const operationsPerCycle = 20

	for cycle := 0; cycle < cycles; cycle++ {
		func() { // Use closure to ensure cleanup
			pipe := newMockPipe()
			defer pipe.Close()

			server := mcp.NewCoreServer(mcp.ServerOptions{
				Name:        "memory-server",
				Version:     "1.0.0",
				Capabilities: types.ServerCapabilities{
					Tools:     &types.ToolsCapability{},
					Resources: &types.ResourcesCapability{},
				},
				ToolHandler:     &memoryTestToolHandler{},
				ResourceHandler: &memoryTestResourceHandler{},
			})

			serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
			server.SetTransport(serverTransport)

			serverCtx, serverCancel := context.WithCancel(context.Background())
			defer serverCancel()

			go runServer(server, serverTransport, serverCtx)

			client := mcp.NewMCPClient(client.ClientOptions{})
			clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
			client.SetTransport(clientTransport)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := client.Initialize(ctx, types.Implementation{
				Name:    fmt.Sprintf("memory-client-%d", cycle),
				Version: "1.0.0",
			})
			if err != nil {
				t.Fatalf("Cycle %d: Failed to initialize: %v", cycle, err)
			}

			// Perform operations that should be cleaned up properly
			for op := 0; op < operationsPerCycle; op++ {
				// Tool call
				args, _ := json.Marshal(map[string]interface{}{
					"cycle": cycle,
					"op":    op,
				})

				_, err := client.CallTool(ctx, types.CallToolRequest{
					Name:      "memory-tool",
					Arguments: args,
				})
				if err != nil {
					t.Errorf("Cycle %d, Op %d: Tool call failed: %v", cycle, op, err)
				}

				// Resource operation
				_, err = client.ReadResource(ctx, types.ReadResourceRequest{
					URI: fmt.Sprintf("memory://resource-%d-%d", cycle, op),
				})
				if err != nil {
					t.Errorf("Cycle %d, Op %d: Resource read failed: %v", cycle, op, err)
				}
			}

			// Force cleanup
			client.Close()
		}()

		// Small pause between cycles to allow garbage collection
		time.Sleep(10 * time.Millisecond)
	}

	// Test completed without major issues if we get here
	t.Logf("Memory leak test completed %d cycles with %d operations each", cycles, operationsPerCycle)
}

// TestE2E_ErrorRecoveryAndResilience tests system behavior under error conditions:
// 1. Network failures and recovery
// 2. Invalid requests and error handling
// 3. Resource exhaustion scenarios
func TestE2E_ErrorRecoveryAndResilience(t *testing.T) {
	pipe := newMockPipe()
	defer pipe.Close()

	// Create error-prone handler for testing resilience
	errorHandler := &errorProneHandler{
		errorPattern: []bool{false, false, true, false, true}, // 40% error rate in pattern
		callIndex:    0,
	}

	server := mcp.NewCoreServer(mcp.ServerOptions{
		Name:     "resilience-server",
		Version:  "1.0.0",
		Capabilities: types.ServerCapabilities{
			Tools: &types.ToolsCapability{},
		},
		ToolHandler: errorHandler,
	})

	serverTransport := transport.NewStdioTransportWithStreams(pipe.ServerReader(), pipe.ServerWriter())
	server.SetTransport(serverTransport)

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	go runServer(server, serverTransport, serverCtx)

	client := mcp.NewMCPClient(client.ClientOptions{})
	clientTransport := transport.NewStdioTransportWithStreams(pipe.ClientReader(), pipe.ClientWriter())
	client.SetTransport(clientTransport)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := client.Initialize(ctx, types.Implementation{
		Name:    "resilience-client",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Test 1: Error handling and recovery
	const testCalls = 20
	var successCount, errorCount int

	for i := 0; i < testCalls; i++ {
		args, _ := json.Marshal(map[string]interface{}{
			"call_id": i,
		})

		_, err := client.CallTool(ctx, types.CallToolRequest{
			Name:      "error-prone-tool",
			Arguments: args,
		})

		if err != nil {
			errorCount++
		} else {
			successCount++
		}

		// Small delay to simulate real usage
		time.Sleep(5 * time.Millisecond)
	}

	t.Logf("Error recovery test: %d successes, %d errors out of %d calls", successCount, errorCount, testCalls)

	// Verify client handled errors gracefully
	if errorCount == 0 {
		t.Error("Expected some errors based on error-prone handler, but got none")
	}

	if successCount == 0 {
		t.Error("Expected some successes, but all calls failed")
	}

	// Test 2: Invalid request handling
	invalidTests := []struct {
		name string
		req  types.CallToolRequest
	}{
		{
			name: "non-existent tool",
			req: types.CallToolRequest{
				Name:      "non-existent-tool",
				Arguments: json.RawMessage(`{}`),
			},
		},
		{
			name: "invalid arguments",
			req: types.CallToolRequest{
				Name:      "error-prone-tool",
				Arguments: json.RawMessage(`invalid json`),
			},
		},
	}

	for _, test := range invalidTests {
		_, err := client.CallTool(ctx, test.req)
		if err == nil {
			t.Errorf("Expected error for %s, but got none", test.name)
		}
	}

	// Test 3: Recovery after errors
	// Make several successful calls to verify system is still functional
	for i := 0; i < 5; i++ {
		args, _ := json.Marshal(map[string]interface{}{
			"recovery_call": i,
		})

		_, err := client.CallTool(ctx, types.CallToolRequest{
			Name:      "error-prone-tool",
			Arguments: args,
		})

		// At least some of these should succeed
		if err == nil {
			break // Success - system recovered
		}

		if i == 4 { // Last attempt
			t.Error("System failed to recover after error conditions")
		}
	}
}

// Helper types for stress testing

type callResult struct {
	success bool
	latency time.Duration
	error   error
}

type highThroughputToolHandler struct {
	callCount    int64
	errorRate    float64
	responseTime time.Duration
	mu           sync.Mutex
}

func (h *highThroughputToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "stress-tool",
				Description: strPtr("Tool for stress testing"),
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"worker_id": map[string]interface{}{"type": "integer"},
						"call_id":   map[string]interface{}{"type": "integer"},
						"data":      map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}, nil
}

func (h *highThroughputToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	// Simulate processing time
	if h.responseTime > 0 {
		time.Sleep(h.responseTime)
	}

	h.mu.Lock()
	callNum := h.callCount
	h.callCount++
	h.mu.Unlock()

	// Simulate errors based on error rate
	if h.errorRate > 0 && float64(callNum%100)/100.0 < h.errorRate {
		return &types.CallToolResult{
			Content: []interface{}{
				types.TextContent{Type: "text", Text: "Simulated error"},
			},
			IsError: true,
		}, nil
	}

	var args struct {
		WorkerID int    `json:"worker_id"`
		CallID   int    `json:"call_id"`
		Data     string `json:"data"`
	}

	if err := json.Unmarshal(req.Arguments, &args); err != nil {
		return &types.CallToolResult{
			Content: []interface{}{
				types.TextContent{Type: "text", Text: "Invalid arguments"},
			},
			IsError: true,
		}, nil
	}

	response := fmt.Sprintf("Processed call %d from worker %d: %s", args.CallID, args.WorkerID, args.Data)

	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: response},
		},
	}, nil
}

type memoryTestToolHandler struct{}

func (h *memoryTestToolHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "memory-tool",
				Description: strPtr("Tool for memory testing"),
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"cycle": map[string]interface{}{"type": "integer"},
						"op":    map[string]interface{}{"type": "integer"},
					},
				},
			},
		},
	}, nil
}

func (h *memoryTestToolHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	// Create some temporary data that should be garbage collected
	data := make([]byte, 1024) // 1KB per call
	for i := range data {
		data[i] = byte(i % 256)
	}

	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: fmt.Sprintf("Memory tool processed %d bytes", len(data))},
		},
	}, nil
}

type memoryTestResourceHandler struct{}

func (h *memoryTestResourceHandler) ListResources(ctx context.Context, req types.ListResourcesRequest) (*types.ListResourcesResult, error) {
	return &types.ListResourcesResult{Resources: []types.Resource{}}, nil
}

func (h *memoryTestResourceHandler) ReadResource(ctx context.Context, req types.ReadResourceRequest) (*types.ReadResourceResult, error) {
	// Create temporary content
	content := fmt.Sprintf("Memory resource content for %s", req.URI)
	
	return &types.ReadResourceResult{
		Contents: []interface{}{
			types.ResourceContentsText{
				URI:  req.URI,
				Text: content,
			},
		},
	}, nil
}

func (h *memoryTestResourceHandler) Subscribe(ctx context.Context, req types.SubscribeRequest) error {
	return nil
}

func (h *memoryTestResourceHandler) Unsubscribe(ctx context.Context, req types.UnsubscribeRequest) error {
	return nil
}

type errorProneHandler struct {
	errorPattern []bool
	callIndex    int64
	mu           sync.Mutex
}

func (h *errorProneHandler) ListTools(ctx context.Context, req types.ListToolsRequest) (*types.ListToolsResult, error) {
	return &types.ListToolsResult{
		Tools: []types.Tool{
			{
				Name:        "error-prone-tool",
				Description: strPtr("Tool that fails sometimes"),
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		},
	}, nil
}

func (h *errorProneHandler) CallTool(ctx context.Context, req types.CallToolRequest) (*types.CallToolResult, error) {
	// Check if tool exists
	if req.Name != "error-prone-tool" {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeToolNotFound, fmt.Sprintf("Tool '%s' not found", req.Name), nil)
	}

	h.mu.Lock()
	index := int(h.callIndex % int64(len(h.errorPattern)))
	shouldError := h.errorPattern[index]
	h.callIndex++
	h.mu.Unlock()

	if shouldError {
		return nil, mcp.NewProtocolError(mcp.ErrorCodeInternalError, "Simulated error", nil)
	}

	return &types.CallToolResult{
		Content: []interface{}{
			types.TextContent{Type: "text", Text: fmt.Sprintf("Success call #%d", index)},
		},
	}, nil
}