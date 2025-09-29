# GitHub Adapter

The GitHub adapter allows you to sync content from GitHub repositories into OpenWebUI knowledge bases. It supports multiple repositories and can map each repository to a different knowledge base.

## Features

- **Multi-repository support**: Sync from multiple GitHub repositories
- **Knowledge base mapping**: Map each repository to a specific OpenWebUI knowledge base
- **Incremental sync**: Only fetches files that have changed since the last sync
- **File filtering**: Automatically filters out binary files and common non-content files
- **Authentication**: Uses GitHub personal access tokens for secure API access

## Configuration

### Environment Variables

Set the following environment variable:

```bash
export GITHUB_TOKEN="your-github-personal-access-token"
```

### Configuration File

Add the following section to your `config.yaml`:

```yaml
github:
  enabled: true
  token: ""  # Set via GITHUB_TOKEN environment variable
  mappings:
    - repository: "owner/repo-name"  # GitHub repository in format "owner/repo"
      knowledge_id: "repo-knowledge-base"
    - repository: "another-owner/another-repo"
      knowledge_id: "another-knowledge-base"
```

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable/disable the GitHub adapter |
| `token` | string | Yes | - | GitHub personal access token (set via `GITHUB_TOKEN` env var) |
| `mappings` | array | Yes | `[]` | List of repository mappings |

### Repository Mapping

Each mapping in the `mappings` array should contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repository` | string | Yes | GitHub repository in format "owner/repo" |
| `knowledge_id` | string | Yes | Target OpenWebUI knowledge base ID |

## GitHub Token Setup

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give it a descriptive name (e.g., "OpenWebUI Content Sync")
4. Select the following scopes:
   - `repo` (Full control of private repositories)
   - `public_repo` (Access public repositories)
5. Click "Generate token"
6. Copy the token and set it as the `GITHUB_TOKEN` environment variable

## File Processing

The GitHub adapter processes files as follows:

### Supported File Types

- **Markdown files** (`.md`, `.markdown`)
- **Text files** (`.txt`, `.text`)
- **Documentation files** (`.rst`, `.adoc`)
- **Code files** (`.py`, `.js`, `.ts`, `.go`, `.java`, `.cpp`, `.c`, `.h`, `.hpp`)
- **Configuration files** (`.yaml`, `.yml`, `.json`, `.toml`, `.ini`, `.cfg`)
- **Shell scripts** (`.sh`, `.bash`, `.zsh`)

### Excluded Files

The adapter automatically excludes:
- Binary files (images, videos, executables, etc.)
- Common non-content files (`.gitignore`, `.gitattributes`, etc.)
- Large files (> 1MB)
- Files in common exclusion directories (`.git/`, `node_modules/`, `vendor/`, etc.)

### File Path Structure

Files are stored with paths that include the repository name:
```
github/owner-repo-name/path/to/file.md
```

## Sync Behavior

- **Initial sync**: Fetches all files from configured repositories
- **Incremental sync**: Only fetches files modified since the last successful sync
- **Error handling**: If a repository fails to sync, other repositories continue processing
- **Rate limiting**: Respects GitHub API rate limits with automatic backoff

## Troubleshooting

### Common Issues

1. **Authentication errors**
   - Verify your GitHub token is valid and has the correct permissions
   - Check that the token hasn't expired

2. **Repository not found**
   - Verify the repository name format is correct: "owner/repo"
   - Ensure the token has access to the repository (for private repos)

3. **Rate limit exceeded**
   - The adapter automatically handles rate limits with exponential backoff
   - Consider reducing the sync frequency if this occurs frequently

4. **Empty knowledge base**
   - Check that the repository contains supported file types
   - Verify the repository has content in the root directory or subdirectories

### Debug Logging

Enable debug logging to see detailed sync information:

```yaml
log_level: debug
```

This will show:
- Which repositories are being processed
- File discovery and filtering details
- API request/response information
- Sync progress and timing

## Example Configuration

```yaml
# Complete example configuration
log_level: info
schedule:
  interval: 1h

storage:
  path: "/data"

openwebui:
  base_url: "http://localhost:8080"
  api_key: "your-openwebui-api-key"

github:
  enabled: true
  token: ""  # Set via GITHUB_TOKEN environment variable
  mappings:
    - repository: "microsoft/vscode"
      knowledge_id: "vscode-docs"
    - repository: "kubernetes/kubernetes"
      knowledge_id: "k8s-docs"
    - repository: "your-org/private-repo"
      knowledge_id: "private-docs"
```

## Security Considerations

- **Token security**: Store your GitHub token securely and never commit it to version control
- **Repository access**: Only grant access to repositories that contain appropriate content
- **Content filtering**: Review the content being synced to ensure it's appropriate for your knowledge base
- **Rate limits**: Be mindful of GitHub API rate limits, especially with large repositories

## Performance Tips

- **Repository size**: Large repositories with many files may take longer to sync
- **Sync frequency**: Balance sync frequency with API rate limits
- **File filtering**: The adapter automatically filters out unnecessary files to improve performance
- **Incremental sync**: Only changed files are processed after the initial sync
