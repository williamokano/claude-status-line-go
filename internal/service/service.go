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

// view holds every value the renderers need, already formatted and colored.
// It is computed once per run so the default renderer and the custom-format
// renderer can never drift apart on how a value is presented.
type view struct {
	model   string
	ctxSize string
	project string
	branch  string
	dirty   string

	ctxPct   int
	ctxBar   string
	ctxColor string

	limit5Pct   int
	limit5Bar   string
	limit5Color string
	limit5Reset string

	weeklyPct   int
	weeklyBar   string
	weeklyColor string
	weeklyReset string

	tokensIn    string
	tokensOut   string
	tokensCache string

	cost float64
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

	v := s.buildView(input)

	out := s.render(v)
	if s.cfg.NoColor {
		out = stripANSI(out)
	}

	fmt.Println(out)
	return nil
}

func (s *Service) buildView(input Input) view {
	ctxPct := int(math.Round(input.ContextWindow.UsedPercentage))
	limit5Pct := int(math.Round(input.RateLimits.FiveHour.UsedPercentage))
	weeklyPct := int(math.Round(input.RateLimits.Weekly.UsedPercentage))

	branch, dirty := s.getGitInfo()

	tokens := input.ContextWindow.CurrentUsage

	return view{
		model:   shortModel(input.Model.DisplayName),
		ctxSize: contextSize(input.ContextWindow.ContextWindowSize),
		project: filepath.Base(input.Workspace.ProjectDir),
		branch:  branch,
		dirty:   dirty,

		ctxPct:   ctxPct,
		ctxBar:   progressBar(ctxPct, s.cfg.BarSize),
		ctxColor: colorForPercent(ctxPct, s.cfg.CtxWarn, s.cfg.CtxCrit),

		limit5Pct:   limit5Pct,
		limit5Bar:   progressBar(limit5Pct, s.cfg.BarSize),
		limit5Color: colorForPercent(limit5Pct, s.cfg.LimitWarn, s.cfg.LimitCrit),
		limit5Reset: timeUntil(input.RateLimits.FiveHour.ResetsAt),

		weeklyPct:   weeklyPct,
		weeklyBar:   progressBar(weeklyPct, s.cfg.BarSize),
		weeklyColor: colorForPercent(weeklyPct, s.cfg.LimitWarn, s.cfg.LimitCrit),
		weeklyReset: timeUntil(input.RateLimits.Weekly.ResetsAt),

		tokensIn:    humanTokens(tokens.InputTokens),
		tokensOut:   humanTokens(tokens.OutputTokens),
		tokensCache: humanTokens(tokens.CacheCreationInputTokens + tokens.CacheReadInputTokens),

		cost: input.Cost.TotalCostUSD,
	}
}

func (s *Service) render(v view) string {
	if s.cfg.Format != "" {
		return s.renderFormat(v)
	}

	top := fmt.Sprintf("%s🧠 %s·%s%s", Cyan, v.model, v.ctxSize, Reset)
	top += fmt.Sprintf(" %s│ 📁 %s%s", Dim, v.project, Reset)

	if v.branch != "" {
		top += fmt.Sprintf(" %s│ 🌿 %s%s%s", Dim, v.branch, v.dirty, Reset)
	}

	bottom := usageSegment("🟡5h", v.limit5Color, v.limit5Bar, v.limit5Pct, v.limit5Reset)
	bottom += fmt.Sprintf("%s %s│ %sCTX %s %d%%%s", Reset, Dim, v.ctxColor, v.ctxBar, v.ctxPct, Reset)

	if s.cfg.ShowTokens {
		bottom += fmt.Sprintf(" %s│ I%s O%s ⚡%s", Dim, v.tokensIn, v.tokensOut, v.tokensCache)
	}

	if s.cfg.ShowWeekly && v.weeklyPct >= s.cfg.WeeklyShowAt {
		bottom += fmt.Sprintf(" %s│ ", Dim)
		bottom += usageSegment("📅7d", v.weeklyColor, v.weeklyBar, v.weeklyPct, v.weeklyReset) + Reset
	}

	if s.cfg.ShowCost {
		bottom += fmt.Sprintf(" %s│ %s$%.2f%s", Dim, Yellow, v.cost, Reset)
	}

	return top + "\n" + bottom
}

// usageSegment renders one rate-limit window — "<label> <bar> <pct>% ↺ <reset>" —
// so the 5-hour and 7-day windows always look alike. The reset countdown is
// dropped when Claude Code doesn't report a reset time for that window.
func usageSegment(label, color, bar string, pct int, resetIn string) string {
	seg := fmt.Sprintf("%s%s %s %d%%", color, label, bar, pct)
	if resetIn != "" {
		seg += fmt.Sprintf(" ↺ %s", resetIn)
	}
	return seg
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

func (s *Service) renderFormat(v view) string {
	repl := strings.NewReplacer(
		"{model}", v.model,
		"{model_name}", "",
		"{ctx_size}", v.ctxSize,
		"{project}", v.project,
		"{branch}", v.branch,
		"{dirty}", v.dirty,
		"{limit_bar}", v.limit5Bar,
		"{limit_pct}", strconv.Itoa(v.limit5Pct),
		"{limit_color}", v.limit5Color,
		"{limit_reset}", v.limit5Reset,
		"{ctx_bar}", v.ctxBar,
		"{ctx_pct}", strconv.Itoa(v.ctxPct),
		"{ctx_color}", v.ctxColor,
		"{tokens_in}", v.tokensIn,
		"{tokens_out}", v.tokensOut,
		"{tokens_cache}", v.tokensCache,
		"{cost}", fmt.Sprintf("%.2f", v.cost),
		"{weekly_pct}", strconv.Itoa(v.weeklyPct),
		"{weekly_bar}", v.weeklyBar,
		"{weekly_color}", v.weeklyColor,
		"{weekly_reset}", v.weeklyReset,
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
	// The weekly window resets days out, where "163h20m" is unreadable noise.
	if hours >= 24 {
		return fmt.Sprintf("%dd%dh", hours/24, hours%24)
	}
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
