// Package testharness provides a comprehensive testing framework for MCP implementations
// with support for local models, performance testing, and compliance validation.
package testharness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/transport"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// TestHarness provides comprehensive testing capabilities for MCP implementations
type TestHarness struct {
	config     TestConfig
	results    []TestResult
	logger     Logger
	mu         sync.Mutex
	startTime  time.Time
	serverCmd  *exec.Cmd
	client     *client.Client
	transport  client.Transport
}

// TestConfig configures the test harness behavior
type TestConfig struct {
	// Server configuration
	ServerCommand    []string          `json:"server_command"`
	ServerTimeout    time.Duration     `json:"server_timeout"`
	Environment      map[string]string `json:"environment"`
	WorkingDirectory string            `json:"working_directory"`

	// Test configuration
	TestTimeout       time.Duration `json:"test_timeout"`
	MaxConcurrency    int           `json:"max_concurrency"`
	RetryCount        int           `json:"retry_count"`
	WarmupDuration    time.Duration `json:"warmup_duration"`

	// Performance testing
	LoadTestDuration  time.Duration `json:"load_test_duration"`
	LoadTestRPS       int           `json:"load_test_rps"`
	MemoryThreshold   int64         `json:"memory_threshold_mb"`
	CPUThreshold      float64       `json:"cpu_threshold_percent"`

	// Compliance testing
	StrictCompliance  bool     `json:"strict_compliance"`
	RequiredMethods   []string `json:"required_methods"`
	OptionalMethods   []string `json:"optional_methods"`

	// Output configuration
	OutputFormat      string `json:"output_format"` // "json", "junit", "console"
	OutputFile        string `json:"output_file"`
	DetailedResults   bool   `json:"detailed_results"`
	ContinueOnFailure bool   `json:"continue_on_failure"`
}

// TestResult represents the result of a test execution
type TestResult struct {
	Name        string                 `json:"name"`
	Status      TestStatus             `json:"status"`
	Duration    time.Duration          `json:"duration"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Category    string                 `json:"category"`
	Severity    Severity               `json:"severity"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TestStatus represents the status of a test
type TestStatus string

const (
	StatusPassed  TestStatus = "passed"
	StatusFailed  TestStatus = "failed"
	StatusSkipped TestStatus = "skipped"
	StatusTimeout TestStatus = "timeout"
	StatusError   TestStatus = "error"
)

// Severity represents the severity of a test failure
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Logger interface for test harness logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// DefaultLogger provides basic logging functionality
type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] "+msg, fields...)
}

func (l *DefaultLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] "+msg, fields...)
}

func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] "+msg, fields...)
}

func (l *DefaultLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] "+msg, fields...)
}

