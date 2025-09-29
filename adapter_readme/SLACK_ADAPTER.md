# Slack Adapter

The Slack adapter allows you to sync messages from Slack channels into OpenWebUI knowledge bases. This enables you to search and reference Slack conversations, decisions, and discussions within your OpenWebUI interface.

## Features

- **Multi-channel support**: Sync from multiple Slack channels
- **Knowledge base mapping**: Map each channel to a specific OpenWebUI knowledge base
- **Thread support**: Include or exclude thread messages
- **Reaction data**: Optionally include emoji reactions
- **Message filtering**: Control the number of messages and time range
- **History management**: Choose between maintaining indefinite history or aging off old messages
- **Incremental sync**: Only fetches new messages since the last sync

## Configuration

### Environment Variables

Set the following environment variable:

```bash
export SLACK_TOKEN="xoxb-your-slack-bot-token"
```

### Configuration File

Add the following section to your `config.yaml`:

```yaml
slack:
  enabled: true
  token: ""  # Set via SLACK_TOKEN environment variable
  channel_mappings:
    - channel_id: "C1234567890"  # Slack channel ID
      channel_name: "general"     # Channel name for display
      knowledge_id: "general-knowledge-base"
    - channel_id: "C0987654321"
      channel_name: "dev-team"
      knowledge_id: "dev-knowledge-base"
    - channel_id: "C1122334455"
      channel_name: "support"
      knowledge_id: "support-knowledge-base"
  days_to_fetch: 30        # Number of days to fetch messages (default: 30)
  maintain_history: false  # Whether to maintain indefinite history or age off (default: false)
  message_limit: 1000      # Max messages per channel per run (default: 1000)
  include_threads: true    # Whether to include thread messages (default: true)
  include_reactions: false # Whether to include reaction data (default: false)
```

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable/disable the Slack adapter |
| `token` | string | Yes | - | Slack bot token (set via `SLACK_TOKEN` env var) |
| `channel_mappings` | array | Yes | `[]` | List of channel mappings |
| `days_to_fetch` | integer | No | `30` | Number of days to fetch messages |
| `maintain_history` | boolean | No | `false` | Whether to maintain indefinite history or age off |
| `message_limit` | integer | No | `1000` | Max messages per channel per run |
| `include_threads` | boolean | No | `true` | Whether to include thread messages |
| `include_reactions` | boolean | No | `false` | Whether to include reaction data |

### Channel Mapping

Each mapping in the `channel_mappings` array should contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `channel_id` | string | Yes | Slack channel ID (starts with 'C') |
| `channel_name` | string | Yes | Channel name for display purposes |
| `knowledge_id` | string | Yes | Target OpenWebUI knowledge base ID |

## Slack Bot Setup

### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click "Create New App"
3. Choose "From scratch"
4. Enter app name and select your workspace

### 2. Configure Bot Permissions

In your app settings, go to "OAuth & Permissions" and add these scopes:

**Bot Token Scopes:**
- `channels:history` - View messages in public channels
- `channels:read` - View basic information about public channels
- `groups:history` - View messages in private channels
- `groups:read` - View basic information about private channels
- `im:history` - View messages in direct messages
- `im:read` - View basic information about direct messages
- `mpim:history` - View messages in group direct messages
- `mpim:read` - View basic information about group direct messages
- `reactions:read` - View emoji reactions (if including reactions)

### 3. Install the App

1. Go to "Install App" in your app settings
2. Click "Install to Workspace"
3. Review permissions and click "Allow"
4. Copy the "Bot User OAuth Token" (starts with `xoxb-`)

### 4. Get Channel IDs

To find channel IDs:

1. Open Slack in your browser
2. Navigate to the channel
3. Look at the URL: `https://yourworkspace.slack.com/messages/C1234567890`
4. The channel ID is the part after `/messages/`

Or use the Slack API:
```bash
curl -H "Authorization: Bearer xoxb-your-token" \
  "https://slack.com/api/conversations.list"
```

## Message Processing

### Message Format

Messages are processed and stored in markdown format:

```markdown
# Channel: #general
**User:** @john.doe
**Timestamp:** 2024-01-15 10:30:00
**Message:**
This is the message content.

**Thread Reply:**
- @jane.smith: This is a thread reply
- @bob.wilson: Another thread reply

**Reactions:** :thumbsup: :heart: :laughing:
```

### Message Types

The adapter processes:
- **Regular messages**: Text messages from users
- **Thread messages**: Replies to messages (if enabled)
- **File attachments**: File names and descriptions
- **Reactions**: Emoji reactions (if enabled)
- **System messages**: Channel join/leave notifications

