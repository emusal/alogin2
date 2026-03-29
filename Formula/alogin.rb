class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.3.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.1/alogin-web-darwin-arm64"
      sha256 "2a25a14968d11cb3f8d74cfa83944eb60a6451570b30e0d1ccc077ed70183a89"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.1/alogin-web-darwin-amd64"
      sha256 "112809725d7853e0af0e483208df15aa7231b575dee5113466e5a18683e1f8ac"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.1/alogin-web-linux-arm64"
      sha256 "3fd488b5724ebf5fe8d641097540445286971afd95b97db066202cbdd5139d8b"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.3.1/alogin-web-linux-amd64"
      sha256 "b52bf273baa89b4e5625adec6afc0f2ead4dc4280dc3119be8770ca35a804ac7"
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
    assert_match "alogin v2.3.1", shell_output("#{bin}/alogin version")
  end
end
