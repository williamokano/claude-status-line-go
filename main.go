package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Input struct {
	Model struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	} `json:"model"`
	Workspace struct {
		ProjectDir string `json:"project_dir"`
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	ContextWindow struct {
		UsedPercentage     float64 `json:"used_percentage"`
		ContextWindowSize  int     `json:"context_window_size"`
		CurrentUsage       Usage   `json:"current_usage"`
	} `json:"context_window"`
	Cost struct {
		TotalCostUSD   float64 `json:"total_cost_usd"`
		TotalDurationMS int64  `json:"total_duration_ms"`
	} `json:"cost"`
	RateLimits struct {
		FiveHour RateLimit `json:"five_hour"`
		Weekly   RateLimit `json:"weekly"`
	} `json:"rate_limits"`
}

type Usage struct {
	InputTokens                int `json:"input_tokens"`
	OutputTokens               int `json:"output_tokens"`
	CacheCreationInputTokens   int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens       int `json:"cache_read_input_tokens"`
}

type RateLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       string  `json:"resets_at"`
}

type Config struct {
	ShowCost     bool  `env:"SHOW_COST" default:"true"`
	ShowWeekly   bool  `env:"SHOW_WEEKLY" default:"true"`
	ShowTokens   bool  `env:"SHOW_TOKENS" default:"true"`
	ShowGit      bool  `env:"SHOW_GIT" default:"true"`
	ShowGitDirty bool  `env:"SHOW_GIT_DIRTY" default:"true"`

	BarSize      int `env:"BAR_SIZE" default:"10"`

	LimitWarn    int `env:"LIMIT_WARN" default:"60"`
	LimitCrit    int `env:"LIMIT_CRIT" default:"85"`

	CtxWarn      int `env:"CTX_WARN" default:"60"`
	CtxCrit      int `env:"CTX_CRIT" default:"85"`

	WeeklyShowAt int `env:"WEEKLY_SHOW_AT" default:"60"`
}

const (
	Reset  = "\033[0m"
	Dim    = "\033[2m"
	Bold   = "\033[1m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Magenta = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

var cfg Config

func main() {
	loadConfig()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	ctxPercent := int(input.ContextWindow.UsedPercentage)
	ctxSize := input.ContextWindow.ContextWindowSize
	tokensIn := input.ContextWindow.CurrentUsage.InputTokens
	tokensOut := input.ContextWindow.CurrentUsage.OutputTokens
	tokensCache := input.ContextWindow.CurrentUsage.CacheCreationInputTokens + input.ContextWindow.CurrentUsage.CacheReadInputTokens
	cost := input.Cost.TotalCostUSD
	limit5 := int(input.RateLimits.FiveHour.UsedPercentage)
	limit5Reset := input.RateLimits.FiveHour.ResetsAt
	weekly := int(input.RateLimits.Weekly.UsedPercentage)

	modelStr := shortModel(input.Model.DisplayName)
	sizeStr := contextSize(ctxSize)

	ctxColor := colorForPercent(ctxPercent, cfg.CtxWarn, cfg.CtxCrit)
	limitColor := colorForPercent(limit5, cfg.LimitWarn, cfg.LimitCrit)

	ctxBar := progressBar(ctxPercent)
	limitBar := progressBar(limit5)

	projectName := filepath.Base(input.Workspace.ProjectDir)

	top := fmt.Sprintf("%s[%s·%s]%s", Cyan, modelStr, sizeStr, Reset)
	top += fmt.Sprintf(" %s📁 %s%s", Yellow, projectName, Reset)

	branch, dirty := getGitInfo()
	if branch != "" {
		top += fmt.Sprintf(" %s🌿 %s%s%s", Green, branch, dirty, Reset)
	}

	bottom := ""
	bottom += fmt.Sprintf("%s5h %s %d%%", limitColor, limitBar, limit5)

	resetIn := timeUntil(limit5Reset)
	if resetIn != "" {
		bottom += fmt.Sprintf(" ↺%s", resetIn)
	}

	bottom += fmt.Sprintf("%s │ ", Reset)
	bottom += fmt.Sprintf("%sCTX %s %d%%%s", ctxColor, ctxBar, ctxPercent, Reset)

	if cfg.ShowTokens {
		bottom += fmt.Sprintf(" │ I%s O%s ⚡%s",
			humanTokens(tokensIn),
			humanTokens(tokensOut),
			humanTokens(tokensCache),
		)
	}

	if cfg.ShowWeekly && weekly >= cfg.WeeklyShowAt {
		bottom += fmt.Sprintf(" │ %s7d %d%%%s", Dim, weekly, Reset)
	}

	if cfg.ShowCost {
		bottom += fmt.Sprintf(" │ %s$%.2f%s", Yellow, cost, Reset)
	}

	fmt.Println(top)
	fmt.Println(bottom)
}

func loadConfig() {
	cfg = Config{
		ShowCost:     getEnvBool("SHOW_COST", true),
		ShowWeekly:   getEnvBool("SHOW_WEEKLY", true),
		ShowTokens:   getEnvBool("SHOW_TOKENS", true),
		ShowGit:      getEnvBool("SHOW_GIT", true),
		ShowGitDirty: getEnvBool("SHOW_GIT_DIRTY", true),

		BarSize:      getEnvInt("BAR_SIZE", 10),

		LimitWarn:    getEnvInt("LIMIT_WARN", 60),
		LimitCrit:    getEnvInt("LIMIT_CRIT", 85),

		CtxWarn:      getEnvInt("CTX_WARN", 60),
		CtxCrit:      getEnvInt("CTX_CRIT", 85),

		WeeklyShowAt: getEnvInt("WEEKLY_SHOW_AT", 60),
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

func shortModel(model string) string {
	switch {
	case strings.Contains(model, "Opus"):
		return "O4.7"
	case strings.Contains(model, "Sonnet"):
		return "S5"
	case strings.Contains(model, "Haiku"):
		return "H"
	default:
		return model
	}
}

func contextSize(size int) string {
	switch size {
	case 200000:
		return "200k"
	case 1000000:
		return "1M"
	default:
		return strconv.Itoa(size)
	}
}

func humanTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return strconv.Itoa(n)
}

func progressBar(pct int) string {
	filled := pct * cfg.BarSize / 100
	empty := cfg.BarSize - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func colorForPercent(pct, warn, crit int) string {
	if pct >= crit {
		return Red
	}
	if pct >= warn {
		return Yellow
	}
	return Green
}

func timeUntil(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z07:00", ts)
		if err != nil {
			return ""
		}
	}
	diff := time.Until(t)
	if diff < 0 {
		diff = 0
	}
	hours := int(diff.Hours())
	minutes := int(diff.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func getGitInfo() (string, string) {
	if !cfg.ShowGit {
		return "", ""
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", ""
	}

	branchCmd := exec.Command("git", "branch", "--show-current")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return "", ""
	}
	branch := strings.TrimSpace(string(branchOut))

	dirty := ""
	if cfg.ShowGitDirty {
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusOut, err := statusCmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
			changed := 0
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					changed++
				}
			}
			if changed > 0 {
				dirty = fmt.Sprintf(" %s●%d%s", Yellow, changed, Reset)
			}
		}
	}

	return branch, dirty
}