class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.3.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.0/alogin-web-darwin-arm64"
      sha256 "ae5f2af2e697066168a5862deb348ec0f2d0151f7d7d8caf569d689574b46922"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.0/alogin-web-darwin-amd64"
      sha256 "947b015938c7b09a1c8e71bc3ad6682dd692395af583ffe3b0f2d37762691fa0"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.0/alogin-web-linux-arm64"
      sha256 "898c1b16a2ccc42ef54d3d7ac386a2195ad128033626a4127119a92a0c41f7d7"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.0/alogin-web-linux-amd64"
      sha256 "13786e8932dc2d67cd3a268c00d4ead69496b808cd736a2bf5f40769710d06b1"
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
    assert_match "alogin v2.3.0", shell_output("#{bin}/alogin version")
  end
end
