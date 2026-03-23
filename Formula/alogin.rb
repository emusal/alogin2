class Alogin < Formula
  desc "Modern SSH connection manager with encrypted credential vault"
  homepage "https://github.com/emusal/alogin2"
  version "2.0.9"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.9/alogin-web-darwin-arm64"
      sha256 "66b423977fdf1c7a06b9c8dc8a14de3b14ef8ee525ce8e9d25f48bd63e49c7f2"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.9/alogin-web-darwin-amd64"
      sha256 "4f9c21e632921ba41686fc55cca856ffda3988bf7cac636b4fa81b4749919feb"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.9/alogin-web-linux-arm64"
      sha256 "d6a2faef7af3de2e047b990869f8bba189f6f61ff1d7ea4b5005c96ac4c0a01a"
    end
    on_intel do
      url "https://github.com/emusal/alogin2/releases/download/v2.0.9/alogin-web-linux-amd64"
      sha256 "96e627fda3ab84ded594cd3561945d0e8cef4f0a3c2c4608a615675b90b1083b"
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
    assert_match "alogin v2.0.9", shell_output("#{bin}/alogin version")
  end
end
