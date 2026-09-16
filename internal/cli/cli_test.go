package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/imakumar98/lorag/internal/notes"
	"github.com/imakumar98/lorag/internal/paths"
	"github.com/imakumar98/lorag/internal/rag"
)

func readyOK() error { return nil }

func TestHelpPrintsUsage(t *testing.T) {
	stdout := &bytes.Buffer{}
	result := Main([]string{"-h"}, Options{Stdout: stdout})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	if !strings.Contains(stdout.String(), "usage: lorag") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "setup") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestSyncCreatesLayoutAndRunsExport(t *testing.T) {
	home := t.TempDir()
	var gotNotes, gotDB string
	stdout := &bytes.Buffer{}
	result := Main([]string{"sync"}, Options{
		Home:         home,
		Stdout:       stdout,
		RequireReady: readyOK,
		SyncNotes: func(notesDir, dbDir string) (int, int, error) {
			gotNotes, gotDB = notesDir, dbDir
			return 3, 1, nil
		},
	})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	p := paths.FromHome(home)
	if info, err := os.Stat(p.DocsDir); err != nil || !info.IsDir() {
		t.Fatal("docs dir missing")
	}
	if _, err := os.Stat(p.ConfigPath); err != nil {
		t.Fatal("config missing")
	}
	config, err := paths.LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if config.ChatModel != "llama3.2:3b" {
		t.Fatalf("chat model = %q", config.ChatModel)
	}
	if gotNotes != p.NotesDir() || gotDB != p.DBDir {
		t.Fatalf("sync args %q %q", gotNotes, gotDB)
	}
}

func TestSyncDoesNotOverwriteChatModel(t *testing.T) {
	home := t.TempDir()
	sync := func(string, string) (int, int, error) { return 0, 0, nil }
	Main([]string{"sync"}, Options{Home: home, Stdout: io.Discard, RequireReady: readyOK, SyncNotes: sync})
	p := paths.FromHome(home)
	if err := paths.SetChatModel(p, "qwen3.5:4b"); err != nil {
		t.Fatal(err)
	}
	Main([]string{"sync"}, Options{Home: home, Stdout: io.Discard, RequireReady: readyOK, SyncNotes: sync})
	config, err := paths.LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if config.ChatModel != "qwen3.5:4b" {
		t.Fatalf("chat model = %q", config.ChatModel)
	}
}

func TestSetupPrintsReadyAndCreatesLayout(t *testing.T) {
	home := t.TempDir()
	stdout := &bytes.Buffer{}
	var setupCalled bool
	result := Main([]string{"setup"}, Options{
		Home:   home,
		Stdout: stdout,
		Setup: func(w io.Writer) error {
			setupCalled = true
			fmt.Fprintln(w, "Ollama is ready.")
			return nil
		},
	})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	if !setupCalled {
		t.Fatal("setup was not called")
	}
	if !strings.Contains(stdout.String(), "Ollama is ready.") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	p := paths.FromHome(home)
	if info, err := os.Stat(p.DocsDir); err != nil || !info.IsDir() {
		t.Fatal("docs dir missing")
	}
	if _, err := os.Stat(p.ConfigPath); err != nil {
		t.Fatal("config missing")
	}
}

