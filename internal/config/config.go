package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/williamokano/claude-status-line-go/internal/plugin"
)

// defaultYAML is the file `config init` writes. Embedding it means the file you
// get is the file in the repository — it can't drift from what the docs show.
//
//go:embed default.yaml
var defaultYAML string

// ErrExists reports that a config file is already there, so init won't
// overwrite hand-written settings by accident.
var ErrExists = errors.New("config file already exists")

// Init writes the commented default config and returns where it put it.
// Passing force replaces an existing file.
func Init(force bool) (string, error) {
	path := Path()
	if path == "" {
		return "", errors.New("could not work out a config directory; set XDG_CONFIG_HOME")
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			return path, ErrExists
		} else if !os.IsNotExist(err) {
			return path, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(defaultYAML), 0o644); err != nil {
		return path, fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

// Default is the config file contents Init writes, exposed for tests.
func Default() string { return defaultYAML }

type Config struct {
	ShowCost     bool
	ShowWeekly   bool
	ShowTokens   bool
	ShowGit      bool
	ShowGitDirty bool

	BarSize int

	LimitWarn int
	LimitCrit int

	CtxWarn int
	CtxCrit int

	WeeklyShowAt int

	NoColor bool
	Format  string

	Plugins []plugin.Spec
}

// file mirrors claude-status-line.yaml. Every scalar is a pointer so an absent
// key stays absent: without that, `show_cost: false` and "key not present"
// look identical and a config file could never turn anything off.
type file struct {
	ShowCost     *bool `yaml:"show_cost"`
	ShowWeekly   *bool `yaml:"show_weekly"`
	ShowTokens   *bool `yaml:"show_tokens"`
	ShowGit      *bool `yaml:"show_git"`
	ShowGitDirty *bool `yaml:"show_git_dirty"`

	BarSize *int `yaml:"bar_size"`

	LimitWarn *int `yaml:"limit_warn"`
	LimitCrit *int `yaml:"limit_crit"`

	CtxWarn *int `yaml:"ctx_warn"`
	CtxCrit *int `yaml:"ctx_crit"`

	WeeklyShowAt *int `yaml:"weekly_show_at"`

	NoColor *bool   `yaml:"no_color"`
	Format  *string `yaml:"format"`

	Plugins []plugin.Spec `yaml:"plugins"`
}

func DefaultConfig() Config {
	return Config{
		ShowCost:     true,
		ShowWeekly:   true,
		ShowTokens:   true,
		ShowGit:      true,
		ShowGitDirty: true,

		BarSize: 10,

		LimitWarn: 60,
		LimitCrit: 85,

		CtxWarn: 60,
		CtxCrit: 85,

		WeeklyShowAt: 60,
	}
}

// Load builds the config from defaults, then the config file, then the
// environment — so a CSL_ variable always wins over the file, and the file
// always wins over the defaults.
func Load() (Config, error) {
	cfg := DefaultConfig()

	if err := applyFile(&cfg, Path()); err != nil {
		// A broken config file must not take the status line down: report it
		// and render with what we have.
		fmt.Fprintf(os.Stderr, "claude-status-line-go: %v\n", err)
	}
	applyEnv(&cfg)

	return cfg, nil
}

// Path is where the config file is read from, honouring XDG_CONFIG_HOME.
func Path() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "claude-status-line-go", "claude-status-line.yaml")
}

func applyFile(cfg *Config, path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no config file is the normal case
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	setBool(&cfg.ShowCost, f.ShowCost)
	setBool(&cfg.ShowWeekly, f.ShowWeekly)
	setBool(&cfg.ShowTokens, f.ShowTokens)
	setBool(&cfg.ShowGit, f.ShowGit)
	setBool(&cfg.ShowGitDirty, f.ShowGitDirty)

	setInt(&cfg.BarSize, f.BarSize)
	setInt(&cfg.LimitWarn, f.LimitWarn)
	setInt(&cfg.LimitCrit, f.LimitCrit)
	setInt(&cfg.CtxWarn, f.CtxWarn)
	setInt(&cfg.CtxCrit, f.CtxCrit)
	setInt(&cfg.WeeklyShowAt, f.WeeklyShowAt)

	setBool(&cfg.NoColor, f.NoColor)
	if f.Format != nil {
		cfg.Format = *f.Format
	}

	cfg.Plugins = append(cfg.Plugins, validPlugins(f.Plugins)...)

	return nil
}

// validPlugins drops plugins that can't work and explains why. Everything here
// is a config mistake that would otherwise fail silently at render time.
func validPlugins(in []plugin.Spec) []plugin.Spec {
	out := make([]plugin.Spec, 0, len(in))
	seen := make(map[string]bool, len(in))

	for _, p := range in {
		switch {
		case p.Name == "":
			warn("skipping a plugin with no name")

		case seen[p.Name]:
			// Names key the cache file and the {plugin.<name>} placeholders, so
			// a duplicate silently shares one cache with the first and makes
			// its placeholders ambiguous.
			warn("skipping a second plugin named %q — names must be unique", p.Name)

		case p.File == "" && p.Command == "":
			warn("skipping plugin %q — it needs a file or command source", p.Name)

		case p.File != "" && p.Command != "":
			warn("plugin %q sets both file and command; using command and ignoring file", p.Name)
			p.File = ""
			seen[p.Name] = true
			out = append(out, p)

		default:
			seen[p.Name] = true
			out = append(out, p)
		}
	}

	return out
}

func warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "claude-status-line-go: "+format+"\n", args...)
}

func applyEnv(cfg *Config) {
	// Every setting is namespaced under CSL_, as documented. Reading the bare
	// names would let an unrelated SHOW_GIT or BAR_SIZE already in the user's
	// environment silently reshape the status line.
	cfg.ShowCost = getEnvBool("CSL_SHOW_COST", cfg.ShowCost)
	cfg.ShowWeekly = getEnvBool("CSL_SHOW_WEEKLY", cfg.ShowWeekly)
	cfg.ShowTokens = getEnvBool("CSL_SHOW_TOKENS", cfg.ShowTokens)
	cfg.ShowGit = getEnvBool("CSL_SHOW_GIT", cfg.ShowGit)
	cfg.ShowGitDirty = getEnvBool("CSL_SHOW_GIT_DIRTY", cfg.ShowGitDirty)

	cfg.BarSize = getEnvInt("CSL_BAR_SIZE", cfg.BarSize)

	cfg.LimitWarn = getEnvInt("CSL_LIMIT_WARN", cfg.LimitWarn)
	cfg.LimitCrit = getEnvInt("CSL_LIMIT_CRIT", cfg.LimitCrit)

	cfg.CtxWarn = getEnvInt("CSL_CTX_WARN", cfg.CtxWarn)
	cfg.CtxCrit = getEnvInt("CSL_CTX_CRIT", cfg.CtxCrit)

	cfg.WeeklyShowAt = getEnvInt("CSL_WEEKLY_SHOW_AT", cfg.WeeklyShowAt)

	cfg.NoColor = getEnvBool("NO_COLOR", cfg.NoColor)
	if v := os.Getenv("CSL_FORMAT"); v != "" {
		cfg.Format = v
	}
}

func setBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

func setInt(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

func getEnvBool(key string, def bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return b
}

func getEnvInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return i
}
