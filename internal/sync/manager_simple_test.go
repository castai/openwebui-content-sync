package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/config"
	"github.com/openwebui-content-sync/internal/mocks"
	"github.com/openwebui-content-sync/internal/openwebui"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	openwebuiConfig := config.OpenWebUIConfig{
		BaseURL: "http://localhost:8080",
		APIKey:  "test-key",
	}
	storageConfig := config.StorageConfig{
		Path: tempDir,
	}

	manager, err := NewManager(openwebuiConfig, storageConfig)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.storagePath != tempDir {
		t.Errorf("Expected storage path %s, got %s", tempDir, manager.storagePath)
	}
}

func TestManager_SetKnowledgeID(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	manager := &Manager{
		storagePath: tempDir,
		fileIndex:   make(map[string]*FileMetadata),
	}

	knowledgeID := "test-knowledge-id"
	manager.SetKnowledgeID(knowledgeID)

	if manager.knowledgeID != knowledgeID {
		t.Errorf("Expected knowledge ID %s, got %s", knowledgeID, manager.knowledgeID)
	}
}

func TestManager_syncFile_NewFile(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	mockClient := &mocks.MockOpenWebUIClient{
		UploadFileFunc: func(ctx context.Context, filename string, content []byte) (*openwebui.File, error) {
			return &openwebui.File{
				ID:       "mock-file-id",
				Filename: filename,
			}, nil
		},
	}

	manager := &Manager{
		openwebuiClient: mockClient,
		storagePath:     tempDir,
		fileIndex:       make(map[string]*FileMetadata),
	}

	file := &adapter.File{
		Path:     "new-file.md",
		Content:  []byte("# New File"),
		Hash:     "test-hash",
		Modified: time.Now(),
		Size:     10,
		Source:   "test",
	}

	ctx := context.Background()
	err := manager.syncFile(ctx, file, "test-source")
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// Check that file was added to index
	fileKey := "new-file.md" // Now using filename as key
	if _, exists := manager.fileIndex[fileKey]; !exists {
		t.Errorf("Expected file to be added to index")
	}

	// Check that file was saved locally
	expectedPath := filepath.Join(tempDir, "files", "test-source", "new-file.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file to be saved locally at %s", expectedPath)
	}
}

func TestManager_syncFile_UnchangedFile(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	mockClient := &mocks.MockOpenWebUIClient{}
	manager := &Manager{
		openwebuiClient: mockClient,
		storagePath:     tempDir,
		fileIndex:       make(map[string]*FileMetadata),
	}

	// Add file to index first
	fileKey := "unchanged-file.md" // Now using filename as key
	manager.fileIndex[fileKey] = &FileMetadata{
		Path:     "unchanged-file.md",
		Hash:     "same-hash",
		FileID:   "existing-file-id",
		Source:   "test-source",
		SyncedAt: time.Now(),
		Modified: time.Now(),
	}

	file := &adapter.File{
		Path:     "unchanged-file.md",
		Content:  []byte("# Unchanged File"),
		Hash:     "same-hash", // Same hash as in index
		Modified: time.Now(),
		Size:     17,
		Source:   "test",
	}

	ctx := context.Background()
	err := manager.syncFile(ctx, file, "test-source")
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// File should not be uploaded again (we can't easily test this without more complex mocking)
	// But we can verify the file index wasn't updated with a new file ID
	if manager.fileIndex[fileKey].FileID != "existing-file-id" {
		t.Errorf("Expected file ID to remain unchanged")
	}
}

func TestManager_saveFileLocally(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	manager := &Manager{
		storagePath: tempDir,
	}

	filePath := filepath.Join(tempDir, "test", "nested", "file.md")
	content := []byte("# Test Content")

	err := manager.saveFileLocally(filePath, content)
	if err != nil {
		t.Fatalf("Failed to save file locally: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Expected file to exist at %s", filePath)
	}

	// Check content
	readContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(readContent))
	}
}

func TestGetFileHash(t *testing.T) {
	content := []byte("test content")
	// Calculate the actual expected hash
	expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"

	hash := GetFileHash(content)
	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}
}

func TestManager_loadFileIndex(t *testing.T) {
	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	manager := &Manager{
		storagePath: tempDir,
		fileIndex:   make(map[string]*FileMetadata),
		indexPath:   filepath.Join(tempDir, "file_index.json"),
	}

	// Test loading non-existent index (should not error)
	err := manager.loadFileIndex()
	if err != nil {
		t.Fatalf("Failed to load non-existent index: %v", err)
	}

	// Create a test index file
	testIndex := map[string]*FileMetadata{
		"file.md": { // Now using filename as key
			Path:     "file.md",
			Hash:     "test-hash",
			FileID:   "test-file-id",
			Source:   "test",
			SyncedAt: time.Now(),
			Modified: time.Now(),
		},
	}

	// Save test index
	manager.fileIndex = testIndex
	err = manager.saveFileIndex()
	if err != nil {
		t.Fatalf("Failed to save test index: %v", err)
	}

	// Create new manager and load index
	newManager := &Manager{
		storagePath: tempDir,
		fileIndex:   make(map[string]*FileMetadata),
		indexPath:   filepath.Join(tempDir, "file_index.json"),
	}

	err = newManager.loadFileIndex()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	if len(newManager.fileIndex) != 1 {
		t.Errorf("Expected 1 file in index, got %d", len(newManager.fileIndex))
	}

	fileKey := "file.md" // Now using filename as key
	if _, exists := newManager.fileIndex[fileKey]; !exists {
		t.Errorf("Expected file %s to be in index", fileKey)
	}
}
