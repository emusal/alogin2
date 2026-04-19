class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.4.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.1/alogin-web-darwin-arm64"
      sha256 "7890d13ff02827b89a8890231a41542fd428c693594f0fd9e2f1899f78cfb691"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.1/alogin-web-darwin-amd64"
      sha256 "9ac4b50606f230f793dfc8d104185f12df8eb87642213f3272980580f1a5fd15"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.1/alogin-web-linux-arm64"
      sha256 "c859231fa2ed5b629038f69a67b0aa4cf19cdd30323a54c63f28a2332f0f41ab"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.1/alogin-web-linux-amd64"
      sha256 "d667b196c534b77e773c701c7939417116ed2ad5cc0b5d36ed5602a6147de6dd"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.4.1/plugins.tar.gz"
    sha256 "37f6de9a03783b47a5d8a99cd68b12d1cb433f26462694831955a15ccda32eee"
  end

  def install
    bin.install Dir["alogin-web-*"].first => "alogin"

    plugin_dir = etc/"alogin/plugins"
    plugin_dir.mkpath
    resource("plugins").stage do
      (plugin_dir).install Dir["plugins/*.yaml"]
    end
  end

  def caveats
    <<~EOS
      To set up shell completions, run:
        alogin completion install            # zsh (default)
        alogin completion install --shell bash
    EOS
  end

  test do
    assert_match "alogin v2.4.1", shell_output("#{bin}/alogin version")
  end
end
