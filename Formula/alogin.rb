class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.4.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.0/alogin-web-darwin-arm64"
      sha256 "4feb1cfb1263535f5f59d75a6f82430560cb7a3a6f2153d590d1458d03e1413e"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.0/alogin-web-darwin-amd64"
      sha256 "4466b3e0c7212d8c0f43472ec37ebd89e531ecfdc8b3408a7282ec2bae616eb3"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.0/alogin-web-linux-arm64"
      sha256 "da62e9bac5139fc0029903d99189e4c6a5ab998b1335301f50fb2234a8cd0f42"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.0/alogin-web-linux-amd64"
      sha256 "6c31f6d2d22266578e5651c55eaa861dfb6f0c68c52776c616bfe6142c335de0"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.4.0/plugins.tar.gz"
    sha256 "d3565231ac34fe0d50ec21e8f828500656dc5ca65592a2bcce817caa7dcc2d9c"
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
    assert_match "alogin v2.4.0", shell_output("#{bin}/alogin version")
  end
end
