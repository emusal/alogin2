# Homebrew Formula for alogin
#
# This file is the reference copy. The authoritative version lives in the
# tap repository: github.com/emusal/homebrew-alogin
#
# It is auto-updated by the GitHub Actions release workflow.
# Do NOT edit checksums here manually — they are overwritten on each release.
#
# Users install with:
#   brew tap emusal/alogin
#   brew install alogin
#
# Upgrade:
#   brew upgrade alogin

class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-web-darwin-arm64"
      sha256 "PLACEHOLDER_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-web-darwin-amd64"
      sha256 "PLACEHOLDER_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-web-linux-arm64"
      sha256 "PLACEHOLDER_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-web-linux-amd64"
      sha256 "PLACEHOLDER_LINUX_AMD64"
    end
  end

  def install
    bin.install Dir["alogin-web-*"].first => "alogin"
  end

  def caveats
    <<~EOS
      To set up shell completions, run:
        alogin completion install            # zsh (default)
        alogin completion install --shell bash
    EOS
  end

  test do
    assert_match "alogin v#{version}", shell_output("#{bin}/alogin version")
  end
end
