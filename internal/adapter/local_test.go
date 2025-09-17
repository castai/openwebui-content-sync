package adapter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/config"
)

func TestNewLocalFolderAdapter(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		config  config.LocalFolderConfig
		wantErr bool
	}{
		{
			name: "valid mapping configuration",
			config: config.LocalFolderConfig{
				Enabled: true,
				Mappings: []config.LocalFolderMapping{
					{FolderPath: tempDir, KnowledgeID: "test-knowledge"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with multiple mappings",
			config: func() config.LocalFolderConfig {
				// Create subdirectory for the test
				subDir := filepath.Join(tempDir, "subdir")
				os.MkdirAll(subDir, 0755)
				return config.LocalFolderConfig{
					Enabled: true,
					Mappings: []config.LocalFolderMapping{
						{FolderPath: tempDir, KnowledgeID: "test-knowledge"},
						{FolderPath: subDir, KnowledgeID: "another-knowledge"},
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "disabled adapter",
			config: config.LocalFolderConfig{
				Enabled: false,
			},
			wantErr: true,
		},
		{
			name: "non-existent folder",
			config: config.LocalFolderConfig{
				Enabled: true,
				Mappings: []config.LocalFolderMapping{
					{FolderPath: "/non/existent/path", KnowledgeID: "test-knowledge"},
				},
			},
			wantErr: true,
		},
		{
			name: "no folders configured",
			config: config.LocalFolderConfig{
				Enabled: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewLocalFolderAdapter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLocalFolderAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Error("NewLocalFolderAdapter() returned nil adapter when no error expected")
			}
		})
	}
}

func TestLocalFolderAdapter_Name(t *testing.T) {
	adapter := &LocalFolderAdapter{}
	if got := adapter.Name(); got != "local" {
		t.Errorf("Name() = %v, want %v", got, "local")
	}
}

func TestLocalFolderAdapter_GetSetLastSync(t *testing.T) {
	adapter := &LocalFolderAdapter{}

	// Test initial last sync
	initialSync := adapter.GetLastSync()
	if !initialSync.IsZero() {
		t.Error("GetLastSync() should return zero time initially")
	}

	// Test setting last sync
	newTime := time.Now()
	adapter.SetLastSync(newTime)
	if !adapter.GetLastSync().Equal(newTime) {
		t.Errorf("SetLastSync() did not update last sync time")
	}
}

func TestLocalFolderAdapter_FetchFiles(t *testing.T) {
	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"test1.txt":        "content 1",
		"test2.md":         "content 2",
		"subdir/test3.txt": "content 3",
		".hidden.txt":      "hidden content",              // Should be ignored
		"binary.bin":       string([]byte{0, 1, 2, 3, 4}), // Should be ignored
	}

	for filename, content := range testFiles {
		fullPath := filepath.Join(tempDir, filename)
		dir := filepath.Dir(fullPath)
		if dir != tempDir {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				t.Fatalf("Failed to create subdirectory: %v", err)
			}
		}
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	config := config.LocalFolderConfig{
		Enabled: true,
		Mappings: []config.LocalFolderMapping{
			{FolderPath: tempDir, KnowledgeID: "test-knowledge"},
		},
	}

	adapter, err := NewLocalFolderAdapter(config)
	if err != nil {
		t.Fatalf("NewLocalFolderAdapter() error = %v", err)
	}

	ctx := context.Background()
	files, err := adapter.FetchFiles(ctx)
	if err != nil {
		t.Fatalf("FetchFiles() error = %v", err)
	}

	// Should find 3 files (test1.txt, test2.md, subdir/test3.txt)
	// Hidden and binary files should be ignored
	expectedCount := 3
	if len(files) != expectedCount {
		t.Errorf("FetchFiles() returned %d files, want %d", len(files), expectedCount)
	}

	// Check that all files have the correct knowledge ID
	for _, file := range files {
		if file.KnowledgeID != "test-knowledge" {
			t.Errorf("File %s has knowledge ID %s, want %s", file.Path, file.KnowledgeID, "test-knowledge")
		}
		if file.Source != "local:"+tempDir {
			t.Errorf("File %s has source %s, want %s", file.Path, file.Source, "local:"+tempDir)
		}
	}
}

func TestLocalFolderAdapter_shouldIgnoreFile(t *testing.T) {
	adapter := &LocalFolderAdapter{}

	tests := []struct {
		filename string
		want     bool
	}{
		{"test.txt", false},
		{".hidden", true},
		{"node_modules", true},
		{"test.log", true},
		{"Thumbs.db", true},
		{"normal_file.py", false},
		{"__pycache__", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := adapter.shouldIgnoreFile(tt.filename); got != tt.want {
				t.Errorf("shouldIgnoreFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestLocalFolderAdapter_isBinaryFile(t *testing.T) {
	adapter := &LocalFolderAdapter{}

	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"text with newlines", []byte("hello\nworld\r\n"), false},
		{"binary with null", []byte{0, 1, 2, 3}, true},
		{"high non-printable ratio", make([]byte, 1000), true},
		{"normal text", []byte("This is normal text content"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adapter.isBinaryFile(tt.content); got != tt.want {
				t.Errorf("isBinaryFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
