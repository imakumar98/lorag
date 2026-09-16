# LocalRag(lorag)

Ask questions about files on your Mac and your Apple Notes. Answers stay local: a Go CLI talks to Ollama, the index lives on disk, and nothing is sent to a cloud API.

Drop `.txt`, `.md`, and `.pdf` files into `~/lorag/docs`, then:

```text
lorag q "What did I write about the Q3 plan?"
```

macOS only.

## Install

```bash
brew install imakumar98/lorag/lorag
lorag setup
lorag sync
```

`lorag setup` starts Ollama, keeps it running at login, and downloads the default models (`llama3.2:3b` and `nomic-embed-text`). That can take a few minutes.

`lorag sync` creates `~/lorag/docs`, exports Apple Notes, and builds the index. macOS may ask for Notes permission; allow it, then run `lorag sync` again.

If `sync` or `q` says `Run lorag setup.`, run setup again.

```bash
brew upgrade lorag
```

## Usage

```bash
lorag setup                               # start Ollama and pull default models
lorag sync                                # export Notes and rebuild the index
lorag q "What is the ACATS fee?"          # quote the question
lorag model                               # show the current chat model
lorag model use qwen3.5:4b                # pull a model and use it for answers
```

Add your own files later:

```bash
cp notes.md ~/lorag/docs/
lorag sync
lorag q "Summarize notes.md"
```

Quote the question. In zsh, `?` and `*` are globs, so `lorag q what is acats?` fails before lorag runs.

`lorag q` needs an index. Run `lorag sync` first.

## Where files live

| Path | Role |
|------|------|
| `~/lorag/docs` | Your documents (`.txt`, `.md`, `.pdf`) |
| `~/lorag/docs/apple-notes/` | Exported Apple Notes |
| `~/lorag/database` | Vector index |
| `~/.lorag/config.toml` | Chat and embedding model names |

Your own files under `~/lorag/docs` are kept. The `apple-notes/` folder is replaced on each successful Notes export. Locked notes and attachments are skipped.

## Build from source

```bash
go build -o lorag ./cmd/lorag
./lorag setup
./lorag sync
```

Requires Go 1.24+ and Ollama.

## Uninstall

```bash
brew uninstall lorag
rm -rf ~/lorag ~/.lorag
```
