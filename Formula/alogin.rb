class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.3.2"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.2/alogin-web-darwin-arm64"
      sha256 "e15a03aa20c2f20053fae3bead9817fdb986c414b6630913af6719ba386ed995"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.2/alogin-web-darwin-amd64"
      sha256 "8488cb76615fd1809d9de0a22c8d577f786351f0b0896f220f677cd47ec390ac"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.2/alogin-web-linux-arm64"
      sha256 "9c27a68e455a0a91bc2c5610a3f43326e5e3644fc113fb70ffdb5b6fb0a9b764"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.2/alogin-web-linux-amd64"
      sha256 "927739983fd70b5cf8c1fcad2cb3e4f7d94c72effa47a733b07f235d28a67c73"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.3.2/plugins.tar.gz"
    sha256 "17d7864a3bcee73ab0e27530f751e208a679d2bf2ffefdfbe2e72676f63937a8"
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
    assert_match "alogin v2.3.2", shell_output("#{bin}/alogin version")
  end
end
