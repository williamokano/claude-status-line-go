package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/williamokano/claude-status-line-go/internal/config"
	"github.com/williamokano/claude-status-line-go/internal/installer"
	"github.com/williamokano/claude-status-line-go/internal/plugin"
	"github.com/williamokano/claude-status-line-go/internal/service"
)

// version is overridden at build time via -ldflags "-X main.version=...".
// Release builds (goreleaser) set it to the git tag. When it's left at the
// default, resolveVersion falls back to Go's embedded VCS build info, which
// covers `go install`, `go build`, and `go run` from any branch/commit.
var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}

	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}

	if dirty {
		return fmt.Sprintf("dev-%s-dirty", revision)
	}
	return fmt.Sprintf("dev-%s", revision)
}

func main() {
	showVersion := flag.BoolP("version", "v", false, "print version and exit")
	help := flag.BoolP("help", "h", false, "print help and exit")
	noColor := flag.Bool("no-color", false, "disable ANSI color output")
	completion := flag.String("completion", "", "print shell completion script (bash|zsh|fish)")
	project := flag.Bool("project", false, "with install: register in ./.claude/settings.json instead of the global settings")
	force := flag.Bool("force", false, "with config init: overwrite an existing config file")

	// Not for people to type: a render spawns this to refresh one plugin's
	// cache out of band, so the command never runs on the render path.
	refreshPlugin := flag.String("refresh-plugin", "", "")
	_ = flag.CommandLine.MarkHidden("refresh-plugin")

	flag.Parse()

	if *refreshPlugin != "" {
		os.Exit(runRefresh(*refreshPlugin))
	}

	if *showVersion {
		fmt.Printf("claude-status-line-go %s\n", resolveVersion())
		os.Exit(0)
	}

	if *help {
		printUsage()
		os.Exit(0)
	}

	if *completion != "" {
		printCompletion(*completion)
		os.Exit(0)
	}

	if flag.Arg(0) == "install" {
		runInstall(*project)
		os.Exit(0)
	}

	if flag.Arg(0) == "plugins" {
		os.Exit(runPlugins(flag.Args()[1:]))
	}

	if flag.Arg(0) == "config" {
		os.Exit(runConfig(flag.Args()[1:], *force))
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	if *noColor {
		cfg.NoColor = true
	}

	svc := service.New(cfg)
	if err := svc.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runConfig handles `config path` and `config init`. The status line itself
// never writes this file: it runs on every render, in every project, sometimes
// from several windows at once, so creating files as a side effect of drawing
// would be surprising and racy. Ask for it instead.
func runConfig(args []string, force bool) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: claude-status-line-go config path|init [--force]")
		return 1
	}

	switch args[0] {
	case "path":
		// Bare, on stdout, so it composes: nvim $(claude-status-line-go config path)
		path := config.Path()
		if path == "" {
			fmt.Fprintln(os.Stderr, "Could not work out a config directory; set XDG_CONFIG_HOME")
			return 1
		}
		fmt.Println(path)
		return 0

	case "init":
		path, err := config.Init(force)
		switch {
		case errors.Is(err, config.ErrExists):
			fmt.Fprintf(os.Stderr, "Config already exists: %s\n", path)
			fmt.Fprintln(os.Stderr, "Edit it, or pass --force to replace it with the defaults.")
			return 1
		case err != nil:
			fmt.Fprintf(os.Stderr, "Could not create the config file: %v\n", err)
			return 1
		}
		fmt.Printf("✓ Wrote the default config\n")
		fmt.Printf("  %s\n\n", path)
		fmt.Printf("Everything in it is already the default, so nothing changes until you edit it.\n")
		fmt.Printf("Open it with:  nvim $(claude-status-line-go config path)\n")
		return 0

	default:
		fmt.Fprintf(os.Stderr, "Unknown: config %s (try: config path, config init)\n", args[0])
		return 1
	}
}

