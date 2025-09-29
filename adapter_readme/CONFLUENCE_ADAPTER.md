# Confluence Adapter

The Confluence adapter allows you to sync content from Atlassian Confluence spaces into OpenWebUI knowledge bases. This adapter uses the Confluence REST API v2 to fetch pages and optionally attachments from specified Confluence spaces and uploads them to OpenWebUI.

## API Compatibility

This adapter uses Confluence REST API v2, which provides:
- Modern cursor-based pagination
- Improved performance and reliability
- Better support for large spaces
- Enhanced metadata and content structure

## Features

- **Page Content Sync**: Fetches all pages from specified Confluence spaces using Confluence API v2
- **Attachment Support**: Optionally downloads and syncs page attachments
- **HTML to Text Conversion**: Converts Confluence's HTML content to plain text
- **Incremental Sync**: Tracks last sync time to avoid re-processing content
- **Multi-Space Support**: Can sync from multiple Confluence spaces
- **Configurable Limits**: Set page limits and control attachment inclusion
- **Cursor-based Pagination**: Uses modern cursor-based pagination for efficient data retrieval

## Configuration

### YAML Configuration

Add the following to your `config.yaml`:

```yaml
confluence:
  enabled: true
  base_url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_key: "your-confluence-api-key"
  spaces:
    - "SPACEKEY1"
    - "SPACEKEY2"
  knowledge_id: "your-knowledge-base-id"
  page_limit: 100
  include_attachments: true
  include_blog_posts: false
```

### Environment Variables

Only the API key can be configured via environment variable (for security):

```bash
CONFLUENCE_API_KEY="your-confluence-api-key"
```

All other configuration should be done in the `config.yaml` file.

### Kubernetes Configuration

#### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: connector-config
data:
  config.yaml: |
    confluence:
      enabled: true
      base_url: "https://your-domain.atlassian.net"
      username: "your-email@example.com"
      spaces:
        - "SPACEKEY1"
        - "SPACEKEY2"
      knowledge_id: "your-knowledge-base-id"
      page_limit: 100
      include_attachments: true
      use_markdown_parser: true
```

#### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: confluence-secrets
type: Opaque
data:
  api-key: <base64-encoded-api-key>
```

## Authentication

The Confluence adapter uses Basic Authentication with your Confluence username and API key. To get an API key:

1. Go to [Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click "Create API token"
3. Give it a label and copy the generated token
4. Use your email address as the username and the token as the API key

## Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `enabled` | boolean | No | `false` | Enable the Confluence adapter |
| `base_url` | string | Yes | - | Your Confluence instance URL (e.g., `https://your-domain.atlassian.net`) |
| `username` | string | Yes | - | Your Confluence username (usually your email) |
| `api_key` | string | Yes | - | Your Confluence API key |
| `spaces` | array | Yes | - | List of Confluence space keys to sync |
| `knowledge_id` | string | No | - | OpenWebUI knowledge base ID to sync content to |
| `page_limit` | integer | No | `100` | Maximum number of pages to fetch per space |
| `include_attachments` | boolean | No | `true` | Whether to download and sync page attachments |
| `include_blog_posts` | boolean | No | `false` | Whether to download and sync blog posts |
| `use_markdown_parser` | boolean | No | `false` | Whether to use markdown parser for HTML content conversion (true = markdown, false = plain text) |

## File Processing

### Page Content

- Confluence pages are converted from HTML to plain text
- Pages are saved as `.md` files with sanitized filenames
- File paths follow the pattern: `{space}/{page-title}.md`

### Attachments

- Only text-based attachments are processed (based on file extension)
- Binary files are skipped
- Attachments are saved in: `{space}/attachments/{filename}`

### Supported File Types

The adapter processes the following file types:
- Markdown (`.md`)
- Text (`.txt`)
- JSON (`.json`)
- YAML (`.yaml`, `.yml`)
- Code files (`.go`, `.py`, `.js`, `.ts`, `.java`, etc.)
- Configuration files (`.env`, `.gitignore`, etc.)
- And many more text-based formats

## Error Handling

- **Authentication Errors**: Invalid credentials will cause the adapter to fail initialization
- **API Errors**: HTTP errors from Confluence API are logged and may cause individual page/attachment processing to fail
- **File Processing Errors**: Individual file processing errors are logged but don't stop the overall sync
- **Network Errors**: Connection timeouts and network issues are handled gracefully

## Logging

The adapter provides detailed logging at the debug level:

```
DEBUG: Fetching files from Confluence space: SPACEKEY1
DEBUG: Found 25 files in space SPACEKEY1
DEBUG: Processing page: Page Title
DEBUG: Downloading attachment: document.pdf
```

## Limitations

1. **API Rate Limits**: Confluence has API rate limits that may affect sync performance
2. **Large Spaces**: Very large spaces with many pages may take significant time to sync
3. **HTML Conversion**: The HTML to text conversion is basic and may not preserve all formatting
4. **Attachment Size**: Large attachments may cause memory issues or timeouts

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   - Verify your username and API key are correct
   - Ensure your API key has the necessary permissions

2. **Space Not Found**
   - Check that the space key is correct
   - Verify you have access to the space

3. **No Content Synced**
   - Check that the space contains pages
   - Verify the `page_limit` setting is appropriate
   - Check logs for API errors

4. **Attachments Not Synced**
   - Ensure `include_attachments` is set to `true`
   - Check that attachments are text-based files
   - Verify you have download permissions for attachments

### Debug Mode

Enable debug logging to see detailed information about the sync process:

```yaml
log_level: debug
```

## Example Usage

### Basic Configuration

```yaml
confluence:
  enabled: true
  base_url: "https://mycompany.atlassian.net"
  username: "john.doe@mycompany.com"
  api_key: "ATATT3xFfGF0..."
  spaces:
    - "DOCS"
    - "WIKI"
  knowledge_id: "fbc18bc4-72c1-40f0-84b1-52055368c583"
```

### Advanced Configuration

```yaml
confluence:
  enabled: true
  base_url: "https://mycompany.atlassian.net"
  username: "john.doe@mycompany.com"
  api_key: "ATATT3xFfGF0..."
  spaces:
    - "DOCS"
    - "WIKI"
    - "PROJECTS"
  knowledge_id: "fbc18bc4-72c1-40f0-84b1-52055368c583"
  page_limit: 500
  include_attachments: true
```

This configuration will sync up to 500 pages from each of the three specified spaces, including all text-based attachments.
