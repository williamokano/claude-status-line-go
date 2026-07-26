package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig points XDG_CONFIG_HOME at a temp dir holding the given YAML.
func writeConfig(t *testing.T, yaml string) {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, "claude-status-line-go")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "claude-status-line.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestFileOverridesDefaults(t *testing.T) {
	writeConfig(t, "bar_size: 3\nlimit_warn: 20\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BarSize != 3 {
		t.Errorf("BarSize = %d, want 3", cfg.BarSize)
	}
	if cfg.LimitWarn != 20 {
		t.Errorf("LimitWarn = %d, want 20", cfg.LimitWarn)
	}
	if cfg.LimitCrit != 85 {
		t.Errorf("LimitCrit = %d, want the default 85", cfg.LimitCrit)
	}
}

// The pointer fields exist for this: `false` in a file has to be
// distinguishable from "key not present", or nothing could be turned off.
func TestFileCanTurnThingsOff(t *testing.T) {
	writeConfig(t, "show_cost: false\nshow_tokens: false\n")

	cfg, _ := Load()
	if cfg.ShowCost {
		t.Error("ShowCost should be false")
	}
	if cfg.ShowTokens {
		t.Error("ShowTokens should be false")
	}
	if !cfg.ShowGit {
		t.Error("ShowGit should still default to true")
	}
}

func TestEnvBeatsFile(t *testing.T) {
	writeConfig(t, "bar_size: 3\n")
	t.Setenv("CSL_BAR_SIZE", "7")

	cfg, _ := Load()
	if cfg.BarSize != 7 {
		t.Errorf("BarSize = %d, want 7 (env wins over file)", cfg.BarSize)
	}
}

func TestMissingFileIsNotAnError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BarSize != 10 {
		t.Errorf("BarSize = %d, want the default 10", cfg.BarSize)
	}
}

// A broken config must not take the status line down with it.
func TestBrokenYAMLFallsBackToDefaults(t *testing.T) {
	writeConfig(t, "bar_size: [this is not an int\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should not return an error: %v", err)
	}
	if cfg.BarSize != 10 {
		t.Errorf("BarSize = %d, want the default 10", cfg.BarSize)
	}
}

func TestPluginsParse(t *testing.T) {
	writeConfig(t, `
plugins:
  - name: issues
    file: /tmp/issues.json
    icon: "🎯"
    bar: true
    display: "{icon} {bar} {value}/{max}"
    thresholds:
      - { at: 0,  color: red }
      - { at: 61, color: green }
`)

	cfg, _ := Load()
	if len(cfg.Plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(cfg.Plugins))
	}
	p := cfg.Plugins[0]
	if p.Name != "issues" || p.File != "/tmp/issues.json" || p.Icon != "🎯" || !p.Bar {
		t.Errorf("plugin parsed wrong: %+v", p)
	}
	if len(p.Thresholds) != 2 || p.Thresholds[1].Color != "green" || p.Thresholds[1].At != 61 {
		t.Errorf("thresholds parsed wrong: %+v", p.Thresholds)
	}
}

func TestUnnamedPluginIsSkipped(t *testing.T) {
	writeConfig(t, "plugins:\n  - file: /tmp/x.json\n  - name: ok\n    file: /tmp/y.json\n")

	cfg, _ := Load()
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Name != "ok" {
		t.Errorf("got %+v, want only the named plugin", cfg.Plugins)
	}
}

// Names key the cache file and the {plugin.<name>} placeholders, so a duplicate
// would silently share one cache with the first and make placeholders ambiguous.
func TestDuplicatePluginNameIsDropped(t *testing.T) {
	writeConfig(t, `
plugins:
  - name: dup
    command: "echo first"
  - name: dup
    command: "echo second"
`)

	cfg, _ := Load()
	if len(cfg.Plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Command != "echo first" {
		t.Errorf("kept %q, want the first declaration", cfg.Plugins[0].Command)
	}
}

func TestPluginWithNoSourceIsDropped(t *testing.T) {
	writeConfig(t, "plugins:\n  - name: empty\n  - name: ok\n    file: /tmp/x.json\n")

	cfg, _ := Load()
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Name != "ok" {
		t.Errorf("got %+v, want only the plugin with a source", cfg.Plugins)
	}
}

// Both sources set is ambiguous. Command wins and file is cleared, so nothing
// downstream has to guess.
func TestPluginWithBothSourcesPrefersCommand(t *testing.T) {
	writeConfig(t, "plugins:\n  - name: both\n    file: /tmp/x.json\n    command: \"echo hi\"\n")

	cfg, _ := Load()
	if len(cfg.Plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(cfg.Plugins))
	}
	if cfg.Plugins[0].Command != "echo hi" || cfg.Plugins[0].File != "" {
		t.Errorf("got command=%q file=%q, want the command kept and file cleared",
			cfg.Plugins[0].Command, cfg.Plugins[0].File)
	}
}
