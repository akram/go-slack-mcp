package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---- tool input structs ----

type whoamiIn struct{}

type getChannelHistoryIn struct {
	ChannelID      string `json:"channel_id" jsonschema:"The channel ID"`
	Limit          int    `json:"limit,omitempty" jsonschema:"Max messages to fetch (default 1000)"`
	Oldest         string `json:"oldest,omitempty" jsonschema:"Only messages after this date (e.g. 2024-01-15 or 2024-01-15T10:30:00)"`
	Latest         string `json:"latest,omitempty" jsonschema:"Only messages before this date (e.g. 2024-01-20 or 2024-01-20T18:00:00)"`
	IncludeThreads bool   `json:"include_threads,omitempty" jsonschema:"If true also fetch all replies in threads (default false)"`
}

type getThreadIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
	ThreadTs  string `json:"thread_ts" jsonschema:"The parent message timestamp"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max messages (default 100)"`
}

type searchMessagesIn struct {
	Query string `json:"query" jsonschema:"The search query text"`
	Sort  string `json:"sort,omitempty" jsonschema:"Sort by timestamp or score (default timestamp)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results to fetch (default 1000)"`
}

type searchChannelMessagesIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID to search within"`
	Query     string `json:"query" jsonschema:"The search query text"`
	Sort      string `json:"sort,omitempty" jsonschema:"Sort by timestamp or score (default timestamp)"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return (default 100)"`
}

type postMessageIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
	Message   string `json:"message" jsonschema:"The message text"`
	ThreadTs  string `json:"thread_ts,omitempty" jsonschema:"Thread timestamp to reply to"`
}

type sendDmIn struct {
	UserID  string `json:"user_id" jsonschema:"The user ID to DM"`
	Message string `json:"message" jsonschema:"The message text"`
}

type postCommandIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
	Command   string `json:"command" jsonschema:"The slash command"`
	Text      string `json:"text" jsonschema:"The command text"`
}

type addReactionIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
	MessageTs string `json:"message_ts" jsonschema:"The message timestamp"`
	Reaction  string `json:"reaction" jsonschema:"The reaction emoji name"`
}

type joinChannelIn struct {
	ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
}

type listJoinedChannelsIn struct {
	ExcludeArchived bool   `json:"exclude_archived,omitempty" jsonschema:"Exclude archived channels (default true)"`
	Limit           int    `json:"limit,omitempty" jsonschema:"Max channels to return (default 1000)"`
	Types           string `json:"types,omitempty" jsonschema:"Channel types comma-separated (default public_channel,private_channel)"`
}

type getChannelIDByNameIn struct {
	ChannelName string `json:"channel_name" jsonschema:"The channel name with or without # prefix"`
}

type refreshChannelCacheIn struct{}
type refreshUserCacheIn struct{}

// ---- output structs ----

type boolResult struct {
	Result bool `json:"result"`
}

type stringResult struct {
	Result string `json:"result"`
}

type intResult struct {
	Result int `json:"result"`
}

type listResult struct {
	Result []any `json:"result"`
}

type channelsResult struct {
	Result []channelEntry `json:"result"`
}

type channelEntry struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	IsIM       bool   `json:"is_im,omitempty"`
	IsMpim     bool   `json:"is_mpim,omitempty"`
}

