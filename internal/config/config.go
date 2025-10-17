package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	LogLevel     string            `yaml:"log_level"`
	Schedule     ScheduleConfig    `yaml:"schedule"`
	Storage      StorageConfig     `yaml:"storage"`
	OpenWebUI    OpenWebUIConfig   `yaml:"openwebui"`
	GitHub       GitHubConfig      `yaml:"github"`
	Confluence   ConfluenceConfig  `yaml:"confluence"`
	LocalFolders LocalFolderConfig `yaml:"local_folders"`
	Slack        SlackConfig       `yaml:"slack"`
}

// ScheduleConfig defines the sync schedule
type ScheduleConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// StorageConfig defines local storage settings
type StorageConfig struct {
	Path string `yaml:"path"`
}

// OpenWebUIConfig defines OpenWebUI API settings
type OpenWebUIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

// RepositoryMapping defines a mapping between a GitHub repository and a knowledge base
type RepositoryMapping struct {
	Repository  string `yaml:"repository"` // Format: "owner/repo"
	KnowledgeID string `yaml:"knowledge_id"`
}

// SpaceMapping defines a mapping between a Confluence space and a knowledge base
type SpaceMapping struct {
	SpaceKey    string `yaml:"space_key"`
	KnowledgeID string `yaml:"knowledge_id"`
}

// ParentPageMapping defines a mapping between a Confluence parent page and a knowledge base
type ParentPageMapping struct {
	ParentPageID string `yaml:"parent_page_id"`
	KnowledgeID  string `yaml:"knowledge_id"`
}

// LocalFolderMapping defines a mapping between a local folder and a knowledge base
type LocalFolderMapping struct {
	FolderPath  string `yaml:"folder_path"`
	KnowledgeID string `yaml:"knowledge_id"`
}

// GitHubConfig defines GitHub adapter settings
type GitHubConfig struct {
	Enabled  bool                `yaml:"enabled"`
	Token    string              `yaml:"token"`
	Mappings []RepositoryMapping `yaml:"mappings"` // Per-repository knowledge mappings
}

// ConfluenceConfig defines Confluence adapter settings
type ConfluenceConfig struct {
	Enabled            bool                `yaml:"enabled"`
	BaseURL            string              `yaml:"base_url"`
	Username           string              `yaml:"username"`
	APIKey             string              `yaml:"api_key"`
	SpaceMappings      []SpaceMapping      `yaml:"space_mappings"`       // Per-space knowledge mappings
	ParentPageMappings []ParentPageMapping `yaml:"parent_page_mappings"` // Per-parent-page knowledge mappings
	PageLimit          int                 `yaml:"page_limit"`
	IncludeAttachments bool                `yaml:"include_attachments"`
	UseMarkdownParser  bool                `yaml:"use_markdown_parser"`
	IncludeBlogPosts   bool                `yaml:"include_blog_posts"`
}

// LocalFolderConfig defines local folder adapter settings
type LocalFolderConfig struct {
	Enabled  bool                 `yaml:"enabled"`
	Mappings []LocalFolderMapping `yaml:"mappings"` // Per-folder knowledge mappings
}

// SlackConfig defines Slack adapter settings
type SlackConfig struct {
	Enabled          bool             `yaml:"enabled"`
	Token            string           `yaml:"token"`
	ChannelMappings  []ChannelMapping `yaml:"channel_mappings"`  // Per-channel knowledge mappings
	RegexPatterns    []RegexPattern   `yaml:"regex_patterns"`    // Regex patterns for auto-discovering channels
	DaysToFetch      int              `yaml:"days_to_fetch"`     // Number of days to fetch messages
	MaintainHistory  bool             `yaml:"maintain_history"`  // Whether to maintain indefinite history or age off
	MessageLimit     int              `yaml:"message_limit"`     // Max messages per channel per run
	IncludeThreads   bool             `yaml:"include_threads"`   // Whether to include thread messages
	IncludeReactions bool             `yaml:"include_reactions"` // Whether to include reaction data
}

// ChannelMapping defines mapping between Slack channels and knowledge bases
type ChannelMapping struct {
	ChannelID   string `yaml:"channel_id"`   // Slack channel ID
	ChannelName string `yaml:"channel_name"` // Slack channel name (for display)
	KnowledgeID string `yaml:"knowledge_id"` // Target knowledge base ID
}

