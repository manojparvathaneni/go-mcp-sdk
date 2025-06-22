package testharness

import (
	"testing"
	"time"
)

func TestTestHarness_Basic(t *testing.T) {
	config := DefaultTestConfig()
	config.ServerCommand = []string{"echo", "hello"}
	config.TestTimeout = 5 * time.Second
	config.ServerTimeout = 2 * time.Second

	harness := NewTestHarness(config)
	
	// Test configuration validation
	if err := ValidateConfig(config); err != nil {
		t.Errorf("Configuration validation failed: %v", err)
	}

	// Test summary functionality
	summary := harness.GetSummary()
	if summary["total"].(int) != 0 {
		t.Errorf("Expected 0 initial tests, got %d", summary["total"])
	}

	// Test result storage
	results := harness.GetResults()
	if len(results) != 0 {
		t.Errorf("Expected 0 initial results, got %d", len(results))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultTestConfig()
	
	// Verify default values
	if config.TestTimeout != 30*time.Second {
		t.Errorf("Expected default test timeout 30s, got %v", config.TestTimeout)
	}
	
	if config.MaxConcurrency != 5 {
		t.Errorf("Expected default max concurrency 5, got %d", config.MaxConcurrency)
	}
	
	if config.OutputFormat != "console" {
		t.Errorf("Expected default output format 'console', got %s", config.OutputFormat)
	}
	
	if !config.StrictCompliance {
		t.Error("Expected default strict compliance to be true")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  TestConfig
		wantErr bool
	}{
		{
			name:    "empty server command",
			config:  TestConfig{},
			wantErr: true,
		},
		{
			name: "negative test timeout",
			config: TestConfig{
				ServerCommand: []string{"test"},
				TestTimeout:   -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid output format",
			config: TestConfig{
				ServerCommand: []string{"test"},
				TestTimeout:   30 * time.Second,
				ServerTimeout: 10 * time.Second,
				MaxConcurrency: 5,
				OutputFormat:  "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: TestConfig{
				ServerCommand:  []string{"test"},
				TestTimeout:    30 * time.Second,
				ServerTimeout:  10 * time.Second,
				MaxConcurrency: 5,
				OutputFormat:   "console",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExampleConfigs(t *testing.T) {
	configs := ExampleConfigs()
	
	expectedScenarios := []string{"basic", "performance", "compliance", "development", "cicd"}
	
	for _, scenario := range expectedScenarios {
		config, exists := configs[scenario]
		if !exists {
			t.Errorf("Expected scenario %s not found", scenario)
			continue
		}
		
		if err := ValidateConfig(config); err != nil {
			t.Errorf("Example config %s is invalid: %v", scenario, err)
		}
	}
}

func TestResultSummary(t *testing.T) {
	harness := NewTestHarness(DefaultTestConfig())
	
	// Add some mock results
	harness.results = []TestResult{
		{Name: "test1", Status: StatusPassed, Duration: 100 * time.Millisecond},
		{Name: "test2", Status: StatusFailed, Duration: 200 * time.Millisecond},
		{Name: "test3", Status: StatusPassed, Duration: 150 * time.Millisecond},
		{Name: "test4", Status: StatusTimeout, Duration: 30 * time.Second},
	}
	
	summary := harness.GetSummary()
	
	if summary["total"].(int) != 4 {
		t.Errorf("Expected 4 total tests, got %d", summary["total"])
	}
	
	if summary["passed"].(int) != 2 {
		t.Errorf("Expected 2 passed tests, got %d", summary["passed"])
	}
	
	if summary["failed"].(int) != 1 {
		t.Errorf("Expected 1 failed test, got %d", summary["failed"])
	}
	
	if summary["timeout"].(int) != 1 {
		t.Errorf("Expected 1 timeout test, got %d", summary["timeout"])
	}
	
	successRate := summary["success_rate"].(float64)
	expectedRate := 50.0 // 2 passed out of 4 total = 50%
	if successRate != expectedRate {
		t.Errorf("Expected success rate %.1f%%, got %.1f%%", expectedRate, successRate)
	}
}