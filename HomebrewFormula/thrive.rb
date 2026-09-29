class Thrive < Formula
  desc "THakur Runtime Isolation Virtualization Engine - Daemonless container runtime"
  homepage "https://github.com/thakurtpr/thrive"
  url "https://github.com/thakurtpr/thrive/archive/refs/tags/v0.4.0.tar.gz"
  version "0.4.0"
  sha256 "c724f8e3d4dbc1dc27f97ddb686413ea509bf1a240d71d8fb055dd322827262d"
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
