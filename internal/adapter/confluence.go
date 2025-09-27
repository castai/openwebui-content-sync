package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/openwebui-content-sync/internal/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

// ConfluenceAdapter implements the Adapter interface for Confluence spaces
type ConfluenceAdapter struct {
	client             *http.Client
	config             config.ConfluenceConfig
	lastSync           time.Time
	spaces             []string
	parentPageIDs      []string
	spaceMappings      map[string]string // space_key -> knowledge_id mapping
	parentPageMappings map[string]string // parent_page_id -> knowledge_id mapping
}

// ConfluenceSpace represents a space from Confluence API
type ConfluenceSpace struct {
	ID                 string                 `json:"id"`
	Key                string                 `json:"key"`
	Name               string                 `json:"name"`
	Type               string                 `json:"type"`
	Status             string                 `json:"status"`
	Description        string                 `json:"description"`
	HomepageID         string                 `json:"homepageId"`
	Icon               interface{}            `json:"icon"`
	SpaceOwnerID       string                 `json:"spaceOwnerId"`
	AuthorID           string                 `json:"authorId"`
	CreatedAt          string                 `json:"createdAt"`
	CurrentActiveAlias string                 `json:"currentActiveAlias"`
	Links              map[string]interface{} `json:"_links"`
}

// ConfluenceSpaceList represents the response from listing spaces
type ConfluenceSpaceList struct {
	Results []ConfluenceSpace      `json:"results"`
	Links   map[string]interface{} `json:"_links"`
}

// ConfluencePage represents a page from Confluence API
type ConfluencePage struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"`
	Title       string                 `json:"title"`
	SpaceID     string                 `json:"spaceId"`
	ParentID    string                 `json:"parentId"`
	ParentType  string                 `json:"parentType"`
	Position    int                    `json:"position"`
	AuthorID    string                 `json:"authorId"`
	OwnerID     string                 `json:"ownerId"`
	LastOwnerID string                 `json:"lastOwnerId"`
	CreatedAt   string                 `json:"createdAt"`
	Version     ConfluenceVersion      `json:"version"`
	Body        ConfluenceBody         `json:"body"`
	Links       map[string]interface{} `json:"_links"`
}

// ConfluenceVersion represents version information
type ConfluenceVersion struct {
	CreatedAt string `json:"createdAt"`
	Message   string `json:"message"`
	Number    int    `json:"number"`
	MinorEdit bool   `json:"minorEdit"`
	AuthorID  string `json:"authorId"`
}

// ConfluenceBody represents the body content
type ConfluenceBody struct {
	View ConfluenceBodyView `json:"view"`
}

// ConfluenceBodyView represents the view content
type ConfluenceBodyView struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

// ConfluencePageList represents the response from listing pages
type ConfluencePageList struct {
	Results []ConfluencePage       `json:"results"`
	Links   map[string]interface{} `json:"_links"`
}

// ConfluenceChildPage represents a child page from the children API
type ConfluenceChildPage struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Title         string `json:"title"`
	SpaceID       string `json:"spaceId"`
	ChildPosition int    `json:"childPosition"`
}

// ConfluenceChildPageList represents the response from listing child pages
type ConfluenceChildPageList struct {
	Results []ConfluenceChildPage  `json:"results"`
	Links   map[string]interface{} `json:"_links"`
}

// ConfluenceAttachment represents an attachment
type ConfluenceAttachment struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	MediaType string                 `json:"mediaType"`
	FileSize  int                    `json:"fileSize"`
	Comment   string                 `json:"comment"`
	PageID    string                 `json:"pageId"`
	SpaceID   string                 `json:"spaceId"`
	Version   ConfluenceVersion      `json:"version"`
	CreatedAt string                 `json:"createdAt"`
	AuthorID  string                 `json:"authorId"`
	Links     map[string]interface{} `json:"_links"`
}

// ConfluenceAttachmentList represents the response from listing attachments
type ConfluenceAttachmentList struct {
	Results []ConfluenceAttachment `json:"results"`
	Links   map[string]interface{} `json:"_links"`
}

