class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.4.3"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.3/alogin-web-darwin-arm64"
      sha256 "e9a66a73c4e618b9f1b919025fe2878ffe864e84612b2594903333efa6c1fe43"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.3/alogin-web-darwin-amd64"
      sha256 "07dc8a51eb2cd0633844fc3d481d1b785304666ee858a6fe164bc07079983d25"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.3/alogin-web-linux-arm64"
      sha256 "9b86e7d718f8680c889be047637fd21c01faf8b4b0c8cd9803f8110cdf4385c0"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.3/alogin-web-linux-amd64"
      sha256 "abc27f597b2511f481d5e52449e55bff1acb54f8343954bde9af8c717e078f75"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.4.3/plugins.tar.gz"
    sha256 "01581536660e57caead2c6a958e5bea9f75a6eac5417d4572d27058251342cfc"
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
    assert_match "alogin v2.4.3", shell_output("#{bin}/alogin version")
  end
end
