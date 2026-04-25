class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.4"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.4/alogin-web-darwin-arm64"
      sha256 "6bcbfdea7d86f259b133db3c793fca45cbb8d07325a0102bd032bb818587a551"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.4/alogin-web-darwin-amd64"
      sha256 "bd17ecd91fb14fe0a99e616e780e5882717c1f41f7ae9fc66274f508ecf5a46c"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.4/alogin-web-linux-arm64"
      sha256 "e6475aacf62c37a8ed5446789c15b5f57e9f895a817eeaa908d1fa408e68f8dc"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.4/alogin-web-linux-amd64"
      sha256 "609eef535fabc748629cf30b2e8ef0e8e2da467715e17773d414569ba3671c30"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.4/plugins.tar.gz"
    sha256 "e000752d5bd9d3024cd2fc3968567fa254cda828a756c9d7ac067317d6d125d8"
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
    assert_match "alogin v2.5.4", shell_output("#{bin}/alogin version")
  end
end