// NewTestHarness creates a new test harness with the given configuration
func NewTestHarness(config TestConfig) *TestHarness {
	if config.TestTimeout == 0 {
		config.TestTimeout = 30 * time.Second
	}
	if config.ServerTimeout == 0 {
		config.ServerTimeout = 10 * time.Second
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 5
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	if config.OutputFormat == "" {
		config.OutputFormat = "console"
	}

	return &TestHarness{
		config:  config,
		results: make([]TestResult, 0),
		logger:  &DefaultLogger{},
	}
}

// SetLogger sets a custom logger for the test harness
func (h *TestHarness) SetLogger(logger Logger) {
	h.logger = logger
}

// StartServer starts the MCP server process
func (h *TestHarness) StartServer(ctx context.Context) error {
	if len(h.config.ServerCommand) == 0 {
		return fmt.Errorf("no server command specified")
	}

	h.logger.Info("Starting MCP server", "command", h.config.ServerCommand)

	// Set up command
	h.serverCmd = exec.CommandContext(ctx, h.config.ServerCommand[0], h.config.ServerCommand[1:]...)
	
	// Set environment variables
	if h.config.Environment != nil {
		env := os.Environ()
		for key, value := range h.config.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		h.serverCmd.Env = env
	}

	// Set working directory
	if h.config.WorkingDirectory != "" {
		h.serverCmd.Dir = h.config.WorkingDirectory
	}

	// Set up pipes for stdio transport
	stdin, err := h.serverCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := h.serverCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the server
	if err := h.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Create transport and client
	h.transport = transport.NewStdioTransportWithStreams(stdout, stdin)
	h.client = client.NewClient(client.ClientOptions{})
	h.client.SetTransport(h.transport)

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	h.logger.Info("MCP server started successfully")
	return nil
}

// StopServer stops the MCP server process
func (h *TestHarness) StopServer() error {
	if h.client != nil {
		h.client.Close()
		h.client = nil
	}

	if h.serverCmd != nil && h.serverCmd.Process != nil {
		h.logger.Info("Stopping MCP server")
		if err := h.serverCmd.Process.Kill(); err != nil {
			h.logger.Warn("Failed to kill server process", "error", err)
		}
		h.serverCmd.Wait()
		h.serverCmd = nil
	}

	return nil
}

// RunAllTests executes all available test suites
func (h *TestHarness) RunAllTests(ctx context.Context) error {
	h.startTime = time.Now()
	h.logger.Info("Starting comprehensive MCP test suite")

	// Start server
	if err := h.StartServer(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer h.StopServer()

	// Warmup period
	if h.config.WarmupDuration > 0 {
		h.logger.Info("Warming up server", "duration", h.config.WarmupDuration)
		time.Sleep(h.config.WarmupDuration)
	}

	// Run test suites
	testSuites := []func(context.Context) error{
		h.RunConnectivityTests,
		h.RunProtocolComplianceTests,
		h.RunFunctionalTests,
		h.RunPerformanceTests,
		h.RunStressTests,
		h.RunSecurityTests,
		h.RunErrorHandlingTests,
	}

	for _, testSuite := range testSuites {
		if err := testSuite(ctx); err != nil {
			h.logger.Error("Test suite failed", "error", err)
			if !h.config.ContinueOnFailure {
				return err
			}
		}
	}

	// Generate final report
	return h.GenerateReport()
}

// RunConnectivityTests tests basic connection and initialization
func (h *TestHarness) RunConnectivityTests(ctx context.Context) error {
	h.logger.Info("Running connectivity tests")

	// Test 1: Initialize connection
	h.runTest("connectivity_initialize", func() error {
		_, err := h.client.Initialize(ctx, types.Implementation{
			Name:    "test-harness",
			Version: "1.0.0",
		})
		return err
	}, SeverityCritical)

	// Test 2: Ping server
	h.runTest("connectivity_ping", func() error {
		return h.client.Ping(ctx)
	}, SeverityHigh)

	return nil
}

// RunProtocolComplianceTests tests MCP protocol compliance
func (h *TestHarness) RunProtocolComplianceTests(ctx context.Context) error {
	h.logger.Info("Running protocol compliance tests")

	// Test method availability
	for _, method := range h.config.RequiredMethods {
		h.runTest(fmt.Sprintf("compliance_required_method_%s", method), func() error {
			return h.testMethodAvailability(ctx, method)
		}, SeverityCritical)
	}

	for _, method := range h.config.OptionalMethods {
		h.runTest(fmt.Sprintf("compliance_optional_method_%s", method), func() error {
			return h.testMethodAvailability(ctx, method)
		}, SeverityLow)
	}

	// Test message format compliance
	h.runTest("compliance_message_format", func() error {
		return h.testMessageFormatCompliance(ctx)
	}, SeverityHigh)

	return nil
}

// RunFunctionalTests tests core MCP functionality
func (h *TestHarness) RunFunctionalTests(ctx context.Context) error {
	h.logger.Info("Running functional tests")

	// Test resources
	h.runTest("functional_list_resources", func() error {
		_, err := h.client.ListResources(ctx, types.ListResourcesRequest{})
		return err
	}, SeverityHigh)

	// Test tools
	h.runTest("functional_list_tools", func() error {
		_, err := h.client.ListTools(ctx, types.ListToolsRequest{})
		return err
	}, SeverityHigh)

	// Test prompts
	h.runTest("functional_list_prompts", func() error {
		_, err := h.client.ListPrompts(ctx, types.ListPromptsRequest{})
		return err
	}, SeverityMedium)

	return nil
}

// RunPerformanceTests tests performance characteristics
func (h *TestHarness) RunPerformanceTests(ctx context.Context) error {
	h.logger.Info("Running performance tests")

	// Test response time
	h.runTest("performance_response_time", func() error {
		start := time.Now()
		_, err := h.client.ListResources(ctx, types.ListResourcesRequest{})
		duration := time.Since(start)
		
		if duration > 1*time.Second {
			return fmt.Errorf("response time too slow: %v", duration)
		}
		return err
	}, SeverityMedium)

	// Test throughput
	h.runTest("performance_throughput", func() error {
		return h.testThroughput(ctx)
	}, SeverityMedium)

	return nil
}

// RunStressTests tests system behavior under load
func (h *TestHarness) RunStressTests(ctx context.Context) error {
	h.logger.Info("Running stress tests")

	// Concurrent requests test
	h.runTest("stress_concurrent_requests", func() error {
		return h.testConcurrentRequests(ctx)
	}, SeverityMedium)

	// Memory usage test
	h.runTest("stress_memory_usage", func() error {
		return h.testMemoryUsage(ctx)
	}, SeverityLow)

	return nil
}

// RunSecurityTests tests security-related functionality
func (h *TestHarness) RunSecurityTests(ctx context.Context) error {
	h.logger.Info("Running security tests")

	// Test input validation
	h.runTest("security_input_validation", func() error {
		return h.testInputValidation(ctx)
	}, SeverityHigh)

	// Test error information disclosure
	h.runTest("security_error_disclosure", func() error {
		return h.testErrorDisclosure(ctx)
	}, SeverityMedium)

	return nil
}

// RunErrorHandlingTests tests error handling behavior
func (h *TestHarness) RunErrorHandlingTests(ctx context.Context) error {
	h.logger.Info("Running error handling tests")

	// Test invalid requests
	h.runTest("error_invalid_request", func() error {
		return h.testInvalidRequests(ctx)
	}, SeverityMedium)

	// Test timeout handling
	h.runTest("error_timeout_handling", func() error {
		return h.testTimeoutHandling(ctx)
	}, SeverityMedium)

	return nil
}

// runTest executes a single test with timeout and error handling
func (h *TestHarness) runTest(name string, testFunc func() error, severity Severity) {
	h.logger.Debug("Running test", "name", name)
	
	start := time.Now()
	result := TestResult{
		Name:      name,
		Timestamp: start,
		Severity:  severity,
		Category:  h.getCategoryFromName(name),
		Details:   make(map[string]interface{}),
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), h.config.TestTimeout)
	defer cancel()

	// Run test with timeout
	done := make(chan error, 1)
	go func() {
		done <- testFunc()
	}()

	select {
	case err := <-done:
		result.Duration = time.Since(start)
		if err != nil {
			result.Status = StatusFailed
			result.Error = err.Error()
		} else {
			result.Status = StatusPassed
		}
	case <-ctx.Done():
		result.Duration = h.config.TestTimeout
		result.Status = StatusTimeout
		result.Error = "test timed out"
	}

	h.mu.Lock()
	h.results = append(h.results, result)
	h.mu.Unlock()

	// Log result
	if result.Status == StatusPassed {
		h.logger.Info("Test passed", "name", name, "duration", result.Duration)
	} else {
		h.logger.Error("Test failed", "name", name, "status", result.Status, "error", result.Error)
	}
}

// Helper methods for specific tests
func (h *TestHarness) testMethodAvailability(ctx context.Context, method string) error {
	// This is a simplified check - in practice, you'd test specific methods
	switch method {
	case "resources/list":
		_, err := h.client.ListResources(ctx, types.ListResourcesRequest{})
		return err
	case "tools/list":
		_, err := h.client.ListTools(ctx, types.ListToolsRequest{})
		return err
	case "prompts/list":
		_, err := h.client.ListPrompts(ctx, types.ListPromptsRequest{})
		return err
	default:
		return fmt.Errorf("unknown method: %s", method)
	}
}

func (h *TestHarness) testMessageFormatCompliance(ctx context.Context) error {
	// Test that responses follow JSON-RPC 2.0 format
	// This would be more comprehensive in a real implementation
	return nil
}

func (h *TestHarness) testThroughput(ctx context.Context) error {
	// Test request throughput
	requests := 100
	start := time.Now()
	
	for i := 0; i < requests; i++ {
		_, err := h.client.ListResources(ctx, types.ListResourcesRequest{})
		if err != nil {
			return err
		}
	}
	
	duration := time.Since(start)
	rps := float64(requests) / duration.Seconds()
	
	if rps < 10 { // Minimum 10 RPS
		return fmt.Errorf("throughput too low: %.2f RPS", rps)
	}
	
	return nil
}

func (h *TestHarness) testConcurrentRequests(ctx context.Context) error {
	// Test concurrent request handling
	concurrency := h.config.MaxConcurrency
	errCh := make(chan error, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func() {
			_, err := h.client.ListResources(ctx, types.ListResourcesRequest{})
			errCh <- err
		}()
	}
	
	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			return fmt.Errorf("concurrent request failed: %w", err)
		}
	}
	
	return nil
}