// RegexPattern defines regex patterns for auto-discovering Slack channels
type RegexPattern struct {
	Pattern     string `yaml:"pattern"`      // Regex pattern to match channel names
	KnowledgeID string `yaml:"knowledge_id"` // Target knowledge base ID for matching channels
	AutoJoin    bool   `yaml:"auto_join"`    // Whether to automatically join matching channels
}

// Load loads configuration from file and environment variables
func Load(path string) (*Config, error) {
	fmt.Printf("Loading configuration from: %s\n", path)

	cfg := &Config{
		LogLevel: "info",
		Schedule: ScheduleConfig{
			Interval: 1 * time.Hour,
		},
		Storage: StorageConfig{
			Path: "/data",
		},
		OpenWebUI: OpenWebUIConfig{
			BaseURL: getEnv("OPENWEBUI_BASE_URL", "http://localhost:8080"),
			APIKey:  getEnv("OPENWEBUI_API_KEY", ""),
		},
		GitHub: GitHubConfig{
			Enabled:  false,
			Token:    getEnv("GITHUB_TOKEN", ""),
			Mappings: []RepositoryMapping{},
		},
		Confluence: ConfluenceConfig{
			Enabled:            false,
			BaseURL:            "",
			Username:           "",
			APIKey:             getEnv("CONFLUENCE_API_KEY", ""),
			SpaceMappings:      []SpaceMapping{},
			ParentPageMappings: []ParentPageMapping{},
			PageLimit:          100,
			IncludeAttachments: true,
			UseMarkdownParser:  false,
			IncludeBlogPosts:   false,
		},
		LocalFolders: LocalFolderConfig{
			Enabled:  false,
			Mappings: []LocalFolderMapping{},
		},
		Slack: SlackConfig{
			Enabled:          false,
			Token:            getEnv("SLACK_TOKEN", ""),
			ChannelMappings:  []ChannelMapping{},
			DaysToFetch:      30,
			MaintainHistory:  false,
			MessageLimit:     1000,
			IncludeThreads:   true,
			IncludeReactions: false,
		},
	}

	fmt.Printf("Default OpenWebUI BaseURL: %s\n", cfg.OpenWebUI.BaseURL)
	fmt.Printf("Confluence API Key loaded: %s\n", func() string {
		if cfg.Confluence.APIKey != "" {
			return "***" + cfg.Confluence.APIKey[len(cfg.Confluence.APIKey)-4:] // Show last 4 chars
		}
		return "NOT SET"
	}())

	// Load from file if it exists
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Config file exists, loading from: %s\n", path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// fmt.Printf("Config file content:\n%s\n", string(data))

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		fmt.Printf("After loading config file - OpenWebUI BaseURL: %s\n", cfg.OpenWebUI.BaseURL)
	} else {
		fmt.Printf("Config file does not exist at: %s (error: %v)\n", path, err)
	}

	// Override with environment variables
	cfg.OpenWebUI.BaseURL = getEnv("OPENWEBUI_BASE_URL", cfg.OpenWebUI.BaseURL)
	cfg.OpenWebUI.APIKey = getEnv("OPENWEBUI_API_KEY", cfg.OpenWebUI.APIKey)
	cfg.GitHub.Token = getEnv("GITHUB_TOKEN", cfg.GitHub.Token)
	cfg.Confluence.APIKey = getEnv("CONFLUENCE_API_KEY", cfg.Confluence.APIKey)
	cfg.Storage.Path = getEnv("STORAGE_PATH", cfg.Storage.Path)

	fmt.Printf("Final OpenWebUI BaseURL: %s\n", cfg.OpenWebUI.BaseURL)
	fmt.Printf("Environment OPENWEBUI_BASE_URL: %s\n", os.Getenv("OPENWEBUI_BASE_URL"))
	fmt.Printf("Final Confluence API Key: %s\n", func() string {
		if cfg.Confluence.APIKey != "" {
			return "***" + cfg.Confluence.APIKey[len(cfg.Confluence.APIKey)-4:] // Show last 4 chars
		}
		return "NOT SET"
	}())
	fmt.Printf("Environment CONFLUENCE_API_KEY: %s\n", func() string {
		env := os.Getenv("CONFLUENCE_API_KEY")
		if env != "" {
			return "***" + env[len(env)-4:] // Show last 4 chars
		}
		return "NOT SET"
	}())

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