func (s *server) registerTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "whoami",
		Description: "Checks authentication & identity.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ whoamiIn) (*mcp.CallToolResult, stringResult, error) {
		s.logToSlack("Checking authentication & identity")
		data, err := s.slack.request("POST", "auth.test", nil)
		if err != nil {
			return nil, stringResult{}, err
		}
		user, _ := data["user"].(string)
		return nil, stringResult{Result: user}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_channel_history",
		Description: `Get the history of a channel with pagination support. Limit parameter controls max messages to fetch (default 1000).

Optional date filtering (accepts ISO 8601 dates or Unix timestamps):
- oldest: Only messages after this date (e.g., "2024-01-15" or "2024-01-15T10:30:00")
- latest: Only messages before this date (e.g., "2024-01-20" or "2024-01-20T18:00:00")
- include_threads: If True, also fetch all replies in threads (default False)

Note: For date-only formats, 'oldest' defaults to start of day (00:00:00) and 'latest' to end of day (23:59:59).`,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getChannelHistoryIn) (*mcp.CallToolResult, listResult, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 1000
		}
		s.logToSlack(fmt.Sprintf("Getting history of channel <#%s> (limit: %d, include_threads: %v)", in.ChannelID, limit, in.IncludeThreads))

		oldestTs := parseTimestamp(in.Oldest, false)
		latestTs := parseTimestamp(in.Latest, true)

		var allMessages []map[string]any
		cursor := ""

		for len(allMessages) < limit {
			payload := map[string]any{
				"channel": in.ChannelID,
				"limit":   int(math.Min(200, float64(limit-len(allMessages)))),
			}
			if oldestTs != "" {
				payload["oldest"] = oldestTs
			}
			if latestTs != "" {
				payload["latest"] = latestTs
			}
			if cursor != "" {
				payload["cursor"] = cursor
			}

			data, err := s.slack.request("GET", "conversations.history", payload)
			if err != nil {
				logStderr("Error getting channel history: %v", err)
				break
			}
			if data == nil || data["ok"] != true {
				errMsg := "Unknown error"
				if data != nil {
					if e, ok := data["error"].(string); ok {
						errMsg = e
					}
				}
				logStderr("Error getting channel history: %s", errMsg)
				break
			}

			msgs := toMapSlice(data["messages"])
			allMessages = append(allMessages, msgs...)

			meta, _ := data["response_metadata"].(map[string]any)
			if meta != nil {
				cursor, _ = meta["next_cursor"].(string)
			} else {
				cursor = ""
			}
			if cursor == "" {
				break
			}
		}

		logStderr("Retrieved %d messages from channel %s", len(allMessages), in.ChannelID)

		if in.IncludeThreads {
			var threadMsgs []map[string]any
			for _, msg := range allMessages {
				if rc, _ := msg["reply_count"].(float64); rc > 0 {
					ts, _ := msg["ts"].(string)
					if ts != "" {
						replies := s.getThreadReplies(in.ChannelID, ts)
						threadMsgs = append(threadMsgs, replies...)
					}
				}
			}
			allMessages = append(allMessages, threadMsgs...)
			logStderr("Retrieved %d additional messages from threads", len(threadMsgs))
		}

		s.prefetchUsers(allMessages)
		result := make([]any, len(allMessages))
		for i, msg := range allMessages {
			result[i] = s.filterMessage(msg)
		}
		return nil, listResult{Result: result}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_thread",
		Description: `Get all messages in a thread given a channel ID and the parent message timestamp.

Use this to read a full conversation thread before replying to it.
The thread_ts is the timestamp of the parent message that started the thread.`,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getThreadIn) (*mcp.CallToolResult, listResult, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 100
		}
		s.logToSlack(fmt.Sprintf("Getting thread %s in channel <#%s> (limit: %d)", in.ThreadTs, in.ChannelID, limit))

		payload := map[string]any{
			"channel": in.ChannelID,
			"ts":      convertThreadTs(in.ThreadTs),
			"limit":   limit,
		}
		data, err := s.slack.request("GET", "conversations.replies", payload)
		if err != nil {
			return nil, listResult{}, err
		}
		if data == nil || data["ok"] != true {
			errMsg := "Unknown error"
			if data != nil {
				if e, ok := data["error"].(string); ok {
					errMsg = e
				}
			}
			logStderr("Error getting thread: %s", errMsg)
			return nil, listResult{Result: []any{}}, nil
		}

		msgs := toMapSlice(data["messages"])
		logStderr("Retrieved %d messages from thread %s", len(msgs), in.ThreadTs)

		s.prefetchUsers(msgs)
		result := make([]any, len(msgs))
		for i, msg := range msgs {
			result[i] = s.filterMessage(msg)
		}
		return nil, listResult{Result: result}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_messages",
		Description: "Search for messages in the workspace with pagination support. Limit parameter controls max results to fetch (default 1000).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchMessagesIn) (*mcp.CallToolResult, listResult, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 1000
		}
		sort := in.Sort
		if sort == "" {
			sort = "timestamp"
		}
		s.logToSlack(fmt.Sprintf("Searching for messages: %s (limit: %d)", in.Query, limit))
		results := s.searchMessages(in.Query, sort, limit)
		return nil, listResult{Result: results}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "search_channel_messages",
		Description: `Search for messages within a specific channel.

Uses Slack's search API with an 'in:<channel>' filter.`,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in searchChannelMessagesIn) (*mcp.CallToolResult, listResult, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 100
		}
		sort := in.Sort
		if sort == "" {
			sort = "timestamp"
		}
		s.logToSlack(fmt.Sprintf("Searching in channel <#%s> for: %s (limit: %d)", in.ChannelID, in.Query, limit))
		scopedQuery := fmt.Sprintf("in:<#%s> %s", in.ChannelID, in.Query)
		results := s.searchMessages(scopedQuery, sort, limit)
		return nil, listResult{Result: results}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "post_message",
		Description: "Post a message to a channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in postMessageIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Posting message to channel <#%s>: %s", in.ChannelID, in.Message))
		ok := s.postMessage(in.ChannelID, in.Message, in.ThreadTs)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "send_dm",
		Description: "Send a direct message to a user.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in sendDmIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Sending direct message to user <@%s>: %s", in.UserID, in.Message))
		data, err := s.slack.request("POST", "conversations.open", map[string]any{
			"users":     in.UserID,
			"return_dm": true,
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		if data == nil || data["ok"] != true {
			return nil, boolResult{Result: false}, nil
		}
		ch, _ := data["channel"].(map[string]any)
		if ch == nil {
			return nil, boolResult{Result: false}, nil
		}
		chID, _ := ch["id"].(string)
		if chID == "" {
			return nil, boolResult{Result: false}, nil
		}
		ok := s.postMessage(chID, in.Message, "")
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "post_command",
		Description: "Post a command to a channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in postCommandIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Posting command to channel <#%s>: %s %s", in.ChannelID, in.Command, in.Text))
		s.joinChannel(in.ChannelID)
		data, err := s.slack.request("POST", "chat.command", map[string]any{
			"channel": in.ChannelID,
			"command": in.Command,
			"text":    in.Text,
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		ok, _ := data["ok"].(bool)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_reaction",
		Description: "Add a reaction to a message.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in addReactionIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Adding reaction to message %s in channel <#%s>: :%s:", in.MessageTs, in.ChannelID, in.Reaction))
		data, err := s.slack.request("POST", "reactions.add", map[string]any{
			"channel":   in.ChannelID,
			"name":      in.Reaction,
			"timestamp": convertThreadTs(in.MessageTs),
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		ok, _ := data["ok"].(bool)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "join_channel",
		Description: "Join a channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in joinChannelIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Joining channel <#%s>", in.ChannelID))
		ok := s.joinChannel(in.ChannelID)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_joined_channels",
		Description: `List channels the authenticated user is a member of.

Uses Slack's users.conversations API. By default returns public and private
channels only. To include DMs and group DMs, set types to e.g.
"public_channel,private_channel,im,mpim".`,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in listJoinedChannelsIn) (*mcp.CallToolResult, channelsResult, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 1000
		}
		types := in.Types
		if types == "" {
			types = "public_channel,private_channel"
		}
		excludeArchived := "true"
		if !in.ExcludeArchived && in.Limit != 0 {
			excludeArchived = "false"
		}

		s.logToSlack(fmt.Sprintf("Listing joined channels (limit: %d, types: %s, exclude_archived: %s)", limit, types, excludeArchived))

		var allChannels []channelEntry
		cursor := ""

		for len(allChannels) < limit {
			payload := map[string]any{
				"types":            types,
				"limit":            int(math.Min(200, float64(limit-len(allChannels)))),
				"exclude_archived": excludeArchived,
			}
			if cursor != "" {
				payload["cursor"] = cursor
			}

			data, err := s.slack.request("GET", "users.conversations", payload)
			if err != nil {
				logStderr("Error listing joined channels: %v", err)
				break
			}
			if data == nil || data["ok"] != true {
				errMsg := "Unknown error"
				if data != nil {
					if e, ok := data["error"].(string); ok {
						errMsg = e
					}
				}
				logStderr("Error listing joined channels: %s", errMsg)
				break
			}

			channels := toMapSlice(data["channels"])
			for _, ch := range channels {
				entry := channelEntry{
					ID:   strVal(ch, "id"),
					Name: strVal(ch, "name"),
				}
				if v, ok := ch["is_private"].(bool); ok {
					entry.IsPrivate = v
				}
				if v, ok := ch["is_archived"].(bool); ok {
					entry.IsArchived = v
				}
				if v, ok := ch["is_im"].(bool); ok && v {
					entry.IsIM = true
				}
				if v, ok := ch["is_mpim"].(bool); ok && v {
					entry.IsMpim = true
				}
				allChannels = append(allChannels, entry)
			}

			meta, _ := data["response_metadata"].(map[string]any)
			if meta != nil {
				cursor, _ = meta["next_cursor"].(string)
			} else {
				cursor = ""
			}
			if cursor == "" {
				break
			}
		}

		logStderr("Listed %d joined channels", len(allChannels))
		return nil, channelsResult{Result: allChannels}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_channel_id_by_name",
		Description: "Get the channel ID by channel name. The channel name can be with or without the # prefix.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getChannelIDByNameIn) (*mcp.CallToolResult, stringResult, error) {
		cleanName := strings.TrimLeft(in.ChannelName, "#")
		s.logToSlack(fmt.Sprintf("Looking up channel ID for channel name: %s", cleanName))

		if id, ok := s.channels.get(cleanName); ok {
			logStderr("Channel '%s' found in cache", cleanName)
			return nil, stringResult{Result: id}, nil
		}

		logStderr("Cache miss for '%s', loading channels...", cleanName)
		s.loadChannelsToCache()
		if id, ok := s.channels.get(cleanName); ok {
			return nil, stringResult{Result: id}, nil
		}

		logStderr("Channel '%s' not found", cleanName)
		return nil, stringResult{Result: ""}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "refresh_channel_cache",
		Description: "Refresh the channel cache. Use this when new channels are created or if channel lookups are failing.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ refreshChannelCacheIn) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack("Refreshing channel cache")
		ok := s.loadChannelsToCache()
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "refresh_user_cache",
		Description: "Clear the user cache. Use this when user handles are outdated or if user lookups are failing. Returns the number of cached entries cleared.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ refreshUserCacheIn) (*mcp.CallToolResult, intResult, error) {
		s.logToSlack("Clearing user cache")
		count := s.users.clear()
		return nil, intResult{Result: count}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_reactions",
		Description: "Get reactions on a message.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
		MessageTs string `json:"message_ts" jsonschema:"The message timestamp"`
	}) (*mcp.CallToolResult, listResult, error) {
		s.logToSlack(fmt.Sprintf("Getting reactions on message %s in channel <#%s>", in.MessageTs, in.ChannelID))
		data, err := s.slack.request("GET", "reactions.get", map[string]any{
			"channel":   in.ChannelID,
			"timestamp": convertThreadTs(in.MessageTs),
		})
		if err != nil {
			return nil, listResult{Result: []any{}}, nil
		}
		if data == nil || data["ok"] != true {
			return nil, listResult{Result: []any{}}, nil
		}
		msg, _ := data["message"].(map[string]any)
		if msg == nil {
			return nil, listResult{Result: []any{}}, nil
		}
		reactions, _ := msg["reactions"].([]any)
		if reactions == nil {
			reactions = []any{}
		}
		return nil, listResult{Result: reactions}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_channel",
		Description: "Rename a channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		ChannelID string `json:"channel_id" jsonschema:"The channel ID to rename"`
		Name      string `json:"name" jsonschema:"The new channel name"`
	}) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Renaming channel <#%s> to %s", in.ChannelID, in.Name))
		data, err := s.slack.request("POST", "conversations.rename", map[string]any{
			"channel": in.ChannelID,
			"name":    in.Name,
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		ok, _ := data["ok"].(bool)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_channel",
		Description: "Create a new channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		Name      string `json:"name" jsonschema:"The channel name"`
		IsPrivate bool   `json:"is_private,omitempty" jsonschema:"Create as private channel"`
	}) (*mcp.CallToolResult, stringResult, error) {
		s.logToSlack(fmt.Sprintf("Creating channel %s (private: %v)", in.Name, in.IsPrivate))
		data, err := s.slack.request("POST", "conversations.create", map[string]any{
			"name":       in.Name,
			"is_private": in.IsPrivate,
		})
		if err != nil {
			return nil, stringResult{Result: ""}, nil
		}
		if data == nil || data["ok"] != true {
			return nil, stringResult{Result: ""}, nil
		}
		ch, _ := data["channel"].(map[string]any)
		if ch == nil {
			return nil, stringResult{Result: ""}, nil
		}
		id, _ := ch["id"].(string)
		return nil, stringResult{Result: id}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "invite_users_to_channel",
		Description: "Invite users to a channel.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		ChannelID string `json:"channel_id" jsonschema:"The channel ID"`
		UserIDs   string `json:"user_ids" jsonschema:"Comma-separated list of user IDs"`
	}) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Inviting users %s to channel <#%s>", in.UserIDs, in.ChannelID))
		data, err := s.slack.request("POST", "conversations.invite", map[string]any{
			"channel": in.ChannelID,
			"users":   in.UserIDs,
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		ok, _ := data["ok"].(bool)
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "send_group_dm",
		Description: "Send a group direct message to multiple users.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		UserIDs string `json:"user_ids" jsonschema:"Comma-separated list of user IDs"`
		Message string `json:"message" jsonschema:"The message text"`
	}) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Sending group DM to users %s: %s", in.UserIDs, in.Message))
		data, err := s.slack.request("POST", "conversations.open", map[string]any{
			"users":     in.UserIDs,
			"return_dm": true,
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		if data == nil || data["ok"] != true {
			return nil, boolResult{Result: false}, nil
		}
		ch, _ := data["channel"].(map[string]any)
		if ch == nil {
			return nil, boolResult{Result: false}, nil
		}
		chID, _ := ch["id"].(string)
		if chID == "" {
			return nil, boolResult{Result: false}, nil
		}
		ok := s.postMessage(chID, in.Message, "")
		return nil, boolResult{Result: ok}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "clear_usergroup",
		Description: "Clear all members from a usergroup.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in struct {
		UsergroupID string `json:"usergroup_id" jsonschema:"The usergroup ID"`
	}) (*mcp.CallToolResult, boolResult, error) {
		s.logToSlack(fmt.Sprintf("Clearing usergroup %s", in.UsergroupID))
		data, err := s.slack.request("POST", "usergroups.users.update", map[string]any{
			"usergroup": in.UsergroupID,
			"users":     "",
		})
		if err != nil {
			return nil, boolResult{Result: false}, nil
		}
		ok, _ := data["ok"].(bool)
		return nil, boolResult{Result: ok}, nil
	})
}

