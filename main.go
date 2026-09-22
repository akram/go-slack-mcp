package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

type server struct {
	slack         *slackClient
	users         *userCache
	channels      *channelCache
	logsChannelID string
	outputFormat  string
	readOnly      bool
}

func main() {
	xoxc := os.Getenv("SLACK_XOXC_TOKEN")
	xoxd := os.Getenv("SLACK_XOXD_TOKEN")
	if xoxc == "" || xoxd == "" {
		fmt.Fprintln(os.Stderr, "Error: SLACK_XOXC_TOKEN and SLACK_XOXD_TOKEN must be set")
		os.Exit(1)
	}

	logsChannelID := os.Getenv("LOGS_CHANNEL_ID")
	if logsChannelID == "" {
		fmt.Fprintln(os.Stderr, "Error: LOGS_CHANNEL_ID must be set")
		os.Exit(1)
	}

	outputFormat := strings.ToLower(os.Getenv("OUTPUT_FORMAT"))
	if outputFormat == "" {
		outputFormat = "compact"
	}

	roVal := strings.ToLower(strings.TrimSpace(os.Getenv("SLACK_MCP_READ_ONLY")))
	readOnly := roVal == "1" || roVal == "true" || roVal == "yes" || roVal == "on"

	exe, _ := os.Executable()
	cacheDir := filepath.Dir(exe)
	if cacheDir == "" || cacheDir == "." {
		cacheDir, _ = os.Getwd()
	}

	s := &server{
		slack:         newSlackClient(xoxc, xoxd),
		users:         newUserCache(cacheDir),
		channels:      newChannelCache(),
		logsChannelID: logsChannelID,
		outputFormat:  outputFormat,
		readOnly:      readOnly,
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "slack",
		Version: version,
	}, nil)

	s.registerTools(mcpServer)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logStderr("serving MCP over stdio")
	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
