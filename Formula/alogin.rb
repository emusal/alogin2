class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.8"
  license "MIT"

  depends_on "tmux"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.8/alogin-web-darwin-arm64"
      sha256 "c38076a14afe619e06358fda1ccb3f1211a46104cb0c03cef14d3887dfbf1705"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.8/alogin-web-darwin-amd64"
      sha256 "de41eb45730ff59e9305f0c20c6584333bc4b9981b9899b4c53f29b0284b6f9d"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.8/alogin-web-linux-arm64"
      sha256 "0d5c65f808122aac98af6123a165156fb877b1642dea5d5fa0188930810ef685"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.8/alogin-web-linux-amd64"
      sha256 "14ff1be8d41c1b68e2d5472a197b883ced8113595a3a376967ea203ac38a9ff3"
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
    assert_match "alogin v2.0.8", shell_output("#{bin}/alogin version")
  end
end