// runPlugins reports what each configured plugin is doing. Claude Code
// discards this tool's stderr, so a plugin that fails silently has nowhere to
// say so — this is where you find out.
func runPlugins(args []string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		return 1
	}

	dir, _ := os.Getwd()

	if len(args) > 0 && args[0] == "refresh" {
		return refreshPlugins(cfg, dir, args[1:])
	}
	if len(args) > 0 && args[0] == "clean" {
		return cleanPlugins(cfg)
	}
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "Unknown: plugins %s (try: plugins, plugins refresh [name], plugins clean)\n", args[0])
		return 1
	}

	if len(cfg.Plugins) == 0 {
		fmt.Printf("No plugins configured in %s\n", config.Path())
		return 0
	}

	fmt.Printf("%s\n\n", config.Path())
	for _, p := range service.New(cfg).InspectPlugins(dir) {
		state := p.State
		switch {
		case p.Source != "command":
		case p.Age == "":
			state += " — the command has not run yet, so the next render will start it"
		case p.Stale:
			state += ", cached " + p.Age + " ago (stale; the next render refreshes it)"
		default:
			state += ", cached " + p.Age + " ago"
		}

		fmt.Printf("%s  [%s]\n", p.Name, p.Source)
		fmt.Printf("  source   %s\n", p.Detail)
		fmt.Printf("  state    %s\n", state)
		if p.Rendered != "" {
			fmt.Printf("  renders  %s\n", p.Rendered)
		}
		if p.Err != "" {
			fmt.Printf("  error    %s\n", p.Err)
		}
		fmt.Println()
	}
	return 0
}

