package testharness

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		TestTimeout:       30 * time.Second,
		ServerTimeout:     10 * time.Second,
		MaxConcurrency:    5,
		RetryCount:        3,
		WarmupDuration:    1 * time.Second,
		LoadTestDuration:  30 * time.Second,
		LoadTestRPS:       10,
		MemoryThreshold:   100, // MB
		CPUThreshold:      80.0, // percent
		StrictCompliance:  true,
		RequiredMethods:   []string{"resources/list", "tools/list"},
		OptionalMethods:   []string{"prompts/list", "roots/list"},
		OutputFormat:      "console",
		DetailedResults:   true,
		ContinueOnFailure: true,
	}
}

// LoadConfigFromFile loads test configuration from a JSON file
func LoadConfigFromFile(filename string) (TestConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return TestConfig{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return TestConfig{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing fields
	defaultConfig := DefaultTestConfig()
	if config.TestTimeout == 0 {
		config.TestTimeout = defaultConfig.TestTimeout
	}
	if config.ServerTimeout == 0 {
		config.ServerTimeout = defaultConfig.ServerTimeout
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = defaultConfig.MaxConcurrency
	}
	if config.RetryCount == 0 {
		config.RetryCount = defaultConfig.RetryCount
	}
	if config.OutputFormat == "" {
		config.OutputFormat = defaultConfig.OutputFormat
	}

	return config, nil
}

// SaveConfigToFile saves test configuration to a JSON file
func SaveConfigToFile(config TestConfig, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ValidateConfig validates a test configuration
func ValidateConfig(config TestConfig) error {
	if len(config.ServerCommand) == 0 {
		return fmt.Errorf("server_command is required")
	}

	if config.TestTimeout <= 0 {
		return fmt.Errorf("test_timeout must be positive")
	}

	if config.ServerTimeout <= 0 {
		return fmt.Errorf("server_timeout must be positive")
	}

	if config.MaxConcurrency <= 0 {
		return fmt.Errorf("max_concurrency must be positive")
	}

	if config.RetryCount < 0 {
		return fmt.Errorf("retry_count cannot be negative")
	}

	validFormats := map[string]bool{
		"console": true,
		"json":    true,
		"junit":   true,
	}
	if !validFormats[config.OutputFormat] {
		return fmt.Errorf("invalid output_format: %s", config.OutputFormat)
	}

	return nil
}

// ExampleConfigs returns example configurations for different scenarios
func ExampleConfigs() map[string]TestConfig {
	configs := make(map[string]TestConfig)

	// Basic testing
	basic := DefaultTestConfig()
	basic.ServerCommand = []string{"./my-mcp-server"}
	basic.LoadTestDuration = 10 * time.Second
	configs["basic"] = basic

	// Performance testing
	performance := DefaultTestConfig()
	performance.ServerCommand = []string{"./my-mcp-server"}
	performance.LoadTestDuration = 60 * time.Second
	performance.LoadTestRPS = 50
	performance.MaxConcurrency = 20
	performance.MemoryThreshold = 200
	performance.CPUThreshold = 90.0
	configs["performance"] = performance

	// Compliance testing
	compliance := DefaultTestConfig()
	compliance.ServerCommand = []string{"./my-mcp-server"}
	compliance.StrictCompliance = true
	compliance.RequiredMethods = []string{
		"resources/list", "resources/read",
		"tools/list", "tools/call",
		"prompts/list", "prompts/get",
	}
	compliance.OptionalMethods = []string{
		"roots/list",
		"sampling/createMessage",
	}
	configs["compliance"] = compliance

	// Development testing
	development := DefaultTestConfig()
	development.ServerCommand = []string{"go", "run", "./cmd/server"}
	development.WorkingDirectory = "/path/to/project"
	development.Environment = map[string]string{
		"DEBUG": "true",
		"LOG_LEVEL": "debug",
	}
	development.ContinueOnFailure = true
	development.DetailedResults = true
	configs["development"] = development

	// CI/CD testing
	cicd := DefaultTestConfig()
	cicd.ServerCommand = []string{"./mcp-server"}
	cicd.OutputFormat = "junit"
	cicd.OutputFile = "test-results.xml"
	cicd.ContinueOnFailure = false
	cicd.StrictCompliance = true
	configs["cicd"] = cicd

	return configs
}

// PrintExampleConfig prints an example configuration file
func PrintExampleConfig(scenario string) {
	configs := ExampleConfigs()
	
	var config TestConfig
	var ok bool
	if scenario == "" {
		config = DefaultTestConfig()
		config.ServerCommand = []string{"./my-mcp-server"}
	} else {
		config, ok = configs[scenario]
		if !ok {
			fmt.Printf("Unknown scenario: %s\n", scenario)
			fmt.Println("Available scenarios:")
			for name := range configs {
				fmt.Printf("  %s\n", name)
			}
			return
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Error generating config: %v\n", err)
		return
	}

	fmt.Println(string(data))
}