package ollama

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStartEnablesBrewLaunchAgent(t *testing.T) {
	prefix := t.TempDir()
	plist := filepath.Join(prefix, "sh.brew.ollama.plist")
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	var cmds []string
	err := startWith(startConfig{
		brewPrefix: func() (string, error) { return prefix, nil },
		home:       home,
		launchctl: func(args ...string) error {
			cmds = append(cmds, strings.Join(args, " "))
			return nil
		},
		spawnServe: func() error {
			t.Fatal("should not spawn serve")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(home, "Library", "LaunchAgents", "sh.brew.ollama.plist")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "<plist/>" {
		t.Fatalf("plist = %q", got)
	}
	uid := strconv.Itoa(os.Getuid())
	if len(cmds) != 2 || !strings.Contains(cmds[0], "bootout gui/"+uid) || !strings.Contains(cmds[1], "bootstrap gui/"+uid) {
		t.Fatalf("cmds = %#v", cmds)
	}
}

func TestStartFallsBackToServeWhenNoPlist(t *testing.T) {
	var spawned bool
	err := startWith(startConfig{
		brewPrefix: func() (string, error) { return "", errors.New("no brew") },
		spawnServe: func() error {
			spawned = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !spawned {
		t.Fatal("did not fall back to ollama serve")
	}
}
