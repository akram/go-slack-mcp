package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var mentionRe = regexp.MustCompile(`<@([A-Z0-9]+)>`)

func (s *server) replaceUserMentions(text string) string {
	if text == "" {
		return text
	}
	return mentionRe.ReplaceAllStringFunc(text, func(match string) string {
		uid := mentionRe.FindStringSubmatch(match)[1]
		handle := s.resolveUserHandle(uid)
		return "@" + handle
	})
}

func (s *server) resolveUserHandle(userID string) string {
	if userID == "" {
		return ""
	}
	if h, ok := s.users.get(userID); ok {
		return h
	}
	data, err := s.slack.request("GET", "users.info", map[string]any{"user": userID})
	if err == nil && data != nil && data["ok"] == true {
		user, _ := data["user"].(map[string]any)
		if user != nil {
			profile, _ := user["profile"].(map[string]any)
			handle := ""
			if profile != nil {
				handle, _ = profile["display_name"].(string)
			}
			if handle == "" {
				handle, _ = user["real_name"].(string)
			}
			if handle == "" {
				handle, _ = user["name"].(string)
			}
			if handle == "" {
				handle = userID
			}
			s.users.set(userID, handle)
			return handle
		}
	}
	s.users.set(userID, userID)
	return userID
}

func (s *server) prefetchUsers(messages []map[string]any) {
	seen := make(map[string]bool)
	for _, msg := range messages {
		if uid, _ := msg["user"].(string); uid != "" {
			seen[uid] = true
		}
		if text, _ := msg["text"].(string); text != "" {
			for _, m := range mentionRe.FindAllStringSubmatch(text, -1) {
				seen[m[1]] = true
			}
		}
	}
	for uid := range seen {
		s.resolveUserHandle(uid)
	}
}

type filteredMessage struct {
	Text      string `json:"text"`
	User      string `json:"user"`
	Ts        string `json:"ts"`
	ThreadTs  string `json:"thread_ts,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	Channel   string `json:"channel_name,omitempty"`
}

func (s *server) filterMessage(msg map[string]any) any {
	text, _ := msg["text"].(string)
	userID, _ := msg["user"].(string)
	ts, _ := msg["ts"].(string)
	threadTs, _ := msg["thread_ts"].(string)

	var channelID, channelName string
	if ch, ok := msg["channel"].(map[string]any); ok {
		channelID, _ = ch["id"].(string)
		channelName, _ = ch["name"].(string)
	}

	handle := s.resolveUserHandle(userID)
	text = s.replaceUserMentions(text)

	if s.outputFormat == "json" {
		fm := filteredMessage{
			Text: text,
			User: handle,
			Ts:   ts,
		}
		if threadTs != "" {
			fm.ThreadTs = threadTs
		}
		if channelID != "" {
			fm.ChannelID = channelID
		}
		if channelName != "" {
			fm.Channel = channelName
		}
		return fm
	}

	result := fmt.Sprintf("[%s] @%s: %s", ts, handle, text)
	if channelID != "" {
		result += fmt.Sprintf(" [channel:%s|%s]", channelID, channelName)
	}
	if threadTs != "" && threadTs != ts {
		result += fmt.Sprintf(" [thread:%s]", threadTs)
	}
	return result
}

func convertThreadTs(ts string) string {
	matched, _ := regexp.MatchString(`^\d+\.\d+$`, ts)
	if matched {
		return ts
	}
	matched16, _ := regexp.MatchString(`^\d{16}$`, ts)
	if matched16 {
		return ts[:10] + "." + ts[10:]
	}
	return ""
}

func parseTimestamp(dateStr string, isEndOfRange bool) string {
	if dateStr == "" {
		return ""
	}
	matched, _ := regexp.MatchString(`^\d+(\.\d+)?$`, dateStr)
	if matched {
		return dateStr
	}
	isDateOnly, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, dateStr)
	if isDateOnly {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			logStderr("Error parsing date '%s': %v", dateStr, err)
			return ""
		}
		if isEndOfRange {
			t = t.Add(24*time.Hour - time.Microsecond)
		}
		return formatUnixMicro(t)
	}
	dateStr = strings.Replace(dateStr, "Z", "+00:00", 1)
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", dateStr)
		if err != nil {
			logStderr("Error parsing date '%s': %v", dateStr, err)
			return ""
		}
		t = t.UTC()
	}
	return formatUnixMicro(t)
}

func formatUnixMicro(t time.Time) string {
	sec := t.Unix()
	usec := t.Nanosecond() / 1000
	return strconv.FormatInt(sec, 10) + "." + fmt.Sprintf("%06d", usec)
}