// ---- helpers ----

func (s *server) postMessage(channelID, message, threadTs string) bool {
	s.joinChannel(channelID)
	payload := map[string]any{
		"channel": channelID,
		"text":    message,
	}
	if threadTs != "" {
		payload["thread_ts"] = convertThreadTs(threadTs)
	}
	data, err := s.slack.request("POST", "chat.postMessage", payload)
	if err != nil {
		return false
	}
	ok, _ := data["ok"].(bool)
	return ok
}

func (s *server) joinChannel(channelID string) bool {
	data, err := s.slack.request("POST", "conversations.join", map[string]any{
		"channel": channelID,
	})
	if err != nil {
		return false
	}
	ok, _ := data["ok"].(bool)
	return ok
}

func (s *server) logToSlack(message string) {
	s.postMessageNoLog(s.logsChannelID, message)
}

func (s *server) postMessageNoLog(channelID, message string) {
	s.joinChannel(channelID)
	s.slack.request("POST", "chat.postMessage", map[string]any{
		"channel": channelID,
		"text":    message,
	})
}

func (s *server) getThreadReplies(channelID, threadTs string) []map[string]any {
	data, err := s.slack.request("GET", "conversations.replies", map[string]any{
		"channel": channelID,
		"ts":      threadTs,
	})
	if err != nil || data == nil || data["ok"] != true {
		errMsg := "Unknown error"
		if data != nil {
			if e, ok := data["error"].(string); ok {
				errMsg = e
			}
		}
		logStderr("Error getting thread replies: %s", errMsg)
		return nil
	}
	msgs := toMapSlice(data["messages"])
	if len(msgs) > 1 {
		return msgs[1:]
	}
	return nil
}

