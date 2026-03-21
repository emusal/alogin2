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
      sha256 "REPLACE_WITH_CHECKSUM_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-darwin-amd64"
      sha256 "REPLACE_WITH_CHECKSUM_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-linux-arm64"
      sha256 "REPLACE_WITH_CHECKSUM_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v#{version}/alogin-linux-amd64"
      sha256 "REPLACE_WITH_CHECKSUM_LINUX_AMD64"
    end
  end

  def install
    # Install the downloaded binary as 'alogin'
    bin.install Dir["alogin-*"].first => "alogin"

    # Generate and install shell completions
    generate_completions_from_executable(bin/"alogin", "completion")
  end

  test do
    assert_match "alogin v#{version}", shell_output("#{bin}/alogin version")
  end
end
