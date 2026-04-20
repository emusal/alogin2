class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.1/alogin-web-darwin-arm64"
      sha256 "200789b2d4863d89a53f1eff4bd07b0e33cf3b6763e955be3d6769300d30d07d"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.1/alogin-web-darwin-amd64"
      sha256 "68dd2ea138ebf59801cfca3753eef0c8998da53a7b57d290a84da10d13fd7a64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.1/alogin-web-linux-arm64"
      sha256 "c7ed0af670367359417c91ac79127376d652c8f8792e29545f4bc62a55e933a6"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.1/alogin-web-linux-amd64"
      sha256 "50c0860b1af7afa370e905fa0378e3f30caa1aba47ee45dbe6d1f09b4f80996f"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.1/plugins.tar.gz"
    sha256 "09e65c03124f010629fd08100e91a16d1ecc879b2606671fd450b3b3b5f575b4"
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
    assert_match "alogin v2.5.1", shell_output("#{bin}/alogin version")
  end
end
