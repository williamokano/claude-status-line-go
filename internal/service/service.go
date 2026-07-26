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
	"sync"
	"time"

	"github.com/williamokano/claude-status-line-go/internal/config"
	"github.com/williamokano/claude-status-line-go/internal/plugin"
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
		UsedPercentage    float64 `json:"used_percentage"`
		ContextWindowSize int     `json:"context_window_size"`
		CurrentUsage      Usage   `json:"current_usage"`
	} `json:"context_window"`
	Cost struct {
		TotalCostUSD    float64 `json:"total_cost_usd"`
		TotalDurationMS int64   `json:"total_duration_ms"`
	} `json:"cost"`
	RateLimits struct {
		FiveHour RateLimit `json:"five_hour"`
		Weekly   RateLimit `json:"seven_day"`
	} `json:"rate_limits"`
}

type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type RateLimit struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

const (
	Reset   = "\033[0m"
	Dim     = "\033[2m"
	Bold    = "\033[1m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

type Service struct {
	cfg config.Config

	// raw is the stdin payload, handed to command plugins so they see the same
	// session data this tool does.
	raw []byte
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
	tokensTotal string
	cacheHitPct int

	cost float64

	plugins []pluginView
}

// pluginView is one plugin resolved for this render: the finished segment plus
// every field addressable from a format string.
type pluginView struct {
	name   string
	render string
	fields map[string]string
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
	s.raw = data

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
	cached := tokens.CacheCreationInputTokens + tokens.CacheReadInputTokens

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
		tokensCache: humanTokens(cached),
		tokensTotal: humanTokens(tokens.InputTokens + cached),
		cacheHitPct: cacheHitPercent(tokens),

		cost: input.Cost.TotalCostUSD,

		plugins: s.buildPlugins(projectDir(input)),
	}
}

// projectDir is what plugins are keyed and run against. Claude Code doesn't
// always report project_dir, so fall back to the working directory.
func projectDir(input Input) string {
	if input.Workspace.ProjectDir != "" {
		return input.Workspace.ProjectDir
	}
	return input.Workspace.CurrentDir
}

// resolvePlugin and spawnRefresh are variables so tests can swap in a slow
// resolver to prove the fan-out, and observe refreshes without starting
// processes.
var (
	resolvePlugin = func(spec plugin.Spec, projectDir string) (plugin.Output, bool, error) {
		return spec.Resolve(projectDir)
	}
	spawnRefresh = plugin.SpawnRefresh
)

// buildPlugins resolves every configured plugin concurrently. Reading a file
// is fast enough that serialising wouldn't show up today, but command sources
// will each cost their own round trip, and in series those add up on a render
// budget measured in single-digit milliseconds.
//
// A plugin that errors or has nothing to say is dropped: a broken segment must
// never cost you the rest of the status line.
func (s *Service) buildPlugins(projectDir string) []pluginView {
	specs := s.cfg.Plugins
	if len(specs) == 0 {
		return nil
	}

	type result struct {
		pv      pluginView
		err     error
		ok      bool
		refresh bool
	}

	// Each goroutine owns one index, so no mutex is needed to fill this.
	results := make([]result, len(specs))

	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func() {
			defer wg.Done()

			data, needsRefresh, err := resolvePlugin(spec, projectDir)
			results[i].refresh = needsRefresh
			if err != nil {
				results[i].err = err
				return
			}
			if data.Hide {
				return
			}

			pv := pluginView{name: spec.Name, fields: pluginFields(spec, data, s.cfg.BarSize)}
			pv.render = s.renderPlugin(spec, data, pv.fields)
			if stripANSI(pv.render) == "" {
				return
			}
			results[i].pv, results[i].ok = pv, true
		}()
	}
	wg.Wait()

	// Kick off refreshes after rendering has everything it needs. Each one is
	// a detached process, so this costs a fork and nothing else — we never
	// wait for the result, we just make sure the next render has a fresher one.
	for i, r := range results {
		if r.refresh {
			if err := spawnRefresh(specs[i].Name, projectDir, s.raw); err != nil {
				fmt.Fprintf(os.Stderr, "claude-status-line-go: plugin %q: refresh: %v\n", specs[i].Name, err)
			}
		}
	}

	// Walk the specs, not the completion order: goroutines finish whenever
	// they finish, but segments must appear in the order they were configured
	// or the line would reshuffle itself between renders.
	out := make([]pluginView, 0, len(specs))
	for i, r := range results {
		switch {
		case r.err != nil:
			fmt.Fprintf(os.Stderr, "claude-status-line-go: plugin %q: %v\n", specs[i].Name, r.err)
		case r.ok:
			out = append(out, r.pv)
		}
	}

	return out
}

// pluginFields flattens a plugin's output into the values a format string can
// reach. Extras land here too, which is how one plugin exposes several pieces
// of information: {plugin.<name>.<key>} for anything it chose to report.
func pluginFields(spec plugin.Spec, data plugin.Output, barSize int) map[string]string {
	f := map[string]string{
		"icon":  spec.Icon,
		"text":  data.Text,
		"label": data.Label,
		"color": spec.ColorName(data),
	}

	if data.Value != nil {
		f["value"] = trimFloat(*data.Value)
	}
	if data.Max != nil {
		f["max"] = trimFloat(*data.Max)
	}

	if pct := data.Percent(); pct >= 0 {
		f["pct"] = strconv.Itoa(pct)
		f["bar"] = progressBar(pct, barSize)
	}

	// Extras never overwrite a reserved field, so a plugin emitting "value"
	// twice can't produce a surprising segment.
	for k, v := range data.Extra {
		if _, taken := f[k]; !taken {
			f[k] = v
		}
	}

	return f
}

