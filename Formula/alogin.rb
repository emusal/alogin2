class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.4"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.4/alogin-web-darwin-arm64"
      sha256 "2593269fc8c5d53006c2aec3ce6b6c0bfe392fc976391ee12b05b4b21810a745"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.4/alogin-web-darwin-amd64"
      sha256 "acf82ccb6e71f0155337a12a433b00d255dd35902d3413da351c3269840dbd94"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.4/alogin-web-linux-arm64"
      sha256 "3da215261e3bbb9e2a0d64fe3b2b85429a9023460d03e5f2fb58c0865f81e495"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.4/alogin-web-linux-amd64"
      sha256 "8a63de40f3af255853b9006caa13bf57338dafa083097437f238025205c98e01"
    end
  end

  def install
    bin.install Dir["alogin-web-*"].first => "alogin"
  end

  def caveats
    <<~EOS
      To set up shell completions, run:
        alogin completion install            # zsh (default)
        alogin completion install --shell bash
    EOS
  end

  test do
    assert_match "alogin v2.0.4", shell_output("#{bin}/alogin version")
  end
end
