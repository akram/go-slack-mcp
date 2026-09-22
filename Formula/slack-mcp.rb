class SlackMcp < Formula
  desc "Slack MCP server - Model Context Protocol connector for Slack"
  homepage "https://github.com/akram/go-slack-mcp"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/akram/go-slack-mcp/releases/download/v0.1.0/slack-mcp-darwin-arm64.tar.gz"
      sha256 "placeholder"
    else
      url "https://github.com/akram/go-slack-mcp/releases/download/v0.1.0/slack-mcp-darwin-amd64.tar.gz"
      sha256 "placeholder"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/akram/go-slack-mcp/releases/download/v0.1.0/slack-mcp-linux-arm64.tar.gz"
      sha256 "placeholder"
    else
      url "https://github.com/akram/go-slack-mcp/releases/download/v0.1.0/slack-mcp-linux-amd64.tar.gz"
      sha256 "placeholder"
    end
  end

  def install
    bin.install Dir["slack-mcp-*"].first => "slack-mcp"
  end

  test do
    assert_match "SLACK_XOXC_TOKEN", shell_output("#{bin}/slack-mcp 2>&1", 1)
  end
end
