class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.7"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.7/alogin-web-darwin-arm64"
      sha256 "332b1043cee8859daa56ac0a8eb9f526e99be704f3af190d31995d43993f97f1"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.7/alogin-web-darwin-amd64"
      sha256 "721d29b319f0b2845798299c36650a84d0d4acf32647db8a6017ca46da8195d8"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.7/alogin-web-linux-arm64"
      sha256 "39a77567ddae88bbd4789c07a4253995bbe59521f3a7d20a1ba9d56469de7371"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.7/alogin-web-linux-amd64"
      sha256 "0df43912b40c0525f8ef2f142707f81e77f1f714e76226c867f7b5c83019a279"
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
    assert_match "alogin v2.0.7", shell_output("#{bin}/alogin version")
  end
end
