# go-slack-mcp

A Slack MCP (Model Context Protocol) server written in Go. Drop-in replacement for [slack-mcp](https://github.com/redhat-community-ai-tools/slack-mcp) (Python) with zero runtime dependencies.

## Why

- Single static binary (14 MB), no Python, no venv, no Docker
- ~5 MB RAM at idle vs ~50 MB for Python
- Instant startup
- Built-in token extraction (no separate Playwright/Python setup)

## Install

### Homebrew

```bash
brew tap akram/go-slack-mcp https://github.com/akram/go-slack-mcp
brew install slack-mcp
```

### From source

```bash
go install github.com/akram/go-slack-mcp@latest
```

### Build locally

```bash
git clone https://github.com/akram/go-slack-mcp.git
cd go-slack-mcp
go build -o slack-mcp .
```

## Quick Start

The fastest way to get going:

```bash
slack-mcp setup
```

This will:
1. Open a browser window for you to log in to Slack
2. Extract your session tokens (xoxc/xoxd) automatically
3. Prompt for a channel ID to use for MCP server logs
4. Write a wrapper script and register the MCP server in Claude Code

### Setup Options

```bash
slack-mcp setup                                    # interactive setup
slack-mcp setup --refresh-tokens                   # re-extract expired tokens
slack-mcp setup --set-logs-channel C01234567       # skip channel prompt
slack-mcp setup --server-name slack-work           # custom MCP server name
slack-mcp setup --workspace-url https://myteam.slack.com/  # specific workspace
```

**Prerequisites for setup:** Chrome or Chromium must be installed (used to open Slack and extract session cookies). No additional downloads needed.

### Token Refresh

Slack session tokens expire periodically. When tools start failing with auth errors:

```bash
slack-mcp setup --refresh-tokens
```

This re-opens the browser, extracts fresh tokens, and updates the stored credentials. No other reconfiguration needed.

## Manual Configuration

If you prefer to manage tokens yourself:

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SLACK_XOXC_TOKEN` | Yes | | Slack xoxc web token |
| `SLACK_XOXD_TOKEN` | Yes | | Slack xoxd cookie token |
| `LOGS_CHANNEL_ID` | Yes | | Channel ID for logging tool invocations |
| `MCP_TRANSPORT` | No | `stdio` | Transport mode |
| `OUTPUT_FORMAT` | No | `compact` | Message format: `compact` (one-line text) or `json` |
| `SLACK_MCP_READ_ONLY` | No | `false` | When `true`/`1`/`yes`/`on`, disables write operations |

### Run directly

```bash
export SLACK_XOXC_TOKEN="xoxc-..."
export SLACK_XOXD_TOKEN="xoxd-..."
export LOGS_CHANNEL_ID="C0B8ADGN3L6"

slack-mcp
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

## Subcommands

| Command | Description |
|---------|-------------|
| `slack-mcp` | Start the MCP server (default, stdio mode) |
| `slack-mcp setup [flags]` | Interactive setup: extract tokens, write config, register in Claude |
| `slack-mcp version` | Print version |

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

## License

Apache-2.0
