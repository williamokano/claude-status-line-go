package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstall_GlobalCreatesSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	res, err := Install(Options{BinPath: "/usr/local/bin/claude-status-line-go"})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	wantPath := filepath.Join(home, ".claude", "settings.json")
	if res.SettingsPath != wantPath {
		t.Errorf("SettingsPath = %q, want %q", res.SettingsPath, wantPath)
	}
	if res.Replaced {
		t.Error("Replaced should be false for a fresh settings file")
	}

	assertStatusLine(t, wantPath, "/usr/local/bin/claude-status-line-go")
}

func TestInstall_ProjectUsesWorkingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	projectDir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	res, err := Install(Options{Project: true, BinPath: "/usr/local/bin/claude-status-line-go"})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	wantPath := filepath.Join(projectDir, ".claude", "settings.json")
	if res.SettingsPath != wantPath {
		t.Errorf("SettingsPath = %q, want %q", res.SettingsPath, wantPath)
	}

	assertStatusLine(t, wantPath, "/usr/local/bin/claude-status-line-go")
}

func TestInstall_PreservesExistingKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	existing := `{"env":{"FOO":"bar"},"model":"opus"}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if _, err := Install(Options{BinPath: "/usr/local/bin/claude-status-line-go"}); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if _, ok := settings["env"]; !ok {
		t.Error("expected existing \"env\" key to be preserved")
	}
	if _, ok := settings["model"]; !ok {
		t.Error("expected existing \"model\" key to be preserved")
	}

	assertStatusLine(t, settingsPath, "/usr/local/bin/claude-status-line-go")
}

func TestInstall_ReportsReplacedWhenDifferent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	existing := `{"statusLine":{"type":"command","command":"some-other-tool"}}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	res, err := Install(Options{BinPath: "/usr/local/bin/claude-status-line-go"})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !res.Replaced {
		t.Error("Replaced should be true when an existing different statusLine is overwritten")
	}
}

func TestInstall_NotReplacedWhenSameCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	existing := `{"statusLine":{"type":"command","command":"/usr/local/bin/claude-status-line-go"}}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	res, err := Install(Options{BinPath: "/usr/local/bin/claude-status-line-go"})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if res.Replaced {
		t.Error("Replaced should be false when the statusLine already matches")
	}
}

func TestInstall_ResolvesExecutableWhenBinPathEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	res, err := Install(Options{})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if res.BinPath == "" {
		t.Error("expected BinPath to be resolved from os.Executable()")
	}
}

func assertStatusLine(t *testing.T, path, wantCommand string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var settings struct {
		StatusLine struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"statusLine"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if settings.StatusLine.Type != "command" {
		t.Errorf("statusLine.type = %q, want %q", settings.StatusLine.Type, "command")
	}
	if settings.StatusLine.Command != wantCommand {
		t.Errorf("statusLine.command = %q, want %q", settings.StatusLine.Command, wantCommand)
	}
}
