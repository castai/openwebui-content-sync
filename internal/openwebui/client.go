package openwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/openwebui-content-sync/internal/utils"
	"github.com/sirupsen/logrus"
)

// Client represents the OpenWebUI API client
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// File represents a file in OpenWebUI
type File struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Hash     string `json:"hash"`
	Filename string `json:"filename"`
	Data     struct {
		Status string `json:"status"`
	} `json:"data"`
	Meta struct {
		Name        string                 `json:"name"`
		ContentType string                 `json:"content_type"`
		Size        int64                  `json:"size"`
		Data        map[string]interface{} `json:"data"`
	} `json:"meta"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
	Status        bool    `json:"status"`
	Path          string  `json:"path"`
	AccessControl *string `json:"access_control"`
}

// Knowledge represents a knowledge source in OpenWebUI
type Knowledge struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Data          interface{}            `json:"data"`
	Meta          interface{}            `json:"meta"`
	AccessControl map[string]interface{} `json:"access_control"`
	CreatedAt     int64                  `json:"created_at"`
	UpdatedAt     int64                  `json:"updated_at"`
	User          map[string]interface{} `json:"user,omitempty"`
	Files         []*File                `json:"files,omitempty"`
}

// NewClient creates a new OpenWebUI API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UploadFile uploads a file to OpenWebUI
func (c *Client) UploadFile(ctx context.Context, filename string, content []byte) (*File, error) {
	url := fmt.Sprintf("%s/api/v1/files/", c.baseURL)

	logrus.Debugf("Uploading file to OpenWebUI: %s (size: %d bytes)", filename, len(content))
	logrus.Debugf("Upload URL: %s", url)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := fileWriter.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write file content: %w", err)
	}

	writer.Close()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		logrus.Debugf("Using API key for authentication")
	} else {
		logrus.Debugf("No API key provided")
	}

	// Send request with retry logic
	logrus.Debugf("Sending file upload request...")

	var resp *http.Response
	retryConfig := utils.DefaultRetryConfig()
	retryConfig.BaseDelay = 2 * time.Second
	retryConfig.MaxDelay = 30 * time.Second

	err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var err error
		resp, err = c.client.Do(req)
		if err != nil {
			return err
		}

		logrus.Debugf("File upload response status: %d %s", resp.StatusCode, resp.Status)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			logrus.Errorf("File upload failed with status %d: %s", resp.StatusCode, string(body))
			resp.Body.Close()
			return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload file after retries: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// logrus.Debugf("File upload response body: %s", string(body))

	// Parse response
	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		logrus.Errorf("Failed to decode file upload response: %v", err)
		logrus.Errorf("Response body was: %s", string(body))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logrus.Debugf("Successfully uploaded file: ID=%s, Filename=%s, Status=%t", file.ID, file.Filename, file.Status)
	return &file, nil
}

// ListKnowledge retrieves all knowledge sources
func (c *Client) ListKnowledge(ctx context.Context) ([]*Knowledge, error) {
	url := fmt.Sprintf("%s/api/v1/knowledge/", c.baseURL)

	logrus.Debugf("Listing all knowledge sources")
	logrus.Debugf("List knowledge URL: %s", url)
	logrus.Debugf("Base URL: %s", c.baseURL)
	logrus.Debugf("Context: %+v", ctx)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logrus.Errorf("Failed to create HTTP request for list knowledge: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Log request details
	logrus.Debugf("Request method: %s", req.Method)
	logrus.Debugf("Request URL: %s", req.URL.String())
	logrus.Debugf("Request headers: %+v", req.Header)

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		logrus.Debugf("Using API key for list knowledge request (length: %d)", len(c.apiKey))
	} else {
		logrus.Debugf("No API key provided for list knowledge request")
	}

	// Log final request headers after API key is set
	logrus.Debugf("Final request headers: %+v", req.Header)

	logrus.Debugf("Sending list knowledge request...")
	resp, err := c.client.Do(req)
	if err != nil {
		logrus.Errorf("HTTP request failed for list knowledge: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logrus.Debugf("List knowledge response status: %d %s", resp.StatusCode, resp.Status)
	logrus.Debugf("Response headers: %+v", resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logrus.Errorf("List knowledge request failed with status %d: %s", resp.StatusCode, string(body))
		logrus.Errorf("Request URL was: %s", req.URL.String())
		logrus.Errorf("Request headers were: %+v", req.Header)
		return nil, fmt.Errorf("list knowledge failed with status %d: %s", resp.StatusCode, string(body))
	}

	var knowledge []*Knowledge
	if err := json.NewDecoder(resp.Body).Decode(&knowledge); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return knowledge, nil
}

// AddFileToKnowledge adds a file to a knowledge source
func (c *Client) AddFileToKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	url := fmt.Sprintf("%s/api/v1/knowledge/%s/file/add", c.baseURL, knowledgeID)

	logrus.Debugf("Adding file to knowledge: fileID=%s, knowledgeID=%s", fileID, knowledgeID)
	logrus.Debugf("Add file URL: %s", url)

	payload := map[string]string{
		"file_id": fileID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// logrus.Debugf("Add file payload: %s", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		logrus.Debugf("Using API key for add file request")
	} else {
		logrus.Debugf("No API key provided for add file request")
	}

	logrus.Debugf("Sending add file to knowledge request...")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logrus.Debugf("Add file to knowledge response status: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		logrus.Errorf("Add file to knowledge failed with status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("add file to knowledge failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Warnf("Failed to read add file response body: %v", err)
	} else {
		logrus.Debugf("Add file to knowledge response body: %s", string(body))
	}

	logrus.Debugf("Successfully added file %s to knowledge %s", fileID, knowledgeID)
	return nil
}

// RemoveFileFromKnowledge removes a file from a knowledge source
func (c *Client) RemoveFileFromKnowledge(ctx context.Context, knowledgeID, fileID string) error {
	url := fmt.Sprintf("%s/api/v1/knowledge/%s/file/remove", c.baseURL, knowledgeID)

	payload := map[string]string{
		"file_id": fileID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remove file from knowledge failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteFile deletes a file from OpenWebUI
func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	url := fmt.Sprintf("%s/api/v1/files/%s", c.baseURL, fileID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	logrus.Debugf("Deleting file from OpenWebUI: fileID=%s", fileID)
	logrus.Debugf("Delete URL: %s", url)
	logrus.Debugf("Using API key for delete request")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	logrus.Debugf("File delete response status: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		logrus.Debugf("File delete response body: %s", string(body))
		return fmt.Errorf("file delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	logrus.Debugf("Successfully deleted file: %s", fileID)
	return nil
}

// GetKnowledgeFiles retrieves files from a specific knowledge source
func (c *Client) GetKnowledgeFiles(ctx context.Context, knowledgeID string) ([]*File, error) {
	url := fmt.Sprintf("%s/api/v1/knowledge/", c.baseURL)

	logrus.Debugf("Getting files from knowledge source: %s", knowledgeID)
	logrus.Debugf("Knowledge files URL: %s", url)
	logrus.Debugf("Base URL: %s", c.baseURL)
	logrus.Debugf("Context: %+v", ctx)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logrus.Errorf("Failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Log request details
	logrus.Debugf("Request method: %s", req.Method)
	logrus.Debugf("Request URL: %s", req.URL.String())
	logrus.Debugf("Request headers: %+v", req.Header)

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		logrus.Debugf("Using API key for knowledge files request (length: %d)", len(c.apiKey))
	} else {
		logrus.Debugf("No API key provided for knowledge files request")
	}

	// Log final request headers after API key is set
	logrus.Debugf("Final request headers: %+v", req.Header)

	logrus.Debugf("Sending knowledge files request...")
	resp, err := c.client.Do(req)
	if err != nil {
		logrus.Errorf("HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logrus.Debugf("Knowledge files response status: %d %s", resp.StatusCode, resp.Status)
	logrus.Debugf("Response headers: %+v", resp.Header)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logrus.Errorf("Knowledge files request failed with status %d: %s", resp.StatusCode, string(body))
		logrus.Errorf("Request URL was: %s", req.URL.String())
		logrus.Errorf("Request headers were: %+v", req.Header)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	//logrus.Debugf("Knowledge files response body: %s", string(body))
	logrus.Debugf("Response body length: %d bytes", len(body))

	var knowledgeList []*Knowledge
	if err := json.Unmarshal(body, &knowledgeList); err != nil {
		logrus.Errorf("Failed to decode knowledge list response: %v", err)
		//logrus.Errorf("Response body was: %s", string(body))
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	//logrus.Debugf("Successfully decoded %d knowledge sources", len(knowledgeList))
	for i, knowledge := range knowledgeList {
		logrus.Debugf("Knowledge[%d]: ID=%s, Name=%s, Files count=%d", i, knowledge.ID, knowledge.Name, len(knowledge.Files))
	}

	// Find the specific knowledge source
	var targetKnowledge *Knowledge
	for _, knowledge := range knowledgeList {
		logrus.Debugf("Checking knowledge ID: %s (looking for: %s)", knowledge.ID, knowledgeID)
		if knowledge.ID == knowledgeID {
			targetKnowledge = knowledge
			logrus.Debugf("Found matching knowledge source: %s", knowledgeID)
			break
		}
	}

	if targetKnowledge == nil {
		//logrus.Warnf("Knowledge source %s not found in %d available sources", knowledgeID, len(knowledgeList))
		logrus.Debugf("Available knowledge source IDs: %v", func() []string {
			ids := make([]string, len(knowledgeList))
			for i, k := range knowledgeList {
				ids[i] = k.ID
			}
			return ids
		}())
		return []*File{}, nil
	}

	logrus.Debugf("Successfully retrieved %d files from knowledge source %s", len(targetKnowledge.Files), knowledgeID)
	for i, file := range targetKnowledge.Files {
		logrus.Debugf("File[%d]: ID=%s, Filename=%s, Status=%t", i, file.ID, file.Filename, file.Status)
	}
	return targetKnowledge.Files, nil
}
