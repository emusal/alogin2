class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.1.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.1/alogin-web-darwin-arm64"
      sha256 "ad79fadd90a89dc3c6ef215078f685cec337b386965ea73b709119a1307a56fe"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.1/alogin-web-darwin-amd64"
      sha256 "a0ab98ac2b8f354b5c43066aff28b08fb89b70b015be8fdbda078188ceb8caf5"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.1/alogin-web-linux-arm64"
      sha256 "0e63094256a18de2f88f377a070bad667a82d353a2644a591133312847469f69"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.1.1/alogin-web-linux-amd64"
      sha256 "0e572f0cd404c8912f6e19cd7461214833db2ac6dfebed239af53a30e06aa36c"
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
    assert_match "alogin v2.1.1", shell_output("#{bin}/alogin version")
  end
end
