# OpenWebUI Content Sync

A Kubernetes-native application that synchronizes content from various sources (GitHub repositories, Confluence spaces) to OpenWebUI knowledge bases using an adapter architecture.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Features

- **Multi-Source Support**: GitHub repositories, Confluence spaces, and local folders
- **Adapter Architecture**: Pluggable adapters for different data sources
- **File Diffing**: Only syncs changed files based on content hashing
- **Persistent Storage**: Uses Kubernetes persistent volumes for local file storage
- **Scheduled Sync**: Configurable sync intervals using cron-like scheduling
- **OpenWebUI Integration**: Full integration with OpenWebUI file and knowledge APIs
- **Confluence Support**: Sync entire spaces or specific parent pages with sub-pages
- **Local Folder Support**: Sync local directories with intelligent file filtering

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Data Sources  │    │   Content Sync   │    │   OpenWebUI     │
│   • GitHub      │───▶│   Application    │───▶│   Knowledge     │
│   • Confluence  │    │   (Adapters)     │    │   Base          │
│   • Local Folders│   │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │   Local Storage  │
                       │   (PVC)          │
                       └──────────────────┘
```

## API Integration

The connector integrates with OpenWebUI using the following APIs:

- `POST /api/v1/files/` - Upload files to OpenWebUI
- `GET /api/v1/knowledge/` - List knowledge sources
- `POST /api/v1/knowledge/{id}/file/add` - Add file to knowledge
- `POST /api/v1/knowledge/{id}/file/remove` - Remove file from knowledge

## Quick Start

### Prerequisites

- Kubernetes cluster
- OpenWebUI instance running
- GitHub repository access

### 1. Configure Secrets

Update the secrets in `k8s/secrets.yaml`:

```bash
# Encode your OpenWebUI API key
echo -n "your-openwebui-api-key" | base64

# Encode your GitHub token
echo -n "your-github-token" | base64

# Encode your Confluence API key
echo -n "your-confluence-api-key" | base64
```

### 2. Update Configuration

Edit `k8s/configmap.yaml` to set:
- GitHub repositories to sync
- Confluence spaces or parent page IDs
- Knowledge IDs for file association
- Sync interval

### 3. Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -f k8s/

# Check deployment status
kubectl get pods -l app=openwebui-content-sync
```

### 4. Local Development

```bash
# Build the application
go build -o connector .

# Run with configuration
./connector -config config.yaml
```

## Usage Examples

### GitHub Adapter

The GitHub adapter syncs files from GitHub repositories to OpenWebUI knowledge bases.

#### GitHub Configuration

Map different repositories to different knowledge bases:

```yaml
github:
  enabled: true
  token: "ghp_your_github_token_here"
  mappings:
    - repository: "microsoft/vscode"
      knowledge_id: "vscode-knowledge-base"
    - repository: "facebook/react"
      knowledge_id: "react-knowledge-base"
    - repository: "your-org/your-repo"
      knowledge_id: "your-custom-knowledge-base"
```

#### GitHub Features

- **Repository Sync**: Syncs all files from specified repositories
- **Multiple Knowledge Bases**: Map different repositories to different knowledge bases
- **File Filtering**: Automatically filters out binary files and common ignore patterns
- **Content Hashing**: Only syncs changed files based on SHA256 hashes
- **Branch Support**: Syncs from the default branch (usually `main` or `master`)

#### GitHub Example Output

```
INFO[0000] Syncing files from adapter: github
DEBU[0000] Fetching files from repository: microsoft/vscode
DEBU[0000] Found 1,234 files in repository microsoft/vscode
INFO[0001] Successfully synced file: README.md
INFO[0001] Successfully synced file: package.json
INFO[0001] Successfully synced file: src/index.js
```

### Confluence Adapter

The Confluence adapter syncs pages from Confluence spaces to OpenWebUI knowledge bases.

#### Confluence Configuration

Map different spaces and parent pages to different knowledge bases:

```yaml
confluence:
  enabled: true
  base_url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_key: "your-confluence-api-key"
  
  # Space mappings (per-space knowledge IDs)
  space_mappings:
    - space_key: "DOCS"
      knowledge_id: "docs-knowledge-base"
    - space_key: "PRODUCT"
      knowledge_id: "product-knowledge-base"
  
  # Parent page mappings (per-parent-page knowledge IDs)
  parent_page_mappings:
    - parent_page_id: "1234567890"
      knowledge_id: "parent-page-knowledge-base"
    - parent_page_id: "0987654321"
      knowledge_id: "another-parent-page-knowledge-base"
  
  page_limit: 100  # Maximum pages to fetch per space (0 = no limit)
  include_attachments: true  # Whether to download and sync page attachments
```

#### Confluence Features