// NewConfluenceAdapter creates a new Confluence adapter
func NewConfluenceAdapter(cfg config.ConfluenceConfig) (*ConfluenceAdapter, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("confluence base URL is required")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("confluence username is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("confluence API key is required")
	}

	// Build space and parent page mappings
	spaceMappings := make(map[string]string)
	parentPageMappings := make(map[string]string)
	spaces := []string{}
	parentPageIDs := []string{}

	// Process space mappings
	for _, mapping := range cfg.SpaceMappings {
		if mapping.SpaceKey != "" && mapping.KnowledgeID != "" {
			spaceMappings[mapping.SpaceKey] = mapping.KnowledgeID
			spaces = append(spaces, mapping.SpaceKey)
		}
	}

	// Process parent page mappings
	for _, mapping := range cfg.ParentPageMappings {
		if mapping.ParentPageID != "" && mapping.KnowledgeID != "" {
			parentPageMappings[mapping.ParentPageID] = mapping.KnowledgeID
			parentPageIDs = append(parentPageIDs, mapping.ParentPageID)
		}
	}

	// If no mappings are configured, return error
	if len(spaces) == 0 && len(parentPageIDs) == 0 {
		return nil, fmt.Errorf("at least one confluence space or parent page mapping must be configured")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &ConfluenceAdapter{
		client:             client,
		config:             cfg,
		spaces:             spaces,
		parentPageIDs:      parentPageIDs,
		spaceMappings:      spaceMappings,
		parentPageMappings: parentPageMappings,
		lastSync:           time.Now(),
	}, nil
}

// Name returns the adapter name
func (c *ConfluenceAdapter) Name() string {
	return "confluence"
}

// FetchFiles fetches files from all configured Confluence spaces and parent pages
func (c *ConfluenceAdapter) FetchFiles(ctx context.Context) ([]*File, error) {
	var allFiles []*File

	logrus.Debugf("Confluence adapter config - ParentPageIDs: %v, Spaces: %v, BaseURL: %s, Username: %s",
		c.parentPageIDs, c.spaces, c.config.BaseURL, c.config.Username)

	// Process parent pages if configured
	if len(c.parentPageIDs) > 0 {
		logrus.Debugf("Using PARENT PAGE mode - Processing %d parent pages", len(c.parentPageIDs))
		for _, parentPageID := range c.parentPageIDs {
			logrus.Debugf("Fetching files from Confluence parent page: %s", parentPageID)

			// Step 1: Get the parent page details
			parentPage, err := c.fetchPageByID(ctx, parentPageID)
			if err != nil {
				logrus.Errorf("Failed to fetch parent page %s: %v", parentPageID, err)
				continue
			}

			logrus.Debugf("Parent page: %s (Space: %s)", parentPage.Title, parentPage.SpaceID)

			// Step 2: Fetch all sub-pages under this parent
			pages, err := c.fetchSubPages(ctx, parentPageID)
			if err != nil {
				logrus.Errorf("Failed to fetch sub-pages for parent %s: %v", parentPageID, err)
				continue
			}

			// Include the parent page itself in the results
			pages = append([]ConfluencePage{parentPage}, pages...)

			logrus.Debugf("Found %d pages under parent page %s", len(pages), parentPage.Title)

			// Step 3: Process each page
			knowledgeID := c.parentPageMappings[parentPageID]
			for _, page := range pages {
				file, err := c.processPage(ctx, page, knowledgeID)
				if err != nil {
					logrus.Errorf("Failed to process page %s: %v", page.Title, err)
					continue
				}
				allFiles = append(allFiles, file)
			}
		}
	}

	// Process spaces if configured
	if len(c.spaces) > 0 {
		logrus.Debugf("Using SPACE mode - Processing %d spaces", len(c.spaces))
		for _, spaceKey := range c.spaces {
			logrus.Debugf("Fetching files from Confluence space: %s", spaceKey)

			// Step 1: Get space ID from space key
			spaceID, err := c.getSpaceID(ctx, spaceKey)
			if err != nil {
				logrus.Errorf("Failed to get space ID for %s: %v", spaceKey, err)
				continue
			}

			logrus.Debugf("Space %s has ID: %s", spaceKey, spaceID)

			// Step 2: Fetch pages from the space
			pages, err := c.fetchSpacePages(ctx, spaceID)
			if err != nil {
				logrus.Errorf("Failed to fetch pages from space %s: %v", spaceKey, err)
				continue
			}

			logrus.Debugf("Found %d pages in space %s", len(pages), spaceKey)

			// Step 3: Process each page
			knowledgeID := c.spaceMappings[spaceKey]
			for _, page := range pages {
				file, err := c.processPage(ctx, page, knowledgeID)
				if err != nil {
					logrus.Errorf("Failed to process page %s: %v", page.Title, err)
					continue
				}
				allFiles = append(allFiles, file)
			}
		}
	}

	c.lastSync = time.Now()
	return allFiles, nil
}

// getSpaceID retrieves the space ID from the space key
func (c *ConfluenceAdapter) getSpaceID(ctx context.Context, spaceKey string) (string, error) {
	// URL encode the space key
	encodedSpaceKey := url.QueryEscape(spaceKey)
	url := fmt.Sprintf("%s/wiki/api/v2/spaces?keys=%s", c.config.BaseURL, encodedSpaceKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.config.Username, c.config.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OpenWebUI-Content-Sync/1.0")

	logrus.Debugf("Confluence space API URL: %s", url)
	logrus.Debugf("Confluence space key - Original: %s, Encoded: %s", spaceKey, encodedSpaceKey)
	logrus.Debugf("Confluence auth - Username: %s, APIKey length: %d", c.config.Username, len(c.config.APIKey))
	logrus.Debugf("Request headers: %+v", req.Header)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Consume body for proper connection reuse
		logrus.Errorf("Confluence space API failed - Status: %d, URL: %s, Response: %s", resp.StatusCode, url, string(body))
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var spaceList ConfluenceSpaceList
	if err := json.NewDecoder(resp.Body).Decode(&spaceList); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(spaceList.Results) == 0 {
		return "", fmt.Errorf("space %s not found", spaceKey)
	}

	return spaceList.Results[0].ID, nil
}

// fetchSpacePages fetches all pages from a space using space ID
func (c *ConfluenceAdapter) fetchSpacePages(ctx context.Context, spaceID string) ([]ConfluencePage, error) {
	var allPages []ConfluencePage
	limit := c.config.PageLimit
	if limit <= 0 {
		limit = 100 // Default limit
	}

	url := fmt.Sprintf("%s/wiki/api/v2/spaces/%s/pages?limit=%d", c.config.BaseURL, spaceID, limit)

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set authentication
		req.SetBasicAuth(c.config.Username, c.config.APIKey)
		req.Header.Set("Accept", "application/json")

		logrus.Debugf("Confluence pages API URL: %s", url)

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d: response body omitted", resp.StatusCode)
		}

		var pageList ConfluencePageList
		if err := json.NewDecoder(resp.Body).Decode(&pageList); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		allPages = append(allPages, pageList.Results...)

		// Check for next page
		nextLink, hasNext := pageList.Links["next"]
		if !hasNext {
			break
		}

		nextURL, ok := nextLink.(string)
		if !ok {
			break
		}
		// Check if nextURL doesn't start with https
		if nextURL != "" && !strings.HasPrefix(nextURL, "https") {
			// Prepend the base URL
			nextURL = c.config.BaseURL + nextURL
		}

		url = nextURL
		url = nextURL
	}

	return allPages, nil
}

