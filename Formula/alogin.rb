class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.0/alogin-web-darwin-arm64"
      sha256 "1267d463b8354f094b35c157d080f459900830f858939df362eef1608ce1630b"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.0/alogin-web-darwin-amd64"
      sha256 "c7c908f7ed73d633e838dbb3f3a3d0d3f9ae219016a2a2ebe54c975ab496b912"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.0/alogin-web-linux-arm64"
      sha256 "c3ca37c2e7c11143fa94808599cfe0c7260b9929e1ed476668662a745558a3a8"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.0/alogin-web-linux-amd64"
      sha256 "b2f37fbba2a3ad67c93552bb9348fdfb68f21aedcc417e5e3dafa56a84f801c7"
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
    assert_match "alogin v2.1.0", shell_output("#{bin}/alogin version")
  end
end
