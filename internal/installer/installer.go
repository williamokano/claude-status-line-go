// Package installer registers claude-status-line-go as the Claude Code
// status line by writing the "statusLine" key into a settings.json file,
// preserving any other keys already present.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	// Project installs into the project-level ./.claude/settings.json
	// (relative to the current working directory) instead of the global
	// ~/.claude/settings.json.
	Project bool

	// BinPath overrides the resolved executable path. Left empty, it is
	// resolved via os.Executable().
	BinPath string
}

type Result struct {
	SettingsPath string
	BinPath      string
	Replaced     bool
}

type statusLineConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

func Install(opts Options) (Result, error) {
	settingsPath, err := resolveSettingsPath(opts.Project)
	if err != nil {
		return Result{}, err
	}

	binPath := opts.BinPath
	if binPath == "" {
		binPath, err = resolveBinPath()
		if err != nil {
			return Result{}, err
		}
	}

	settings, err := readSettings(settingsPath)
	if err != nil {
		return Result{}, err
	}

	replaced := hasDifferentStatusLine(settings, binPath)

	newVal, err := json.Marshal(statusLineConfig{Type: "command", Command: binPath})
	if err != nil {
		return Result{}, fmt.Errorf("encode statusLine config: %w", err)
	}
	settings["statusLine"] = newVal

	if err := writeSettings(settingsPath, settings); err != nil {
		return Result{}, err
	}

	return Result{SettingsPath: settingsPath, BinPath: binPath, Replaced: replaced}, nil
}

func hasDifferentStatusLine(settings map[string]json.RawMessage, binPath string) bool {
	raw, ok := settings["statusLine"]
	if !ok {
		return false
	}

	var existing statusLineConfig
	if err := json.Unmarshal(raw, &existing); err != nil {
		return true
	}
	return existing.Type != "command" || existing.Command != binPath
}

func resolveSettingsPath(project bool) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func resolveBinPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

func readSettings(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) == 0 {
		return map[string]json.RawMessage{}, nil
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]json.RawMessage{}
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
