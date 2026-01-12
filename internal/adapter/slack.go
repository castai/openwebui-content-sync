package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openwebui-content-sync/internal/config"
	"github.com/openwebui-content-sync/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// SlackAdapter implements the Adapter interface for Slack
type SlackAdapter struct {
	config         config.SlackConfig
	client         *slack.Client
	lastSync       time.Time
	storageDir     string
	cachedChannels []slack.Channel // Cache channels for the entire sync session
}

// channelHasHistory returns true if we've previously stored any messages for the channel
func (s *SlackAdapter) channelHasHistory(channelID string) bool {
	filePath := filepath.Join(s.storageDir, "slack", "channels", channelID, "messages.json")
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

// SlackMessage represents a Slack message with metadata
type SlackMessage struct {
	Timestamp   string              `json:"timestamp"`
	User        string              `json:"user"`
	Text        string              `json:"text"`
	Channel     string              `json:"channel"` // stores channel name
	ThreadTS    string              `json:"thread_ts,omitempty"`
	Reactions   []SlackReaction     `json:"reactions,omitempty"`
	Files       []SlackFile         `json:"files,omitempty"`
	Attachments []SlackAttachment   `json:"attachments,omitempty"`
	Replies     []SlackMessageReply `json:"replies,omitempty"`
}

// SlackMessage represents a Slack message with metadata
type SlackMessageReply struct {
	Timestamp   string            `json:"timestamp"`
	User        string            `json:"user"`
	Text        string            `json:"text"`
	ThreadTS    string            `json:"thread_ts,omitempty"`
	Channel     string            `json:"channel"` // stores channel name
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
	logrus.Infof("Initializing Slack adapter with config: enabled=%v, channels=%d, days_to_fetch=%d, message_limit=%d",
		cfg.Enabled, len(cfg.ChannelMappings), cfg.DaysToFetch, cfg.MessageLimit)

	if !cfg.Enabled {
		logrus.Info("Slack adapter is disabled")
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

	// Log channel mappings
	for i, mapping := range cfg.ChannelMappings {
		logrus.Infof("Channel mapping %d: ID=%s, Name=%s, KnowledgeID=%s",
			i+1, mapping.ChannelID, mapping.ChannelName, mapping.KnowledgeID)
	}

	// Set defaults
	if cfg.DaysToFetch <= 0 {
		cfg.DaysToFetch = 30
		logrus.Infof("Set default days_to_fetch to %d", cfg.DaysToFetch)
	}
	if cfg.MessageLimit <= 0 {
		cfg.MessageLimit = 1000
		logrus.Infof("Set default message_limit to %d", cfg.MessageLimit)
	}

	client := slack.New(cfg.Token)
	logrus.Infof("Created Slack client with token starting with: %s", cfg.Token[:10]+"...")

	// Test the connection (skip for test tokens)
	if !strings.HasPrefix(cfg.Token, "xoxb-test-") {
		authTest, err := client.AuthTest()
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate with Slack: %w", err)
		}
		logrus.Infof("Successfully authenticated with Slack as: %s (team: %s)", authTest.User, authTest.Team)
	} else {
		logrus.Debugf("Skipping authentication for test token")
	}

	// Create storage directory for Slack
	slackStoragePath := filepath.Join(storageDir, "slack", "channels")
	if err := os.MkdirAll(slackStoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create slack storage directory: %w", err)
	}
	logrus.Infof("Created Slack storage directory: %s", slackStoragePath)

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
	logrus.Infof("Starting Slack adapter fetch with config: enabled=%v, maintain_history=%v, days_to_fetch=%d, message_limit=%d, include_threads=%v, include_reactions=%v",
		s.config.Enabled, s.config.MaintainHistory, s.config.DaysToFetch, s.config.MessageLimit, s.config.IncludeThreads, s.config.IncludeReactions)

	// Only clear channel cache if it's been more than 5 minutes since last discovery
	// This prevents unnecessary API calls during frequent syncs
	if s.cachedChannels != nil && len(s.cachedChannels) > 0 {
		logrus.Debugf("Using existing channel cache (%d channels) - skipping fresh discovery", len(s.cachedChannels))
	} else {
		logrus.Debugf("No cached channels available - will perform fresh discovery")
	}

	// Return empty slice if adapter is disabled
	if !s.config.Enabled {
		logrus.Infof("Slack adapter is disabled, returning empty files")
		return []*File{}, nil
	}

	var files []*File
	now := time.Now()

	// Calculate time range for fetching messages
	var oldestTime time.Time
	if s.config.MaintainHistory {
		// If maintaining history, fetch from last sync time
		if s.lastSync.IsZero() {
			// First run: fetch from the last N days
			oldestTime = now.AddDate(0, 0, -s.config.DaysToFetch)
			logrus.Infof("First run with maintain_history: fetching last %d days from %s (Unix: %d)", s.config.DaysToFetch, oldestTime.Format(time.RFC3339), oldestTime.Unix())
		} else {
			oldestTime = s.lastSync
			logrus.Infof("Maintaining history: fetching from last sync time %s (Unix: %d)", oldestTime.Format(time.RFC3339), oldestTime.Unix())
		}
	} else {
		// If not maintaining history, fetch only the last N days
		oldestTime = now.AddDate(0, 0, -s.config.DaysToFetch)
		logrus.Infof("Not maintaining history: fetching last %d days from %s (Unix: %d)", s.config.DaysToFetch, oldestTime.Format(time.RFC3339), oldestTime.Unix())
	}

	logrus.Infof("Time range for fetching messages: %s to %s", oldestTime.Format(time.RFC3339), now.Format(time.RFC3339))
	logrus.Infof("Unix timestamps: oldest=%d, latest=%d", oldestTime.Unix(), now.Unix())

	// Validate time range
	if oldestTime.After(now) {
		logrus.Errorf("Invalid time range: oldest time (%s) is after latest time (%s)",
			oldestTime.Format(time.RFC3339), now.Format(time.RFC3339))
		return []*File{}, fmt.Errorf("invalid time range: oldest time is after latest time")
	}

	timeRange := now.Sub(oldestTime)
	logrus.Infof("Time range duration: %v", timeRange)

	// Discover channels using regex patterns
	discoveredChannels, err := s.discoverChannelsByRegex(ctx)
	if err != nil {
		logrus.Warnf("Failed to discover channels by regex: %v", err)
	} else if len(discoveredChannels) > 0 {
		logrus.Infof("Discovered %d channels using regex patterns", len(discoveredChannels))
	}

	// Load locally known channels from storage to ensure we keep syncing even if discovery is rate limited
	localChannels := s.listLocalChannels()
	if len(localChannels) > 0 {
		logrus.Infof("Found %d locally known channels from storage", len(localChannels))
	}

	// Combine explicit channel mappings with discovered channels
	allChannels := make([]config.ChannelMapping, 0, len(s.config.ChannelMappings)+len(discoveredChannels)+len(localChannels))
	allChannels = append(allChannels, s.config.ChannelMappings...)
	allChannels = append(allChannels, discoveredChannels...)
	allChannels = append(allChannels, localChannels...)

	// Deduplicate by ChannelID, prefer explicit > discovered > local naming
	seenByID := make(map[string]config.ChannelMapping)
	for _, src := range []struct{ list []config.ChannelMapping }{
		{list: localChannels},
		{list: discoveredChannels},
		{list: s.config.ChannelMappings},
	} {
		for _, m := range src.list {
			if m.ChannelID == "" { // skip invalid
				continue
			}
			if existing, ok := seenByID[m.ChannelID]; ok {
				// Promote knowledge ID or name if missing in existing
				if existing.KnowledgeID == "" && m.KnowledgeID != "" {
					existing.KnowledgeID = m.KnowledgeID
				}
				if (existing.ChannelName == "" || existing.ChannelName == existing.ChannelID) && m.ChannelName != "" {
					existing.ChannelName = m.ChannelName
				}
				seenByID[m.ChannelID] = existing
				continue
			}
			seenByID[m.ChannelID] = m
		}
	}
	allChannels = allChannels[:0]
	for _, v := range seenByID {
		// Ensure all channels have a knowledge ID - use the first available one or fallback
		if v.KnowledgeID == "" {
			// Try to find a knowledge ID from explicit mappings first
			for _, explicit := range s.config.ChannelMappings {
				if explicit.KnowledgeID != "" {
					v.KnowledgeID = explicit.KnowledgeID
					break
				}
			}
			// If still empty, use the first regex pattern's knowledge ID
			if v.KnowledgeID == "" && len(s.config.RegexPatterns) > 0 {
				v.KnowledgeID = s.config.RegexPatterns[0].KnowledgeID
			}
			logrus.Debugf("Assigned knowledge ID %s to channel %s (%s)", v.KnowledgeID, v.ChannelName, v.ChannelID)
		}
		allChannels = append(allChannels, v)
	}

	logrus.Infof("Processing %d total channels (%d explicit mappings + %d discovered + %d local)",
		len(allChannels), len(s.config.ChannelMappings), len(discoveredChannels), len(localChannels))

	// Track processed channel IDs to ensure parity with local storage
	processed := make(map[string]bool)

	// Process each channel mapping
	for i, mapping := range allChannels {
		logrus.Infof("Processing channel %d/%d: %s (%s)", i+1, len(allChannels), mapping.ChannelName, mapping.ChannelID)

		// Test channel access first
		if err := s.testChannelAccess(mapping.ChannelID, mapping.ChannelName); err != nil {
			logrus.Errorf("Failed to access channel %s (%s): %v", mapping.ChannelName, mapping.ChannelID, err)
			// Continue processing other channels even if one fails
			continue
		}

		// Determine effective oldest time per channel
		effectiveOldest := oldestTime
		if s.config.MaintainHistory && !s.channelHasHistory(mapping.ChannelID) {
			// First time seeing this channel locally: backfill last N days
			effectiveOldest = now.AddDate(0, 0, -s.config.DaysToFetch)
			logrus.Infof("First local sync for channel %s (%s): backfilling last %d days from %s",
				mapping.ChannelName, mapping.ChannelID, s.config.DaysToFetch, effectiveOldest.Format(time.RFC3339))
		}

		// Fetch messages from the channel
		messages, err := s.fetchChannelMessages(ctx, mapping.ChannelID, mapping.ChannelName, effectiveOldest, now)
		if err != nil {
			logrus.Errorf("Failed to fetch messages from channel %s: %v", mapping.ChannelName, err)
			continue
		}

		// When maintaining history, we should create a file even if no new messages were found
		// because we want to include all historical messages
		if len(messages) == 0 && !s.config.MaintainHistory {
			logrus.Warnf("No new messages found in channel %s (%s)", mapping.ChannelName, mapping.ChannelID)
			continue
		}

		// When maintaining history, generate file content from deduplicated storage to avoid duplicates
		var fileContent string
		if s.config.MaintainHistory {
			// Save first (dedup inside), then load back for content generation
			if len(messages) > 0 {
				if err := s.saveMessagesToStorage(mapping.ChannelID, mapping.ChannelName, messages); err != nil {
					logrus.Warnf("Failed to save messages to storage for channel %s: %v", mapping.ChannelName, err)
				}
			}
			stored, err := s.loadMessagesFromStorage(mapping.ChannelID)
			if err != nil {
				logrus.Warnf("Failed to load messages from storage for channel %s: %v", mapping.ChannelName, err)
				// Fallback to current messages
				fileContent, err = s.messagesToFileContent(messages, mapping.ChannelName)
			} else {
				fileContent, err = s.messagesToFileContent(stored, mapping.ChannelName)
			}
		} else {
			fileContent, err = s.messagesToFileContent(messages, mapping.ChannelName)
		}
		if err != nil {
			logrus.Errorf("Failed to convert messages to file content for channel %s: %v", mapping.ChannelName, err)
			continue
		}

		// Skip creating file if content is empty
		if len(fileContent) == 0 {
			logrus.Warnf("No content generated for channel %s (%s), skipping file creation", mapping.ChannelName, mapping.ChannelID)
			continue
		}

		// Create file metadata
		filename := fmt.Sprintf("%s_messages.md", sanitizeChannelName(mapping.ChannelName))
		// Store just the filename here. The sync manager will place it under
		// data/files/<source>/ so avoiding a leading "slack/" prevents a duplicate
		// "slack/slack" path.
		filePath := filename

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
		processed[mapping.ChannelID] = true
		logrus.Debugf("Created file for channel %s (%s) -> %s (knowledge: %s)", mapping.ChannelName, mapping.ChannelID, filename, mapping.KnowledgeID)

		// Save messages to local storage for history tracking (no-op if not maintaining history)
		if !s.config.MaintainHistory {
			if err := s.saveMessagesToStorage(mapping.ChannelID, mapping.ChannelName, messages); err != nil {
				logrus.Warnf("Failed to save messages to storage for channel %s: %v", mapping.ChannelName, err)
			}

			// Fallback: for any locally known channels not processed (e.g., due to discovery rate limit
			// or missing access in this run), build files directly from stored history so that
			// data/slack/channels/* count matches data/files/slack/* count.
			for _, local := range localChannels {
				if processed[local.ChannelID] {
					continue
				}
				// Load stored messages; skip if none
				stored, err := s.loadMessagesFromStorage(local.ChannelID)
				if err != nil || len(stored) == 0 {
					continue
				}
				// Determine channel name from stored messages or mapping
				channelName := local.ChannelName
				if channelName == "" {
					last := stored[len(stored)-1]
					if last.Channel != "" {
						channelName = last.Channel
					} else {
						channelName = local.ChannelID
					}
				}
				content, err := s.messagesToFileContent(stored, channelName)
				if err != nil || len(content) == 0 {
					continue
				}
				filename := fmt.Sprintf("%s_messages.md", sanitizeChannelName(channelName))
				file := &File{
					Path:        filename,
					Content:     []byte(content),
					Hash:        fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
					Modified:    now,
					Size:        int64(len(content)),
					Source:      "slack",
					KnowledgeID: local.KnowledgeID,
				}
				files = append(files, file)
				logrus.Debugf("Added file from stored history for channel %s (%s)", channelName, local.ChannelID)
			}
		}

		logrus.Debugf("Processed %d messages from channel %s", len(messages), mapping.ChannelName)

		// Add a longer delay between channels to avoid Slack rate limits
		if i < len(allChannels)-1 { // Don't delay after the last channel
			delay := 500 * time.Millisecond // Increased delay for Slack rate limiting
			logrus.Debugf("Waiting %v before processing next channel to avoid rate limits", delay)
			time.Sleep(delay)
		}
	}

	// Update last sync time
	s.lastSync = now

	logrus.Infof("Fetched %d files from Slack channels", len(files))
	logrus.Infof("Channel processing summary: %d total channels, %d files created, %d channels processed",
		len(allChannels), len(files), len(processed))

	// Log any channels that weren't processed
	unprocessed := 0
	for _, mapping := range allChannels {
		if !processed[mapping.ChannelID] {
			unprocessed++
			logrus.Warnf("Channel %s (%s) was not processed - likely failed channel access test", mapping.ChannelName, mapping.ChannelID)
		}
	}
	if unprocessed > 0 {
		logrus.Warnf("%d channels were not processed due to errors (out of %d total channels)", unprocessed, len(allChannels))
	}

	// Save channel tracking file
	if err := s.saveChannelTracking(allChannels, processed); err != nil {
		logrus.Warnf("Failed to save channel tracking file: %v", err)
	}

	return files, nil
}

// fetchChannelMessages retrieves messages from a specific Slack channel
func (s *SlackAdapter) fetchChannelMessages(ctx context.Context, channelID, channelName string, oldestTime, latestTime time.Time) ([]SlackMessage, error) {
	logrus.Infof("Fetching messages from channel %s (%s) from %s to %s",
		channelName, channelID, oldestTime.Format(time.RFC3339), latestTime.Format(time.RFC3339))

	var allMessages []SlackMessage
	latest := latestTime.Unix()
	oldest := oldestTime.Unix()
	cursor := ""

	// Keep original latest time for consistent time range
	originalLatest := latest

	// Load existing messages from storage
	existingMessages, err := s.loadMessagesFromStorage(channelID)
	if err != nil {
		logrus.Debugf("No existing messages found for channel %s: %v", channelID, err)
		existingMessages = []SlackMessage{}
	} else {
		logrus.Infof("Loaded %d existing messages from storage for channel %s", len(existingMessages), channelID)
	}

	// Create a map of existing message timestamps for deduplication
	existingTimestamps := make(map[string]bool)
	for _, msg := range existingMessages {
		existingTimestamps[msg.Timestamp] = true
	}
	logrus.Infof("Created deduplication map with %d existing timestamps for channel %s", len(existingTimestamps), channelID)

	pageCount := 0
	for {
		pageCount++
		logrus.Infof("Fetching page %d for channel %s (cursor: %s)", pageCount, channelID, cursor)

		params := slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Latest:    fmt.Sprintf("%d", originalLatest), // Use original latest time
			Oldest:    fmt.Sprintf("%d", oldest),
			Limit:     200, // Slack API limit
			Cursor:    cursor,
		}

		logrus.Debugf("API call parameters: ChannelID=%s, Latest=%s, Oldest=%s, Limit=%d, Cursor=%s",
			params.ChannelID, params.Latest, params.Oldest, params.Limit, params.Cursor)

		var history *slack.GetConversationHistoryResponse
		retryConfig := utils.DefaultRetryConfig()
		retryConfig.BaseDelay = 1 * time.Second
		retryConfig.MaxDelay = 5 * time.Minute // Allow longer delays for Slack rate limits
		retryConfig.MaxRetries = 5             // More retries for Slack API

		err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
			var err error
			history, err = s.client.GetConversationHistory(&params)
			return err
		})

		if err != nil {
			logrus.Errorf("Failed to get conversation history for channel %s after retries: %v", channelID, err)
			return nil, fmt.Errorf("failed to get conversation history after retries: %w", err)
		}

		logrus.Infof("API response for channel %s: %d messages, has_more=%v, next_cursor=%s",
			channelID, len(history.Messages), history.HasMore, history.ResponseMetaData.NextCursor)

		// Convert Slack messages to our format
		newMessagesCount := 0
		for _, msg := range history.Messages {
			// Skip if we already have this message
			if existingTimestamps[msg.Timestamp] {
				logrus.Debugf("Skipping duplicate message with timestamp %s", msg.Timestamp)
				continue
			}

			// Determine if this is a thread starter message
			// A message is a thread starter if:
			// - ThreadTimestamp is empty (regular message that might have replies)
			// - OR ThreadTimestamp equals Timestamp (explicit thread starter)
			// We should NOT fetch replies for messages that are replies themselves
			// (ThreadTimestamp != "" && ThreadTimestamp != Timestamp)
			var replies []slack.Msg
			isThreadStarter := msg.Msg.ThreadTimestamp == "" || msg.Msg.ThreadTimestamp == msg.Msg.Timestamp
			isReply := msg.Msg.ThreadTimestamp != "" && msg.Msg.ThreadTimestamp != msg.Msg.Timestamp

			if isThreadStarter && !isReply {
				// Fetch thread replies for thread starter messages
				var err error
				replies, err = s.fetchThreadReplies(ctx, channelID, msg.Msg.Timestamp)
				if err != nil {
					logrus.Warnf("Failed to fetch thread replies for message %s: %v", msg.Msg.Timestamp, err)
					// Continue processing even if thread fetch fails
					replies = []slack.Msg{}
				} else {
					logrus.Debugf("Fetched %d replies for thread starter message %s", len(replies), msg.Msg.Timestamp)
				}

				// Add a small delay to respect Slack rate limits
				time.Sleep(100 * time.Millisecond)
			} else if isReply {
				// This is a reply message - skip fetching replies (it's already a reply)
				logrus.Debugf("Skipping thread reply fetch for reply message %s (parent: %s)", msg.Msg.Timestamp, msg.Msg.ThreadTimestamp)
			}

			slackMsg := s.convertSlackMessage(msg.Msg, channelName, replies)
			allMessages = append(allMessages, slackMsg)
			newMessagesCount++

			logrus.Debugf("Added message: timestamp=%s, user=%s, text_length=%d, replies=%d",
				msg.Timestamp, msg.User, len(msg.Text), len(replies))
		}

		logrus.Infof("Processed %d new messages from page %d for channel %s", newMessagesCount, pageCount, channelID)

		// Break if no more messages or reached limit
		if history.ResponseMetaData.NextCursor == "" || len(history.Messages) == 0 {
			logrus.Infof("Reached end of messages for channel %s (has_more=%v, messages=%d)",
				channelID, history.HasMore, len(history.Messages))
			break
		}

		// Check if we've reached the message limit
		if len(allMessages) >= s.config.MessageLimit {
			logrus.Infof("Reached message limit (%d) for channel %s", s.config.MessageLimit, channelID)
			break
		}

		cursor = history.ResponseMetaData.NextCursor
	}

	logrus.Infof("Total new messages fetched for channel %s: %d", channelID, len(allMessages))

	// Return only newly fetched messages; merging with existing will be handled by storage layer
	return allMessages, nil
}

