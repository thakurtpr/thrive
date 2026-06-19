class Thrive < Formula
  desc "THakur Runtime Isolation Virtualization Engine - Daemonless container runtime"
  homepage "https://github.com/thakurtpr/thrive"
  url "https://github.com/thakurtpr/thrive/archive/refs/tags/v0.1.1.tar.gz"
  version "0.1.1"
  sha256 "fd9fbc7b1fd0e7c115f2e3882d9f729f16c53fa406e415b3738bb52954416472"
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
