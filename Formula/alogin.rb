class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.0/alogin-web-darwin-arm64"
      sha256 "d6b57bf784b241882f320cca1cdeba7dbd1b181e65d44b1681d07a6e78a1f65f"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.0/alogin-web-darwin-amd64"
      sha256 "2fe2a2725271e72f4bfa1c8f992b0957c6358b855c813c5fac41ce8d14338694"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.0/alogin-web-linux-arm64"
      sha256 "c1dd3f6db232e4f164b00fc3afa560c92031bbb033b521383abc6b1401d19e0c"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.0/alogin-web-linux-amd64"
      sha256 "ad28c42572c397239a5e41804d7e2033ccc3a40b45e8646ac8b891fd58bd3a9e"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.0/plugins.tar.gz"
    sha256 "03b9090f09da6cfe9dff863371bbcf4fe7ea228a0c4cf1c06b094fb7f0331b55"
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
    assert_match "alogin v2.5.0", shell_output("#{bin}/alogin version")
  end
end