// fetchThreadReplies retrieves replies to a specific thread
func (s *SlackAdapter) fetchThreadReplies(ctx context.Context, channelID, threadTS string) ([]slack.Msg, error) {
	// Check if threads are enabled
	if !s.config.IncludeThreads {
		return []slack.Msg{}, nil
	}

	// If threadTS is empty, this is not a thread
	if threadTS == "" {
		return []slack.Msg{}, nil
	}

	logrus.Debugf("Fetching thread replies for thread %s in channel %s", threadTS, channelID)

	var allReplies []slack.Msg
	cursor := ""
	pageCount := 0

	for {
		pageCount++
		logrus.Debugf("Fetching thread replies page %d for thread %s (cursor: %s)", pageCount, threadTS, cursor)

		params := slack.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Limit:     200, // Slack API limit
			Cursor:    cursor,
		}

		var replies []slack.Message
		var hasMore bool
		var nextCursor string
		var err error

		retryConfig := utils.DefaultRetryConfig()
		retryConfig.BaseDelay = 1 * time.Second
		retryConfig.MaxDelay = 5 * time.Minute // Allow longer delays for Slack rate limits
		retryConfig.MaxRetries = 5             // More retries for Slack API

		err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
			replies, hasMore, nextCursor, err = s.client.GetConversationReplies(&params)
			return err
		})

		if err != nil {
			logrus.Errorf("Failed to get conversation replies for thread %s after retries: %v", threadTS, err)
			return nil, fmt.Errorf("failed to get conversation replies after retries: %w", err)
		}

		logrus.Debugf("API response for thread %s: %d messages, has_more=%v, next_cursor=%s",
			threadTS, len(replies), hasMore, nextCursor)

		// Skip the first message (it's the parent message itself)
		// Only include actual replies
		if len(replies) > 0 {
			for i, reply := range replies {
				if i == 0 {
					// First message is the parent - skip it
					continue
				}
				// Convert slack.Message to slack.Msg
				allReplies = append(allReplies, reply.Msg)
			}
		}

		// Break if no more pages
		if !hasMore || nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	logrus.Debugf("Fetched %d replies for thread %s", len(allReplies), threadTS)
	return allReplies, nil
}

