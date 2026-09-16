package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/imakumar98/lorag/internal/notes"
	"github.com/imakumar98/lorag/internal/ollama"
	"github.com/imakumar98/lorag/internal/paths"
	"github.com/imakumar98/lorag/internal/rag"
)

const usageText = `usage: lorag [-h] {setup,sync,q,model} ...

Ask questions over local documents and Apple Notes.

positional arguments:
  {setup,sync,q,model}

options:
  -h, --help  show this help message and exit
`

type ModelPullError struct {
	Msg string
}

func (e *ModelPullError) Error() string {
	return e.Msg
}

type Options struct {
	Home         string
	Stdout       io.Writer
	Stderr       io.Writer
	SyncNotes    func(notesDir, dbDir string) (int, int, error)
	PullModel    func(name string) error
	Ask          func(query, docsDir, dbDir, embedModel, chatModel string) (string, []string, error)
	Setup        func(stdout io.Writer) error
	RequireReady func() error
}

func Main(args []string, opts Options) int {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}

	home := opts.Home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	}
	p := paths.FromHome(home)

	switch args[0] {
	case "setup":
		if len(args) != 1 {
			fmt.Fprint(stderr, usageText)
			return 2
		}
		return cmdSetup(p, stdout, stderr, opts)
	case "sync":
		if len(args) != 1 {
			fmt.Fprint(stderr, usageText)
			return 2
		}
		return cmdSync(p, stdout, stderr, opts)
	case "q":
		if len(args) < 2 {
			fmt.Fprint(stderr, usageText)
			return 2
		}
		return cmdQuestion(p, strings.Join(args[1:], " "), stdout, stderr, opts)
	case "model":
		return cmdModel(p, args[1:], stdout, stderr, opts)
	default:
		fmt.Fprint(stderr, usageText)
		return 2
	}
}

func cmdSetup(p paths.Paths, stdout, stderr io.Writer, opts Options) int {
	if err := paths.EnsureLayout(p); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := paths.WriteDefaultConfig(p); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	setup := opts.Setup
	if setup == nil {
		setup = defaultSetup
	}
	if err := setup(stdout); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	return 0
}

func defaultSetup(w io.Writer) error {
	return ollama.Setup(w, ollama.Deps{})
}

func cmdSync(p paths.Paths, stdout, stderr io.Writer, opts Options) int {
	if err := paths.EnsureLayout(p); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := paths.WriteDefaultConfig(p); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	config, err := paths.LoadConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := requireReady(opts, config.EmbedModel); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	syncFn := opts.SyncNotes
	if syncFn == nil {
		syncFn = defaultSync(p)
	}
	exported, skipped, err := syncFn(p.NotesDir(), p.DBDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	fmt.Fprintf(stdout, "Exported %d notes; skipped %d; index rebuilt.\n", exported, skipped)
	return 0
}

func defaultSync(p paths.Paths) func(notesDir, dbDir string) (int, int, error) {
	return func(notesDir, dbDir string) (int, int, error) {
		config, err := paths.LoadConfig(p)
		if err != nil {
			return 0, 0, err
		}
		return notes.Sync(notesDir, dbDir, func() ([]notes.AppleNote, int, error) {
			return notes.FetchNotes(runtime.GOOS, runCommand)
		}, func(docsDir, stagingDBDir string) error {
			return rag.Rebuild(docsDir, stagingDBDir, config.EmbedModel, nil)
		})
	}
}

func cmdQuestion(p paths.Paths, query string, stdout, stderr io.Writer, opts Options) int {
	config, err := paths.LoadConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := requireReady(opts, config.EmbedModel, config.ChatModel); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	ask := opts.Ask
	if ask == nil {
		ask = func(query, docsDir, dbDir, embedModel, chatModel string) (string, []string, error) {
			return rag.AnswerQuestion(query, docsDir, dbDir, embedModel, chatModel, rag.AskDeps{})
		}
	}
	answer, sources, err := ask(query, p.DocsDir, p.DBDir, config.EmbedModel, config.ChatModel)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	fmt.Fprintln(stdout, answer)
	if len(sources) > 0 {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Sources:")
		for _, source := range sources {
			fmt.Fprintf(stdout, "- %s\n", source)
		}
	}
	return 0
}

func cmdModel(p paths.Paths, args []string, stdout, stderr io.Writer, opts Options) int {
	if len(args) == 0 {
		config, err := paths.LoadConfig(p)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, config.ChatModel)
		return 0
	}
	if args[0] != "use" || len(args) != 2 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	return cmdModelUse(p, args[1], stdout, stderr, opts)
}

func cmdModelUse(p paths.Paths, name string, stdout, stderr io.Writer, opts Options) int {
	if err := requireReady(opts); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	pull := opts.PullModel
	if pull == nil {
		pull = PullOllamaModel
	}
	if err := pull(name); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err.Error())
		return 1
	}
	if err := paths.SetChatModel(p, name); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, name)
	return 0
}

func requireReady(opts Options, models ...string) error {
	if opts.RequireReady != nil {
		return opts.RequireReady()
	}
	return ollama.Ready(ollama.Deps{}, models...)
}

func PullOllamaModel(name string) error {
	cmd := exec.Command("ollama", "pull", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || isNotFound(err) {
			return &ModelPullError{Msg: "Ollama is not installed. Install it with `brew install ollama`."}
		}
		return &ModelPullError{Msg: fmt.Sprintf("Could not pull model %s.", name)}
	}
	return nil
}

func isNotFound(err error) bool {
	var pathErr *os.PathError
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return true
	}
	return errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist)
}

func runCommand(name string, args []string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
