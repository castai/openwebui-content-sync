package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/config"
)

func TestGitHubAdapter_Name(t *testing.T) {
	adapter := &GitHubAdapter{}
	if adapter.Name() != "github" {
		t.Errorf("Expected name 'github', got '%s'", adapter.Name())
	}
}

func TestGitHubAdapter_GetSetLastSync(t *testing.T) {
	adapter := &GitHubAdapter{}
	now := time.Now()

	adapter.SetLastSync(now)
	if !adapter.GetLastSync().Equal(now) {
		t.Errorf("Expected last sync time %v, got %v", now, adapter.GetLastSync())
	}
}

func TestNewGitHubAdapter(t *testing.T) {
	tests := []struct {
		name        string
		config      config.GitHubConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: config.GitHubConfig{
				Token: "test-token",
				Mappings: []config.RepositoryMapping{
					{Repository: "owner/repo", KnowledgeID: "knowledge-id"},
				},
			},
			expectError: false,
		},
		{
			name: "missing token",
			config: config.GitHubConfig{
				Token: "",
				Mappings: []config.RepositoryMapping{
					{Repository: "owner/repo", KnowledgeID: "knowledge-id"},
				},
			},
			expectError: true,
		},
		{
			name: "no mappings",
			config: config.GitHubConfig{
				Token:    "test-token",
				Mappings: []config.RepositoryMapping{},
			},
			expectError: true,
		},
		{
			name: "invalid repository format",
			config: config.GitHubConfig{
				Token: "test-token",
				Mappings: []config.RepositoryMapping{
					{Repository: "invalid-repo", KnowledgeID: "knowledge-id"},
				},
			},
			expectError: false, // This will fail later during fetch
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewGitHubAdapter(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if adapter == nil {
				t.Errorf("Expected adapter but got nil")
			}
		})
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.md", true},
		{"test.txt", true},
		{"test.go", true},
		{"test.py", true},
		{"test.js", true},
		{"test.ts", true},
		{"test.json", true},
		{"test.yaml", true},
		{"test.yml", true},
		{"test.xml", true},
		{"test.html", true},
		{"test.css", true},
		{"test.sh", true},
		{"test.dockerfile", true},
		{"test.gitignore", true},
		{"test.env", true},
		{"test.png", false},
		{"test.jpg", false},
		{"test.jpeg", false},
		{"test.gif", false},
		{"test.exe", false},
		{"test.dll", false},
		{"test.so", false},
		{"test.dylib", false},
		{"test", true},     // No extension should be considered text
		{"test.TXT", true}, // Case insensitive
		{"test.MD", true},  // Case insensitive
	}

	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			result := isTextFile(test.filename)
			if result != test.expected {
				t.Errorf("isTextFile(%s) = %v, expected %v", test.filename, result, test.expected)
			}
		})
	}
}

func TestGitHubAdapter_FetchFiles(t *testing.T) {
	// This test would require mocking the GitHub API
	// For now, we'll test the error cases
	config := config.GitHubConfig{
		Token: "invalid-token",
		Mappings: []config.RepositoryMapping{
			{Repository: "nonexistent/owner", KnowledgeID: "knowledge-id"},
		},
	}

	adapter, err := NewGitHubAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	ctx := context.Background()
	_, err = adapter.FetchFiles(ctx)
	if err == nil {
		t.Errorf("Expected error for invalid repository, got none")
	}
}

func TestFile_String(t *testing.T) {
	file := &File{
		Path:     "test.md",
		Hash:     "abc123",
		Size:     100,
		Source:   "github",
		Modified: time.Now(),
	}

	// Test that File struct can be created and accessed
	if file.Path != "test.md" {
		t.Errorf("Expected path 'test.md', got '%s'", file.Path)
	}
	if file.Hash != "abc123" {
		t.Errorf("Expected hash 'abc123', got '%s'", file.Hash)
	}
	if file.Size != 100 {
		t.Errorf("Expected size 100, got %d", file.Size)
	}
	if file.Source != "github" {
		t.Errorf("Expected source 'github', got '%s'", file.Source)
	}
}