// convertSlackMessage converts a Slack message to our format
func (s *SlackAdapter) convertSlackMessage(msg slack.Msg, channelName string, replies []slack.Msg) SlackMessage {
	slackMsg := SlackMessage{
		Timestamp: msg.Timestamp,
		User:      msg.User,
		Text:      msg.Text,
		Channel:   channelName,
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

	// Convert replies to our format
	if len(replies) > 0 {
		for _, reply := range replies {
			slackMsg.Replies = append(slackMsg.Replies, SlackMessageReply{
				Timestamp:   reply.Timestamp,
				User:        reply.User,
				Text:        reply.Text,
				Channel:     channelName,
				ThreadTS:    reply.ThreadTimestamp,
				Reactions:   s.convertReactions(reply.Reactions),
				Files:       s.convertFiles(reply.Files),
				Attachments: s.convertAttachments(reply.Attachments),
			})
		}
	}

	return slackMsg
}

// convertReactions converts Slack reactions to our format
func (s *SlackAdapter) convertReactions(reactions []slack.ItemReaction) []SlackReaction {
	var converted []SlackReaction
	for _, reaction := range reactions {
		converted = append(converted, SlackReaction{
			Name:  reaction.Name,
			Count: reaction.Count,
			Users: reaction.Users,
		})
	}
	return converted
}

// convertFiles converts Slack files to our format
func (s *SlackAdapter) convertFiles(files []slack.File) []SlackFile {
	var converted []SlackFile
	for _, file := range files {
		converted = append(converted, SlackFile{
			ID:       file.ID,
			Name:     file.Name,
			Title:    file.Title,
			Mimetype: file.Mimetype,
			URL:      file.URLPrivate,
		})
	}
	return converted
}

// convertAttachments converts Slack attachments to our format
func (s *SlackAdapter) convertAttachments(attachments []slack.Attachment) []SlackAttachment {
	var converted []SlackAttachment
	for _, attachment := range attachments {
		converted = append(converted, SlackAttachment{
			Title:      attachment.Title,
			Text:       attachment.Text,
			Fallback:   attachment.Fallback,
			Color:      attachment.Color,
			AuthorName: attachment.AuthorName,
		})
	}
	return converted
}

// testChannelAccess tests if the bot can access the channel and attempts to join if needed
func (s *SlackAdapter) testChannelAccess(channelID, channelName string) error {
	logrus.Debugf("Testing access to channel %s (%s)", channelName, channelID)

	// Get channel information to check membership
	channel, err := s.client.GetConversationInfo(&slack.GetConversationInfoInput{
		ChannelID: channelID,
	})
	if err != nil {
		logrus.Warnf("Failed to get channel info for %s (%s): %v - will attempt to process anyway", channelName, channelID, err)
		return nil // Don't fail - some channels might be accessible during actual processing
	}

	logrus.Debugf("Channel info: Name=%s, ID=%s, IsMember=%v, IsPrivate=%v, NumMembers=%d",
		channel.Name, channel.ID, channel.IsMember, channel.IsPrivate, channel.NumMembers)

	// Check if bot is a member of the channel
	if !channel.IsMember {
		logrus.Infof("Bot is not a member of channel %s (%s) - attempting to join", channelName, channelID)
		if err := s.joinChannel(context.Background(), channelID); err != nil {
			// Log detailed error information
			s.logJoinError(channelName, channelID, err)

			// Check if this is a permanent error that should skip the channel
			if s.isPermanentJoinError(err) {
				logrus.Errorf("Permanent join error for channel %s (%s): %v - skipping channel", channelName, channelID, err)
				return fmt.Errorf("permanent join error for channel %s (%s): %w", channelName, channelID, err)
			} else {
				logrus.Warnf("Retryable join error for channel %s (%s): %v - will attempt to process anyway", channelName, channelID, err)
				// Don't return error for retryable errors - continue processing
			}
		} else {
			logrus.Infof("Successfully joined channel %s (%s)", channelName, channelID)
		}
	} else {
		logrus.Debugf("Bot is already a member of channel %s (%s)", channelName, channelID)
	}

	return nil
}

// logJoinError logs detailed information about channel join errors to a file
func (s *SlackAdapter) logJoinError(channelName, channelID string, joinErr error) {
	// Create error log file path
	errorLogPath := filepath.Join(s.storageDir, "slack", "join_errors.log")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(errorLogPath), 0755); err != nil {
		logrus.Errorf("Failed to create error log directory: %v", err)
		return
	}

	// Open file for appending
	file, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logrus.Errorf("Failed to open error log file: %v", err)
		return
	}
	defer file.Close()

	// Write detailed error information
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(file, "[%s] JOIN_ERROR: Channel=%s ID=%s Error=%v\n",
		timestamp, channelName, channelID, joinErr)

	logrus.Debugf("Join error logged to: %s", errorLogPath)
}

