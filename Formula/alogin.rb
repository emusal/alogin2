class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.5.3"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-darwin-arm64"
      sha256 "2a6b2b590874dc0339e3a5e3efa53c0849a70cec08272d92f6e4835c18383e29"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-darwin-amd64"
      sha256 "0e029745bf74052a3044379ff61898d05c6a6e091c17a5af624f55e7a4781f28"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-linux-arm64"
      sha256 "f65b3f76c4e24d0a3b2d07c1c32e299c94b2b61b140b96ea1a00f99eabca1964"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.5.3/alogin-web-linux-amd64"
      sha256 "a1f257ce60ebbf47b2d8137254ad7e70c933a34e7e762e5bf6319a1c1725c021"
    end
  end

  resource "plugins" do
    url "https://github.com/emusal/alogin2/releases/download/v2.5.3/plugins.tar.gz"
    sha256 "91d1245c6ee8a519852f4222472c0821aab52dfb3b57d1508ec3800bdd1af7dc"
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