- **Space Sync**: Sync all pages from specified Confluence spaces
- **Parent Page Sync**: Sync specific parent pages and all their sub-pages
- **Multiple Knowledge Bases**: Map different spaces and parent pages to different knowledge bases
- **Multiple Parent Pages**: Support for multiple parent page IDs in a single configuration
- **Mixed Configuration**: Can sync both entire spaces and specific parent pages simultaneously
- **HTML to Text**: Converts Confluence HTML content to plain text
- **Filename Sanitization**: Converts page titles to safe filenames (e.g., "Call Summary Best Practices" → `call_summary_best_practices.txt`)
- **Content Formatting**: Includes webui links and page content in uploaded files

#### Confluence Example Output

```
INFO[0000] Syncing files from adapter: confluence
DEBU[0000] Using PARENT PAGE mode - Processing 2 parent pages
DEBU[0000] Fetching files from Confluence parent page: 1234567890
DEBU[0000] Parent page: PoV Guide (Space: 2088140816)
DEBU[0000] Found 4 pages under parent page PoV Guide
DEBU[0000] Fetching files from Confluence parent page: 0987654321
DEBU[0000] Parent page: API Documentation (Space: 2088140816)
DEBU[0000] Found 3 pages under parent page API Documentation
INFO[0001] Successfully synced file: call_summary_best_practices.txt
INFO[0001] Successfully synced file: enabling_features_using_admin_apiconsole.txt
INFO[0001] Successfully synced file: api_endpoints_reference.txt
INFO[0001] Successfully synced file: authentication_guide.txt
```

#### Benefits of Multiple Knowledge Mappings

- **Organized Content**: Keep different types of content in separate knowledge bases
- **Targeted Search**: Users can search within specific knowledge bases for more relevant results
- **Access Control**: Different knowledge bases can have different access permissions
- **Content Management**: Easier to manage and update specific types of content
- **Performance**: Smaller knowledge bases can provide faster search results

**Example Use Cases:**
- Map different GitHub repositories to different knowledge bases (e.g., frontend docs, backend docs, API docs)
- Map different Confluence spaces to different knowledge bases (e.g., product docs, engineering docs, marketing docs)
- Map specific parent pages to specialized knowledge bases (e.g., troubleshooting guides, user manuals, API references)

#### Finding Confluence Page IDs

To find a Confluence page ID:

1. Open the page in your browser
2. Look at the URL: `https://your-domain.atlassian.net/wiki/spaces/SPACEKEY/pages/1234567890/Page+Title`
3. The page ID is `1234567890`

#### Confluence Content Format

Each uploaded file contains:
```
/spaces/SPACEKEY/pages/1234567890/Page+Title

[Page content converted from HTML to plain text]
```

## Local Folders Adapter

The Local Folders adapter allows you to sync files from local directories to OpenWebUI knowledge bases. This is useful for syncing documentation, notes, or other local content.

### Local Folders Configuration

Map different local folders to different knowledge bases:

```yaml
local_folders:
  enabled: true
  mappings:
    - folder_path: "/path/to/docs"
      knowledge_id: "docs-knowledge-base"
    - folder_path: "/path/to/guides"
      knowledge_id: "guides-knowledge-base"
    - folder_path: "/path/to/notes"
      knowledge_id: "notes-knowledge-base"
```

### Local Folders Features

- **Recursive Sync**: Syncs all files from specified directories recursively
- **Multiple Knowledge Bases**: Map different folders to different knowledge bases
- **File Filtering**: Automatically filters out binary files and common ignore patterns
- **Content Hashing**: Only syncs changed files based on SHA256 hashes
- **Hidden File Filtering**: Ignores hidden files (starting with `.`)
- **Binary File Detection**: Automatically skips binary files

### Local Folders Example Output

```
INFO[0000] Syncing files from adapter: local
DEBU[0000] Fetching files from local folder: /path/to/docs
DEBU[0000] Found 15 files in folder /path/to/docs (knowledge_id: docs-knowledge-base)
INFO[0001] Successfully synced file: README.md
INFO[0001] Successfully synced file: installation.md
INFO[0001] Successfully synced file: api-reference.md
INFO[0001] Successfully synced file: subfolder/advanced-usage.md
```

### Ignored Files

The local folders adapter automatically ignores:
- Hidden files (starting with `.`)
- Binary files (detected by content analysis)
- Common system files: `Thumbs.db`, `.DS_Store`, `desktop.ini`
- Common development files: `node_modules`, `__pycache__`, `.git`, etc.
- Temporary files: `*.log`, `*.tmp`, `*.temp`, `*.swp`, `*.swo`

### Multi-Adapter Configuration

You can run GitHub, Confluence, and Local Folders adapters simultaneously:

