class EinoCli < Formula
  desc "AI Agent TUI with tools, MCP and ACP support, built on Eino"
  homepage "https://github.com/tk103331/eino-cli"
  version "__VERSION__"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino-darwin-arm64"
    sha256 "__SHA_EINO_DARWIN_ARM64__"
  elsif OS.mac?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino-darwin-amd64"
    sha256 "__SHA_EINO_DARWIN_AMD64__"
  elsif Hardware::CPU.arm?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino-linux-arm64"
    sha256 "__SHA_EINO_LINUX_ARM64__"
  else
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino-linux-amd64"
    sha256 "__SHA_EINO_LINUX_AMD64__"
  end

  def install
    bin.install Dir["eino-*"].first => "eino"
    bin.install_symlink "eino" => "eino-cli"
  end

  test do
    system "#{bin}/eino", "--version"
  end
end
