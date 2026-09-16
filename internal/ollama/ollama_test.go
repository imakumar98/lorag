package ollama

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestReadyFailsWhenOllamaIsMissing(t *testing.T) {
	err := Ready(Deps{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	})
	if err == nil || err.Error() != SetupHint {
		t.Fatalf("error = %v", err)
	}
}

func TestReadyFailsWhenServerIsDown(t *testing.T) {
	err := Ready(Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "", errors.New("connection refused") },
	})
	if err == nil || err.Error() != SetupHint {
		t.Fatalf("error = %v", err)
	}
}

func TestReadySucceedsWhenServerResponds(t *testing.T) {
	err := Ready(Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "NAME\nllama3.2:3b\n", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadyFailsWhenRequiredModelIsMissing(t *testing.T) {
	err := Ready(Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "NAME\nllama3.2:3b\n", nil },
	}, "nomic-embed-text")
	if err == nil || err.Error() != SetupHint {
		t.Fatalf("error = %v", err)
	}
}

func TestSetupFailsWhenOllamaIsMissing(t *testing.T) {
	err := Setup(io.Discard, Deps{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	})
	if err == nil || err.Error() != NotInstalledMsg {
		t.Fatalf("error = %v", err)
	}
}

func TestSetupStartsServerThenPullsDefaultModels(t *testing.T) {
	var started bool
	var pulled []string
	listCalls := 0
	stdout := &bytes.Buffer{}
	err := Setup(stdout, Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List: func() (string, error) {
			listCalls++
			if !started {
				return "", errors.New("not running")
			}
			return "NAME\n", nil
		},
		Start: func() error {
			started = true
			return nil
		},
		Pull: func(name string, w io.Writer) error {
			pulled = append(pulled, name)
			return nil
		},
		Sleep: func(time.Duration) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("did not start Ollama")
	}
	if listCalls < 2 {
		t.Fatalf("listCalls = %d", listCalls)
	}
	if len(pulled) != 2 || pulled[0] != DefaultEmbedModel || pulled[1] != DefaultChatModel {
		t.Fatalf("pulled = %#v", pulled)
	}
	if !strings.Contains(stdout.String(), ReadyMsg) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestSetupSkipsStartWhenAlreadyRunning(t *testing.T) {
	var started bool
	err := Setup(io.Discard, Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "NAME\nllama3.2:3b\n", nil },
		Start: func() error {
			started = true
			return nil
		},
		Pull: func(string, io.Writer) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if started {
		t.Fatal("started Ollama even though it was running")
	}
}

func TestSetupWaitsForServerAfterStart(t *testing.T) {
	var sleeps int
	listCalls := 0
	err := Setup(io.Discard, Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List: func() (string, error) {
			listCalls++
			if listCalls < 3 {
				return "", errors.New("not running")
			}
			return "NAME\n", nil
		},
		Start: func() error { return nil },
		Pull:  func(string, io.Writer) error { return nil },
		Sleep: func(time.Duration) { sleeps++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	if sleeps != 1 {
		t.Fatalf("sleeps = %d", sleeps)
	}
}

func TestSetupFailsWhenServerDoesNotComeUp(t *testing.T) {
	err := Setup(io.Discard, Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "", errors.New("not running") },
		Start:    func() error { return nil },
		Sleep:    func(time.Duration) {},
	})
	if err == nil || err.Error() != DidNotStartMsg {
		t.Fatalf("error = %v", err)
	}
}

func TestSetupReportsPullFailure(t *testing.T) {
	err := Setup(io.Discard, Deps{
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
		List:     func() (string, error) { return "NAME\n", nil },
		Pull: func(name string, w io.Writer) error {
			return errors.New("network")
		},
	})
	if err == nil || err.Error() != "Could not pull model nomic-embed-text." {
		t.Fatalf("error = %v", err)
	}
}
