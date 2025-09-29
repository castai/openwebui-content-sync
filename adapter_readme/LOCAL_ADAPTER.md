# Local Folder Adapter

The Local Folder adapter allows you to sync content from local directories on your filesystem into OpenWebUI knowledge bases. This is useful for syncing documentation, notes, or other content stored locally.

## Features

- **Multi-directory support**: Sync from multiple local directories
- **Knowledge base mapping**: Map each directory to a specific OpenWebUI knowledge base
- **Recursive scanning**: Automatically scans subdirectories for content
- **File filtering**: Automatically filters out binary files and common non-content files
- **Incremental sync**: Only processes files that have changed since the last sync
- **Path preservation**: Maintains directory structure in the knowledge base

## Configuration

### Configuration File

Add the following section to your `config.yaml`:

```yaml
local_folders:
  enabled: true
  mappings:
    - folder_path: "/path/to/documentation"
      knowledge_id: "docs-knowledge-base"
    - folder_path: "/path/to/notes"
      knowledge_id: "notes-knowledge-base"
    - folder_path: "/home/user/projects/docs"
      knowledge_id: "project-docs"
```

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable/disable the local folder adapter |
| `mappings` | array | Yes | `[]` | List of folder mappings |

### Folder Mapping

Each mapping in the `mappings` array should contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `folder_path` | string | Yes | Absolute path to the local directory |
| `knowledge_id` | string | Yes | Target OpenWebUI knowledge base ID |

## Directory Requirements

### Path Format

- Use **absolute paths** for all directory mappings
- Paths must exist and be readable by the application
- Avoid paths with spaces or special characters (use quotes if necessary)

### Permissions

The application must have:
- **Read access** to all configured directories
- **Execute access** to traverse subdirectories
- **Read access** to all files within the directories

## File Processing

The Local Folder adapter processes files as follows:

### Supported File Types

- **Markdown files** (`.md`, `.markdown`)
- **Text files** (`.txt`, `.text`)
- **Documentation files** (`.rst`, `.adoc`)
- **Code files** (`.py`, `.js`, `.ts`, `.go`, `.java`, `.cpp`, `.c`, `.h`, `.hpp`)
- **Configuration files** (`.yaml`, `.yml`, `.json`, `.toml`, `.ini`, `.cfg`)
- **Shell scripts** (`.sh`, `.bash`, `.zsh`)
- **HTML files** (`.html`, `.htm`)

### Excluded Files

The adapter automatically excludes:
- Binary files (images, videos, executables, etc.)
- Common non-content files (`.gitignore`, `.gitattributes`, etc.)
- Large files (> 1MB)
- Hidden files and directories (starting with `.`)
- Common exclusion directories (`node_modules/`, `vendor/`, `.git/`, etc.)

### File Path Structure

Files are stored with paths that preserve the directory structure:
```
local/folder-name/subdirectory/file.md
```

## Sync Behavior

- **Initial sync**: Scans all configured directories and processes all supported files
- **Incremental sync**: Only processes files modified since the last successful sync
- **Error handling**: If a directory fails to sync, other directories continue processing
- **File monitoring**: Uses file modification timestamps to detect changes

## Use Cases

### Documentation Sync

Sync local documentation directories:

```yaml
local_folders:
  enabled: true
  mappings:
    - folder_path: "/home/user/docs"
      knowledge_id: "user-docs"
    - folder_path: "/opt/company/docs"
      knowledge_id: "company-docs"
```

### Project Documentation

Sync project-specific documentation:

```yaml
local_folders:
  enabled: true
  mappings:
    - folder_path: "/home/user/projects/my-app/docs"
      knowledge_id: "my-app-docs"
    - folder_path: "/home/user/projects/api-docs"
      knowledge_id: "api-docs"
```

### Notes and Knowledge Base

Sync personal or team notes:

```yaml
local_folders:
  enabled: true
  mappings:
    - folder_path: "/home/user/notes"
      knowledge_id: "personal-notes"
    - folder_path: "/shared/team-notes"
      knowledge_id: "team-notes"
```

## Troubleshooting

### Common Issues

1. **Directory not found**
   - Verify the directory path exists and is accessible
   - Check that the path is absolute (starts with `/`)
   - Ensure the application has read permissions

2. **Permission denied**
   - Check file and directory permissions
   - Ensure the application user can read the directories
   - Verify execute permissions on parent directories

3. **Empty knowledge base**
   - Check that directories contain supported file types
   - Verify files are not hidden or in excluded directories
   - Check file size limits (files > 1MB are excluded)

4. **Sync not updating**
   - Verify file modification timestamps are updating
   - Check that files are being modified (not just accessed)
   - Ensure the application has write access to the storage directory

### Debug Logging

Enable debug logging to see detailed sync information:

```yaml
log_level: debug
```

This will show:
- Which directories are being scanned
- File discovery and filtering details
- File processing progress
- Sync timing and statistics

## Security Considerations

- **File access**: Only sync directories that contain appropriate content
- **Path traversal**: The adapter validates paths to prevent directory traversal attacks
- **Content filtering**: Review the content being synced to ensure it's appropriate
- **Permissions**: Run the application with minimal required permissions

## Performance Tips

- **Directory size**: Large directories with many files may take longer to sync
- **File filtering**: The adapter automatically filters out unnecessary files
- **Incremental sync**: Only changed files are processed after the initial sync
- **Storage location**: Use fast storage for the application's data directory

## Example Configuration

```yaml
# Complete example configuration
log_level: info
schedule:
  interval: 30m

storage:
  path: "/data"

openwebui:
  base_url: "http://localhost:8080"
  api_key: "your-openwebui-api-key"

local_folders:
  enabled: true
  mappings:
    - folder_path: "/home/user/docs"
      knowledge_id: "user-docs"
    - folder_path: "/opt/company/knowledge-base"
      knowledge_id: "company-kb"
    - folder_path: "/shared/project-docs"
      knowledge_id: "project-docs"
```

## Docker Considerations

When running in Docker, ensure that:

1. **Volume mounts** are properly configured for local directories
2. **Permissions** are set correctly for the container user
3. **Paths** are accessible from within the container

Example Docker volume mount:

```yaml
volumes:
  - /host/path/to/docs:/container/path/to/docs:ro
```

## File System Monitoring

The adapter uses file modification timestamps to detect changes. For optimal performance:

- Avoid frequently modifying files unnecessarily
- Use proper file locking when editing files
- Consider using a file system that supports efficient timestamp updates
- Monitor disk space to ensure sufficient storage for the application