### Excluded Content

The adapter automatically excludes:
- Messages from bots (unless specifically configured)
- Deleted messages
- Messages older than the configured `days_to_fetch`
- Messages exceeding the `message_limit`

## Sync Behavior

### Initial Sync

- Fetches messages from the last `days_to_fetch` days
- Respects the `message_limit` per channel
- Processes all configured channels

### Incremental Sync

- Only fetches new messages since the last sync
- Maintains sync state per channel
- Handles rate limiting automatically

### History Management

Two modes are available:

1. **Age-off mode** (`maintain_history: false`):
   - Only keeps messages from the last `days_to_fetch` days
   - Older messages are automatically removed
   - Reduces storage usage

2. **Indefinite history** (`maintain_history: true`):
   - Keeps all messages indefinitely
   - Only fetches new messages on subsequent syncs
   - Requires more storage but preserves all history

## Use Cases

### Team Knowledge Base

Sync important team discussions:

```yaml
slack:
  enabled: true
  channel_mappings:
    - channel_id: "C1234567890"
      channel_name: "general"
      knowledge_id: "team-general"
    - channel_id: "C0987654321"
      channel_name: "dev-team"
      knowledge_id: "dev-discussions"
  days_to_fetch: 90
  maintain_history: true
  include_threads: true
```

### Support Documentation

Sync support channel discussions:

```yaml
slack:
  enabled: true
  channel_mappings:
    - channel_id: "C1122334455"
      channel_name: "support"
      knowledge_id: "support-knowledge"
  days_to_fetch: 30
  maintain_history: false
  include_threads: true
  include_reactions: true
```

### Project Discussions

Sync project-specific channels:

```yaml
slack:
  enabled: true
  channel_mappings:
    - channel_id: "C5555666677"
      channel_name: "project-alpha"
      knowledge_id: "project-alpha-docs"
    - channel_id: "C8888999900"
      channel_name: "project-beta"
      knowledge_id: "project-beta-docs"
  days_to_fetch: 60
  maintain_history: true
```

## Troubleshooting

### Common Issues

1. **Authentication errors**
   - Verify your Slack token is valid and starts with `xoxb-`
   - Check that the bot has been installed to your workspace
   - Ensure the token hasn't expired

2. **Channel access denied**
   - Verify the bot has been added to the channels you want to sync
   - Check that the bot has the required permissions
   - Ensure channel IDs are correct

3. **No messages synced**
   - Check that channels have messages within the `days_to_fetch` range
   - Verify the `message_limit` isn't too restrictive
   - Ensure channels aren't empty or archived

4. **Rate limit exceeded**
   - The adapter automatically handles rate limits with backoff
   - Consider reducing sync frequency if this occurs frequently
   - Check if you're hitting Slack's API rate limits

### Debug Logging

Enable debug logging to see detailed sync information:

```yaml
log_level: debug
```

This will show:
- Which channels are being processed
- Message fetching progress
- API request/response details
- Sync timing and statistics

## Security Considerations

- **Token security**: Store your Slack token securely and never commit it to version control
- **Channel access**: Only sync channels that contain appropriate content
- **Content filtering**: Review the content being synced to ensure it's appropriate
- **Privacy**: Be mindful of private channels and sensitive information

## Performance Tips

- **Message limits**: Set appropriate `message_limit` values to balance completeness with performance
- **Days to fetch**: Adjust `days_to_fetch` based on your needs
- **Thread inclusion**: Disable `include_threads` if you don't need thread context
- **Reaction inclusion**: Disable `include_reactions` to reduce data volume

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

slack:
  enabled: true
  token: ""  # Set via SLACK_TOKEN environment variable
  channel_mappings:
    - channel_id: "C1234567890"
      channel_name: "general"
      knowledge_id: "general-discussions"
    - channel_id: "C0987654321"
      channel_name: "dev-team"
      knowledge_id: "dev-discussions"
    - channel_id: "C1122334455"
      channel_name: "support"
      knowledge_id: "support-knowledge"
  days_to_fetch: 30
  maintain_history: false
  message_limit: 1000
  include_threads: true
  include_reactions: false
```

## Rate Limits

Slack has the following rate limits for bots:

- **Tier 1**: 1+ per minute
- **Tier 2**: 20+ per minute
- **Tier 3**: 50+ per minute
- **Tier 4**: 100+ per minute

The adapter automatically handles rate limiting with exponential backoff. For optimal performance:

- Avoid syncing too many channels simultaneously
- Use appropriate sync intervals
- Monitor your API usage in the Slack app dashboard