// isPermanentJoinError checks if a join error is permanent and should skip the channel
func (s *SlackAdapter) isPermanentJoinError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	permanentErrors := []string{
		"is_archived",
		"channel_not_found",
		"cant_invite_self",
		"invalid_auth",
		"account_inactive",
		"token_revoked",
		"not_authed",
		"invalid_auth",
	}

	for _, permanentErr := range permanentErrors {
		if strings.Contains(errStr, permanentErr) {
			return true
		}
	}

	return false
}

// saveChannelTracking saves a tracking file with all channels and their knowledge IDs
func (s *SlackAdapter) saveChannelTracking(allChannels []config.ChannelMapping, processed map[string]bool) error {
	trackingPath := filepath.Join(s.storageDir, "slack", "channels", "channel_tracking.txt")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(trackingPath), 0755); err != nil {
		return fmt.Errorf("failed to create tracking directory: %w", err)
	}

	file, err := os.Create(trackingPath)
	if err != nil {
		return fmt.Errorf("failed to create tracking file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "Slack Channel Tracking Report\n")
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Total Channels: %d\n", len(allChannels))
	fmt.Fprintf(file, "Processed Channels: %d\n", len(processed))
	fmt.Fprintf(file, "Unprocessed Channels: %d\n", len(allChannels)-len(processed))
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "%-50s %-20s %-40s %-10s\n", "Channel Name", "Channel ID", "Knowledge ID", "Status")
	fmt.Fprintf(file, "%s\n", strings.Repeat("-", 120))

	// Sort channels by name for better readability
	sort.Slice(allChannels, func(i, j int) bool {
		return allChannels[i].ChannelName < allChannels[j].ChannelName
	})

	// Write channel details
	for _, mapping := range allChannels {
		status := "PROCESSED"
		if !processed[mapping.ChannelID] {
			status = "FAILED"
		}

		knowledgeID := mapping.KnowledgeID
		if knowledgeID == "" {
			knowledgeID = "DEFAULT"
		}

		fmt.Fprintf(file, "%-50s %-20s %-40s %-10s\n",
			mapping.ChannelName,
			mapping.ChannelID,
			knowledgeID,
			status)
	}

	logrus.Infof("Channel tracking file saved to: %s", trackingPath)
	return nil
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

		// Add replies if they exist
		if len(msg.Replies) > 0 {
			content.WriteString("**Replies:**\n")
			for _, reply := range msg.Replies {
				replyTime, err := strconv.ParseFloat(reply.Timestamp, 64)
				if err != nil {
					logrus.Warnf("Failed to parse reply timestamp %s: %v", reply.Timestamp, err)
					continue
				}

				content.WriteString(fmt.Sprintf("  - Reply at %s by %s:\n", time.Unix(int64(replyTime), 0).Format("2006-01-02 15:04:05"), reply.User))
				if reply.Text != "" {
					content.WriteString(fmt.Sprintf("    %s\n", reply.Text))
				}
			}
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

	// Deduplicate by timestamp while preserving order
	seen := make(map[string]bool, len(existingMessages))
	for _, m := range existingMessages {
		seen[m.Timestamp] = true
	}

	deduped := make([]SlackMessage, 0, len(existingMessages)+len(messages))
	deduped = append(deduped, existingMessages...)
	added := 0
	for _, m := range messages {
		if m.Timestamp == "" {
			continue
		}
		if !seen[m.Timestamp] {
			deduped = append(deduped, m)
			seen[m.Timestamp] = true
			added++
		}
	}
	logrus.Infof("Deduplicated Slack messages for channel %s: existing=%d, new=%d, added=%d, total=%d", channelName, len(existingMessages), len(messages), added, len(deduped))

	// Sort by timestamp
	sort.Slice(deduped, func(i, j int) bool {
		ts1, _ := strconv.ParseFloat(deduped[i].Timestamp, 64)
		ts2, _ := strconv.ParseFloat(deduped[j].Timestamp, 64)
		return ts1 < ts2
	})

	// Save to file
	filePath := filepath.Join(storagePath, "messages.json")
	data, err := json.MarshalIndent(deduped, "", "  ")
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

	// Ensure Channel field is set to channel name in case older entries had ID
	// Note: We cannot map ID->name here reliably without an index; newer fetch
	// now writes channel names so this preserves forward correctness.

	return messages, nil
}

// listLocalChannels scans local storage and returns channel mappings for any channel directories found.
// ChannelName is set to the directory name if a recent message file includes a channel name; otherwise uses ID.
func (s *SlackAdapter) listLocalChannels() []config.ChannelMapping {
	root := filepath.Join(s.storageDir, "slack", "channels")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var mappings []config.ChannelMapping
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		chID := e.Name()
		// Try to read messages.json and extract a channel name from the last entry
		name := chID
		msgs, err := s.loadMessagesFromStorage(chID)
		if err == nil && len(msgs) > 0 {
			last := msgs[len(msgs)-1]
			if last.Channel != "" {
				name = last.Channel
			}
		}
		mappings = append(mappings, config.ChannelMapping{
			ChannelID:   chID,
			ChannelName: name,
			KnowledgeID: "", // may be filled by discovered/explicit merge
		})
	}
	return mappings
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

// discoverChannelsByRegex discovers channels that match the configured regex patterns
func (s *SlackAdapter) discoverChannelsByRegex(ctx context.Context) ([]config.ChannelMapping, error) {
	if len(s.config.RegexPatterns) == 0 {
		logrus.Debugf("No regex patterns configured for channel discovery")
		return []config.ChannelMapping{}, nil
	}

	logrus.Infof("Starting channel discovery using %d regex patterns", len(s.config.RegexPatterns))

	// Get all channels the bot can see (use cache if available, otherwise fetch)
	var channels []slack.Channel
	var err error

	if len(s.cachedChannels) > 0 {
		logrus.Debugf("Using cached channel list (%d channels)", len(s.cachedChannels))
		channels = s.cachedChannels
	} else {
		logrus.Debugf("Fetching fresh channel list...")
		channels, err = s.getAllChannels(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get channels: %w", err)
		}
		// Cache the channels for this sync session
		s.cachedChannels = channels
		logrus.Debugf("Cached %d channels for this sync session", len(channels))
	}

	logrus.Debugf("Found %d total channels to evaluate against regex patterns", len(channels))

	// Log all discovered channels for debugging
	logrus.Infof("All discovered channels:")
	for i, channel := range channels {
		if i < 10 { // Log first 10 channels
			logrus.Infof("  %d: %s (%s) - Member: %v, Private: %v", i+1, channel.Name, channel.ID, channel.IsMember, channel.IsPrivate)
		}
	}
	if len(channels) > 10 {
		logrus.Infof("  ... and %d more channels", len(channels)-10)
	}

	// Debug: Log all discovered channels for analysis
	logrus.Infof("All discovered channels:")
	for i, channel := range channels {
		if i < 10 { // Log first 10 channels
			logrus.Infof("Channel %d: %s (%s) - Member: %v, Private: %v",
				i+1, channel.Name, channel.ID, channel.IsMember, channel.IsPrivate)
		}
	}

	var discoveredChannels []config.ChannelMapping
	seenChannels := make(map[string]bool) // Track channels we've already processed

	// Process each regex pattern
	for _, pattern := range s.config.RegexPatterns {
		logrus.Infof("Evaluating regex pattern: %s (knowledge: %s, auto_join: %v)",
			pattern.Pattern, pattern.KnowledgeID, pattern.AutoJoin)

		// Compile the regex pattern
		regex, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			logrus.Errorf("Invalid regex pattern '%s': %v", pattern.Pattern, err)
			continue
		}

		// Find channels that match this pattern
		for _, channel := range channels {
			// Skip if we've already processed this channel
			if seenChannels[channel.ID] {
				continue
			}

			// Check if channel name matches the pattern
			if regex.MatchString(channel.Name) {
				logrus.Debugf("Regex match: pattern='%s' channel='%s' id='%s'", pattern.Pattern, channel.Name, channel.ID)
				logrus.Infof("Channel '%s' (%s) matches pattern '%s'", channel.Name, channel.ID, pattern.Pattern)

				// Check if we need to join the channel
				if pattern.AutoJoin && !channel.IsMember {
					logrus.Infof("Auto-joining channel '%s' (%s)", channel.Name, channel.ID)
					if err := s.joinChannel(ctx, channel.ID); err != nil {
						logrus.Errorf("Failed to join channel '%s' (%s): %v", channel.Name, channel.ID, err)
						// Log detailed error information
						s.logJoinError(channel.Name, channel.ID, err)
						continue
					}
					logrus.Infof("Successfully joined channel '%s' (%s)", channel.Name, channel.ID)
				}

				// Add to discovered channels
				discoveredChannels = append(discoveredChannels, config.ChannelMapping{
					ChannelID:   channel.ID,
					ChannelName: channel.Name,
					KnowledgeID: pattern.KnowledgeID,
				})

				seenChannels[channel.ID] = true
				logrus.Infof("Added discovered channel: %s (%s) -> knowledge %s",
					channel.Name, channel.ID, pattern.KnowledgeID)
			}
		}
	}

	logrus.Infof("Channel discovery completed: found %d matching channels", len(discoveredChannels))
	return discoveredChannels, nil
}

