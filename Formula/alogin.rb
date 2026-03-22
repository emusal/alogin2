class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.6"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.6/alogin-web-darwin-arm64"
      sha256 "dc379cae05230139991566212bda366bf5ca3a0156033e56a70ed8e37fe822d9"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.6/alogin-web-darwin-amd64"
      sha256 "e133120fee4ced11646024ae040c2efa426dee84742e2de50d560b72d79e65f4"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.6/alogin-web-linux-arm64"
      sha256 "4eda8939e6fb5825b4071d8cc746f08aa3d5274aba712a8c6d91378a59a18d89"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.6/alogin-web-linux-amd64"
      sha256 "4bb1957f29eb4d9d03e648f14298c228070ffb5a077dc2b81f68e146390f7de1"
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
    assert_match "alogin v2.0.6", shell_output("#{bin}/alogin version")
  end
end