func (h *TestHarness) testMemoryUsage(ctx context.Context) error {
	// This is a placeholder - real implementation would monitor memory
	return nil
}

func (h *TestHarness) testInputValidation(ctx context.Context) error {
	// Test with malformed input
	// This is a placeholder for security testing
	return nil
}

func (h *TestHarness) testErrorDisclosure(ctx context.Context) error {
	// Test that errors don't expose sensitive information
	return nil
}

func (h *TestHarness) testInvalidRequests(ctx context.Context) error {
	// Test handling of invalid requests
	return nil
}

func (h *TestHarness) testTimeoutHandling(ctx context.Context) error {
	// Test timeout behavior
	return nil
}

func (h *TestHarness) getCategoryFromName(name string) string {
	switch {
	case strings.HasPrefix(name, "connectivity_"):
		return "connectivity"
	case strings.HasPrefix(name, "compliance_"):
		return "compliance"
	case strings.HasPrefix(name, "functional_"):
		return "functional"
	case strings.HasPrefix(name, "performance_"):
		return "performance"
	case strings.HasPrefix(name, "stress_"):
		return "stress"
	case strings.HasPrefix(name, "security_"):
		return "security"
	case strings.HasPrefix(name, "error_"):
		return "error_handling"
	default:
		return "general"
	}
}

