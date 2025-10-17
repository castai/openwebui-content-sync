package mocks

import (
	"context"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/openwebui"
)

// MockOpenWebUIClient is a mock implementation of OpenWebUI client
type MockOpenWebUIClient struct {
	UploadFileFunc              func(ctx context.Context, filename string, content []byte) (*openwebui.File, error)
	GetFileFunc                 func(ctx context.Context, fileID string) (*openwebui.File, error)
	ListKnowledgeFunc           func(ctx context.Context) ([]*openwebui.Knowledge, error)
	AddFileToKnowledgeFunc      func(ctx context.Context, knowledgeID, fileID string) error
	RemoveFileFromKnowledgeFunc func(ctx context.Context, knowledgeID, fileID string) error
	GetKnowledgeFilesFunc       func(ctx context.Context, knowledgeID string) ([]*openwebui.File, error)
	DeleteFileFunc              func(ctx context.Context, fileID string) error
}

// UploadFile mocks the UploadFile method
func (m *MockOpenWebUIClient) UploadFile(ctx context.Context, filename string, content []byte) (*openwebui.File, error) {
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(ctx, filename, content)
	}
	return &openwebui.File{
		ID:       "mock-file-id",
		Filename: filename,
		UserID:   "test-user",
		Hash:     "mock-hash",
		Data: struct {
			Status string `json:"status"`
		}{
			Status: "pending",
		},
		Meta: struct {
			Name        string                 `json:"name"`
			ContentType string                 `json:"content_type"`
			Size        int64                  `json:"size"`
			Data        map[string]interface{} `json:"data"`
		}{
			Name:        filename,
			ContentType: "text/markdown",
			Size:        0,
			Data:        map[string]interface{}{},
		},
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
		Status:        true,
		Path:          "/app/backend/data/uploads/mock-file-id_" + filename,
		AccessControl: nil,
	}, nil
}

// GetFile mocks the GetFile method
func (m *MockOpenWebUIClient) GetFile(ctx context.Context, fileID string) (*openwebui.File, error) {
	if m.GetFileFunc != nil {
		return m.GetFileFunc(ctx, fileID)
	}
	return &openwebui.File{
		ID:       fileID,
		Filename: "mock-file.md",
		UserID:   "test-user",
		Hash:     "mock-hash",
		Data: struct {
			Status string `json:"status"`
		}{
			Status: "processed", // Default to processed status
		},
		Meta: struct {
			Name        string                 `json:"name"`
			ContentType string                 `json:"content_type"`
			Size        int64                  `json:"size"`
			Data        map[string]interface{} `json:"data"`
		}{
			Name:        "mock-file.md",
			ContentType: "text/markdown",
			Size:        100,
			Data:        map[string]interface{}{},
		},
		Status: true,
	}, nil
}

// ListKnowledge mocks the ListKnowledge method
func (m *MockOpenWebUIClient) ListKnowledge(ctx context.Context) ([]*openwebui.Knowledge, error) {
	if m.ListKnowledgeFunc != nil {
		return m.ListKnowledgeFunc(ctx)
	}
	return []*openwebui.Knowledge{
		{
			ID:            "mock-knowledge-id",
			UserID:        "test-user",
			Name:          "Test Knowledge",
			Description:   "Mock knowledge base",
			Data:          nil,
			Meta:          nil,
			AccessControl: map[string]interface{}{},
			CreatedAt:     time.Now().Unix(),
			UpdatedAt:     time.Now().Unix(),
		},
	}, nil
}

// AddFileToKnowledge mocks the AddFileToKnowledge method
func (m *MockOpenWebUIClient) AddFileToKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	if m.AddFileToKnowledgeFunc != nil {
		return m.AddFileToKnowledgeFunc(ctx, knowledgeID, fileID)
	}
	return nil
}

// RemoveFileFromKnowledge mocks the RemoveFileFromKnowledge method
func (m *MockOpenWebUIClient) RemoveFileFromKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	if m.RemoveFileFromKnowledgeFunc != nil {
		return m.RemoveFileFromKnowledgeFunc(ctx, knowledgeID, fileID)
	}
	return nil
}

// GetKnowledgeFiles mocks the GetKnowledgeFiles method
func (m *MockOpenWebUIClient) GetKnowledgeFiles(ctx context.Context, knowledgeID string) ([]*openwebui.File, error) {
	if m.GetKnowledgeFilesFunc != nil {
		return m.GetKnowledgeFilesFunc(ctx, knowledgeID)
	}
	return []*openwebui.File{
		{
			ID:       "existing-file-1",
			Filename: "existing-file-1.md",
			UserID:   "test-user",
			Hash:     "existing-hash-1",
			Data: struct {
				Status string `json:"status"`
			}{
				Status: "processed",
			},
			Meta: struct {
				Name        string                 `json:"name"`
				ContentType string                 `json:"content_type"`
				Size        int64                  `json:"size"`
				Data        map[string]interface{} `json:"data"`
			}{
				Name:        "existing-file-1.md",
				ContentType: "text/markdown",
				Size:        1000,
				Data:        map[string]interface{}{"source": "github"},
			},
			CreatedAt:     time.Now().Unix() - 3600, // 1 hour ago
			UpdatedAt:     time.Now().Unix() - 1800, // 30 minutes ago
			Status:        true,
			Path:          "existing-file-1.md",
			AccessControl: nil,
		},
	}, nil
}

// DeleteFile mocks the DeleteFile method
func (m *MockOpenWebUIClient) DeleteFile(ctx context.Context, fileID string) error {
	if m.DeleteFileFunc != nil {
		return m.DeleteFileFunc(ctx, fileID)
	}
	return nil
}

// MockAdapter is a mock implementation of the Adapter interface
type MockAdapter struct {
	NameFunc        func() string
	FetchFilesFunc  func(ctx context.Context) ([]*adapter.File, error)
	GetLastSyncFunc func() time.Time
	SetLastSyncFunc func(t time.Time)
	lastSync        time.Time
}

// Name mocks the Name method
func (m *MockAdapter) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-adapter"
}

// FetchFiles mocks the FetchFiles method
func (m *MockAdapter) FetchFiles(ctx context.Context) ([]*adapter.File, error) {
	if m.FetchFilesFunc != nil {
		return m.FetchFilesFunc(ctx)
	}
	return []*adapter.File{
		{
			Path:     "test.md",
			Content:  []byte("# Test File"),
			Hash:     "test-hash",
			Modified: time.Now(),
			Size:     10,
			Source:   "mock",
		},
	}, nil
}

// GetLastSync mocks the GetLastSync method
func (m *MockAdapter) GetLastSync() time.Time {
	if m.GetLastSyncFunc != nil {
		return m.GetLastSyncFunc()
	}
	return m.lastSync
}

// SetLastSync mocks the SetLastSync method
func (m *MockAdapter) SetLastSync(t time.Time) {
	if m.SetLastSyncFunc != nil {
		m.SetLastSyncFunc(t)
	} else {
		m.lastSync = t
	}
}