// getAllChannels retrieves all channels the bot can access
func (s *SlackAdapter) getAllChannels(ctx context.Context) ([]slack.Channel, error) {
	logrus.Debugf("Fetching all accessible channels...")

	var allChannels []slack.Channel
	cursor := ""
	pageCount := 0

	// Use reasonable delays to avoid rate limits, but don't artificially limit discovery
	const perPageDelay = 200 * time.Millisecond

	for {
		// Respect a per-page delay to smooth out bursts
		if pageCount > 0 {
			select {
			case <-time.After(perPageDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Get channels with pagination using retry logic
		var channels []slack.Channel
		var nextCursor string
		var err error

		retryConfig := utils.DefaultRetryConfig()
		retryConfig.BaseDelay = 2 * time.Second
		retryConfig.MaxDelay = 5 * time.Minute // Allow longer delays for Slack rate limits
		retryConfig.MaxRetries = 5             // More retries for Slack API

		err = utils.RetryWithBackoff(ctx, retryConfig, func() error {
			channels, nextCursor, err = s.client.GetConversations(&slack.GetConversationsParameters{
				Types:  []string{"public_channel", "private_channel"},
				Cursor: cursor,
				Limit:  200, // Maximum allowed by Slack API
			})
			return err
		})

		if err != nil {
			logrus.Errorf("Failed to get conversations after retries: %v", err)
			return nil, fmt.Errorf("failed to get conversations after retries: %w", err)
		}

		logrus.Debugf("Retrieved %d channels (cursor: %s)", len(channels), cursor)

		// Log each channel name for debugging
		for _, channel := range channels {
			logrus.Debugf("Retrieved channel: %s (ID: %s, Member: %v, Private: %v)",
				channel.Name, channel.ID, channel.IsMember, channel.IsPrivate)
		}

		allChannels = append(allChannels, channels...)
		pageCount++

		// Check if we have more pages
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logrus.Debugf("Total channels retrieved: %d", len(allChannels))
	return allChannels, nil
}

// joinChannel attempts to join a Slack channel with retry logic
func (s *SlackAdapter) joinChannel(ctx context.Context, channelID string) error {
	logrus.Debugf("Attempting to join channel: %s", channelID)

	retryConfig := utils.DefaultRetryConfig()
	retryConfig.BaseDelay = 1 * time.Second
	retryConfig.MaxDelay = 30 * time.Second
	retryConfig.MaxRetries = 3

	err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
		// Use the Slack API to join the channel
		_, _, _, err := s.client.JoinConversation(channelID)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to join channel %s after retries: %w", channelID, err)
	}

	logrus.Debugf("Successfully joined channel: %s", channelID)
	return nil
}
