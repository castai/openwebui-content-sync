package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_DefaultConfig(t *testing.T) {
	// Test loading with non-existent file (should use defaults)
	cfg, err := Load("non-existent-config.yaml")
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
	if cfg.Storage.Path != "/data" {
		t.Errorf("Expected storage path '/data', got '%s'", cfg.Storage.Path)
	}
	if cfg.OpenWebUI.BaseURL != "http://localhost:8080" {
		t.Errorf("Expected OpenWebUI base URL 'http://localhost:8080', got '%s'", cfg.OpenWebUI.BaseURL)
	}
	if cfg.GitHub.Enabled != false {
		t.Errorf("Expected GitHub enabled false, got %v", cfg.GitHub.Enabled)
	}
}

func TestLoad_FromFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
log_level: debug
schedule:
  interval: 2h
storage:
  path: /custom/data
openwebui:
  base_url: "https://custom.openwebui.com"
  api_key: "custom-api-key"
github:
  enabled: true
  token: "custom-token"
  mappings:
    - repository: "owner/repo1"
      knowledge_id: "custom-knowledge-id"
    - repository: "owner/repo2"
      knowledge_id: "custom-knowledge-id"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	// Check loaded values
	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.LogLevel)
	}
	if cfg.Schedule.Interval != 2*time.Hour {
		t.Errorf("Expected schedule interval 2h, got %v", cfg.Schedule.Interval)
	}
	if cfg.Storage.Path != "/custom/data" {
		t.Errorf("Expected storage path '/custom/data', got '%s'", cfg.Storage.Path)
	}
	if cfg.OpenWebUI.BaseURL != "https://custom.openwebui.com" {
		t.Errorf("Expected OpenWebUI base URL 'https://custom.openwebui.com', got '%s'", cfg.OpenWebUI.BaseURL)
	}
	if cfg.OpenWebUI.APIKey != "custom-api-key" {
		t.Errorf("Expected OpenWebUI API key 'custom-api-key', got '%s'", cfg.OpenWebUI.APIKey)
	}
	if cfg.GitHub.Enabled != true {
		t.Errorf("Expected GitHub enabled true, got %v", cfg.GitHub.Enabled)
	}
	if cfg.GitHub.Token != "custom-token" {
		t.Errorf("Expected GitHub token 'custom-token', got '%s'", cfg.GitHub.Token)
	}
	if len(cfg.GitHub.Mappings) != 2 {
		t.Errorf("Expected 2 repository mappings, got %d", len(cfg.GitHub.Mappings))
	}
	if cfg.GitHub.Mappings[0].Repository != "owner/repo1" {
		t.Errorf("Expected first repository 'owner/repo1', got '%s'", cfg.GitHub.Mappings[0].Repository)
	}
	if cfg.GitHub.Mappings[0].KnowledgeID != "custom-knowledge-id" {
		t.Errorf("Expected first knowledge ID 'custom-knowledge-id', got '%s'", cfg.GitHub.Mappings[0].KnowledgeID)
	}
}

