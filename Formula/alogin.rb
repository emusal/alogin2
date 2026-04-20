class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.2"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.2/alogin-web-darwin-arm64"
      sha256 "8bb0c6dfe464b43eeedc2b5f92a222038b23bd7e503eac97691d2cd3f11038f2"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.2/alogin-web-darwin-amd64"
      sha256 "8ff0997a143963c8b057f80163cb57e8370dd4f40dae924a1a0503c29222358d"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.2/alogin-web-linux-arm64"
      sha256 "9f00d3274ab303d29809f9888b6c6dabb3315590de3728aef3e32a9b4220466f"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.2/alogin-web-linux-amd64"
      sha256 "7d99a7dc1b7260300510d49472e122e5dec87c4e28fb24f44cbb5f669e7bded7"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.2/plugins.tar.gz"
    sha256 "53c5056871e922325e3b708929169a2e568175b2aeebeeab29965d28a253b731"
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
    assert_match "alogin v2.5.2", shell_output("#{bin}/alogin version")
  end
end
