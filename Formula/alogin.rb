class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.3"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.3/alogin-web-darwin-arm64"
      sha256 "a813f9bb9c2d7334618ebc1e180588a06c78081d912a434c960dfd71ca467a7c"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.3/alogin-web-darwin-amd64"
      sha256 "4281421c43d3b30ef32dfa3ffa7891ee7c5bd19f85bab5c231c0b0174be882f4"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.3/alogin-web-linux-arm64"
      sha256 "88373e9b9d5ba0756281d3f84df600e9f181de438d9c61874c3c94ca3c9d5645"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.3/alogin-web-linux-amd64"
      sha256 "65371b238ce41231a21c2747a2495aacbc519aec7f0fb0a0592a069002469db4"
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
    assert_match "alogin v2.0.3", shell_output("#{bin}/alogin version")
  end
end
