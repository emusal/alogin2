class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.2"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.2/alogin-web-darwin-arm64"
      sha256 "98684ba317f0f2fddd0ac9e05f309f195bf33cc85aabd4f6e23bc0bda0c8ee9b"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.2/alogin-web-darwin-amd64"
      sha256 "bc2cf56040a9dae6a960a5de6a0217383538177282f6d5952d670065a476a1e9"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.2/alogin-web-linux-arm64"
      sha256 "e5b5f33efe2445e805656c0665e4e685d9ad03256e6af1d2e99f1e22d385d50f"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.2/alogin-web-linux-amd64"
      sha256 "c444d68ad05524352aa737398893e9eac53077e8af137bdf7828629720d82bad"
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
    assert_match "alogin v2.0.2", shell_output("#{bin}/alogin version")
  end
end
