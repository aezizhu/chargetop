class Chargetop < Formula
  desc "Real-time Battery Monitor TUI for macOS"
  homepage "https://github.com/aezizhu/chargetop"
  url "https://github.com/aezizhu/chargetop/archive/refs/tags/v1.5.1.tar.gz"
  version "1.5.1"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Placeholder, update after release
  license "Apache-2.0"
  head "https://github.com/aezizhu/chargetop.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"chargetop"
  end

  test do
    assert_match "battery-mon", shell_output("#{bin}/battery-mon --help", 1)
  end
end