// fetchPageByID fetches a specific page by its ID
func (c *ConfluenceAdapter) fetchPageByID(ctx context.Context, pageID string) (ConfluencePage, error) {
	url := fmt.Sprintf("%s/wiki/api/v2/pages/%s", c.config.BaseURL, pageID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ConfluencePage{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.config.Username, c.config.APIKey)
	req.Header.Set("Accept", "application/json")

	logrus.Debugf("Confluence page API URL: %s", url)
	resp, err := c.client.Do(req)
	if err != nil {
		return ConfluencePage{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Consume body for proper connection reuse
		logrus.Errorf("Confluence page API failed - Status: %d, URL: %s, Response: %s", resp.StatusCode, url, string(body))
		return ConfluencePage{}, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var page ConfluencePage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return ConfluencePage{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return page, nil
}

// fetchSubPages fetches all sub-pages under a specific parent page
func (c *ConfluenceAdapter) fetchSubPages(ctx context.Context, parentPageID string) ([]ConfluencePage, error) {
	var allPages []ConfluencePage
	limit := c.config.PageLimit
	if limit <= 0 {
		limit = 100 // Default limit
	}

	url := fmt.Sprintf("%s/wiki/api/v2/pages/%s/children?limit=%d", c.config.BaseURL, parentPageID, limit)

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set authentication
		req.SetBasicAuth(c.config.Username, c.config.APIKey)
		req.Header.Set("Accept", "application/json")

		logrus.Debugf("Confluence sub-pages API URL: %s", url)

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d: response body omitted", resp.StatusCode)
		}

		var childPageList ConfluenceChildPageList
		if err := json.NewDecoder(resp.Body).Decode(&childPageList); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		// Convert child pages to full pages by fetching each one
		for _, childPage := range childPageList.Results {
			fullPage, err := c.fetchPageByID(ctx, childPage.ID)
			if err != nil {
				logrus.Errorf("Failed to fetch full page details for %s: %v", childPage.ID, err)
				continue
			}
			allPages = append(allPages, fullPage)
		}

		// Check for next page
		nextLink, hasNext := childPageList.Links["next"]
		if !hasNext {
			break
		}

		nextURL, ok := nextLink.(string)
		if !ok {
			break
		}

		url = nextURL
	}

	return allPages, nil
}

// processPage processes a single page and returns a File
func (c *ConfluenceAdapter) processPage(ctx context.Context, page ConfluencePage, knowledgeID string) (*File, error) {
	// Get the page body with content
	pageBody, err := c.fetchPageBody(ctx, page.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page body: %w", err)
	}

	// Create filename from title
	filename := c.SanitizeFilename(page.Title) + ".txt"

	// Format content as webui link + body content
	webuiLink := ""
	if webui, exists := page.Links["webui"]; exists {
		if webuiStr, ok := webui.(string); ok {
			webuiLink = webuiStr
		}
	}

	content := fmt.Sprintf("%s\n\n%s", webuiLink, pageBody)

	// Create file content
	fileContent := []byte(content)

	// Generate content hash for change detection
	hash := sha256.Sum256(fileContent)
	contentHash := base64.StdEncoding.EncodeToString(hash[:])

	return &File{
		Path:        filename,
		Content:     fileContent,
		Hash:        contentHash,
		Modified:    c.lastSync,
		Size:        int64(len(fileContent)),
		Source:      "confluence",
		KnowledgeID: knowledgeID,
	}, nil
}

// fetchPageBody fetches the body content of a specific page
func (c *ConfluenceAdapter) fetchPageBody(ctx context.Context, pageID string) (string, error) {
	url := fmt.Sprintf("%s/wiki/api/v2/pages/%s?body-format=view", c.config.BaseURL, pageID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.config.Username, c.config.APIKey)
	req.Header.Set("Accept", "application/json")

	logrus.Debugf("Confluence page body API URL: %s", url)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: response body omitted", resp.StatusCode)
	}

	var page ConfluencePage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract content from body.view.value
	if page.Body.View.Value != "" {
		// Convert HTML to plain text
		return c.HtmlToText(page.Body.View.Value), nil
	}

	return "", fmt.Errorf("no content found in page body")
}

// htmlToText converts HTML content to plain text
func (c *ConfluenceAdapter) HtmlToText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		logrus.Warnf("Failed to parse HTML: %v", err)
		return htmlContent
	}

	var text strings.Builder
	c.extractText(doc, &text)
	return strings.TrimSpace(text.String())
}

