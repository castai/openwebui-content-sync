package sync

import (
	"context"

	"github.com/openwebui-content-sync/internal/adapter"
)

// ManagerInterface defines the interface for sync manager operations
type ManagerInterface interface {
	SyncFiles(ctx context.Context, adapters []adapter.Adapter) error
	SetKnowledgeID(knowledgeID string)
	InitializeFileIndex(ctx context.Context, adapters []adapter.Adapter) error
}
