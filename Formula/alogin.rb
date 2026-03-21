# Homebrew Formula for alogin
#
# This file belongs in a separate tap repository: github.com/<you>/homebrew-alogin
#   homebrew-alogin/
#     Formula/
#       alogin.rb   ← place this file here
#
# After each release, update `version` and the sha256 values from checksums.txt.
#
# Users install with:
#   brew tap <you>/alogin
#   brew install alogin

class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-darwin-arm64"
      sha256 "4dc8c7aca920f3e6da57572af2d86f46df692f271213a671d18aee29cd96b0be"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-darwin-amd64"
      sha256 "25b314f9663cecdd3ba253bfb9bd6420137f5b5bf8e46709eb96e8e2de015489"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-linux-arm64"
      sha256 "6a305a046d54eaac8d9dac6f8c5ab360546f7d388e3a710c4b4a077d8f662e25"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-linux-amd64"
      sha256 "59dfbde0cef098672dd0249af2c6486c2b0c887608b4a9bb62160540c8e38ad6"
    end
  end

  def install
    bin.install Dir["alogin-*"].first => "alogin"
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
