# go-slack-mcp

A Slack MCP (Model Context Protocol) server written in Go. Drop-in replacement for [slack-mcp](https://github.com/redhat-community-ai-tools/slack-mcp) (Python) with zero runtime dependencies.

## Why

- Single static binary (12 MB), no Python, no venv, no Docker
- ~5 MB RAM at idle vs ~50 MB for Python
- Instant startup

## Setup

### Build

```bash
go build -o slack-mcp .
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SLACK_XOXC_TOKEN` | Yes | | Slack xoxc web token |
| `SLACK_XOXD_TOKEN` | Yes | | Slack xoxd cookie token |
| `LOGS_CHANNEL_ID` | Yes | | Channel ID for logging tool invocations |
| `MCP_TRANSPORT` | No | `stdio` | Transport mode |
| `OUTPUT_FORMAT` | No | `compact` | Message format: `compact` (one-line text) or `json` |
| `SLACK_MCP_READ_ONLY` | No | `false` | When `true`/`1`/`yes`/`on`, disables write operations |

### Run

```bash
export SLACK_XOXC_TOKEN="xoxc-..."
export SLACK_XOXD_TOKEN="xoxd-..."
export LOGS_CHANNEL_ID="C0B8ADGN3L6"

./slack-mcp
```

### Claude Code Integration

Add to your Claude Code MCP settings (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "slack": {
      "command": "/path/to/slack-mcp",
      "env": {
        "SLACK_XOXC_TOKEN": "xoxc-...",
        "SLACK_XOXD_TOKEN": "xoxd-...",
        "LOGS_CHANNEL_ID": "C0B8ADGN3L6"
      }
    }
  }
}
```

Or use a wrapper script that sources tokens from a file (see the Python repo's `setup-slack-mcp.py` for token extraction).

## Tools

### Messaging

| Tool | Description |
|------|-------------|
| `post_message` | Post a message to a channel (supports thread replies via `thread_ts`) |
| `send_dm` | Send a direct message to a user |
| `send_group_dm` | Send a group DM to multiple users |
| `post_command` | Execute a slash command in a channel |

### Reading

| Tool | Description |
|------|-------------|
| `get_channel_history` | Get channel messages with pagination, date filtering, and optional thread expansion |
| `get_thread` | Get all messages in a thread |
| `search_messages` | Search messages across the workspace |
| `search_channel_messages` | Search messages within a specific channel |

### Reactions

| Tool | Description |
|------|-------------|
| `add_reaction` | Add an emoji reaction to a message |
| `get_reactions` | Get all reactions on a message |

### Channels

| Tool | Description |
|------|-------------|
| `list_joined_channels` | List channels the authenticated user has joined |
| `get_channel_id_by_name` | Look up a channel ID by name |
| `join_channel` | Join a channel |
| `create_channel` | Create a new channel (public or private) |
| `rename_channel` | Rename a channel |
| `invite_users_to_channel` | Invite users to a channel |

### Identity & Cache

| Tool | Description |
|------|-------------|
| `whoami` | Check authentication and return the current user |
| `refresh_channel_cache` | Reload the channel name-to-ID cache |
| `refresh_user_cache` | Clear the user handle cache |
| `clear_usergroup` | Clear all members from a usergroup |

## Token Extraction

The server uses Slack's internal web API with `xoxc`/`xoxd` tokens (the same cookie-based auth that the Slack web client uses). See the [Python slack-mcp setup guide](https://github.com/redhat-community-ai-tools/slack-mcp) for instructions on extracting these tokens from your browser.

## License

Apache-2.0
