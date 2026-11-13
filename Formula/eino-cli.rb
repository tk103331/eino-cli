class EinoCli < Formula
  desc "AI Agent TUI with tools, MCP and ACP support, built on Eino"
  homepage "https://github.com/tk103331/eino-cli"
  version "__VERSION__"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino_#{version}_darwin_arm64.tar.gz"
    sha256 "__SHA_EINO_DARWIN_ARM64__"
  elsif OS.mac?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino_#{version}_darwin_amd64.tar.gz"
    sha256 "__SHA_EINO_DARWIN_AMD64__"
  elsif Hardware::CPU.arm?
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino_#{version}_linux_arm64.tar.gz"
    sha256 "__SHA_EINO_LINUX_ARM64__"
  else
    url "https://github.com/tk103331/eino-cli/releases/download/v#{version}/eino_#{version}_linux_amd64.tar.gz"
    sha256 "__SHA_EINO_LINUX_AMD64__"
  end

  def install
    bin.install "eino"
    bin.install_symlink "eino" => "eino-cli"
  end

  test do
    system "#{bin}/eino", "--version"
  end
end
