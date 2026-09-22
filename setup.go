package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	defaultWorkspaceURL = "https://app.slack.com/client/"
	profileDirName      = ".slack-token-extractor"
)

type setupConfig struct {
	refreshTokens  bool
	logsChannel    string
	skipVerify     bool
	workspaceURL   string
	installDir     string
	serverName     string
	binaryPath     string
}

func defaultInstallDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "slack-mcp")
}

func defaultProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, profileDirName, "browser-profile")
}

func tokensFilePath(installDir string) string {
	return filepath.Join(installDir, "tokens.env")
}

func wrapperScriptPath(installDir string) string {
	return filepath.Join(installDir, "run-slack-mcp.sh")
}

func tokensExist(installDir string) bool {
	data, err := os.ReadFile(tokensFilePath(installDir))
	if err != nil {
		return false
	}
	text := string(data)
	return strings.Contains(text, "SLACK_MCP_XOXC_TOKEN=xoxc-") && strings.Contains(text, "SLACK_MCP_XOXD_TOKEN=")
}

func runSetup(cfg setupConfig) error {
	fmt.Println()
	fmt.Println("  Slack MCP Setup for Claude Code (Go edition)")
	fmt.Println("  =============================================")
	fmt.Println()
	fmt.Println("  One manual step required: log in to Slack when the")
	fmt.Println("  browser opens. Everything else is automatic.")
	fmt.Println()

	if cfg.installDir == "" {
		cfg.installDir = defaultInstallDir()
	}
	if cfg.workspaceURL == "" {
		cfg.workspaceURL = defaultWorkspaceURL
	}
	if cfg.serverName == "" {
		cfg.serverName = "slack"
	}
	if cfg.binaryPath == "" {
		exe, _ := os.Executable()
		cfg.binaryPath = exe
	}

	if err := os.MkdirAll(cfg.installDir, 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	channelID, err := extractTokens(cfg)
	if err != nil {
		return err
	}

	logsChannel := cfg.logsChannel
	if logsChannel == "" && channelID != "" {
		logsChannel = channelID
	}
	if logsChannel == "" {
		fmt.Println()
		fmt.Println("  The MCP server writes activity logs to a Slack channel.")
		fmt.Println("  Any channel works. A self-DM or DM with Slackbot is easiest.")
		fmt.Println()
		for {
			fmt.Print("  Enter channel ID (starts with C, D, or G): ")
			fmt.Scanln(&logsChannel)
			logsChannel = strings.TrimSpace(logsChannel)
			if len(logsChannel) >= 9 && (logsChannel[0] == 'C' || logsChannel[0] == 'D' || logsChannel[0] == 'G') {
				break
			}
			fmt.Println("  Not a valid Slack ID. Try again.")
		}
	}

	writeWrapperScript(cfg, logsChannel)
	registerMCPInClaude(cfg)

	fmt.Println()
	fmt.Println("  Setup complete!")
	fmt.Println("  ===============")
	fmt.Println()
	fmt.Printf("  MCP server name : %s\n", cfg.serverName)
	fmt.Printf("  Wrapper script  : %s\n", wrapperScriptPath(cfg.installDir))
	fmt.Printf("  Tokens          : %s\n", tokensFilePath(cfg.installDir))
	fmt.Printf("  Binary          : %s\n", cfg.binaryPath)
	fmt.Println()
	fmt.Println("  Start a new Claude Code session to activate the plugin.")
	fmt.Println("  Test it by asking: 'What is my username in Slack?'")
	fmt.Println()
	fmt.Println("  To refresh tokens when they expire:")
	fmt.Printf("    %s setup --refresh-tokens\n", cfg.binaryPath)
	fmt.Println()

	return nil
}

func extractTokens(cfg setupConfig) (string, error) {
	fmt.Println("  Extracting Slack session tokens")
	fmt.Println("  -------------------------------")

	if tokensExist(cfg.installDir) && !cfg.refreshTokens {
		fmt.Printf("  Tokens already exist at %s\n", tokensFilePath(cfg.installDir))
		fmt.Println("  Run with --refresh-tokens to re-extract.")
		return "", nil
	}

	profileDir := defaultProfileDir()
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return "", fmt.Errorf("create profile dir: %w", err)
	}

	fmt.Println("  Launching browser...")
	fmt.Println("  Log in to Slack in the browser window that opens.")
	fmt.Println()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.UserDataDir(profileDir),
		chromedp.WindowSize(1280, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	if err := chromedp.Run(ctx, chromedp.Navigate(cfg.workspaceURL)); err != nil {
		return "", fmt.Errorf("navigate to Slack: %w", err)
	}

	time.Sleep(3 * time.Second)

	var currentURL string
	if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err != nil {
		return "", fmt.Errorf("get URL: %w", err)
	}

	channelID := ""
	needsLogin := strings.Contains(currentURL, "signin") || strings.Contains(currentURL, "sign_in")

	if needsLogin {
		fmt.Println("  ========================================================")
		fmt.Println("  Log in to Slack in the browser window.")
		fmt.Println()
		fmt.Println("  The MCP server logs its activity to a Slack channel.")
		fmt.Println("  Navigate to any channel or DM you want to use and note")
		fmt.Println("  the channel ID from the URL:")
		fmt.Println("    https://app.slack.com/client/TXXXXXXXX/DXXXXXXXXX")
		fmt.Println("                                           ^^^^^^^^^^^^ this part")
		fmt.Println("    (IDs start with C, D, or G)")
		fmt.Println()
		fmt.Println("  Then come back here.")
		fmt.Println("  ========================================================")
		fmt.Println()

		for {
			fmt.Print("  Enter the channel ID you have selected: ")
			fmt.Scanln(&channelID)
			channelID = strings.TrimSpace(channelID)
			if len(channelID) >= 9 && (channelID[0] == 'C' || channelID[0] == 'D' || channelID[0] == 'G') {
				break
			}
			fmt.Println("  Not a valid Slack ID. Try again.")
		}

		fmt.Println()
		fmt.Print("  Proceed with fetching Slack tokens? [Y/n] ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "" && answer != "y" && answer != "yes" {
			fmt.Println("  Aborted.")
			return "", fmt.Errorf("aborted by user")
		}

		fmt.Println()
		fmt.Println("  Fetching tokens... The browser window will close shortly.")

		// Wait for Slack to load after login
		for i := 0; i < 30; i++ {
			time.Sleep(time.Second)
			chromedp.Run(ctx, chromedp.Location(&currentURL))
			if strings.Contains(currentURL, "/client/") {
				time.Sleep(3 * time.Second)
				break
			}
		}
	}

	// Extract team ID from URL
	teamIDRe := regexp.MustCompile(`/client/([A-Z0-9]+)`)
	matches := teamIDRe.FindStringSubmatch(currentURL)
	var teamID string
	if len(matches) > 1 {
		teamID = matches[1]
	}

	if teamID == "" {
		// Try localStorage
		var result string
		chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			try {
				const c = JSON.parse(localStorage.localConfig_v2 || '{}');
				return Object.keys(c.teams || {})[0] || '';
			} catch { return ''; }
		})()`, &result))
		teamID = result
	}

	if teamID == "" {
		return "", fmt.Errorf("could not determine team ID - is Slack fully loaded?")
	}
	fmt.Printf("  Found team ID: %s\n", teamID)

	// Extract xoxc token from localStorage
	var xoxc string
	chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`((tid) => {
		try {
			const c = JSON.parse(localStorage.localConfig_v2 || '{}');
			if (c.teams?.[tid]?.token) return c.teams[tid].token;
			for (const d of Object.values(c.teams || {}))
				if (d.token?.startsWith('xoxc-')) return d.token;
		} catch {}
		return '';
	})(%q)`, teamID), &xoxc))

	if xoxc == "" {
		// Fallback: scan page HTML
		chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const m = document.body.innerHTML.match(/"token":"(xoxc-[^"]+)"/);
			return m ? m[1] : '';
		})()`, &xoxc))
	}

	if xoxc == "" {
		return "", fmt.Errorf("could not extract XOXC token")
	}
	fmt.Println("  Found XOXC token")

	// Extract xoxd cookie
	var cookies []*network.Cookie
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = network.GetCookies().Do(ctx)
		return err
	})); err != nil {
		return "", fmt.Errorf("get cookies: %w", err)
	}

	var xoxd string
	for _, c := range cookies {
		if c.Name == "d" && strings.Contains(c.Domain, "slack.com") {
			xoxd = c.Value
			break
		}
	}

	if xoxd == "" {
		return "", fmt.Errorf("could not extract XOXD token (cookie 'd' not found)")
	}
	fmt.Println("  Found XOXD token")

	// Save tokens
	tokensPath := tokensFilePath(cfg.installDir)
	content := fmt.Sprintf("# Slack session tokens (auto-generated by slack-mcp setup)\n# Team ID: %s\n\nSLACK_MCP_XOXC_TOKEN=%s\nSLACK_MCP_XOXD_TOKEN=%s\n", teamID, xoxc, xoxd)
	if err := os.WriteFile(tokensPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write tokens: %w", err)
	}
	fmt.Printf("  Tokens saved to: %s\n", tokensPath)

	return channelID, nil
}

