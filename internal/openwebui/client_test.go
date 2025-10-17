package openwebui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080", "test-api-key")
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.baseURL != "http://localhost:8080" {
		t.Errorf("Expected baseURL 'http://localhost:8080', got '%s'", client.baseURL)
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("Expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}
}

func TestClient_UploadFile(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		content        []byte
		serverResponse File
		serverStatus   int
		expectError    bool
	}{
		{
			name:     "successful upload",
			filename: "test.md",
			content:  []byte("# Test"),
			serverResponse: File{
				ID:       "file-123",
				Filename: "test.md",
				UserID:   "user-123",
				Hash:     "hash-123",
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
					Name:        "test.md",
					ContentType: "text/markdown",
					Size:        6,
					Data:        map[string]interface{}{},
				},
				CreatedAt:     time.Now().Unix(),
				UpdatedAt:     time.Now().Unix(),
				Status:        true,
				Path:          "/app/backend/data/uploads/file-123_test.md",
				AccessControl: nil,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "server error",
			filename:     "test.md",
			content:      []byte("# Test"),
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				t.Logf("Request %d: %s %s", requestCount, r.Method, r.URL.Path)

				// Handle POST requests for file uploads
				if r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v1/files/") {
					if r.Header.Get("Authorization") != "Bearer test-api-key" {
						t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
					}
					w.WriteHeader(tt.serverStatus)
					if tt.serverStatus == http.StatusOK {
						json.NewEncoder(w).Encode(tt.serverResponse)
					} else {
						w.Write([]byte("Server Error"))
					}
				} else if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v1/files/") {
					// Handle GET requests for file polling (file processing status)
					if r.Header.Get("Authorization") != "Bearer test-api-key" {
						t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
					}

					// Extract file ID from path
					pathParts := strings.Split(r.URL.Path, "/")
					fileID := pathParts[len(pathParts)-1]

					// Return file with "processed" status to complete polling quickly
					fileResponse := map[string]interface{}{
						"id":       fileID,
						"filename": "test-file.md",
						"user_id":  "test-user",
						"hash":     "test-hash",
						"data": map[string]interface{}{
							"status": "processed",
						},
						"meta": map[string]interface{}{
							"name":         "test-file.md",
							"content_type": "text/markdown",
							"size":         100,
							"data":         map[string]interface{}{},
						},
						"status": true,
					}

					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(fileResponse)
				} else {
					// Handle other requests gracefully
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key")
			ctx := context.Background()

			result, err := client.UploadFile(ctx, tt.filename, tt.content)

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

			if result.ID != tt.serverResponse.ID {
				t.Errorf("Expected ID %s, got %s", tt.serverResponse.ID, result.ID)
			}
			if result.Filename != tt.serverResponse.Filename {
				t.Errorf("Expected Filename %s, got %s", tt.serverResponse.Filename, result.Filename)
			}
		})
	}
}

func TestClient_ListKnowledge(t *testing.T) {
	expectedKnowledge := []*Knowledge{
		{
			ID:            "knowledge-123",
			UserID:        "user-123",
			Name:          "Test Knowledge",
			Description:   "Test Description",
			Data:          nil,
			Meta:          nil,
			AccessControl: map[string]interface{}{},
			CreatedAt:     time.Now().Unix(),
			UpdatedAt:     time.Now().Unix(),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/knowledge/") {
			t.Errorf("Expected path to contain /api/v1/knowledge/, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedKnowledge)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")
	ctx := context.Background()

	result, err := client.ListKnowledge(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != len(expectedKnowledge) {
		t.Fatalf("Expected %d knowledge items, got %d", len(expectedKnowledge), len(result))
	}

	if result[0].ID != expectedKnowledge[0].ID {
		t.Errorf("Expected ID %s, got %s", expectedKnowledge[0].ID, result[0].ID)
	}
}

func TestClient_AddFileToKnowledge(t *testing.T) {
	tests := []struct {
		name         string
		knowledgeID  string
		fileID       string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "successful add",
			knowledgeID:  "knowledge-123",
			fileID:       "file-123",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "server error",
			knowledgeID:  "knowledge-123",
			fileID:       "file-123",
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				expectedPath := "/api/v1/knowledge/" + tt.knowledgeID + "/file/add"
				if !strings.Contains(r.URL.Path, expectedPath) {
					t.Errorf("Expected path to contain %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-api-key" {
					t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
				}

				// Check request body
				var requestBody map[string]string
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}
				if requestBody["file_id"] != tt.fileID {
					t.Errorf("Expected file_id %s, got %s", tt.fileID, requestBody["file_id"])
				}

				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key")
			ctx := context.Background()

			err := client.AddFileToKnowledge(ctx, tt.knowledgeID, tt.fileID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestClient_RemoveFileFromKnowledge(t *testing.T) {
	tests := []struct {
		name         string
		knowledgeID  string
		fileID       string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "successful remove",
			knowledgeID:  "knowledge-123",
			fileID:       "file-123",
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "server error",
			knowledgeID:  "knowledge-123",
			fileID:       "file-123",
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				expectedPath := "/api/v1/knowledge/" + tt.knowledgeID + "/file/remove"
				if !strings.Contains(r.URL.Path, expectedPath) {
					t.Errorf("Expected path to contain %s, got %s", expectedPath, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-api-key" {
					t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
				}

				// Check request body
				var requestBody map[string]string
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}
				if requestBody["file_id"] != tt.fileID {
					t.Errorf("Expected file_id %s, got %s", tt.fileID, requestBody["file_id"])
				}

				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key")
			ctx := context.Background()

			err := client.RemoveFileFromKnowledge(ctx, tt.knowledgeID, tt.fileID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
