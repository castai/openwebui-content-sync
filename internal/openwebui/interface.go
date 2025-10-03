package openwebui

import (
	"context"
)

// ClientInterface defines the interface for OpenWebUI client operations
type ClientInterface interface {
	UploadFile(ctx context.Context, filename string, content []byte) (*File, error)
	GetFile(ctx context.Context, fileID string) (*File, error)
	ListKnowledge(ctx context.Context) ([]*Knowledge, error)
	AddFileToKnowledge(ctx context.Context, knowledgeID, fileID string) error
	RemoveFileFromKnowledge(ctx context.Context, knowledgeID, fileID string) error
	GetKnowledgeFiles(ctx context.Context, knowledgeID string) ([]*File, error)
}
