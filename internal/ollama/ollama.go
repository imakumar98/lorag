package ollama

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/imakumar98/lorag/internal/paths"
)

const (
	SetupHint         = "Run lorag setup."
	NotInstalledMsg   = "Ollama is not installed. Install it with `brew install ollama`."
	DidNotStartMsg    = "Ollama did not start. Open the Ollama app and run `lorag setup` again."
	ReadyMsg          = "Ollama is ready."
	DefaultChatModel  = paths.DefaultChatModel
	DefaultEmbedModel = paths.DefaultEmbedModel
	waitAttempts      = 30
	waitInterval      = time.Second
)

type Deps struct {
	LookPath func(file string) (string, error)
	List     func() (string, error)
	Start    func() error
	Pull     func(name string, stdout io.Writer) error
	Sleep    func(time.Duration)
}

func Ready(deps Deps, models ...string) error {
	if _, err := lookPath(deps)("ollama"); err != nil {
		return errors.New(SetupHint)
	}
	out, err := list(deps)()
	if err != nil {
		return errors.New(SetupHint)
	}
	for _, model := range models {
		if !hasModel(out, model) {
			return errors.New(SetupHint)
		}
	}
	return nil
}

func hasModel(listOutput, name string) bool {
	for _, line := range strings.Split(listOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		installed := fields[0]
		if installed == name || strings.HasPrefix(installed, name+":") {
			return true
		}
	}
	return false
}

func Setup(stdout io.Writer, deps Deps) error {
	if _, err := lookPath(deps)("ollama"); err != nil {
		return errors.New(NotInstalledMsg)
	}
	if _, err := list(deps)(); err != nil {
		if err := start(deps)(); err != nil {
			return err
		}
		if err := waitUntilReady(deps); err != nil {
			return err
		}
	}
	for _, model := range []string{DefaultEmbedModel, DefaultChatModel} {
		if err := pull(deps)(model, stdout); err != nil {
			return fmt.Errorf("Could not pull model %s.", model)
		}
	}
	fmt.Fprintln(stdout, ReadyMsg)
	return nil
}

func waitUntilReady(deps Deps) error {
	sleepFn := deps.Sleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	for i := 0; i < waitAttempts; i++ {
		if _, err := list(deps)(); err == nil {
			return nil
		}
		if i == waitAttempts-1 {
			break
		}
		sleepFn(waitInterval)
	}
	return errors.New(DidNotStartMsg)
}

func lookPath(deps Deps) func(string) (string, error) {
	if deps.LookPath != nil {
		return deps.LookPath
	}
	return exec.LookPath
}

func list(deps Deps) func() (string, error) {
	if deps.List != nil {
		return deps.List
	}
	return defaultList
}

func start(deps Deps) func() error {
	if deps.Start != nil {
		return deps.Start
	}
	return defaultStart
}

func pull(deps Deps) func(string, io.Writer) error {
	if deps.Pull != nil {
		return deps.Pull
	}
	return defaultPull
}

func defaultList() (string, error) {
	cmd := exec.Command("ollama", "list")
	out, err := cmd.Output()
	return string(out), err
}

func defaultPull(name string, stdout io.Writer) error {
	cmd := exec.Command("ollama", "pull", name)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	return cmd.Run()
}
