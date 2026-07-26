package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/williamokano/claude-status-line-go/internal/config"
)

func TestShortModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Opus", "Opus 4.8", "Opus 4.8"},
		{"Sonnet", "Sonnet 5", "Sonnet 5"},
		{"Haiku", "Haiku 4.5", "Haiku 4.5"},
		{"Fable", "Fable 5", "Fable 5"},
		{"Unrecognized", "Some Future Model", "Some Future Model"},
		{"Empty", "", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortModel(tt.input)
			if result != tt.expected {
				t.Errorf("shortModel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContextSize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"200k", 200000, "200k"},
		{"1M", 1000000, "1M"},
		{"Custom", 500000, "500000"},
		{"Zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contextSize(tt.input)
			if result != tt.expected {
				t.Errorf("contextSize(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHumanTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"Millions", 2300000, "2.3M"},
		{"Millions exact", 1000000, "1.0M"},
		{"Thousands", 77000, "77k"},
		{"Thousands exact", 1000, "1k"},
		{"Hundreds", 500, "500"},
		{"Zero", 0, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := humanTokens(tt.input)
			if result != tt.expected {
				t.Errorf("humanTokens(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		pct      int
		barSize  int
		expected string
	}{
		{"0%", 0, 10, "░░░░░░░░░░"},
		{"10%", 10, 10, "█░░░░░░░░░"},
		{"50%", 50, 10, "█████░░░░░"},
		{"100%", 100, 10, "██████████"},
		{"Custom bar size 50%", 50, 20, "██████████░░░░░░░░░░"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := progressBar(tt.pct, tt.barSize)
			if result != tt.expected {
				t.Errorf("progressBar(%d, %d) = %q, want %q", tt.pct, tt.barSize, result, tt.expected)
			}
			if utf8.RuneCountInString(result) != tt.barSize {
				t.Errorf("progressBar length = %d, want %d", utf8.RuneCountInString(result), tt.barSize)
			}
		})
	}
}

func TestColorForPercent(t *testing.T) {
	tests := []struct {
		name     string
		pct      int
		warn     int
		crit     int
		expected string
	}{
		{"Below warn", 50, 60, 85, Green},
		{"At warn", 60, 60, 85, Yellow},
		{"Between warn and crit", 75, 60, 85, Yellow},
		{"At crit", 85, 60, 85, Red},
		{"Above crit", 90, 60, 85, Red},
		{"Custom thresholds", 50, 40, 60, Yellow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := colorForPercent(tt.pct, tt.warn, tt.crit)
			if result != tt.expected {
				t.Errorf("colorForPercent(%d, %d, %d) = %q, want %q", tt.pct, tt.warn, tt.crit, result, tt.expected)
			}
		})
	}
}

func TestTimeUntil(t *testing.T) {
	// Add a second of slack so the countdown doesn't tick down into the next
	// lower bucket between building the timestamp and formatting it.
	in := func(d time.Duration) int64 { return time.Now().Add(d + time.Second).Unix() }

	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Minutes only", in(45 * time.Minute), "45m"},
		{"Hours and minutes", in(2*time.Hour + 30*time.Minute), "2h30m"},
		{"Just under a day", in(23*time.Hour + 30*time.Minute), "23h30m"},
		{"Days and hours", in(2*24*time.Hour + 4*time.Hour), "2d4h"},
		{"Whole days", in(6 * 24 * time.Hour), "6d0h"},
		{"Past time", in(-1 * time.Hour), "0m"},
		{"Zero", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := timeUntil(tt.input); result != tt.expected {
				t.Errorf("timeUntil(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestService_Run(t *testing.T) {
	cfg := config.Config{
		ShowCost:     true,
		ShowWeekly:   true,
		ShowTokens:   true,
		ShowGit:      false,
		ShowGitDirty: false,
		BarSize:      10,
		LimitWarn:    60,
		LimitCrit:    85,
		CtxWarn:      60,
		CtxCrit:      85,
		WeeklyShowAt: 60,
	}

	svc := New(cfg)

	input := `{
		"model": {"display_name": "Opus 4.8", "id": "claude-opus-4-8"},
		"workspace": {"project_dir": "/home/user/payments-api", "current_dir": "/home/user/payments-api"},
		"context_window": {
			"used_percentage": 68,
			"context_window_size": 1000000,
			"current_usage": {
				"input_tokens": 420000,
				"output_tokens": 77000,
				"cache_creation_input_tokens": 2300000,
				"cache_read_input_tokens": 0
			}
		},
		"cost": {"total_cost_usd": 7.92, "total_duration_ms": 123456},
		"rate_limits": {
			"five_hour": {"used_percentage": 83, "resets_at": 1784586600},
			"seven_day": {"used_percentage": 74, "resets_at": 1785191400}
		}
	}`

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	rIn, wIn, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = rIn
	go func() {
		defer wIn.Close()
		wIn.WriteString(input)
	}()

	err := svc.Run()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	buf := new(strings.Builder)
	io.Copy(buf, rOut)
	output := buf.String()

	if !strings.Contains(output, "🧠 Opus 4.8·1M") {
		t.Errorf("output missing model: %s", output)
	}
	if !strings.Contains(output, "payments-api") {
		t.Errorf("output missing project: %s", output)
	}
	if !strings.Contains(output, "5h") {
		t.Errorf("output missing rate limit: %s", output)
	}
	if !strings.Contains(output, "CTX") {
		t.Errorf("output missing context: %s", output)
	}
	if !strings.Contains(output, "I420k") {
		t.Errorf("output missing input tokens: %s", output)
	}
	if !strings.Contains(output, "O77k") {
		t.Errorf("output missing output tokens: %s", output)
	}
	if !strings.Contains(output, "⚡2.3M") {
		t.Errorf("output missing cache tokens: %s", output)
	}
	if !weeklySegment.MatchString(output) {
		t.Errorf("output missing weekly bar/percentage: %s", output)
	}
	if !strings.Contains(output, "$7.92") {
		t.Errorf("output missing cost: %s", output)
	}
}

// weeklySegment matches the weekly window rendered the same way as the 5-hour
// one: icon, progress bar, percentage.
var weeklySegment = regexp.MustCompile(`📅7d [█░]{10} 74%`)

// TestService_Run_WeeklyMirrorsFiveHour pins the weekly window to the same
// shape as the 5-hour window — bar, percentage and reset countdown — rather
// than the bare percentage it used to print.
func TestService_Run_WeeklyMirrorsFiveHour(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowGit = false
	cfg.ShowGitDirty = false

	weeklyReset := time.Now().Add(2*24*time.Hour + 4*time.Hour + time.Second).Unix()
	fiveHourReset := time.Now().Add(1*time.Hour + 11*time.Minute + time.Second).Unix()

	input := fmt.Sprintf(`{
		"model": {"display_name": "Opus 5", "id": "claude-opus-5"},
		"workspace": {"project_dir": "/home/user/payments-api"},
		"context_window": {"used_percentage": 37, "context_window_size": 1000000},
		"cost": {"total_cost_usd": 39.80},
		"rate_limits": {
			"five_hour": {"used_percentage": 12, "resets_at": %d},
			"seven_day": {"used_percentage": 62, "resets_at": %d}
		}
	}`, fiveHourReset, weeklyReset)

	output, err := runWithInput(t, New(cfg), input)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	plain := stripANSI(output)

	if !strings.Contains(plain, "🟡5h █░░░░░░░░░ 12% ↺ 1h11m") {
		t.Errorf("five-hour window not rendered as expected: %s", plain)
	}
	if !strings.Contains(plain, "📅7d ██████░░░░ 62% ↺ 2d4h") {
		t.Errorf("weekly window should mirror the five-hour window: %s", plain)
	}
}

// TestService_Run_WeeklyWithoutReset covers Claude Code not reporting a weekly
// reset time: the bar and percentage still render, just without a countdown.
func TestService_Run_WeeklyWithoutReset(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowGit = false
	cfg.ShowGitDirty = false

	input := `{
		"model": {"display_name": "Opus 5", "id": "claude-opus-5"},
		"workspace": {"project_dir": "/home/user/payments-api"},
		"context_window": {"used_percentage": 37, "context_window_size": 1000000},
		"cost": {"total_cost_usd": 1.00},
		"rate_limits": {
			"five_hour": {"used_percentage": 12},
			"seven_day": {"used_percentage": 90}
		}
	}`

	output, err := runWithInput(t, New(cfg), input)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	plain := stripANSI(output)

	if !strings.Contains(plain, "📅7d █████████░ 90%") {
		t.Errorf("weekly window missing: %s", plain)
	}
	if strings.Contains(plain, "↺") {
		t.Errorf("no reset time reported, so no countdown should render: %s", plain)
	}
}

// TestService_Run_WeeklyColor confirms the weekly window is colored by the
// same thresholds as the 5-hour one instead of always being dim.
func TestService_Run_WeeklyColor(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ShowGit = false
	cfg.ShowGitDirty = false

	input := `{
		"model": {"display_name": "Opus 5", "id": "claude-opus-5"},
		"workspace": {"project_dir": "/home/user/payments-api"},
		"context_window": {"used_percentage": 10, "context_window_size": 1000000},
		"rate_limits": {
			"five_hour": {"used_percentage": 5},
			"seven_day": {"used_percentage": 95}
		}
	}`

	output, err := runWithInput(t, New(cfg), input)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(output, Red+"📅7d") {
		t.Errorf("weekly at 95%% should render red (crit): %q", output)
	}
}

// Test with minimal config (no tokens, no weekly, no cost)
func TestService_Run_MinimalConfig(t *testing.T) {
	cfg := config.Config{
		ShowCost:     false,
		ShowWeekly:   false,
		ShowTokens:   false,
		ShowGit:      false,
		ShowGitDirty: false,
		BarSize:      10,
		LimitWarn:    60,
		LimitCrit:    85,
		CtxWarn:      60,
		CtxCrit:      85,
		WeeklyShowAt: 60,
	}

	svc := New(cfg)

	input := `{
		"model": {"display_name": "Sonnet 5", "id": "claude-sonnet-5"},
		"workspace": {"project_dir": "/home/user/test", "current_dir": "/home/user/test"},
		"context_window": {
			"used_percentage": 50,
			"context_window_size": 200000,
			"current_usage": {
				"input_tokens": 100000,
				"output_tokens": 50000,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens": 0
			}
		},
		"cost": {"total_cost_usd": 1.50, "total_duration_ms": 50000},
		"rate_limits": {
			"five_hour": {"used_percentage": 30, "resets_at": 1784586600},
			"seven_day": {"used_percentage": 20, "resets_at": 1785191400}
		}
	}`

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	rIn, wIn, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = rIn
	go func() {
		defer wIn.Close()
		wIn.WriteString(input)
	}()

	err := svc.Run()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	buf := new(strings.Builder)
	io.Copy(buf, rOut)
	output := buf.String()

	// Should not contain tokens, weekly, or cost
	if strings.Contains(output, "I100k") {
		t.Errorf("output should not contain tokens: %s", output)
	}
	if strings.Contains(output, "7d") {
		t.Errorf("output should not contain weekly: %s", output)
	}
	if strings.Contains(output, "$1.50") {
		t.Errorf("output should not contain cost: %s", output)
	}

	// Should contain model, project, rate limit, context
	if !strings.Contains(output, "Sonnet 5·200k") {
		t.Errorf("output missing model: %s", output)
	}
	if !strings.Contains(output, "test") {
		t.Errorf("output missing project: %s", output)
	}
	if !strings.Contains(output, "30%") {
		t.Errorf("output missing rate limit: %s", output)
	}
	if !strings.Contains(output, "CTX") {
		t.Errorf("output missing context: %s", output)
	}
}

func TestService_Run_CustomBarSize(t *testing.T) {
	cfg := config.Config{
		ShowCost:     true,
		ShowWeekly:   true,
		ShowTokens:   true,
		ShowGit:      false,
		ShowGitDirty: false,
		BarSize:      20,
		LimitWarn:    60,
		LimitCrit:    85,
		CtxWarn:      60,
		CtxCrit:      85,
		WeeklyShowAt: 60,
	}

	svc := New(cfg)
	if svc.cfg.BarSize != 20 {
		t.Errorf("BarSize = %d, want 20", svc.cfg.BarSize)
	}

	bar := progressBar(50, svc.cfg.BarSize)
	if utf8.RuneCountInString(bar) != 20 {
		t.Errorf("progressBar length = %d, want 20", utf8.RuneCountInString(bar))
	}
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
		t.Error("progressBar should contain both filled and empty chars")
	}
}

// runWithInput pipes input into svc.Run() and captures whatever it writes to stdout.
func runWithInput(t *testing.T, svc *Service, input string) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	rIn, wIn, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = rIn
	go func() {
		defer wIn.Close()
		wIn.WriteString(input)
	}()

	err := svc.Run()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	buf := new(strings.Builder)
	io.Copy(buf, rOut)
	return buf.String(), err
}

// TestService_Run_PartialJSON reproduces the real-world crash: one field
// arrives as a type the struct doesn't expect (mirrors the actual
// rate_limits.resets_at bug, where Claude Code sends a number but an older
// version of this code declared it as a string). Every other field is
// well-formed and must still render instead of the whole status line going
// silent over one bad field.
func TestService_Run_PartialJSON(t *testing.T) {
	cfg := config.Config{
		ShowCost:     true,
		ShowWeekly:   true,
		ShowTokens:   true,
		ShowGit:      false,
		ShowGitDirty: false,
		BarSize:      10,
		LimitWarn:    60,
		LimitCrit:    85,
		CtxWarn:      60,
		CtxCrit:      85,
		WeeklyShowAt: 60,
	}

	svc := New(cfg)

	input := `{
		"model": {"display_name": "Sonnet 5", "id": "claude-sonnet-5"},
		"workspace": {"project_dir": "/home/user/payments-api", "current_dir": "/home/user/payments-api"},
		"context_window": {
			"used_percentage": 42,
			"context_window_size": 200000,
			"current_usage": {
				"input_tokens": 1000,
				"output_tokens": 200,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens": 0
			}
		},
		"cost": {"total_cost_usd": 3.21, "total_duration_ms": 1000},
		"rate_limits": {
			"five_hour": {"used_percentage": 55, "resets_at": "not-a-unix-timestamp"},
			"seven_day": {"used_percentage": 20, "resets_at": 1785191400}
		}
	}`

	output, err := runWithInput(t, svc, input)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !strings.Contains(output, "🧠 Sonnet 5·200k") {
		t.Errorf("output missing model: %s", output)
	}
	if !strings.Contains(output, "payments-api") {
		t.Errorf("output missing project: %s", output)
	}
	if !strings.Contains(output, "55%") {
		t.Errorf("output missing rate limit: %s", output)
	}
	if !strings.Contains(output, "42%") {
		t.Errorf("output missing context: %s", output)
	}
	if !strings.Contains(output, "$3.21") {
		t.Errorf("output missing cost: %s", output)
	}
}

// TestService_Run_InvalidJSON confirms totally unparseable stdin still
// produces a best-effort status line and a nil error rather than aborting
// with no output, which is what causes Claude Code to blank the status line
// until restart.
func TestService_Run_InvalidJSON(t *testing.T) {
	svc := New(config.DefaultConfig())

	output, err := runWithInput(t, svc, "not json at all")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("expected a best-effort status line, got empty output")
	}
}

// TestService_Run_DumpsFailedInput verifies that unparseable stdin is saved
// to the temp dir under a recognizable name so the payload can be inspected
// after the fact.
func TestService_Run_DumpsFailedInput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	svc := New(config.DefaultConfig())
	raw := `{"model": {"display_name": 123}} trailing garbage`

	if _, err := runWithInput(t, svc, raw); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(tmp, failedInputPrefix+"*.json"))
	if err != nil {
		t.Fatalf("glob error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one dump file, got %d: %v", len(matches), matches)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading dump file: %v", err)
	}
	if string(content) != raw {
		t.Errorf("dump content = %q, want the raw stdin payload %q", content, raw)
	}
}