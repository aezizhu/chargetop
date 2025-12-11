class BatteryMon < Formula
  desc "Real-time Battery Monitor TUI for macOS"
  homepage "https://github.com/aezizhu/battery-mon"
  url "https://github.com/aezizhu/battery-mon/archive/refs/heads/main.tar.gz"
  version "1.0.0"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000" # Placeholder, update after release
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"battery-mon"
  end

  test do
    assert_match "battery-mon", shell_output("#{bin}/battery-mon --help", 1)
  end
end
