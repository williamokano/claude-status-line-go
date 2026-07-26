package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestInitWritesAConfigYouCanLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := Init(false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if want := filepath.Join(dir, "claude-status-line-go", "claude-status-line.yaml"); path != want {
		t.Errorf("wrote to %s, want %s", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not on disk: %v", err)
	}

	// The whole point is that it's editable, so it has to load cleanly.
	if _, err := Load(); err != nil {
		t.Fatalf("Load of a freshly written config: %v", err)
	}
}

// An untouched config must change nothing, or `config init` would silently
// reconfigure someone's status line.
func TestInitDefaultsMatchTheBuiltInDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := Init(false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !reflect.DeepEqual(got, DefaultConfig()) {
		t.Errorf("a freshly written config changes behaviour:\n got %+v\nwant %+v", got, DefaultConfig())
	}
}

func TestInitRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, err := Init(false)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(path, []byte("bar_size: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(false); !errors.Is(err, ErrExists) {
		t.Fatalf("second Init returned %v, want ErrExists", err)
	}

	// The hand-written value has to survive the refusal.
	cfg, _ := Load()
	if cfg.BarSize != 3 {
		t.Errorf("BarSize = %d, want the hand-written 3 to be untouched", cfg.BarSize)
	}
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path, _ := Init(false)
	if err := os.WriteFile(path, []byte("bar_size: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Init(true); err != nil {
		t.Fatalf("Init(force): %v", err)
	}
	cfg, _ := Load()
	if cfg.BarSize != 10 {
		t.Errorf("BarSize = %d, want the default 10 back", cfg.BarSize)
	}
}

func TestInitCreatesMissingDirectories(t *testing.T) {
	// XDG_CONFIG_HOME pointing somewhere that doesn't exist yet is normal on a
	// fresh machine.
	dir := filepath.Join(t.TempDir(), "deep", "nested")
	t.Setenv("XDG_CONFIG_HOME", dir)

	if _, err := Init(false); err != nil {
		t.Fatalf("Init: %v", err)
	}
}

func TestDefaultYAMLParses(t *testing.T) {
	var into map[string]any
	if err := yaml.Unmarshal([]byte(Default()), &into); err != nil {
		t.Fatalf("the embedded default is not valid YAML: %v", err)
	}
}

// Every uncommented key must be one the loader actually reads, or the file
// would document settings that quietly do nothing — which is the exact bug
// this config file had before it was wired up at all.
func TestDefaultYAMLHasNoUnknownKeys(t *testing.T) {
	var into map[string]any
	if err := yaml.Unmarshal([]byte(Default()), &into); err != nil {
		t.Fatal(err)
	}

	known := map[string]bool{}
	ft := reflect.TypeOf(file{})
	for i := range ft.NumField() {
		if tag := ft.Field(i).Tag.Get("yaml"); tag != "" {
			known[tag] = true
		}
	}

	for k := range into {
		if !known[k] {
			t.Errorf("default.yaml sets %q, which the loader does not read", k)
		}
	}
}

// The repository's example-config.yaml is what people read on GitHub; the
// embedded copy is what they get on disk. They have to be the same file.
func TestExampleConfigMatchesEmbeddedDefault(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "example-config.yaml"))
	if err != nil {
		t.Skipf("example-config.yaml unavailable: %v", err)
	}
	if strings.TrimSpace(string(b)) != strings.TrimSpace(Default()) {
		t.Error("example-config.yaml and internal/config/default.yaml have drifted; " +
			"copy internal/config/default.yaml over example-config.yaml")
	}
}

// The commented plugin examples are the ones people uncomment first, so they
// have to be valid once uncommented.
func TestCommentedPluginExamplesAreValid(t *testing.T) {
	var body []string
	for _, line := range strings.Split(Default(), "\n") {
		if strings.HasPrefix(line, "# plugins:") || (len(body) > 0 && strings.HasPrefix(line, "#")) {
			body = append(body, strings.TrimPrefix(strings.TrimPrefix(line, "#"), " "))
			continue
		}
		if len(body) > 0 && strings.TrimSpace(line) == "" {
			break
		}
	}
	if len(body) == 0 {
		t.Fatal("no commented plugins block found in the default config")
	}

	var into struct {
		Plugins []plugin0 `yaml:"plugins"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(body, "\n")), &into); err != nil {
		t.Fatalf("uncommenting the plugins block yields invalid YAML: %v", err)
	}
	if len(into.Plugins) < 2 {
		t.Errorf("got %d example plugins, want both the command and file examples", len(into.Plugins))
	}
	for _, p := range into.Plugins {
		if p.Name == "" {
			t.Error("an example plugin has no name")
		}
		if p.File == "" && p.Command == "" {
			t.Errorf("example plugin %q has no source", p.Name)
		}
	}
}

// plugin0 mirrors just enough of plugin.Spec to validate the examples without
// importing it here.
type plugin0 struct {
	Name    string `yaml:"name"`
	File    string `yaml:"file"`
	Command string `yaml:"command"`
}
