package adapter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v56/github"
	"github.com/openwebui-content-sync/internal/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// GitHubAdapter implements the Adapter interface for GitHub repositories
type GitHubAdapter struct {
	client       *github.Client
	config       config.GitHubConfig
	lastSync     time.Time
	repositories []string
	mappings     map[string]string // repository -> knowledge_id mapping
}

// NewGitHubAdapter creates a new GitHub adapter
func NewGitHubAdapter(cfg config.GitHubConfig) (*GitHubAdapter, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// Build repository mappings
	mappings := make(map[string]string)
	repos := []string{}

	// Process mappings
	for _, mapping := range cfg.Mappings {
		if mapping.Repository != "" && mapping.KnowledgeID != "" {
			mappings[mapping.Repository] = mapping.KnowledgeID
			repos = append(repos, mapping.Repository)
		}
	}

	if len(repos) == 0 {
		return nil, fmt.Errorf("at least one repository mapping must be configured")
	}

	return &GitHubAdapter{
		client:       client,
		config:       cfg,
		repositories: repos,
		mappings:     mappings,
		lastSync:     time.Now().Add(-24 * time.Hour), // Default to 24 hours ago
	}, nil
}

// Name returns the adapter name
func (g *GitHubAdapter) Name() string {
	return "github"
}

// FetchFiles retrieves files from GitHub repositories
func (g *GitHubAdapter) FetchFiles(ctx context.Context) ([]*File, error) {
	var files []*File

	for _, repo := range g.repositories {
		logrus.Debugf("Fetching files from repository: %s", repo)
		knowledgeID := g.mappings[repo]
		repoFiles, err := g.fetchRepositoryFiles(ctx, repo, knowledgeID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch files from repository %s: %w", repo, err)
		}
		logrus.Debugf("Found %d files in repository %s (knowledge_id: %s)", len(repoFiles), repo, knowledgeID)
		files = append(files, repoFiles...)
	}

	logrus.Debugf("Total files fetched: %d", len(files))
	return files, nil
}

// fetchRepositoryFiles fetches files from a specific repository
func (g *GitHubAdapter) fetchRepositoryFiles(ctx context.Context, repo string, knowledgeID string) ([]*File, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format, expected 'owner/repo'")
	}

	owner, repoName := parts[0], parts[1]

	// Get repository contents
	_, contents, _, err := g.client.Repositories.GetContents(ctx, owner, repoName, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository contents: %w", err)
	}

	var files []*File
	for _, content := range contents {
		fileList, err := g.processContent(ctx, owner, repoName, content, "", knowledgeID)
		if err != nil {
			continue // Skip files that can't be processed
		}
		if fileList != nil {
			files = append(files, fileList...)
		}
	}

	return files, nil
}

// processContent processes a GitHub content item recursively
func (g *GitHubAdapter) processContent(ctx context.Context, owner, repo string, content *github.RepositoryContent, path string, knowledgeID string) ([]*File, error) {
	if content == nil {
		return nil, nil
	}

	currentPath := filepath.Join(path, content.GetName())

	// Skip binary files and non-text files
	if content.GetType() == "file" {
		// Check if it's a text file
		if !isTextFile(content.GetName()) {
			return nil, nil
		}

		// Get file content
		fileContent, err := g.getFileContent(ctx, owner, repo, content)
		if err != nil {
			return nil, fmt.Errorf("failed to get file content: %w", err)
		}

		// Calculate hash
		hash := fmt.Sprintf("%x", sha256.Sum256(fileContent))

		return []*File{{
			Path:        currentPath,
			Content:     fileContent,
			Hash:        hash,
			Modified:    time.Now(), // GitHub API doesn't provide modification time for content
			Size:        int64(len(fileContent)),
			Source:      fmt.Sprintf("%s/%s", owner, repo),
			KnowledgeID: knowledgeID,
		}}, nil
	}

	// If it's a directory, recurse
	if content.GetType() == "dir" {
		_, contents, _, err := g.client.Repositories.GetContents(ctx, owner, repo, content.GetPath(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get directory contents: %w", err)
		}

		var allFiles []*File
		for _, subContent := range contents {
			files, err := g.processContent(ctx, owner, repo, subContent, currentPath, knowledgeID)
			if err != nil {
				continue
			}
			if files != nil {
				allFiles = append(allFiles, files...)
			}
		}

		return allFiles, nil
	}

	return nil, nil
}

// getFileContent retrieves the actual content of a file
func (g *GitHubAdapter) getFileContent(ctx context.Context, owner, repo string, content *github.RepositoryContent) ([]byte, error) {
	fileContent, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if fileContent != "" {
		// Content is already available (for small files)
		return []byte(fileContent), nil
	}

	// For larger files, we need to download them
	url := content.GetDownloadURL()
	if url == "" {
		return nil, fmt.Errorf("no download URL available for file")
	}

	resp, err := g.client.Client().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// isTextFile checks if a file is likely to be a text file
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	// Common text file extensions
	textExts := map[string]bool{
		".md":              true,
		".txt":             true,
		".json":            true,
		".yaml":            true,
		".yml":             true,
		".go":              true,
		".py":              true,
		".js":              true,
		".ts":              true,
		".java":            true,
		".cpp":             true,
		".c":               true,
		".h":               true,
		".hpp":             true,
		".cs":              true,
		".php":             true,
		".rb":              true,
		".rs":              true,
		".swift":           true,
		".kt":              true,
		".scala":           true,
		".sh":              true,
		".bash":            true,
		".zsh":             true,
		".fish":            true,
		".ps1":             true,
		".sql":             true,
		".xml":             true,
		".html":            true,
		".css":             true,
		".scss":            true,
		".sass":            true,
		".less":            true,
		".dockerfile":      true,
		".gitignore":       true,
		".gitattributes":   true,
		".editorconfig":    true,
		".env":             true,
		".env.example":     true,
		".env.local":       true,
		".env.production":  true,
		".env.development": true,
		".env.test":        true,
	}

	return textExts[ext] || ext == ""
}

// GetLastSync returns the last sync timestamp
func (g *GitHubAdapter) GetLastSync() time.Time {
	return g.lastSync
}

// SetLastSync updates the last sync timestamp
func (g *GitHubAdapter) SetLastSync(t time.Time) {
	g.lastSync = t
}
