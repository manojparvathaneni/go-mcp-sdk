// mcp-test is a command-line tool for testing MCP server implementations
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/testharness"
)

const (
	version = "1.0.0"
	usage   = `mcp-test - MCP Server Test Harness

Usage:
  mcp-test [options] <command> [args...]

Commands:
  run      Run tests against an MCP server
  config   Generate example configuration files
  validate Validate a configuration file
  version  Print version information

Options:
  -config string    Configuration file path (default "mcp-test.json")
  -server string    Server command to run (overrides config)
  -timeout duration Test timeout (overrides config)
  -format string    Output format: console, json, junit (overrides config)
  -output string    Output file (overrides config)
  -verbose          Enable verbose logging
  -help             Show help

Examples:
  # Run tests with default config
  mcp-test run ./my-server

  # Run tests with custom config file
  mcp-test -config my-config.json run

  # Generate example configuration
  mcp-test config > mcp-test.json

  # Generate config for specific scenario
  mcp-test config performance > performance-test.json

  # Validate configuration file
  mcp-test validate mcp-test.json

  # Run with overrides
  mcp-test -timeout 60s -format json run ./my-server
`
)

func main() {
	var (
		configFile = flag.String("config", "mcp-test.json", "Configuration file path")
		serverCmd  = flag.String("server", "", "Server command to run (overrides config)")
		timeout    = flag.Duration("timeout", 0, "Test timeout (overrides config)")
		format     = flag.String("format", "", "Output format: console, json, junit (overrides config)")
		output     = flag.String("output", "", "Output file (overrides config)")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		help       = flag.Bool("help", false, "Show help")
	)

	flag.Usage = func() {
		fmt.Print(usage)
	}

	flag.Parse()

	if *help || len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(0)
	}

	command := flag.Args()[0]
	args := flag.Args()[1:]

	switch command {
	case "run":
		runTests(*configFile, *serverCmd, *timeout, *format, *output, *verbose, args)
	case "config":
		generateConfig(args)
	case "validate":
		validateConfig(args)
	case "version":
		fmt.Printf("mcp-test version %s\n", version)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func runTests(configFile, serverCmd string, timeout time.Duration, format, output string, verbose bool, args []string) {
	// Load configuration
	var config testharness.TestConfig
	var err error

	if _, err := os.Stat(configFile); err == nil {
		config, err = testharness.LoadConfigFromFile(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			os.Exit(1)
		}
	} else {
		config = testharness.DefaultTestConfig()
	}

	// Apply command-line overrides
	if serverCmd != "" {
		config.ServerCommand = strings.Fields(serverCmd)
	} else if len(args) > 0 {
		config.ServerCommand = args
	}

	if timeout > 0 {
		config.TestTimeout = timeout
	}

	if format != "" {
		config.OutputFormat = format
	}

	if output != "" {
		config.OutputFile = output
	}

	// Validate configuration
	if err := testharness.ValidateConfig(config); err != nil {
		fmt.Printf("Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create test harness
	harness := testharness.NewTestHarness(config)

	if verbose {
		harness.SetLogger(&verboseLogger{})
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, stopping tests...")
		cancel()
	}()

	// Run tests
	fmt.Printf("Starting MCP test harness (version %s)\n", version)
	fmt.Printf("Server command: %v\n", config.ServerCommand)
	fmt.Printf("Output format: %s\n", config.OutputFormat)
	if config.OutputFile != "" {
		fmt.Printf("Output file: %s\n", config.OutputFile)
	}
	fmt.Println()

	start := time.Now()
	err = harness.RunAllTests(ctx)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Test execution failed: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	summary := harness.GetSummary()
	fmt.Printf("\nTest execution completed in %v\n", duration)
	fmt.Printf("Results: %d total, %d passed, %d failed (%.1f%% success rate)\n",
		summary["total"], summary["passed"], summary["failed"], summary["success_rate"])

	// Exit with non-zero code if tests failed
	if summary["failed"].(int) > 0 {
		os.Exit(1)
	}
}

func generateConfig(args []string) {
	scenario := ""
	if len(args) > 0 {
		scenario = args[0]
	}

	testharness.PrintExampleConfig(scenario)
}

func validateConfig(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: configuration file path required")
		os.Exit(1)
	}

	configFile := args[0]
	config, err := testharness.LoadConfigFromFile(configFile)
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		os.Exit(1)
	}

	if err := testharness.ValidateConfig(config); err != nil {
		fmt.Printf("Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration file %s is valid\n", configFile)
}

// verboseLogger provides detailed logging output
type verboseLogger struct{}

func (l *verboseLogger) Info(msg string, fields ...interface{}) {
	fmt.Printf("[INFO] %s", msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			fmt.Printf(" %s=%v", fields[i], fields[i+1])
		}
	}
	fmt.Println()
}

func (l *verboseLogger) Warn(msg string, fields ...interface{}) {
	fmt.Printf("[WARN] %s", msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			fmt.Printf(" %s=%v", fields[i], fields[i+1])
		}
	}
	fmt.Println()
}

func (l *verboseLogger) Error(msg string, fields ...interface{}) {
	fmt.Printf("[ERROR] %s", msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			fmt.Printf(" %s=%v", fields[i], fields[i+1])
		}
	}
	fmt.Println()
}

func (l *verboseLogger) Debug(msg string, fields ...interface{}) {
	fmt.Printf("[DEBUG] %s", msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			fmt.Printf(" %s=%v", fields[i], fields[i+1])
		}
	}
	fmt.Println()
}