// extractText recursively extracts text from HTML nodes
func (c *ConfluenceAdapter) extractText(n *html.Node, text *strings.Builder) {
	if n.Type == html.TextNode {
		text.WriteString(n.Data)
		return
	}

	// Handle special elements
	if n.Type == html.ElementNode {
		switch n.Data {
		case "br":
			text.WriteString("\n")
			return
		case "p":
			// Add line break before paragraph (except if it's the first element)
			if text.Len() > 0 && !strings.HasSuffix(text.String(), "\n") {
				text.WriteString("\n")
			}
			// Process children
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				c.extractText(child, text)
			}
			// Add double line break after paragraph
			text.WriteString("\n\n")
			return
		case "div", "h1", "h2", "h3", "h4", "h5", "h6":
			// Add line break before other block elements (except if it's the first element)
			if text.Len() > 0 && !strings.HasSuffix(text.String(), "\n") {
				text.WriteString("\n")
			}
			// Process children
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				c.extractText(child, text)
			}
			// Add single line break after other block elements
			text.WriteString("\n")
			return
		}
	}

	// Process children for other elements
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.extractText(child, text)
	}
}

// sanitizeFilename converts a title to a safe filename
func (c *ConfluenceAdapter) SanitizeFilename(title string) string {
	// Convert to lowercase and replace spaces with underscores
	filename := strings.ToLower(title)

	// Replace special characters with underscores (but preserve dots for extensions)
	reg := regexp.MustCompile(`[^a-z0-9\s_.-]`)
	filename = reg.ReplaceAllString(filename, "_")

	// Replace spaces and multiple underscores with single underscore
	reg = regexp.MustCompile(`[\s_]+`)
	filename = reg.ReplaceAllString(filename, "_")

	// Remove leading/trailing underscores
	filename = strings.Trim(filename, "_")

	// Limit length to 100 characters
	if len(filename) > 100 {
		filename = filename[:100]
	}

	// Ensure it's not empty
	if filename == "" {
		filename = "untitled"
	}

	return filename
}

// GetLastSync returns the last sync time
func (c *ConfluenceAdapter) GetLastSync() time.Time {
	return c.lastSync
}

// SetLastSync sets the last sync time
func (c *ConfluenceAdapter) SetLastSync(t time.Time) {
	c.lastSync = t
}