func (s *server) loadChannelsToCache() bool {
	s.channels.clear()
	cursor := ""
	for {
		payload := map[string]any{
			"exclude_archived": "true",
			"types":            "public_channel,private_channel",
			"limit":            200,
		}
		if cursor != "" {
			payload["cursor"] = cursor
		}
		data, err := s.slack.request("GET", "conversations.list", payload)
		if err != nil || data == nil || data["ok"] != true {
			errMsg := "Unknown error"
			if data != nil {
				if e, ok := data["error"].(string); ok {
					errMsg = e
				}
			}
			logStderr("Error loading channels to cache: %s", errMsg)
			return s.channels.size() > 0
		}
		channels := toMapSlice(data["channels"])
		for _, ch := range channels {
			name := strVal(ch, "name")
			id := strVal(ch, "id")
			if name != "" && id != "" {
				s.channels.set(name, id)
			}
		}
		meta, _ := data["response_metadata"].(map[string]any)
		if meta != nil {
			cursor, _ = meta["next_cursor"].(string)
		} else {
			cursor = ""
		}
		if cursor == "" {
			break
		}
	}
	logStderr("Loaded %d channels into cache", s.channels.size())
	return true
}

func (s *server) searchMessages(query, sort string, limit int) []any {
	var allMatches []map[string]any
	page := 1

	for len(allMatches) < limit {
		count := limit - len(allMatches)
		if count > 100 {
			count = 100
		}
		data, err := s.slack.request("GET", "search.messages", map[string]any{
			"query": query,
			"sort":  sort,
			"count": count,
			"page":  page,
		})
		if err != nil {
			logStderr("Error searching messages: %v", err)
			break
		}
		if data == nil || data["ok"] != true {
			errMsg := "Unknown error"
			if data != nil {
				if e, ok := data["error"].(string); ok {
					errMsg = e
				}
			}
			logStderr("Error searching messages: %s", errMsg)
			break
		}

		messagesData, _ := data["messages"].(map[string]any)
		if messagesData == nil {
			break
		}
		matches := toMapSlice(messagesData["matches"])
		allMatches = append(allMatches, matches...)

		pagination, _ := messagesData["pagination"].(map[string]any)
		totalPages := 1
		if pagination != nil {
			if tp, ok := pagination["page_count"].(float64); ok {
				totalPages = int(tp)
			}
		}
		if page >= totalPages || len(matches) == 0 {
			break
		}
		page++
	}

	logStderr("Retrieved %d search results for query: %s", len(allMatches), query)

	s.prefetchUsers(allMatches)
	result := make([]any, len(allMatches))
	for i, msg := range allMatches {
		result[i] = s.filterMessage(msg)
	}
	return result
}

// ---- utility ----

func toMapSlice(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
