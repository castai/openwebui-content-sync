package adapter

import (
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/config"
)

func TestNewConfluenceAdapter(t *testing.T) {
	tests := []struct {
		name    string
		config  config.ConfluenceConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: config.ConfluenceConfig{
				BaseURL:  "https://test.atlassian.net",
				Username: "test@example.com",
				APIKey:   "test-key",
				SpaceMappings: []config.SpaceMapping{
					{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			config: config.ConfluenceConfig{
				Username: "test@example.com",
				APIKey:   "test-key",
				SpaceMappings: []config.SpaceMapping{
					{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: config.ConfluenceConfig{
				BaseURL: "https://test.atlassian.net",
				APIKey:  "test-key",
				SpaceMappings: []config.SpaceMapping{
					{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			config: config.ConfluenceConfig{
				BaseURL:  "https://test.atlassian.net",
				Username: "test@example.com",
				SpaceMappings: []config.SpaceMapping{
					{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing mappings",
			config: config.ConfluenceConfig{
				BaseURL:       "https://test.atlassian.net",
				Username:      "test@example.com",
				APIKey:        "test-key",
				SpaceMappings: []config.SpaceMapping{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewConfluenceAdapter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfluenceAdapter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && adapter == nil {
				t.Error("NewConfluenceAdapter() returned nil adapter when no error expected")
			}
		})
	}
}

func TestConfluenceAdapter_Name(t *testing.T) {
	config := config.ConfluenceConfig{
		BaseURL:  "https://test.atlassian.net",
		Username: "test@example.com",
		APIKey:   "test-key",
		SpaceMappings: []config.SpaceMapping{
			{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
		},
	}

	adapter, err := NewConfluenceAdapter(config)
	if err != nil {
		t.Fatalf("NewConfluenceAdapter() error = %v", err)
	}

	if adapter.Name() != "confluence" {
		t.Errorf("Name() = %v, want %v", adapter.Name(), "confluence")
	}
}

func TestConfluenceAdapter_GetSetLastSync(t *testing.T) {
	config := config.ConfluenceConfig{
		BaseURL:  "https://test.atlassian.net",
		Username: "test@example.com",
		APIKey:   "test-key",
		SpaceMappings: []config.SpaceMapping{
			{SpaceKey: "TEST", KnowledgeID: "knowledge-id"},
		},
	}

	adapter, err := NewConfluenceAdapter(config)
	if err != nil {
		t.Fatalf("NewConfluenceAdapter() error = %v", err)
	}

	// Test initial last sync
	initialSync := adapter.GetLastSync()
	if initialSync.IsZero() {
		t.Error("GetLastSync() returned zero time")
	}

	// Test setting last sync
	newTime := time.Now()
	adapter.SetLastSync(newTime)
	if !adapter.GetLastSync().Equal(newTime) {
		t.Errorf("SetLastSync() did not update last sync time")
	}
}

func TestSanitizeFilename(t *testing.T) {
	adapter := &ConfluenceAdapter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"normal-file.txt", "normal-file.txt"},
		{"file/with/slashes.txt", "file_with_slashes.txt"},
		{"file:with:colons.txt", "file_with_colons.txt"},
		{"file*with*asterisks.txt", "file_with_asterisks.txt"},
		{"file?with?questions.txt", "file_with_questions.txt"},
		{"file\"with\"quotes.txt", "file_with_quotes.txt"},
		{"file<with>brackets.txt", "file_with_brackets.txt"},
		{"file|with|pipes.txt", "file_with_pipes.txt"},
		{"very-long-filename-that-should-be-truncated-because-it-exceeds-the-maximum-length-limit-of-one-hundred-characters.txt", "very-long-filename-that-should-be-truncated-because-it-exceeds-the-maximum-length-limit-of-one-hundr"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHtmlToText(t *testing.T) {
	adapter := &ConfluenceAdapter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello world</p>", "Hello world"},
		{"<div>Line 1<br>Line 2</div>", "Line 1\nLine 2"},
		{"<p>Paragraph 1</p><p>Paragraph 2</p>", "Paragraph 1\n\nParagraph 2"},
		{"<h1>Title</h1><p>Content</p>", "Title\nContent"},
		{"<strong>Bold</strong> text", "Bold text"},
		{"<em>Italic</em> text", "Italic text"},
		{"", ""},
		{"Plain text without HTML", "Plain text without HTML"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.HtmlToText(tt.input)
			if result != tt.expected {
				t.Errorf("HtmlToText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Note: FetchFiles test would require mocking HTTP requests
// This would be more complex and would typically use a library like httptest
// or a mocking framework like gomock
