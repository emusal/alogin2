class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.3"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-darwin-arm64"
      sha256 "e7c2b9647673c15a5a4ee1bf95d655e0ac3517d24b4c44ceb670920a67115ded"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-darwin-amd64"
      sha256 "946e3039bfe6aa292ec88c8a7bf71c45fb5f21255592555eb3898ea10b8ff7ce"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-linux-arm64"
      sha256 "a31694b8936969aa17ddac80cc7d64df3513032be68cb0be40992d9921c86400"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-linux-amd64"
      sha256 "d2c7ca969cd97e2333033bba70278c9a57f75218a1d16ebf7e3526206cf959d3"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.3/plugins.tar.gz"
    sha256 "357718eba23c31239a11e0cf5ba569db0dad589a4c958ba71fc9ade59e9d5c79"
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
    assert_match "alogin v2.5.3", shell_output("#{bin}/alogin version")
  end
end