// renderPlugin draws the segment. "raw" opts out entirely; otherwise the host
// owns the appearance so plugin segments match the native ones.
func (s *Service) renderPlugin(spec plugin.Spec, data plugin.Output, f map[string]string) string {
	if data.Raw != "" {
		return data.Raw
	}

	body := spec.Display
	if body == "" {
		body = defaultDisplay(f)
	}
	body = expandFields(body, f)

	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	if c := ansiColor(f["color"]); c != "" {
		return c + body + Reset
	}
	return body
}

// defaultDisplay is the layout a plugin gets when it doesn't ask for one:
// icon, bar, then the most specific text the plugin gave us.
func defaultDisplay(f map[string]string) string {
	parts := make([]string, 0, 3)
	if f["icon"] != "" {
		parts = append(parts, "{icon}")
	}
	if f["bar"] != "" {
		parts = append(parts, "{bar}")
	}

	switch {
	case f["text"] != "":
		parts = append(parts, "{text}")
	case f["value"] != "" && f["max"] != "":
		parts = append(parts, "{value}/{max}")
	case f["value"] != "":
		parts = append(parts, "{value}")
	case f["label"] != "":
		parts = append(parts, "{label}")
	}

	return strings.Join(parts, " ")
}

// expandFields substitutes {key} for any field the plugin reported, plus the
// shared colour tokens. Unknown placeholders are cleared rather than left as
// literal braces on the status line.
func expandFields(tpl string, f map[string]string) string {
	pairs := make([]string, 0, len(f)*2+len(ansiTokens)*2)
	for k, v := range f {
		pairs = append(pairs, "{"+k+"}", v)
	}
	for k, v := range ansiTokens {
		pairs = append(pairs, k, v)
	}
	out := strings.NewReplacer(pairs...).Replace(tpl)
	return unresolvedPlaceholder.ReplaceAllString(out, "")
}

var unresolvedPlaceholder = regexp.MustCompile(`\{[a-zA-Z0-9_.:-]*\}`)

var ansiTokens = map[string]string{
	"{reset}": Reset, "{dim}": Dim, "{bold}": Bold,
	"{red}": Red, "{green}": Green, "{yellow}": Yellow,
	"{blue}": Blue, "{magenta}": Magenta, "{cyan}": Cyan, "{white}": White,
}

func ansiColor(name string) string {
	switch name {
	case "red":
		return Red
	case "green":
		return Green
	case "yellow":
		return Yellow
	case "blue":
		return Blue
	case "magenta":
		return Magenta
	case "cyan":
		return Cyan
	case "white":
		return White
	case "dim":
		return Dim
	default:
		return ""
	}
}

func trimFloat(f float64) string {
	if f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
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
		bottom += fmt.Sprintf(" %s│ Σ%s ↓%s ⚡%d%%", Dim, v.tokensTotal, v.tokensOut, v.cacheHitPct)
	}

	if s.cfg.ShowWeekly && v.weeklyPct >= s.cfg.WeeklyShowAt {
		bottom += fmt.Sprintf(" %s│ ", Dim)
		bottom += usageSegment("📅7d", v.weeklyColor, v.weeklyBar, v.weeklyPct, v.weeklyReset) + Reset
	}

	// Plugins land in declaration order, just before cost — cost reads as the
	// terminator of the line, so appending after it looks like an afterthought.
	for _, p := range v.plugins {
		bottom += fmt.Sprintf(" %s│ %s", Dim, p.render)
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
	// {plugin.<name>} is the finished segment; {plugin.<name>.<field>} reaches
	// the parts, including whatever extra keys the plugin reported. The
	// plugin. prefix keeps the namespace clear of the built-ins, so adding a
	// built-in later can never collide with someone's plugin name.
	pluginPairs := make([]string, 0, 16)
	for _, p := range v.plugins {
		pluginPairs = append(pluginPairs, "{plugin."+p.name+"}", p.render)
		for k, val := range p.fields {
			pluginPairs = append(pluginPairs, "{plugin."+p.name+"."+k+"}", val)
		}
	}

	repl := strings.NewReplacer(append(pluginPairs,
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
		"{tokens_total}", v.tokensTotal,
		"{cache_hit_pct}", strconv.Itoa(v.cacheHitPct),
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
	)...)
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

// cacheHitPercent reports what share of the prompt was served from the cache.
//
// The three input fields partition the prompt: input_tokens is only the part
// that missed the cache entirely, so the prompt size is all three summed. Just
// the read tokens count as a hit — creation tokens were written on this call
// and billed at a premium, so folding them in would report a hit for a miss.
func cacheHitPercent(u Usage) int {
	total := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(u.CacheReadInputTokens) / float64(total) * 100))
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
