class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-darwin-arm64"
      sha256 "79713bbabdbf34d9d9c4ec17c1c9cb48eee048b9b705d9ca242f17ec4dab8725"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-darwin-amd64"
      sha256 "de8ee21286c8196a459a7fb074fce8568b00a34874d2d2c74c7cd3e70014ff91"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-linux-arm64"
      sha256 "fd93f19f5023ee44a8c967a21e019f2cd1c759801fa48a426f10107f7bb5e45c"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-linux-amd64"
      sha256 "4c8a76c09a8f49c6c77509e3370d7f303491920603a08b152253a2ea64e0f864"
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
    assert_match "alogin v#{version}", shell_output("#{bin}/alogin version")
  end
end