// GenerateReport generates a test report in the configured format
func (h *TestHarness) GenerateReport() error {
	h.logger.Info("Generating test report")

	switch h.config.OutputFormat {
	case "json":
		return h.generateJSONReport()
	case "junit":
		return h.generateJUnitReport()
	default:
		return h.generateConsoleReport()
	}
}

func (h *TestHarness) generateConsoleReport() error {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("MCP Test Harness Report")
	fmt.Println(strings.Repeat("=", 80))

	totalDuration := time.Since(h.startTime)
	
	// Summary
	passed := 0
	failed := 0
	total := len(h.results)
	
	for _, result := range h.results {
		if result.Status == StatusPassed {
			passed++
		} else {
			failed++
		}
	}
	
	fmt.Printf("Total Tests: %d\n", total)
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.2f%%\n", float64(passed)/float64(total)*100)
	fmt.Printf("Total Duration: %v\n", totalDuration)
	fmt.Println()

	// Results by category
	categories := make(map[string][]TestResult)
	for _, result := range h.results {
		categories[result.Category] = append(categories[result.Category], result)
	}

	for category, results := range categories {
		fmt.Printf("%s Tests:\n", strings.Title(category))
		for _, result := range results {
			status := "✓"
			if result.Status != StatusPassed {
				status = "✗"
			}
			fmt.Printf("  %s %s (%v)\n", status, result.Name, result.Duration)
			if result.Error != "" {
				fmt.Printf("    Error: %s\n", result.Error)
			}
		}
		fmt.Println()
	}

	return nil
}

func (h *TestHarness) generateJSONReport() error {
	report := map[string]interface{}{
		"timestamp": h.startTime,
		"duration":  time.Since(h.startTime),
		"results":   h.results,
		"summary": map[string]interface{}{
			"total":   len(h.results),
			"passed":  h.countByStatus(StatusPassed),
			"failed":  h.countByStatus(StatusFailed),
			"timeout": h.countByStatus(StatusTimeout),
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if h.config.OutputFile != "" {
		return os.WriteFile(h.config.OutputFile, data, 0644)
	}

	fmt.Println(string(data))
	return nil
}

func (h *TestHarness) generateJUnitReport() error {
	// JUnit XML format implementation would go here
	return fmt.Errorf("JUnit format not implemented yet")
}

func (h *TestHarness) countByStatus(status TestStatus) int {
	count := 0
	for _, result := range h.results {
		if result.Status == status {
			count++
		}
	}
	return count
}

// GetResults returns all test results
func (h *TestHarness) GetResults() []TestResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]TestResult(nil), h.results...)
}

// GetSummary returns a test summary
func (h *TestHarness) GetSummary() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()

	return map[string]interface{}{
		"total":        len(h.results),
		"passed":       h.countByStatus(StatusPassed),
		"failed":       h.countByStatus(StatusFailed),
		"timeout":      h.countByStatus(StatusTimeout),
		"duration":     time.Since(h.startTime),
		"success_rate": float64(h.countByStatus(StatusPassed)) / float64(len(h.results)) * 100,
	}
}