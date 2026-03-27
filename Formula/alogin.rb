class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.2.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.2.0/alogin-web-darwin-arm64"
      sha256 "cc405e8f663684b32f20c4cfcd629a5847a26fee7659dc1bd9dfa079f788bfd1"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.2.0/alogin-web-darwin-amd64"
      sha256 "b104ae2d3608e3fe8baffbcef0de58bb56c36083638ab83ef26764e75550f920"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.2.0/alogin-web-linux-arm64"
      sha256 "700770509351e76f0b2b42828a4a1f9c06c646a5d8375d95831fda05ca305167"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.2.0/alogin-web-linux-amd64"
      sha256 "6aee8ff560fb18027a5a7a9441ddc1d36be93892e0a21d0227e1449437b5d065"
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
    assert_match "alogin v2.2.0", shell_output("#{bin}/alogin version")
  end
end
