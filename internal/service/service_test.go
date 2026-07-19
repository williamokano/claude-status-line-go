package service

import (
	"io"
	"os"
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
		{"Opus", "Opus 4.7", "O4.7"},
		{"Sonnet", "Claude 3.5 Sonnet", "S5"},
		{"Haiku", "Claude 3 Haiku", "H"},
		{"Unknown", "Unknown Model", "Unknown Model"},
		{"Empty", "", ""},
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
	future := time.Now().Add(2*time.Hour + 30*time.Minute).Format(time.RFC3339)
	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Future time", future, "2h30m"},
		{"Past time", past, "0m"},
		{"Empty string", "", ""},
		{"Invalid format", "invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timeUntil(tt.input)
			if tt.name == "Future time" && result == "" {
				t.Errorf("timeUntil(%q) = %q, want non-empty", tt.input, result)
			}
			if tt.name == "Empty string" && result != "" {
				t.Errorf("timeUntil(%q) = %q, want empty", tt.input, result)
			}
			if tt.name == "Invalid format" && result != "" {
				t.Errorf("timeUntil(%q) = %q, want empty", tt.input, result)
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
		"model": {"display_name": "Opus 4.7", "id": "opus-4.7"},
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
			"five_hour": {"used_percentage": 83, "resets_at": "2026-07-19T22:30:00Z"},
			"weekly": {"used_percentage": 74, "resets_at": "2026-07-26T00:00:00Z"}
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

	if !strings.Contains(output, "🧠 O4.7·1M") {
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
	if !strings.Contains(output, "7d 74%") {
		t.Errorf("output missing weekly: %s", output)
	}
	if !strings.Contains(output, "$7.92") {
		t.Errorf("output missing cost: %s", output)
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
		"model": {"display_name": "Sonnet 3.5", "id": "sonnet-3.5"},
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
			"five_hour": {"used_percentage": 30, "resets_at": "2026-07-19T22:30:00Z"},
			"weekly": {"used_percentage": 20, "resets_at": "2026-07-26T00:00:00Z"}
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
	if !strings.Contains(output, "S5·200k") {
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