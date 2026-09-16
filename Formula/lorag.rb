class Lorag < Formula
  desc "Ask questions over local documents and Apple Notes"
  homepage "https://github.com/imakumar98/lorag"
  url "https://github.com/imakumar98/lorag/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "da4ee569764198cbe6ef2642cb579102fe63d595d76d0f7e05bc562779d00eb9"
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
