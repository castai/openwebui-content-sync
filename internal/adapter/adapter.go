package adapter

import (
	"context"
	"time"
)

// File represents a file from an external source
type File struct {
	Path        string    `json:"path"`
	Content     []byte    `json:"content"`
	Hash        string    `json:"hash"`
	Modified    time.Time `json:"modified"`
	Size        int64     `json:"size"`
	Source      string    `json:"source"`
	KnowledgeID string    `json:"knowledge_id,omitempty"` // Optional: specific knowledge base ID for this file
}

// Adapter defines the interface for data source adapters
type Adapter interface {
	// Name returns the adapter name
	Name() string

	// FetchFiles retrieves files from the data source
	FetchFiles(ctx context.Context) ([]*File, error)

	// GetLastSync returns the last sync timestamp
	GetLastSync() time.Time

	// SetLastSync updates the last sync timestamp
	SetLastSync(t time.Time)
}
