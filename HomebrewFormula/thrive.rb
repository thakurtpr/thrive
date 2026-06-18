class Thrive < Formula
  desc "THakur Runtime Isolation Virtualization Engine - Daemonless container runtime"
  homepage "https://github.com/thakurtpr/thrive"
  url "https://github.com/thakurtpr/thrive/archive/refs/tags/v0.1.0.tar.gz"
  version "0.1.0"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"
  head "https://github.com/thakurtpr/thrive.git"

  depends_on "go" => :build

  def install
    ENV["GOTOOLCHAIN"] = "auto"
    ENV["CGO_ENABLED"] = "0"

    system "go", "build", "-o", bin/"thrive", "./cmd/thrive"
  end

  test do
    system "#{bin}/thrive", "--help"
  end
end
