class Thrive < Formula
  desc "THakur Runtime Isolation Virtualization Engine - Daemonless container runtime"
  homepage "https://github.com/thakurprasadrout/thrive"
  url "https://github.com/thakurprasadrout/thrive.git"
  version "0.1.0"
  license "MIT"
  head "https://github.com/thakurprasadrout/thrive.git"

  depends_on "go" => :build
  depends_on "gcc" => :build
  depends_on "fuse" => :optional
  depends_on "libseccomp" => :optional

  def install
    ENV["GOTOOLCHAIN"] = "auto"
    ENV["GOOS"] = "linux"
    ENV["CGO_ENABLED"] = "1"

    system "go", "build", "-o", bin/"thrive", "./cmd/thrive"
  end

  test do
    system "#{bin}/thrive", "--version"
  end
end