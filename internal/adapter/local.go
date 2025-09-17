// OpenWebUI Content Sync
// Copyright (C) 2025  OpenWebUI Content Sync Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package adapter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openwebui-content-sync/internal/config"
	"github.com/sirupsen/logrus"
)

// LocalFolderAdapter implements the Adapter interface for local folders
type LocalFolderAdapter struct {
	config   config.LocalFolderConfig
	lastSync time.Time
	folders  []string
	mappings map[string]string // folder_path -> knowledge_id mapping
}

// NewLocalFolderAdapter creates a new local folder adapter
func NewLocalFolderAdapter(cfg config.LocalFolderConfig) (*LocalFolderAdapter, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("local folder adapter is disabled")
	}

	// Build folder mappings
	mappings := make(map[string]string)
	folders := []string{}

	// Process mappings
	for _, mapping := range cfg.Mappings {
		if mapping.FolderPath != "" && mapping.KnowledgeID != "" {
			// Validate folder exists
			if _, err := os.Stat(mapping.FolderPath); os.IsNotExist(err) {
				return nil, fmt.Errorf("folder does not exist: %s", mapping.FolderPath)
			}
			mappings[mapping.FolderPath] = mapping.KnowledgeID
			folders = append(folders, mapping.FolderPath)
		}
	}

	if len(folders) == 0 {
		return nil, fmt.Errorf("at least one local folder mapping must be configured")
	}

	return &LocalFolderAdapter{
		config:   cfg,
		folders:  folders,
		mappings: mappings,
		lastSync: time.Now().Add(-24 * time.Hour), // Default to 24 hours ago
	}, nil
}

// Name returns the adapter name
func (l *LocalFolderAdapter) Name() string {
	return "local"
}

// FetchFiles retrieves files from local folders
func (l *LocalFolderAdapter) FetchFiles(ctx context.Context) ([]*File, error) {
	var files []*File

	for _, folder := range l.folders {
		logrus.Debugf("Fetching files from local folder: %s", folder)
		knowledgeID := l.mappings[folder]
		folderFiles, err := l.fetchFolderFiles(ctx, folder, knowledgeID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch files from folder %s: %w", folder, err)
		}
		logrus.Debugf("Found %d files in folder %s (knowledge_id: %s)", len(folderFiles), folder, knowledgeID)
		files = append(files, folderFiles...)
	}

	logrus.Debugf("Total files fetched: %d", len(files))
	return files, nil
}

// fetchFolderFiles fetches files from a specific folder recursively
func (l *LocalFolderAdapter) fetchFolderFiles(ctx context.Context, folderPath string, knowledgeID string) ([]*File, error) {
	var files []*File

	err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logrus.Warnf("Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip hidden files and common ignore patterns
		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, ".") || l.shouldIgnoreFile(baseName) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			logrus.Warnf("Failed to read file %s: %v", path, err)
			return nil
		}

		// Skip binary files (basic check)
		if l.isBinaryFile(content) {
			logrus.Debugf("Skipping binary file: %s", path)
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			logrus.Warnf("Failed to get file info for %s: %v", path, err)
			return nil
		}

		// Calculate relative path from the folder root
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			logrus.Warnf("Failed to calculate relative path for %s: %v", path, err)
			return nil
		}

		// Calculate hash
		hash := fmt.Sprintf("%x", sha256.Sum256(content))

		file := &File{
			Path:        relPath,
			Content:     content,
			Hash:        hash,
			Modified:    info.ModTime(),
			Size:        info.Size(),
			Source:      fmt.Sprintf("local:%s", folderPath),
			KnowledgeID: knowledgeID,
		}

		files = append(files, file)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", folderPath, err)
	}

	return files, nil
}

// shouldIgnoreFile checks if a file should be ignored based on common patterns
func (l *LocalFolderAdapter) shouldIgnoreFile(filename string) bool {
	// Check for hidden files (starting with .)
	if strings.HasPrefix(filename, ".") {
		return true
	}

	ignorePatterns := []string{
		"node_modules", "vendor", ".git", ".svn", ".hg",
		"__pycache__", ".pytest_cache", ".coverage",
	}

	// Check for specific patterns
	lowerName := strings.ToLower(filename)
	for _, pattern := range ignorePatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	// Check for specific filenames
	specificFiles := []string{"thumbs.db", ".ds_store", "desktop.ini"}
	for _, file := range specificFiles {
		if lowerName == file {
			return true
		}
	}

	// Check for file extensions
	extensions := []string{".log", ".tmp", ".temp", ".swp", ".swo"}
	for _, ext := range extensions {
		if strings.HasSuffix(lowerName, ext) {
			return true
		}
	}

	return false
}

// isBinaryFile checks if content appears to be binary
func (l *LocalFolderAdapter) isBinaryFile(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < len(content) && i < 1024; i++ {
		if content[i] == 0 {
			return true
		}
	}

	// Check for high ratio of non-printable characters
	nonPrintable := 0
	checkLen := len(content)
	if checkLen > 1024 {
		checkLen = 1024
	}

	for i := 0; i < checkLen; i++ {
		if content[i] < 32 && content[i] != 9 && content[i] != 10 && content[i] != 13 {
			nonPrintable++
		}
	}

	// If more than 30% of characters are non-printable, consider it binary
	return float64(nonPrintable)/float64(checkLen) > 0.3
}

// GetLastSync returns the last sync time
func (l *LocalFolderAdapter) GetLastSync() time.Time {
	return l.lastSync
}

// SetLastSync sets the last sync time
func (l *LocalFolderAdapter) SetLastSync(t time.Time) {
	l.lastSync = t
}
