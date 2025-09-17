package sync

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/config"
	"github.com/openwebui-content-sync/internal/openwebui"
	"github.com/sirupsen/logrus"
)

// Manager handles synchronization between adapters and OpenWebUI
type Manager struct {
	openwebuiClient openwebui.ClientInterface
	storagePath     string
	knowledgeID     string
	fileIndex       map[string]*FileMetadata
	indexPath       string
}

// FileMetadata stores metadata about synced files
type FileMetadata struct {
	Path        string    `json:"path"`
	Hash        string    `json:"hash"`
	FileID      string    `json:"file_id"`
	Source      string    `json:"source"`
	KnowledgeID string    `json:"knowledge_id,omitempty"`
	SyncedAt    time.Time `json:"synced_at"`
	Modified    time.Time `json:"modified"`
}

// NewManager creates a new sync manager
func NewManager(openwebuiConfig config.OpenWebUIConfig, storageConfig config.StorageConfig) (*Manager, error) {
	client := openwebui.NewClient(openwebuiConfig.BaseURL, openwebuiConfig.APIKey)

	// Ensure storage directory exists
	if err := os.MkdirAll(storageConfig.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	indexPath := filepath.Join(storageConfig.Path, "file_index.json")

	manager := &Manager{
		openwebuiClient: client,
		storagePath:     storageConfig.Path,
		indexPath:       indexPath,
		fileIndex:       make(map[string]*FileMetadata),
	}

	// Load existing file index
	if err := manager.loadFileIndex(); err != nil {
		logrus.Warnf("Failed to load file index: %v", err)
	}

	return manager, nil
}

// SetKnowledgeID sets the knowledge ID for file operations
func (m *Manager) SetKnowledgeID(knowledgeID string) {
	logrus.Debugf("Setting knowledge ID: %s", knowledgeID)
	m.knowledgeID = knowledgeID
}

// InitializeFileIndex populates the file index with existing files from OpenWebUI
func (m *Manager) InitializeFileIndex(ctx context.Context, adapters []adapter.Adapter) error {
	// Collect all knowledge IDs that will be used by adapters
	knowledgeIDs := make(map[string]bool)

	// Add fallback knowledge ID if set
	if m.knowledgeID != "" {
		knowledgeIDs[m.knowledgeID] = true
	}

	// Collect knowledge IDs from adapters
	for _, adpt := range adapters {
		files, err := adpt.FetchFiles(ctx)
		if err != nil {
			logrus.Warnf("Failed to fetch files from adapter %s during initialization: %v", adpt.Name(), err)
			continue
		}

		for _, file := range files {
			if file.KnowledgeID != "" {
				knowledgeIDs[file.KnowledgeID] = true
			}
		}
	}

	if len(knowledgeIDs) == 0 {
		logrus.Debugf("No knowledge IDs found, skipping file index initialization")
		return nil
	}

	logrus.Info("Initializing file index from OpenWebUI knowledge bases...")

	// Create a new file index to replace the existing one
	newFileIndex := make(map[string]*FileMetadata)

	// Initialize file index for each knowledge base
	for knowledgeID := range knowledgeIDs {
		logrus.Debugf("Initializing file index for knowledge base: %s", knowledgeID)

		// Get files from the knowledge source
		files, err := m.openwebuiClient.GetKnowledgeFiles(ctx, knowledgeID)
		if err != nil {
			logrus.Warnf("Failed to get files from knowledge source %s: %v", knowledgeID, err)
			continue
		}

		logrus.Debugf("Found %d existing files in knowledge source %s", len(files), knowledgeID)

		// Add files to new index
		for _, file := range files {
			// Use filename as path if no path is available
			filePath := file.Path
			if filePath == "" {
				filePath = file.Meta.Name
			}

			// For existing files from OpenWebUI, use just the filename as the key
			// This avoids the "unknown:filename" issue and makes it easier to match
			// with files that will be synced from adapters
			fileKey := filePath

			// Use file ID as hash since OpenWebUI doesn't provide content hash
			// This means we won't detect content changes, but we can track file existence
			fileHash := file.ID
			if file.Hash != "" {
				fileHash = file.Hash
			}

			// Create file metadata
			metadata := &FileMetadata{
				Path:        filePath,
				Hash:        fileHash,
				FileID:      file.ID,
				Source:      "openwebui", // Mark as existing from OpenWebUI
				KnowledgeID: knowledgeID, // Set the specific knowledge ID
				SyncedAt:    time.Unix(file.CreatedAt, 0),
				Modified:    time.Unix(file.UpdatedAt, 0),
			}

			newFileIndex[fileKey] = metadata
			logrus.Debugf("Added existing file to index: %s (ID: %s, Hash: %s, Knowledge: %s)", filePath, file.ID, fileHash, knowledgeID)
		}
	}

	// Replace the old file index with the new one
	m.fileIndex = newFileIndex
	logrus.Infof("File index initialized with %d existing files from %d knowledge bases", len(m.fileIndex), len(knowledgeIDs))

	// Save the updated index
	if err := m.saveFileIndex(); err != nil {
		logrus.Warnf("Failed to save initialized file index: %v", err)
	}

	return nil
}

// SyncFiles synchronizes files from adapters to OpenWebUI
func (m *Manager) SyncFiles(ctx context.Context, adapters []adapter.Adapter) error {
	logrus.Info("Starting file synchronization")

	// List available knowledge sources for debugging
	logrus.Debugf("Listing available knowledge sources...")
	knowledgeList, err := m.openwebuiClient.ListKnowledge(ctx)
	if err != nil {
		logrus.Warnf("Failed to list knowledge sources: %v", err)
	} else {
		logrus.Debugf("Available knowledge sources:")
		for _, knowledge := range knowledgeList {
			logrus.Debugf("  - ID: %s, Name: %s, Description: %s", knowledge.ID, knowledge.Name, knowledge.Description)
		}
	}

	// Track files that are currently present in repositories
	currentFiles := make(map[string]bool)

	for _, adpt := range adapters {
		logrus.Infof("Syncing files from adapter: %s", adpt.Name())

		files, err := adpt.FetchFiles(ctx)
		if err != nil {
			logrus.Errorf("Failed to fetch files from adapter %s: %v", adpt.Name(), err)
			continue
		}

		logrus.Debugf("Fetched %d files from adapter %s", len(files), adpt.Name())

		for _, file := range files {
			filename := filepath.Base(file.Path)
			currentFiles[filename] = true // Track by filename to match OpenWebUI behavior

			if err := m.syncFile(ctx, file, adpt.Name()); err != nil {
				logrus.Errorf("Failed to sync file %s: %v", file.Path, err)
				continue
			}
		}

		// Update last sync time
		adpt.SetLastSync(time.Now())
	}

	// Clean up orphaned files (files that are no longer in repositories)
	if err := m.cleanupOrphanedFiles(ctx, currentFiles); err != nil {
		logrus.Errorf("Failed to cleanup orphaned files: %v", err)
	}

	// Save updated file index
	if err := m.saveFileIndex(); err != nil {
		logrus.Errorf("Failed to save file index: %v", err)
	}

	logrus.Info("File synchronization completed")
	return nil
}

// syncFile synchronizes a single file
func (m *Manager) syncFile(ctx context.Context, file *adapter.File, source string) error {
	filename := filepath.Base(file.Path)

	// Find existing file by multiple criteria
	var existing *FileMetadata
	var exists bool
	var matchReason string

	// First, try to find by exact filename match
	if existing, exists = m.fileIndex[filename]; exists {
		matchReason = "filename"
	} else {
		// If not found by filename, search by hash to find potential matches
		for _, metadata := range m.fileIndex {
			if metadata.Hash == file.Hash {
				existing = metadata
				exists = true
				matchReason = "hash"
				break
			}
		}
	}

	if exists {
		logrus.Debugf("Found existing file %s by %s (existing: %s, new: %s)", filename, matchReason, existing.Path, file.Path)

		// Check if it's the same content
		if existing.Hash == file.Hash {
			logrus.Debugf("File %s unchanged, skipping", file.Path)
			return nil
		}
		logrus.Infof("File %s has changed, updating", file.Path)
	}

	if exists {
		// Check if the file is already in the correct knowledge base
		fileKnowledgeID := file.KnowledgeID
		if fileKnowledgeID == "" {
			fileKnowledgeID = m.knowledgeID
		}

		existingKnowledgeID := existing.KnowledgeID
		if existingKnowledgeID == "" {
			existingKnowledgeID = m.knowledgeID
		}

		// If the file exists in the same knowledge base, check if it needs updating
		if existingKnowledgeID == fileKnowledgeID {
			// For files from OpenWebUI (source: "openwebui"), we can't compare content hashes
			// since OpenWebUI doesn't provide them. We should always upload from adapters
			// to ensure the content is up to date.
			if existing.Source == "openwebui" {
				logrus.Debugf("File %s exists in OpenWebUI knowledge %s, will upload to ensure content is up to date", file.Path, fileKnowledgeID)
				// Continue with upload to ensure content is current
			} else {
				// For files from adapters, compare content hashes
				if existing.Hash == file.Hash {
					logrus.Debugf("File %s unchanged, skipping", file.Path)
					return nil
				}
				logrus.Infof("File %s has changed, updating", file.Path)
			}

			// Remove old file from knowledge if knowledge ID is set
			if fileKnowledgeID != "" && existing.FileID != "" {
				logrus.Debugf("Removing old file %s from knowledge %s", existing.FileID, fileKnowledgeID)
				if err := m.openwebuiClient.RemoveFileFromKnowledge(ctx, fileKnowledgeID, existing.FileID); err != nil {
					logrus.Warnf("Failed to remove old file from knowledge: %v", err)
					// Continue with upload even if removal fails
				} else {
					logrus.Debugf("Successfully removed old file from knowledge")
				}
			}
		} else {
			// File exists in a different knowledge base, we need to upload it to the new one
			logrus.Debugf("File %s exists in different knowledge base (%s -> %s), uploading to new knowledge base", file.Path, existingKnowledgeID, fileKnowledgeID)
		}
	}

	// Save file to local storage
	localPath := filepath.Join(m.storagePath, "files", source, file.Path)
	if err := m.saveFileLocally(localPath, file.Content); err != nil {
		return fmt.Errorf("failed to save file locally: %w", err)
	}

	// Upload to OpenWebUI
	logrus.Debugf("Starting file upload to OpenWebUI for: %s", file.Path)
	uploadedFile, err := m.openwebuiClient.UploadFile(ctx, filepath.Base(file.Path), file.Content)
	if err != nil {
		return fmt.Errorf("failed to upload file to OpenWebUI: %w", err)
	}

	logrus.Debugf("File uploaded successfully: ID=%s, Filename=%s", uploadedFile.ID, uploadedFile.Filename)

	// Add to knowledge if knowledge ID is set (use file's knowledge ID if available, otherwise manager's)
	knowledgeID := file.KnowledgeID
	if knowledgeID == "" {
		knowledgeID = m.knowledgeID
	}

	if knowledgeID != "" {
		logrus.Debugf("Adding file %s to knowledge %s", uploadedFile.ID, knowledgeID)
		if err := m.openwebuiClient.AddFileToKnowledge(ctx, knowledgeID, uploadedFile.ID); err != nil {
			logrus.Errorf("Failed to add file to knowledge: %v", err)
			return fmt.Errorf("failed to add file to knowledge: %w", err)
		}
		logrus.Debugf("File successfully added to knowledge")
	} else {
		logrus.Warnf("No knowledge ID set, file uploaded but not added to any knowledge base")
	}

	// Update file index - only if file doesn't exist or was updated
	if !exists || existing.Hash != file.Hash {
		// Use filename as the key to match OpenWebUI behavior
		key := filepath.Base(file.Path)

		// If we found an existing file by hash but with different filename, update the key
		if exists && matchReason == "hash" && existing.Path != file.Path {
			// Remove the old entry and add with new key
			delete(m.fileIndex, filepath.Base(existing.Path))
			logrus.Debugf("Updating file key from %s to %s", filepath.Base(existing.Path), key)
		}

		m.fileIndex[key] = &FileMetadata{
			Path:        file.Path, // Store full path in metadata
			Hash:        file.Hash,
			FileID:      uploadedFile.ID,
			Source:      source,
			KnowledgeID: knowledgeID,
			SyncedAt:    time.Now(),
			Modified:    file.Modified,
		}
		logrus.Debugf("Updated file index with file: %s (ID: %s, key: %s)", file.Path, uploadedFile.ID, key)
	} else {
		logrus.Debugf("File %s already exists and unchanged, keeping existing metadata", file.Path)
	}

	logrus.Debugf("File index now contains %d files", len(m.fileIndex))

	logrus.Infof("Successfully synced file: %s", file.Path)
	return nil
}

// cleanupOrphanedFiles removes files from OpenWebUI that are no longer present in repositories
func (m *Manager) cleanupOrphanedFiles(ctx context.Context, currentFiles map[string]bool) error {
	logrus.Debugf("Checking for orphaned files...")

	var orphanedFiles []string
	for fileKey, metadata := range m.fileIndex {
		// Check if file is still present in current files
		filename := filepath.Base(metadata.Path)
		if filename == "" {
			filename = filepath.Base(fileKey)
		}

		// A file is orphaned if:
		// 1. It's not in the current files list by filename
		// 2. It's from OpenWebUI (not from an adapter)
		// 3. It has a valid file ID (can be removed)
		if !currentFiles[filename] && metadata.Source == "openwebui" && metadata.FileID != "" {
			orphanedFiles = append(orphanedFiles, fileKey)
			logrus.Debugf("Marking file as orphaned: %s (filename: %s, source: %s)", fileKey, filename, metadata.Source)
		} else if !currentFiles[filename] {
			logrus.Debugf("File not in current files but keeping: %s (filename: %s, source: %s, fileID: %s)", fileKey, filename, metadata.Source, metadata.FileID)
		}
	}

	if len(orphanedFiles) == 0 {
		logrus.Debugf("No orphaned files found")
		return nil
	}

	logrus.Infof("Found %d orphaned files to remove", len(orphanedFiles))

	for _, fileKey := range orphanedFiles {
		metadata := m.fileIndex[fileKey]

		// Remove from knowledge if knowledge ID is set (use file's knowledge ID if available, otherwise manager's)
		knowledgeID := metadata.KnowledgeID
		if knowledgeID == "" {
			knowledgeID = m.knowledgeID
		}

		if knowledgeID != "" && metadata.FileID != "" {
			logrus.Debugf("Removing orphaned file %s (ID: %s) from knowledge %s", metadata.Path, metadata.FileID, knowledgeID)
			if err := m.openwebuiClient.RemoveFileFromKnowledge(ctx, knowledgeID, metadata.FileID); err != nil {
				logrus.Warnf("Failed to remove orphaned file from knowledge: %v", err)
				// Continue with other files even if one fails
			} else {
				logrus.Debugf("Successfully removed orphaned file from knowledge")
			}
		} else {
			logrus.Debugf("Skipping orphaned file %s - no knowledge ID or file ID available", metadata.Path)
		}

		// Remove from file index
		delete(m.fileIndex, fileKey)
		logrus.Infof("Removed orphaned file: %s", metadata.Path)
	}

	return nil
}

// saveFileLocally saves a file to the local storage
func (m *Manager) saveFileLocally(path string, content []byte) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// loadFileIndex loads the file index from disk
func (m *Manager) loadFileIndex() error {
	if _, err := os.Stat(m.indexPath); os.IsNotExist(err) {
		return nil // Index doesn't exist yet
	}

	data, err := os.ReadFile(m.indexPath)
	if err != nil {
		return fmt.Errorf("failed to read file index: %w", err)
	}

	if err := json.Unmarshal(data, &m.fileIndex); err != nil {
		return fmt.Errorf("failed to unmarshal file index: %w", err)
	}

	return nil
}

// saveFileIndex saves the file index to disk
func (m *Manager) saveFileIndex() error {
	logrus.Debugf("Saving file index to: %s", m.indexPath)
	logrus.Debugf("File index contains %d files", len(m.fileIndex))

	data, err := json.MarshalIndent(m.fileIndex, "", "  ")
	if err != nil {
		logrus.Errorf("Failed to marshal file index: %v", err)
		return fmt.Errorf("failed to marshal file index: %w", err)
	}

	logrus.Debugf("File index JSON size: %d bytes", len(data))

	if err := os.WriteFile(m.indexPath, data, 0644); err != nil {
		logrus.Errorf("Failed to write file index to %s: %v", m.indexPath, err)
		return fmt.Errorf("failed to write file index: %w", err)
	}

	logrus.Debugf("Successfully saved file index to: %s", m.indexPath)
	return nil
}

// GetFileHash calculates the hash of a file
func GetFileHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}
