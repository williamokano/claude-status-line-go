package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/williamokano/claude-status-line-go/internal/config"
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

	flag.Parse()

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

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: claude-status-line-go [options]

Reads Claude Code JSON from stdin and prints a formatted status line.

Options:
  -h, --help              print this help
  -v, --version           print version
      --no-color          disable ANSI color output (also via NO_COLOR env)
      --completion SHELL  print completion script (bash, zsh, fish)

Environment variables:
  CSL_SHOW_COST           show session cost (default: true)
  CSL_SHOW_WEEKLY          show weekly usage (default: true)
  CSL_SHOW_TOKENS          show token counts (default: true)
  CSL_SHOW_GIT             show git branch (default: true)
  CSL_SHOW_GIT_DIRTY       show dirty file count (default: true)
  CSL_BAR_SIZE             progress bar width (default: 10)
  CSL_LIMIT_WARN           rate limit warning threshold %% (default: 60)
  CSL_LIMIT_CRIT           rate limit critical threshold %% (default: 85)
  CSL_CTX_WARN             context warning threshold %% (default: 60)
  CSL_CTX_CRIT             context critical threshold %% (default: 85)
  CSL_WEEKLY_SHOW_AT       show weekly when >= this %% (default: 60)
  NO_COLOR                 set to any value to disable ANSI colors

See https://github.com/williamokano/claude-status-line-go
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
