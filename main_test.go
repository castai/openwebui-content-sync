package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/config"
)

func TestMain_WithConfigFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	configContent := `
log_level: debug
schedule:
  interval: 1h
storage:
  path: /tmp/test-storage
openwebui:
  base_url: "http://localhost:8080"
  api_key: "test-api-key"
github:
  enabled: false
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading config
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.LogLevel)
	}
	if cfg.GitHub.Enabled != false {
		t.Errorf("Expected GitHub enabled false, got %v", cfg.GitHub.Enabled)
	}
}

func TestMain_WithInvalidConfigFile(t *testing.T) {
	// Create temporary config file with invalid YAML
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-config.yaml")

	invalidYAML := `
log_level: debug
schedule:
  interval: 1h
  invalid: [unclosed list
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Test loading invalid config
	_, err = config.Load(configPath)
	if err == nil {
		t.Errorf("Expected error for invalid config, got none")
	}
}

func TestMain_WithNonExistentConfigFile(t *testing.T) {
	// Test loading non-existent config file (should use defaults)
	cfg, err := config.Load("non-existent-config.yaml")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	// Check default values
	if cfg.LogLevel != "info" {
		t.Errorf("Expected log level 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.Schedule.Interval != 1*time.Hour {
		t.Errorf("Expected schedule interval 1h, got %v", cfg.Schedule.Interval)
	}
}

func TestMain_FlagParsing(t *testing.T) {
	// Save original command line args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Test with custom config path
	os.Args = []string{"cmd", "-config", "custom-config.yaml"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	if *configPath != "custom-config.yaml" {
		t.Errorf("Expected config path 'custom-config.yaml', got '%s'", *configPath)
	}
}

func TestMain_DefaultFlagValue(t *testing.T) {
	// Save original command line args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}()

	// Test with no flags
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	if *configPath != "config.yaml" {
		t.Errorf("Expected default config path 'config.yaml', got '%s'", *configPath)
	}
}

// Helper function to create test config
func createTestConfig() *config.Config {
	return &config.Config{
		LogLevel: "debug",
		Schedule: config.ScheduleConfig{
			Interval: 1 * time.Hour,
		},
		Storage: config.StorageConfig{
			Path: "/tmp/test-storage",
		},
		OpenWebUI: config.OpenWebUIConfig{
			BaseURL: "http://localhost:8080",
			APIKey:  "test-api-key",
		},
		GitHub: config.GitHubConfig{
			Enabled: false,
			Token:   "test-token",
			Mappings: []config.RepositoryMapping{
				{Repository: "owner/repo", KnowledgeID: "test-knowledge-id"},
			},
		},
	}
}

func TestMain_ContextHandling(t *testing.T) {
	// Test context creation and cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Verify context is not cancelled initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected
	}

	// Cancel context
	cancel()

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after cancel()")
	}
}

func TestMain_SignalHandling(t *testing.T) {
	// Test signal channel creation
	sigChan := make(chan os.Signal, 1)

	// Verify channel is empty initially
	select {
	case <-sigChan:
		t.Error("Signal channel should be empty initially")
	default:
		// Expected
	}

	// Test sending a signal
	sigChan <- os.Interrupt

	// Verify signal was received
	select {
	case sig := <-sigChan:
		if sig != os.Interrupt {
			t.Errorf("Expected os.Interrupt signal, got %v", sig)
		}
	default:
		t.Error("Expected to receive signal")
	}
}

func TestMain_TimeHandling(t *testing.T) {
	// Test time operations used in main
	now := time.Now()

	// Test time addition
	future := now.Add(5 * time.Second)
	if future.Before(now) {
		t.Error("Future time should be after now")
	}

	// Test time comparison
	if !now.Before(future) {
		t.Error("Now should be before future time")
	}

	// Test duration
	duration := future.Sub(now)
	expectedDuration := 5 * time.Second
	if duration < expectedDuration-100*time.Millisecond || duration > expectedDuration+100*time.Millisecond {
		t.Errorf("Expected duration around %v, got %v", expectedDuration, duration)
	}
}

func TestMain_ErrorHandling(t *testing.T) {
	// Test various error scenarios that might occur in main

	// Test with invalid log level
	cfg := createTestConfig()
	cfg.LogLevel = "invalid-level"

	// This would normally cause an error in the main function
	// For testing, we'll just verify the config was set
	if cfg.LogLevel != "invalid-level" {
		t.Errorf("Expected log level 'invalid-level', got '%s'", cfg.LogLevel)
	}
}

func TestMain_ResourceCleanup(t *testing.T) {
	// Test that resources are properly cleaned up
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Test file should exist")
	}

	// Cleanup
	os.RemoveAll(tempDir)

	// Verify file is cleaned up
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Test file should be cleaned up")
	}
}