func TestSetupReportsErrorToStderr(t *testing.T) {
	stderr := &bytes.Buffer{}
	result := Main([]string{"setup"}, Options{
		Home:   t.TempDir(),
		Stderr: stderr,
		Stdout: io.Discard,
		Setup: func(io.Writer) error {
			return errors.New("Ollama is not installed. Install it with `brew install ollama`.")
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if !strings.Contains(stderr.String(), "brew install ollama") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestInitIsNotACommand(t *testing.T) {
	stderr := &bytes.Buffer{}
	result := Main([]string{"init"}, Options{Stderr: stderr, Stdout: io.Discard})
	if result != 2 {
		t.Fatalf("result = %d", result)
	}
}

func TestSyncRequiresSetupWhenOllamaIsNotReady(t *testing.T) {
	stderr := &bytes.Buffer{}
	var synced bool
	result := Main([]string{"sync"}, Options{
		Home:   t.TempDir(),
		Stderr: stderr,
		Stdout: io.Discard,
		RequireReady: func() error {
			return errors.New("Run lorag setup.")
		},
		SyncNotes: func(string, string) (int, int, error) {
			synced = true
			return 0, 0, nil
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if synced {
		t.Fatal("sync ran before Ollama was ready")
	}
	if !strings.Contains(stderr.String(), "Run lorag setup.") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestQRequiresSetupWhenOllamaIsNotReady(t *testing.T) {
	stderr := &bytes.Buffer{}
	var asked bool
	result := Main([]string{"q", "hello"}, Options{
		Home:   t.TempDir(),
		Stderr: stderr,
		Stdout: io.Discard,
		RequireReady: func() error {
			return errors.New("Run lorag setup.")
		},
		Ask: func(string, string, string, string, string) (string, []string, error) {
			asked = true
			return "", nil, nil
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if asked {
		t.Fatal("ask ran before Ollama was ready")
	}
	if !strings.Contains(stderr.String(), "Run lorag setup.") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestModelUseRequiresSetupWhenOllamaIsNotReady(t *testing.T) {
	home := t.TempDir()
	Main([]string{"sync"}, Options{
		Home:         home,
		Stdout:       io.Discard,
		RequireReady: func() error { return nil },
		SyncNotes:    func(string, string) (int, int, error) { return 0, 0, nil },
	})
	stderr := &bytes.Buffer{}
	var pulled bool
	result := Main([]string{"model", "use", "qwen3.5:4b"}, Options{
		Home:   home,
		Stderr: stderr,
		Stdout: io.Discard,
		RequireReady: func() error {
			return errors.New("Run lorag setup.")
		},
		PullModel: func(string) error {
			pulled = true
			return nil
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if pulled {
		t.Fatal("pull ran before Ollama was ready")
	}
	if !strings.Contains(stderr.String(), "Run lorag setup.") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSyncReportsNotesErrorToStderr(t *testing.T) {
	stderr := &bytes.Buffer{}
	result := Main([]string{"sync"}, Options{
		Home:         t.TempDir(),
		Stderr:       stderr,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		SyncNotes: func(string, string) (int, int, error) {
			return 0, 0, &notes.ExportError{Msg: "Apple Notes sync is macOS-only."}
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if !strings.Contains(stderr.String(), "Apple Notes sync is macOS-only.") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestQJoinsRemainingArgs(t *testing.T) {
	home := t.TempDir()
	p := paths.FromHome(home)
	if err := paths.EnsureLayout(p); err != nil {
		t.Fatal(err)
	}
	if err := paths.WriteDefaultConfig(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p.DBDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stdout := &bytes.Buffer{}
	var gotQuery string
	result := Main([]string{"q", "What", "is", "ACATS?"}, Options{
		Home:         home,
		Stdout:       stdout,
		RequireReady: readyOK,
		Ask: func(query, docsDir, dbDir, embedModel, chatModel string) (string, []string, error) {
			gotQuery = query
			return "Fee is waived", []string{"/tmp/a.txt"}, nil
		},
	})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	if gotQuery != "What is ACATS?" {
		t.Fatalf("query = %q", gotQuery)
	}
	if !strings.Contains(stdout.String(), "Fee is waived") || !strings.Contains(stdout.String(), "/tmp/a.txt") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestQWithoutIndexPrintsHelperError(t *testing.T) {
	stderr := &bytes.Buffer{}
	result := Main([]string{"q", "hello"}, Options{
		Home:         t.TempDir(),
		Stderr:       stderr,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		Ask: func(string, string, string, string, string) (string, []string, error) {
			return "", nil, &rag.QuestionError{Msg: "No index found. Run `lorag sync`."}
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	if !strings.Contains(stderr.String(), "lorag sync") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestQWithoutQueryIsAnError(t *testing.T) {
	result := Main([]string{"q"}, Options{Stderr: io.Discard, Stdout: io.Discard})
	if result != 2 {
		t.Fatalf("result = %d", result)
	}
}

func TestModelPrintsCurrentChatModel(t *testing.T) {
	home := t.TempDir()
	Main([]string{"sync"}, Options{
		Home:         home,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		SyncNotes:    func(string, string) (int, int, error) { return 0, 0, nil },
	})
	stdout := &bytes.Buffer{}
	result := Main([]string{"model"}, Options{Home: home, Stdout: stdout})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	if !strings.Contains(stdout.String(), "llama3.2:3b") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestModelUseSavesNameAfterSuccessfulPull(t *testing.T) {
	home := t.TempDir()
	Main([]string{"sync"}, Options{
		Home:         home,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		SyncNotes:    func(string, string) (int, int, error) { return 0, 0, nil },
	})
	var pulled string
	result := Main([]string{"model", "use", "qwen3.5:4b"}, Options{
		Home:         home,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		PullModel: func(name string) error {
			pulled = name
			return nil
		},
	})
	if result != 0 {
		t.Fatalf("result = %d", result)
	}
	if pulled != "qwen3.5:4b" {
		t.Fatalf("pulled = %q", pulled)
	}
	config, err := paths.LoadConfig(paths.FromHome(home))
	if err != nil {
		t.Fatal(err)
	}
	if config.ChatModel != "qwen3.5:4b" {
		t.Fatalf("chat model = %q", config.ChatModel)
	}
}

func TestModelUseDoesNotSaveWhenPullFails(t *testing.T) {
	home := t.TempDir()
	Main([]string{"sync"}, Options{
		Home:         home,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		SyncNotes:    func(string, string) (int, int, error) { return 0, 0, nil },
	})
	stderr := &bytes.Buffer{}
	result := Main([]string{"model", "use", "missing:model"}, Options{
		Home:         home,
		Stderr:       stderr,
		Stdout:       io.Discard,
		RequireReady: readyOK,
		PullModel: func(string) error {
			return &ModelPullError{Msg: "pull failed"}
		},
	})
	if result != 1 {
		t.Fatalf("result = %d", result)
	}
	config, err := paths.LoadConfig(paths.FromHome(home))
	if err != nil {
		t.Fatal(err)
	}
	if config.ChatModel != "llama3.2:3b" {
		t.Fatalf("chat model = %q", config.ChatModel)
	}
}
