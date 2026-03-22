class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.5"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.5/alogin-web-darwin-arm64"
      sha256 "d97503b893bec8a4bf769a4cbf658fdf6858cbbad07ace89861c10770ede3147"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.5/alogin-web-darwin-amd64"
      sha256 "114a6a07c0052418b035d8d5583df8cf3ca0a112ae80bcb06cc7e7bfed5ed5be"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.5/alogin-web-linux-arm64"
      sha256 "50b0743e98bdb0b85f0708741c1f89a9d6a7d064ce3d298c3cfb18b34c8077cc"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.5/alogin-web-linux-amd64"
      sha256 "347794bf0dcb5159d931ab109c556d72ff123f5c951b2161059a7cc8447bbd7f"
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
    assert_match "alogin v2.0.5", shell_output("#{bin}/alogin version")
  end
end
