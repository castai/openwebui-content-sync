package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openwebui-content-sync/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// SlackAdapter implements the Adapter interface for Slack
type SlackAdapter struct {
	config     config.SlackConfig
	client     *slack.Client
	lastSync   time.Time
	storageDir string
}

// SlackMessage represents a Slack message with metadata
type SlackMessage struct {
	Timestamp   string            `json:"timestamp"`
	User        string            `json:"user"`
	Text        string            `json:"text"`
	Channel     string            `json:"channel"`
	ThreadTS    string            `json:"thread_ts,omitempty"`
	Reactions   []SlackReaction   `json:"reactions,omitempty"`
	Files       []SlackFile       `json:"files,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackReaction represents a reaction on a message
type SlackReaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// SlackFile represents a file attachment
type SlackFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Mimetype string `json:"mimetype"`
	URL      string `json:"url_private"`
}

// SlackAttachment represents a message attachment
type SlackAttachment struct {
	Title      string `json:"title"`
	Text       string `json:"text"`
	Fallback   string `json:"fallback"`
	Color      string `json:"color"`
	AuthorName string `json:"author_name"`
}

// NewSlackAdapter creates a new Slack adapter
func NewSlackAdapter(cfg config.SlackConfig, storageDir string) (*SlackAdapter, error) {
	if !cfg.Enabled {
		// Return a disabled adapter without error
		// Still create storage directory for consistency
		slackStoragePath := filepath.Join(storageDir, "slack", "channels")
		if err := os.MkdirAll(slackStoragePath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create slack storage directory: %w", err)
		}

		return &SlackAdapter{
			config:     cfg,
			client:     nil,
			storageDir: storageDir,
			lastSync:   time.Time{},
		}, nil
	}

	if cfg.Token == "" {
		return nil, fmt.Errorf("slack token is required")
	}

	if len(cfg.ChannelMappings) == 0 {
		return nil, fmt.Errorf("at least one channel mapping must be configured")
	}

	// Set defaults
	if cfg.DaysToFetch <= 0 {
		cfg.DaysToFetch = 30
	}
	if cfg.MessageLimit <= 0 {
		cfg.MessageLimit = 1000
	}

	client := slack.New(cfg.Token)

	// Create storage directory for Slack
	slackStoragePath := filepath.Join(storageDir, "slack", "channels")
	if err := os.MkdirAll(slackStoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create slack storage directory: %w", err)
	}

	return &SlackAdapter{
		config:     cfg,
		client:     client,
		storageDir: storageDir,
		lastSync:   time.Time{}, // Start with zero time
	}, nil
}

// Name returns the adapter name
func (s *SlackAdapter) Name() string {
	return "slack"
}

// FetchFiles retrieves messages from Slack channels and converts them to files
func (s *SlackAdapter) FetchFiles(ctx context.Context) ([]*File, error) {
	logrus.Debugf("Fetching messages from Slack channels...")

	// Return empty slice if adapter is disabled
	if !s.config.Enabled {
		logrus.Debugf("Slack adapter is disabled, returning empty files")
		return []*File{}, nil
	}

	var files []*File
	now := time.Now()

	// Calculate time range for fetching messages
	var oldestTime time.Time
	if s.config.MaintainHistory {
		// If maintaining history, fetch from last sync time
		oldestTime = s.lastSync
	} else {
		// If not maintaining history, fetch only the last N days
		oldestTime = now.AddDate(0, 0, -s.config.DaysToFetch)
	}

	logrus.Debugf("Fetching messages from %s to %s", oldestTime.Format(time.RFC3339), now.Format(time.RFC3339))

	// Process each channel mapping
	for _, mapping := range s.config.ChannelMappings {
		logrus.Debugf("Processing channel: %s (%s)", mapping.ChannelName, mapping.ChannelID)

		// Fetch messages from the channel
		messages, err := s.fetchChannelMessages(ctx, mapping.ChannelID, oldestTime, now)
		if err != nil {
			logrus.Errorf("Failed to fetch messages from channel %s: %v", mapping.ChannelName, err)
			continue
		}

		if len(messages) == 0 {
			logrus.Debugf("No new messages found in channel %s", mapping.ChannelName)
			continue
		}

		// Convert messages to file content
		fileContent, err := s.messagesToFileContent(messages, mapping.ChannelName)
		if err != nil {
			logrus.Errorf("Failed to convert messages to file content for channel %s: %v", mapping.ChannelName, err)
			continue
		}

		// Create file metadata
		filename := fmt.Sprintf("%s_messages.md", sanitizeChannelName(mapping.ChannelName))
		filePath := fmt.Sprintf("slack/%s", filename)

		file := &File{
			Path:        filePath,
			Content:     []byte(fileContent),
			Hash:        fmt.Sprintf("%x", sha256.Sum256([]byte(fileContent))),
			Modified:    now,
			Size:        int64(len(fileContent)),
			Source:      "slack",
			KnowledgeID: mapping.KnowledgeID,
		}

		files = append(files, file)

		// Save messages to local storage for history tracking
		if err := s.saveMessagesToStorage(mapping.ChannelID, mapping.ChannelName, messages); err != nil {
			logrus.Warnf("Failed to save messages to storage for channel %s: %v", mapping.ChannelName, err)
		}

		logrus.Debugf("Processed %d messages from channel %s", len(messages), mapping.ChannelName)
	}

	// Update last sync time
	s.lastSync = now

	logrus.Infof("Fetched %d files from Slack channels", len(files))
	return files, nil
}

// fetchChannelMessages retrieves messages from a specific Slack channel
func (s *SlackAdapter) fetchChannelMessages(ctx context.Context, channelID string, oldestTime, latestTime time.Time) ([]SlackMessage, error) {
	var allMessages []SlackMessage
	latest := latestTime.Unix()
	oldest := oldestTime.Unix()
	cursor := ""

	// Load existing messages from storage
	existingMessages, err := s.loadMessagesFromStorage(channelID)
	if err != nil {
		logrus.Debugf("No existing messages found for channel %s: %v", channelID, err)
		existingMessages = []SlackMessage{}
	}

	// Create a map of existing message timestamps for deduplication
	existingTimestamps := make(map[string]bool)
	for _, msg := range existingMessages {
		existingTimestamps[msg.Timestamp] = true
	}

	for {
		params := slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Latest:    fmt.Sprintf("%d", latest),
			Oldest:    fmt.Sprintf("%d", oldest),
			Limit:     200, // Slack API limit
			Cursor:    cursor,
		}

		history, err := s.client.GetConversationHistory(&params)
		if err != nil {
			return nil, fmt.Errorf("failed to get conversation history: %w", err)
		}

		// Convert Slack messages to our format
		for _, msg := range history.Messages {
			// Skip if we already have this message
			if existingTimestamps[msg.Timestamp] {
				continue
			}

			slackMsg := s.convertSlackMessage(msg.Msg, channelID)
			allMessages = append(allMessages, slackMsg)

			// Update latest timestamp for pagination
			if ts, err := strconv.ParseFloat(msg.Timestamp, 64); err == nil {
				latest = int64(ts)
			}
		}

		// Break if no more messages or reached limit
		if history.ResponseMetaData.NextCursor == "" || len(history.Messages) == 0 {
			break
		}

		// Check if we've reached the message limit
		if len(allMessages) >= s.config.MessageLimit {
			logrus.Debugf("Reached message limit (%d) for channel %s", s.config.MessageLimit, channelID)
			break
		}

		cursor = history.ResponseMetaData.NextCursor
	}

	// If maintaining history, merge with existing messages
	if s.config.MaintainHistory {
		allMessages = append(existingMessages, allMessages...)
		// Sort by timestamp
		sort.Slice(allMessages, func(i, j int) bool {
			ts1, _ := strconv.ParseFloat(allMessages[i].Timestamp, 64)
			ts2, _ := strconv.ParseFloat(allMessages[j].Timestamp, 64)
			return ts1 < ts2
		})
	}

	return allMessages, nil
}

// convertSlackMessage converts a Slack message to our format
func (s *SlackAdapter) convertSlackMessage(msg slack.Msg, channelID string) SlackMessage {
	slackMsg := SlackMessage{
		Timestamp: msg.Timestamp,
		User:      msg.User,
		Text:      msg.Text,
		Channel:   channelID,
		ThreadTS:  msg.ThreadTimestamp,
	}

	// Add reactions if enabled
	if s.config.IncludeReactions && len(msg.Reactions) > 0 {
		for _, reaction := range msg.Reactions {
			slackMsg.Reactions = append(slackMsg.Reactions, SlackReaction{
				Name:  reaction.Name,
				Count: reaction.Count,
				Users: reaction.Users,
			})
		}
	}

	// Add files if present
	if len(msg.Files) > 0 {
		for _, file := range msg.Files {
			slackMsg.Files = append(slackMsg.Files, SlackFile{
				ID:       file.ID,
				Name:     file.Name,
				Title:    file.Title,
				Mimetype: file.Mimetype,
				URL:      file.URLPrivate,
			})
		}
	}

	// Add attachments if present
	if len(msg.Attachments) > 0 {
		for _, attachment := range msg.Attachments {
			slackMsg.Attachments = append(slackMsg.Attachments, SlackAttachment{
				Title:      attachment.Title,
				Text:       attachment.Text,
				Fallback:   attachment.Fallback,
				Color:      attachment.Color,
				AuthorName: attachment.AuthorName,
			})
		}
	}

	return slackMsg
}

// messagesToFileContent converts Slack messages to markdown content
func (s *SlackAdapter) messagesToFileContent(messages []SlackMessage, channelName string) (string, error) {
	var content strings.Builder

	// Add header
	content.WriteString(fmt.Sprintf("# Slack Messages - %s\n\n", channelName))
	content.WriteString(fmt.Sprintf("**Channel:** %s\n", channelName))
	content.WriteString(fmt.Sprintf("**Total Messages:** %d\n", len(messages)))
	content.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format(time.RFC3339)))
	content.WriteString("---\n\n")

	// Add messages
	for _, msg := range messages {
		timestamp, err := strconv.ParseFloat(msg.Timestamp, 64)
		if err != nil {
			logrus.Warnf("Failed to parse timestamp %s: %v", msg.Timestamp, err)
			continue
		}

		msgTime := time.Unix(int64(timestamp), 0)
		content.WriteString(fmt.Sprintf("## %s\n", msgTime.Format("2006-01-02 15:04:05")))

		if msg.User != "" {
			content.WriteString(fmt.Sprintf("**User:** %s\n", msg.User))
		}

		if msg.Text != "" {
			content.WriteString(fmt.Sprintf("**Message:**\n%s\n", msg.Text))
		}

		// Add thread information
		if msg.ThreadTS != "" {
			content.WriteString(fmt.Sprintf("**Thread:** %s\n", msg.ThreadTS))
		}

		// Add reactions
		if len(msg.Reactions) > 0 {
			content.WriteString("**Reactions:**\n")
			for _, reaction := range msg.Reactions {
				content.WriteString(fmt.Sprintf("- :%s: %d (%s)\n", reaction.Name, reaction.Count, strings.Join(reaction.Users, ", ")))
			}
		}

		// Add files
		if len(msg.Files) > 0 {
			content.WriteString("**Files:**\n")
			for _, file := range msg.Files {
				content.WriteString(fmt.Sprintf("- %s (%s)\n", file.Name, file.Mimetype))
			}
		}

		// Add attachments
		if len(msg.Attachments) > 0 {
			content.WriteString("**Attachments:**\n")
			for _, attachment := range msg.Attachments {
				if attachment.Title != "" {
					content.WriteString(fmt.Sprintf("- **%s**\n", attachment.Title))
				}
				if attachment.Text != "" {
					content.WriteString(fmt.Sprintf("  %s\n", attachment.Text))
				}
			}
		}

		content.WriteString("\n---\n\n")
	}

	return content.String(), nil
}

// saveMessagesToStorage saves messages to local storage for history tracking
func (s *SlackAdapter) saveMessagesToStorage(channelID, channelName string, messages []SlackMessage) error {
	if !s.config.MaintainHistory {
		return nil // Don't save if not maintaining history
	}

	storagePath := filepath.Join(s.storageDir, "slack", "channels", channelID)
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Load existing messages
	existingMessages, err := s.loadMessagesFromStorage(channelID)
	if err != nil {
		existingMessages = []SlackMessage{}
	}

	// Merge messages
	allMessages := append(existingMessages, messages...)

	// Sort by timestamp
	sort.Slice(allMessages, func(i, j int) bool {
		ts1, _ := strconv.ParseFloat(allMessages[i].Timestamp, 64)
		ts2, _ := strconv.ParseFloat(allMessages[j].Timestamp, 64)
		return ts1 < ts2
	})

	// Save to file
	filePath := filepath.Join(storagePath, "messages.json")
	data, err := json.MarshalIndent(allMessages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// loadMessagesFromStorage loads messages from local storage
func (s *SlackAdapter) loadMessagesFromStorage(channelID string) ([]SlackMessage, error) {
	filePath := filepath.Join(s.storageDir, "slack", "channels", channelID, "messages.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var messages []SlackMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	return messages, nil
}

// GetLastSync returns the last sync time
func (s *SlackAdapter) GetLastSync() time.Time {
	return s.lastSync
}

// SetLastSync updates the last sync time
func (s *SlackAdapter) SetLastSync(t time.Time) {
	s.lastSync = t
}

// sanitizeChannelName sanitizes channel name for use in filenames
func sanitizeChannelName(name string) string {
	// Remove # prefix and replace invalid characters
	name = strings.TrimPrefix(name, "#")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, "@", "_")
	name = strings.ReplaceAll(name, "#", "_")

	// Remove any trailing underscores
	name = strings.TrimRight(name, "_")
	return name
}