```yaml
github:
  enabled: true
  token: "your-github-token"
  repositories:
    - "your-org/docs"
  knowledge_id: "docs-knowledge-base"

confluence:
  enabled: true
  base_url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_key: "your-confluence-api-key"
  parent_page_ids:
    - "1234567890"
    - "0987654321"
  knowledge_id: "confluence-knowledge-base"

local_folders:
  enabled: true
  folders:
    - "/path/to/local/docs"
    - "/path/to/notes"
  knowledge_id: "local-knowledge-base"
```

## Configuration

### Environment Variables

- `OPENWEBUI_BASE_URL`: OpenWebUI instance URL
- `OPENWEBUI_API_KEY`: OpenWebUI API key
- `GITHUB_TOKEN`: GitHub personal access token
- `GITHUB_KNOWLEDGE_ID`: OpenWebUI knowledge ID for GitHub files
- `CONFLUENCE_API_KEY`: Confluence API key
- `CONFLUENCE_BASE_URL`: Confluence instance URL (optional, can be set in config)
- `CONFLUENCE_USERNAME`: Confluence username (optional, can be set in config)
- `CONFLUENCE_KNOWLEDGE_ID`: OpenWebUI knowledge ID for Confluence files
- `STORAGE_PATH`: Local storage path (default: /data)
- `LOG_LEVEL`: Log level (debug, info, warn, error)

### Configuration File

```yaml
log_level: info

schedule:
  interval: 1h  # Sync interval

storage:
  path: /data

openwebui:
  base_url: "http://localhost:8080"
  api_key: ""

# GitHub adapter configuration
github:
  enabled: true
  token: ""  # Set via GITHUB_TOKEN environment variable
  repositories:
    - "owner/repo1"
    - "owner/repo2"
  knowledge_id: ""  # Set via GITHUB_KNOWLEDGE_ID environment variable

# Confluence adapter configuration
confluence:
  enabled: false
  base_url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_key: ""  # Set via CONFLUENCE_API_KEY environment variable
  spaces:
    - "SPACEKEY1"
    - "SPACEKEY2"
  parent_page_ids: []  # Optional: specific parent page IDs to process sub-pages only
  knowledge_id: ""  # Set via CONFLUENCE_KNOWLEDGE_ID environment variable
  page_limit: 100  # Maximum pages to fetch per space (0 = no limit)
  include_attachments: true  # Whether to download and sync page attachments
```

## Adapter Architecture

The application uses an adapter pattern to support multiple data sources:

```go
type Adapter interface {
    Name() string
    FetchFiles(ctx context.Context) ([]*File, error)
    GetLastSync() time.Time
    SetLastSync(t time.Time)
}
```

### Current Adapters

- **GitHub Adapter**: Syncs files from GitHub repositories
  - Supports multiple repositories
  - File filtering and content hashing
  - Branch-based syncing
- **Confluence Adapter**: Syncs pages from Confluence spaces
  - Space-based syncing (all pages in space)
  - Parent page syncing (specific page and sub-pages)
  - HTML to text conversion
  - Filename sanitization
- **Extensible**: Easy to add new adapters (GitLab, Bitbucket, Notion, etc.)

## File Sync Process

1. **Fetch**: Adapters fetch files from data sources
2. **Hash**: Calculate SHA256 hash of file content
3. **Compare**: Compare with previously synced files
4. **Upload**: Upload new/changed files to OpenWebUI
5. **Associate**: Add files to knowledge base
6. **Index**: Update local file index

## Monitoring

The application provides structured logging and health checks:

```bash
# View logs
kubectl logs -l app=openwebui-content-sync

# Check health
kubectl exec -it <pod-name> -- ps aux | grep connector
```

## Troubleshooting

### Common Issues

1. **Authentication Errors**: Verify API keys and tokens
   - GitHub: Check `GITHUB_TOKEN` environment variable
   - Confluence: Check `CONFLUENCE_API_KEY` and credentials
2. **Network Issues**: Check OpenWebUI connectivity
3. **Storage Issues**: Verify PVC is mounted correctly
4. **Sync Failures**: Check adapter configuration
   - GitHub: Verify repository names and access permissions
   - Confluence: Verify space keys or parent page IDs
5. **Confluence-Specific Issues**:
   - **Empty Results**: Check if parent page ID exists and has sub-pages
   - **Permission Errors**: Verify Confluence API key has read access to spaces
   - **Page Not Found**: Ensure page IDs are correct (check URLs)

### Debug Mode

Enable debug logging:

```yaml
log_level: debug
```

## Development

### Adding New Adapters

1. Implement the `Adapter` interface
2. Add configuration options
3. Register in main application
4. Add tests

### Building

```bash
# Build for local development
go build -o connector .

# Build Docker image
docker build -t openwebui-content-sync .

# Build multi-architecture image
make docker-build-multi

# Build for specific architecture
make docker-build-amd64
make docker-build-arm64
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
