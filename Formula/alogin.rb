class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-darwin-arm64"
      sha256 "a2175d9e9035cbc5eb9db2465c45204c0653f0aceff3d69fd71eed6d676cdf8d"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-darwin-amd64"
      sha256 "8ca51d3c93c1c76b2143b990ca4025b5837a6cb8fa0792ce89eb219c8a3af4b3"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-linux-arm64"
      sha256 "f74bb4934cbcc572e3934bcb58442396d793c513d8528bccc55abce7ee4f35b7"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.1/alogin-web-linux-amd64"
      sha256 "7af9b895ad876ee0405e572d0c0bf447d3139f39a9a4e18a9d3d000f6950faee"
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