func writeWrapperScript(cfg setupConfig, logsChannel string) {
	fmt.Println()
	fmt.Println("  Writing MCP wrapper script")
	fmt.Println("  --------------------------")

	content := fmt.Sprintf(`#!/usr/bin/env bash
# Slack MCP wrapper -- auto-generated by slack-mcp setup
set -euo pipefail

TOKENS="%s"

if [[ ! -f "$TOKENS" ]]; then
  echo "Error: $TOKENS not found." >&2
  echo "Re-run: %s setup --refresh-tokens" >&2
  exit 1
fi

# shellcheck source=/dev/null
source "$TOKENS"

export SLACK_MCP_READ_ONLY=false
export SLACK_XOXC_TOKEN="${SLACK_MCP_XOXC_TOKEN}"
export SLACK_XOXD_TOKEN="${SLACK_MCP_XOXD_TOKEN}"
export MCP_TRANSPORT=stdio
export LOGS_CHANNEL_ID="%s"

exec %s
`, tokensFilePath(cfg.installDir), cfg.binaryPath, logsChannel, cfg.binaryPath)

	path := wrapperScriptPath(cfg.installDir)
	os.WriteFile(path, []byte(content), 0o755)
	fmt.Printf("  Written: %s\n", path)
}

func registerMCPInClaude(cfg setupConfig) {
	fmt.Println()
	fmt.Println("  Registering MCP server in Claude Code settings")
	fmt.Println("  -----------------------------------------------")

	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude.json")

	settings := make(map[string]any)
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &settings)
	}

	servers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[cfg.serverName] = map[string]any{
		"type":    "stdio",
		"command": wrapperScriptPath(cfg.installDir),
		"args":    []string{},
		"env":     map[string]string{},
	}
	settings["mcpServers"] = servers

	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, append(data, '\n'), 0o644)
	fmt.Printf("  Registered '%s' in %s\n", cfg.serverName, settingsPath)
}
