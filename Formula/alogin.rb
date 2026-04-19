class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.4.2"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.2/alogin-web-darwin-arm64"
      sha256 "64825bff21b73590549e4cb5ff70992fdfc5da8f9b6917c922857d7bcdce715a"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.2/alogin-web-darwin-amd64"
      sha256 "c1fddf440e7c41ca47260b50e9084e74acde0052a1907b411138efdc9ee62098"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.2/alogin-web-linux-arm64"
      sha256 "ae2f5489c1e91f37263bd2ef961c854a051083bcac38de37f472deb1a02d020e"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.4.2/alogin-web-linux-amd64"
      sha256 "65b0bc8cb466648b45795636b982c58545f2601084aece56be623222852481e9"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.4.2/plugins.tar.gz"
    sha256 "009f4e89522c7484426c5acfed239d418d78ad3d34bce1c56cb0672ad9cd260c"
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
    assert_match "alogin v2.4.2", shell_output("#{bin}/alogin version")
  end
end