func TestLoad_EnvironmentOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("OPENWEBUI_BASE_URL", "https://env.openwebui.com")
	os.Setenv("OPENWEBUI_API_KEY", "env-api-key")
	os.Setenv("GITHUB_TOKEN", "env-github-token")
	os.Setenv("GITHUB_KNOWLEDGE_ID", "env-knowledge-id")
	os.Setenv("STORAGE_PATH", "/env/storage")
	defer func() {
		os.Unsetenv("OPENWEBUI_BASE_URL")
		os.Unsetenv("OPENWEBUI_API_KEY")
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("GITHUB_KNOWLEDGE_ID")
		os.Unsetenv("STORAGE_PATH")
	}()

	cfg, err := Load("non-existent-config.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check environment overrides
	if cfg.OpenWebUI.BaseURL != "https://env.openwebui.com" {
		t.Errorf("Expected OpenWebUI base URL 'https://env.openwebui.com', got '%s'", cfg.OpenWebUI.BaseURL)
	}
	if cfg.OpenWebUI.APIKey != "env-api-key" {
		t.Errorf("Expected OpenWebUI API key 'env-api-key', got '%s'", cfg.OpenWebUI.APIKey)
	}
	if cfg.GitHub.Token != "env-github-token" {
		t.Errorf("Expected GitHub token 'env-github-token', got '%s'", cfg.GitHub.Token)
	}
	// Note: GitHub knowledge ID is now handled via mappings, not environment variables
	if cfg.Storage.Path != "/env/storage" {
		t.Errorf("Expected storage path '/env/storage', got '%s'", cfg.Storage.Path)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create temporary config file with invalid YAML
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-config.yaml")

	invalidYAML := `
log_level: debug
schedule:
  interval: 2h
  invalid: [unclosed list
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Errorf("Expected error for invalid YAML, got none")
	}
}

func TestLoad_FileAndEnvironment(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
log_level: debug
openwebui:
  base_url: "https://file.openwebui.com"
  api_key: "file-api-key"
github:
  token: "file-token"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set environment variables (should override file values)
	os.Setenv("OPENWEBUI_BASE_URL", "https://env.openwebui.com")
	os.Setenv("GITHUB_TOKEN", "env-token")
	defer func() {
		os.Unsetenv("OPENWEBUI_BASE_URL")
		os.Unsetenv("GITHUB_TOKEN")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Environment should override file values
	if cfg.OpenWebUI.BaseURL != "https://env.openwebui.com" {
		t.Errorf("Expected environment to override file value, got '%s'", cfg.OpenWebUI.BaseURL)
	}
	if cfg.GitHub.Token != "env-token" {
		t.Errorf("Expected environment to override file value, got '%s'", cfg.GitHub.Token)
	}

	// File values should be used where environment is not set
	if cfg.OpenWebUI.APIKey != "file-api-key" {
		t.Errorf("Expected file value to be used, got '%s'", cfg.OpenWebUI.APIKey)
	}
}

func TestGetEnv(t *testing.T) {
	// Test with existing environment variable
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected 'test-value', got '%s'", result)
	}

	// Test with non-existing environment variable
	result = getEnv("NON_EXISTING_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}

	// Test with empty environment variable
	os.Setenv("EMPTY_VAR", "")
	defer os.Unsetenv("EMPTY_VAR")

	result = getEnv("EMPTY_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default' for empty env var, got '%s'", result)
	}
}

func TestConfig_StructFields(t *testing.T) {
	cfg := &Config{
		LogLevel: "debug",
		Schedule: ScheduleConfig{
			Interval: 2 * time.Hour,
		},
		Storage: StorageConfig{
			Path: "/test/path",
		},
		OpenWebUI: OpenWebUIConfig{
			BaseURL: "https://test.com",
			APIKey:  "test-key",
		},
		GitHub: GitHubConfig{
			Enabled: true,
			Token:   "github-token",
			Mappings: []RepositoryMapping{
				{Repository: "owner/repo", KnowledgeID: "knowledge-id"},
			},
		},
	}

	// Test that all fields can be set and accessed
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel not set correctly")
	}
	if cfg.Schedule.Interval != 2*time.Hour {
		t.Errorf("Schedule.Interval not set correctly")
	}
	if cfg.Storage.Path != "/test/path" {
		t.Errorf("Storage.Path not set correctly")
	}
	if cfg.OpenWebUI.BaseURL != "https://test.com" {
		t.Errorf("OpenWebUI.BaseURL not set correctly")
	}
	if cfg.OpenWebUI.APIKey != "test-key" {
		t.Errorf("OpenWebUI.APIKey not set correctly")
	}
	if cfg.GitHub.Enabled != true {
		t.Errorf("GitHub.Enabled not set correctly")
	}
	if cfg.GitHub.Token != "github-token" {
		t.Errorf("GitHub.Token not set correctly")
	}
	if len(cfg.GitHub.Mappings) != 1 {
		t.Errorf("GitHub.Mappings not set correctly")
	}
	if cfg.GitHub.Mappings[0].Repository != "owner/repo" {
		t.Errorf("GitHub.Mappings[0].Repository not set correctly")
	}
	if cfg.GitHub.Mappings[0].KnowledgeID != "knowledge-id" {
		t.Errorf("GitHub.Mappings[0].KnowledgeID not set correctly")
	}
}
