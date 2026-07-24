package service

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/williamokano/claude-status-line-go/internal/config"
)

var ansiRegexp = regexp.MustCompile(`\033\[[0-9;]*m`)

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
		Weekly   RateLimit `json:"seven_day"`
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
	ResetsAt       int64   `json:"resets_at"`
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

type Service struct {
	cfg config.Config
}

func New(cfg config.Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Run() error {
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		return fmt.Errorf("stdin is a terminal; pipe Claude Code JSON to this tool")
	}

	data, _ := io.ReadAll(os.Stdin)

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		// encoding/json fills in every field it can decode before hitting
		// a bad one, so input still holds whatever was parseable. Render
		// with that instead of blanking the whole status line over one
		// unexpected or missing field.
		fmt.Fprintf(os.Stderr, "claude-status-line-go: partial JSON parse: %v\n", err)
		if path, dumpErr := dumpFailedInput(data); dumpErr == nil {
			fmt.Fprintf(os.Stderr, "claude-status-line-go: raw input saved to %s\n", path)
		}
	}

	ctxPercent := int(math.Round(input.ContextWindow.UsedPercentage))
	ctxSize := input.ContextWindow.ContextWindowSize
	tokensIn := input.ContextWindow.CurrentUsage.InputTokens
	tokensOut := input.ContextWindow.CurrentUsage.OutputTokens
	tokensCache := input.ContextWindow.CurrentUsage.CacheCreationInputTokens + input.ContextWindow.CurrentUsage.CacheReadInputTokens
	cost := input.Cost.TotalCostUSD
	limit5 := int(math.Round(input.RateLimits.FiveHour.UsedPercentage))
	limit5Reset := input.RateLimits.FiveHour.ResetsAt
	weekly := int(math.Round(input.RateLimits.Weekly.UsedPercentage))

	modelStr := shortModel(input.Model.DisplayName)
	sizeStr := contextSize(ctxSize)

	ctxColor := colorForPercent(ctxPercent, s.cfg.CtxWarn, s.cfg.CtxCrit)
	limitColor := colorForPercent(limit5, s.cfg.LimitWarn, s.cfg.LimitCrit)

	ctxBar := progressBar(ctxPercent, s.cfg.BarSize)
	limitBar := progressBar(limit5, s.cfg.BarSize)

	projectName := filepath.Base(input.Workspace.ProjectDir)

	if s.cfg.Format != "" {
		out := s.renderFormat(modelStr, sizeStr, ctxPercent, limit5, ctxBar, limitBar,
			ctxColor, limitColor, cost, projectName, tokensIn, tokensOut, tokensCache, weekly, limit5Reset)
		if s.cfg.NoColor {
			out = stripANSI(out)
		}
		fmt.Println(out)
		return nil
	}

	top := fmt.Sprintf("%s🧠 %s·%s%s", Cyan, modelStr, sizeStr, Reset)
	top += fmt.Sprintf(" %s│ 📁 %s%s", Dim, projectName, Reset)

	branch, dirty := s.getGitInfo()
	if branch != "" {
		top += fmt.Sprintf(" %s│ 🌿 %s%s%s", Dim, branch, dirty, Reset)
	}

	bottom := ""
	bottom += fmt.Sprintf("%s🟡5h %s %d%%", limitColor, limitBar, limit5)

	resetIn := timeUntil(limit5Reset)
	if resetIn != "" {
		bottom += fmt.Sprintf(" ↺ %s", resetIn)
	}

	bottom += fmt.Sprintf("%s %s│ %sCTX %s %d%%%s", Reset, Dim, ctxColor, ctxBar, ctxPercent, Reset)

	if s.cfg.ShowTokens {
		bottom += fmt.Sprintf(" %s│ I%s O%s ⚡%s",
			Dim,
			humanTokens(tokensIn),
			humanTokens(tokensOut),
			humanTokens(tokensCache),
		)
	}

	if s.cfg.ShowWeekly && weekly >= s.cfg.WeeklyShowAt {
		bottom += fmt.Sprintf(" %s│ %s7d %d%%%s", Dim, Dim, weekly, Reset)
	}

	if s.cfg.ShowCost {
		bottom += fmt.Sprintf(" %s│ %s$%.2f%s", Dim, Yellow, cost, Reset)
		}

	out := top + "\n" + bottom

	if s.cfg.NoColor {
		out = stripANSI(out)
	}

	fmt.Println(out)
	return nil
}

// failedInputPrefix names the debug dumps written when stdin fails to parse:
// <tmpdir>/claude-status-line-go-parse-fail-<timestamp>-<rand>.json
const failedInputPrefix = "claude-status-line-go-parse-fail-"

// dumpFailedInput saves the raw stdin payload that failed to parse so it can
// be inspected later. The dump must never take the status line down with it,
// so callers treat a returned error as log-and-move-on.
func dumpFailedInput(data []byte) (string, error) {
	pattern := failedInputPrefix + time.Now().Format("20060102-150405") + "-*.json"
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func (s *Service) renderFormat(modelStr, sizeStr string, ctxPercent, limit5 int,
	ctxBar, limitBar, ctxColor, limitColor string, cost float64, projectName string,
	tokensIn, tokensOut, tokensCache, weekly int, limit5Reset int64) string {

	branch, dirty := s.getGitInfo()
	resetIn := timeUntil(limit5Reset)

	repl := strings.NewReplacer(
		"{model}", modelStr,
		"{model_name}", "",
		"{ctx_size}", sizeStr,
		"{project}", projectName,
		"{branch}", branch,
		"{dirty}", dirty,
		"{limit_bar}", limitBar,
		"{limit_pct}", strconv.Itoa(limit5),
		"{limit_color}", limitColor,
		"{limit_reset}", resetIn,
		"{ctx_bar}", ctxBar,
		"{ctx_pct}", strconv.Itoa(ctxPercent),
		"{ctx_color}", ctxColor,
		"{tokens_in}", humanTokens(tokensIn),
		"{tokens_out}", humanTokens(tokensOut),
		"{tokens_cache}", humanTokens(tokensCache),
		"{cost}", fmt.Sprintf("%.2f", cost),
		"{weekly_pct}", strconv.Itoa(weekly),
		"{reset}", Reset,
		"{dim}", Dim,
		"{bold}", Bold,
		"{red}", Red,
		"{green}", Green,
		"{yellow}", Yellow,
		"{blue}", Blue,
		"{magenta}", Magenta,
		"{cyan}", Cyan,
		"{white}", White,
	)
	return repl.Replace(s.cfg.Format)
}

func shortModel(model string) string {
	if model == "" {
		return "Unknown"
	}
	return model
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

func progressBar(pct, barSize int) string {
	filled := pct * barSize / 100
	empty := barSize - filled
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

func timeUntil(unixSeconds int64) string {
	if unixSeconds == 0 {
		return ""
	}
	t := time.Unix(unixSeconds, 0)
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

func (s *Service) getGitInfo() (string, string) {
	if !s.cfg.ShowGit {
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
	if s.cfg.ShowGitDirty {
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
