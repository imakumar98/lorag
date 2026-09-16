class Lorag < Formula
  desc "Ask questions over local documents and Apple Notes"
  homepage "https://github.com/imakumar98/lorag"
  url "https://github.com/imakumar98/lorag/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "7d436a7c8deaad43ee4855e71fec03aa33055eb5c2904b06d612f60fe73269e5"
  license :cannot_represent
  head "https://github.com/imakumar98/lorag.git", branch: "main"

  depends_on "go" => :build
  depends_on "ollama"
  depends_on :macos

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/lorag"
  end

  def caveats
    <<~EOS
      Start Ollama and download the default models:

        lorag setup

      Then export Apple Notes and build the index:

        lorag sync

      macOS may ask for Notes permission; allow it, then run lorag sync again.
    EOS
  end

  test do
    assert_match "usage: lorag", shell_output("#{bin}/lorag -h")
  end
end