// refreshPlugins runs command plugins in the foreground and reports the result,
// so a broken command shows its error instead of leaving an empty segment.
func refreshPlugins(cfg config.Config, dir string, names []string) int {
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}

	failed, ran := 0, 0
	for _, spec := range cfg.Plugins {
		if len(wanted) > 0 && !wanted[spec.Name] {
			continue
		}
		if spec.Command == "" {
			continue // file sources have nothing to refresh
		}
		ran++
		if err := spec.Refresh(dir, nil); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", spec.Name, err)
			failed++
			continue
		}
		fmt.Printf("✓ %s\n", spec.Name)
	}

	if ran == 0 {
		fmt.Fprintln(os.Stderr, "No command plugins matched")
		return 1
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// cleanPlugins drops cache entries that can't be useful: plugins no longer in
// the config, and entries nothing has touched in a fortnight. Cached values are
// per project, so they accumulate as you move between repos.
func cleanPlugins(cfg config.Config) int {
	names := make([]string, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		names = append(names, p.Name)
	}

	pruned, err := plugin.Prune(names, plugin.MaxCacheAge)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not clean %s: %v\n", plugin.CacheDir(), err)
		return 1
	}

	if pruned.Total() == 0 {
		fmt.Printf("Nothing to clean in %s\n", plugin.CacheDir())
		return 0
	}
	if len(pruned.Orphaned) > 0 {
		fmt.Printf("Removed %d entr%s for plugins no longer configured: %s\n",
			len(pruned.Orphaned), plural(len(pruned.Orphaned)), strings.Join(pruned.Orphaned, ", "))
	}
	if len(pruned.Expired) > 0 {
		fmt.Printf("Removed %d entr%s untouched for over %d days: %s\n",
			len(pruned.Expired), plural(len(pruned.Expired)),
			int(plugin.MaxCacheAge.Hours()/24), strings.Join(pruned.Expired, ", "))
	}
	return 0
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// runRefresh is the detached half of a command plugin: run it, cache the
// result, exit. Nobody is waiting on this, so failures go to stderr and the
// exit code exists only for anyone debugging by hand.
func runRefresh(name string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		return 1
	}

	input, _ := io.ReadAll(os.Stdin)

	dir := os.Getenv("CSL_PROJECT_DIR")
	if dir == "" {
		dir, _ = os.Getwd()
	}

	for _, spec := range cfg.Plugins {
		if spec.Name != name {
			continue
		}
		err := spec.Refresh(dir, input)

		// This process is detached and nothing is waiting on it, so it's the
		// right place to tidy up — pruning here keeps it off the render path
		// and means nobody has to remember to run `plugins clean`.
		names := make([]string, 0, len(cfg.Plugins))
		for _, p := range cfg.Plugins {
			names = append(names, p.Name)
		}
		if _, pruneErr := plugin.Prune(names, plugin.MaxCacheAge); pruneErr != nil {
			fmt.Fprintf(os.Stderr, "claude-status-line-go: pruning cache: %v\n", pruneErr)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "claude-status-line-go: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(os.Stderr, "claude-status-line-go: no plugin named %q\n", name)
	return 1
}

func runInstall(project bool) {
	res, err := installer.Install(installer.Options{Project: project})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install error: %v\n", err)
		os.Exit(1)
	}

	scope := "global"
	if project {
		scope = "project"
	}

	fmt.Printf("✓ Registered claude-status-line-go as the %s Claude Code status line\n", scope)
	fmt.Printf("  settings: %s\n", res.SettingsPath)
	fmt.Printf("  command:  %s\n", res.BinPath)
	if res.Replaced {
		fmt.Println("  (replaced an existing statusLine configuration)")
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: claude-status-line-go [options]
       claude-status-line-go install [--project]

Reads Claude Code JSON from stdin and prints a formatted status line.

Commands:
  install              register this binary as the Claude Code status line
                       by writing it into ~/.claude/settings.json
  plugins              show each configured plugin, its state and what it
                       renders — Claude Code hides stderr, so this is how
                       you find out why a plugin isn't showing up
  plugins refresh [name]
                       run command plugins now, in the foreground, and
                       report any errors
  plugins clean        drop cached values for plugins that are no longer
                       configured, and any untouched for over two weeks
  config path          print the config file path, so it composes:
                         nvim $(claude-status-line-go config path)
  config init [--force]
                       write the commented default config, ready to edit

Options:
  -h, --help              print this help
  -v, --version           print version
      --no-color          disable ANSI color output (also via NO_COLOR env)
      --completion SHELL  print completion script (bash, zsh, fish)
      --project           with install: use ./.claude/settings.json instead of global

Environment variables:
  CSL_SHOW_COST           show session cost (default: true)
  CSL_SHOW_WEEKLY          show weekly usage (default: true)
  CSL_SHOW_TOKENS          show token counts (default: true)
  CSL_SHOW_GIT             show git branch (default: true)
  CSL_SHOW_GIT_DIRTY       show dirty file count (default: true)
  CSL_BAR_SIZE             progress bar width (default: 10)
  CSL_LIMIT_WARN           rate limit warning threshold %% (default: 60)
  CSL_LIMIT_CRIT           rate limit critical threshold %% (default: 85)
                           both apply to the 5-hour and weekly windows
  CSL_CTX_WARN             context warning threshold %% (default: 60)
  CSL_CTX_CRIT             context critical threshold %% (default: 85)
  CSL_WEEKLY_SHOW_AT       show weekly when >= this %% (default: 60)
  NO_COLOR                 set to any value to disable ANSI colors

Configuration file:
  ~/.config/claude-status-line-go/claude-status-line.yaml
  Holds the same settings, plus plugins that add your own segments.
  Precedence is defaults, then this file, then the environment.

See https://okano.dev/claude-status-line-go/
`)
}

func printCompletion(shell string) {
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "Unknown shell: %s (supported: bash, zsh, fish)\n", shell)
		os.Exit(1)
	}
}

//go:embed completions/bash.sh
var bashCompletion string

//go:embed completions/zsh.zsh
var zshCompletion string

//go:embed completions/fish.fish
var fishCompletion string
