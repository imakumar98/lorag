package ollama

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type startConfig struct {
	brewPrefix func() (string, error)
	home       string
	launchctl  func(args ...string) error
	spawnServe func() error
}

func defaultStart() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return startWith(startConfig{
		brewPrefix: brewPrefixOllama,
		home:       home,
		launchctl:  runLaunchctl,
		spawnServe: spawnServe,
	})
}

func startWith(cfg startConfig) error {
	prefix, err := cfg.brewPrefix()
	if err == nil {
		matches, globErr := filepath.Glob(filepath.Join(prefix, "*.plist"))
		if globErr == nil && len(matches) > 0 {
			if err := enableLaunchAgent(matches[0], cfg.home, cfg.launchctl); err == nil {
				return nil
			}
		}
	}
	return cfg.spawnServe()
}

func brewPrefixOllama() (string, error) {
	out, err := exec.Command("brew", "--prefix", "ollama").Output()
	if err != nil {
		return "", err
	}
	return string(trimNewline(out)), nil
}

func runLaunchctl(args ...string) error {
	return exec.Command("/bin/launchctl", args...).Run()
}

func enableLaunchAgent(src, home string, launchctl func(...string) error) error {
	destDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, filepath.Base(src))
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = launchctl("bootout", domain, dest)
	return launchctl("bootstrap", domain, dest)
}

func spawnServe() error {
	cmd := exec.Command("ollama", "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
