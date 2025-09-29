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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/config"
)

func TestNewSlackAdapter(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		config      config.SlackConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: config.SlackConfig{
				Enabled: true,
				Token:   "xoxb-test-token",
				ChannelMappings: []config.ChannelMapping{
					{
						ChannelID:   "C1234567890",
						ChannelName: "test-channel",
						KnowledgeID: "test-knowledge",
					},
				},
				DaysToFetch:      30,
				MaintainHistory:  false,
				MessageLimit:     1000,
				IncludeThreads:   true,
				IncludeReactions: false,
			},
			expectError: false,
		},
		{
			name: "missing token",
			config: config.SlackConfig{
				Enabled: true,
				Token:   "",
			},
			expectError: true,
		},
		{
			name: "disabled adapter",
			config: config.SlackConfig{
				Enabled: false,
				Token:   "",
			},
			expectError: false, // Should not error even without token when disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewSlackAdapter(tt.config, tempDir)

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

			if adapter == nil {
				t.Errorf("Expected adapter but got nil")
				return
			}

			// Test basic methods
			if adapter.Name() != "slack" {
				t.Errorf("Expected adapter name 'slack', got '%s'", adapter.Name())
			}

			// Test GetLastSync returns zero time initially
			if !adapter.GetLastSync().IsZero() {
				t.Errorf("Expected zero time for GetLastSync, got %v", adapter.GetLastSync())
			}

			// Test SetLastSync
			testTime := time.Now()
			adapter.SetLastSync(testTime)
			if !adapter.GetLastSync().Equal(testTime) {
				t.Errorf("Expected GetLastSync to return %v, got %v", testTime, adapter.GetLastSync())
			}
		})
	}
}

func TestSlackAdapter_FetchFiles_NoToken(t *testing.T) {
	tempDir := t.TempDir()

	config := config.SlackConfig{
		Enabled: true,
		Token:   "", // No token
	}

	_, err := NewSlackAdapter(config, tempDir)
	if err == nil {
		t.Errorf("Expected error for missing token, got none")
		return
	}
}

func TestSlackAdapter_FetchFiles_Disabled(t *testing.T) {
	tempDir := t.TempDir()

	config := config.SlackConfig{
		Enabled: false,
		Token:   "xoxb-test-token",
	}

	adapter, err := NewSlackAdapter(config, tempDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// FetchFiles should return empty slice when disabled
	files, err := adapter.FetchFiles(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if len(files) != 0 {
		t.Errorf("Expected empty files slice when disabled, got %d files", len(files))
	}
}

func TestSlackAdapter_StorageDirectory(t *testing.T) {
	tempDir := t.TempDir()

	config := config.SlackConfig{
		Enabled: true,
		Token:   "xoxb-test-token",
		ChannelMappings: []config.ChannelMapping{
			{
				ChannelID:   "C1234567890",
				ChannelName: "test-channel",
				KnowledgeID: "test-knowledge",
			},
		},
	}

	adapter, err := NewSlackAdapter(config, tempDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// Check that adapter was created successfully
	if adapter == nil {
		t.Errorf("Expected adapter but got nil")
	}

	// Check that storage directory was created
	expectedStorageDir := filepath.Join(tempDir, "slack", "channels")
	if _, err := os.Stat(expectedStorageDir); os.IsNotExist(err) {
		t.Errorf("Expected storage directory %s to be created", expectedStorageDir)
	}
}

func TestSanitizeChannelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"#general", "general"},
		{"dev-team", "dev-team"},
		{"test channel", "test_channel"},
		{"test@channel", "test_channel"},
		{"test#channel", "test_channel"},
		{"test/channel", "test_channel"},
		{"test\\channel", "test_channel"},
		{"test:channel", "test_channel"},
		{"test*channel", "test_channel"},
		{"test?channel", "test_channel"},
		{"test<channel>", "test_channel"},
		{"test|channel", "test_channel"},
		{"test\"channel\"", "test_channel"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeChannelName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeChannelName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlackAdapter_InterfaceCompliance(t *testing.T) {
	tempDir := t.TempDir()

	config := config.SlackConfig{
		Enabled: true,
		Token:   "xoxb-test-token",
		ChannelMappings: []config.ChannelMapping{
			{
				ChannelID:   "C1234567890",
				ChannelName: "test-channel",
				KnowledgeID: "test-knowledge",
			},
		},
	}

	adapter, err := NewSlackAdapter(config, tempDir)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// Test that SlackAdapter implements the Adapter interface
	var _ Adapter = adapter

	// Test all interface methods exist and work
	ctx := context.Background()

	// Name method
	name := adapter.Name()
	if name != "slack" {
		t.Errorf("Expected name 'slack', got '%s'", name)
	}

	// GetLastSync method
	syncTime := adapter.GetLastSync()
	if !syncTime.IsZero() {
		t.Errorf("Expected zero time for GetLastSync, got %v", syncTime)
	}

	// SetLastSync method
	testTime := time.Now()
	adapter.SetLastSync(testTime)
	if !adapter.GetLastSync().Equal(testTime) {
		t.Errorf("Expected GetLastSync to return %v, got %v", testTime, adapter.GetLastSync())
	}

	// FetchFiles method (this will fail with actual API call, but we can test the method exists)
	// We'll use a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err = adapter.FetchFiles(ctx)
	// We expect an error due to timeout or invalid token, but the method should exist
	if err == nil {
		t.Log("FetchFiles completed without error (unexpected)")
	}
}

// Benchmark tests
func BenchmarkSanitizeChannelName(b *testing.B) {
	testName := "#test-channel-with-special-chars!@#$%^&*()"
	for i := 0; i < b.N; i++ {
		sanitizeChannelName(testName)
	}
}

func BenchmarkSlackAdapter_Creation(b *testing.B) {
	tempDir := b.TempDir()

	config := config.SlackConfig{
		Enabled: true,
		Token:   "xoxb-test-token",
		ChannelMappings: []config.ChannelMapping{
			{
				ChannelID:   "C1234567890",
				ChannelName: "test-channel",
				KnowledgeID: "test-knowledge",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter, err := NewSlackAdapter(config, tempDir)
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
		if adapter == nil {
			b.Errorf("Expected adapter but got nil")
		}
	}
}
