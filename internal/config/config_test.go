package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ShowCost {
		t.Error("ShowCost should be true by default")
	}
	if !cfg.ShowWeekly {
		t.Error("ShowWeekly should be true by default")
	}
	if !cfg.ShowTokens {
		t.Error("ShowTokens should be true by default")
	}
	if !cfg.ShowGit {
		t.Error("ShowGit should be true by default")
	}
	if !cfg.ShowGitDirty {
		t.Error("ShowGitDirty should be true by default")
	}
	if cfg.BarSize != 10 {
		t.Errorf("BarSize should be 10, got %d", cfg.BarSize)
	}
	if cfg.LimitWarn != 60 {
		t.Errorf("LimitWarn should be 60, got %d", cfg.LimitWarn)
	}
	if cfg.LimitCrit != 85 {
		t.Errorf("LimitCrit should be 85, got %d", cfg.LimitCrit)
	}
	if cfg.CtxWarn != 60 {
		t.Errorf("CtxWarn should be 60, got %d", cfg.CtxWarn)
	}
	if cfg.CtxCrit != 85 {
		t.Errorf("CtxCrit should be 85, got %d", cfg.CtxCrit)
	}
	if cfg.WeeklyShowAt != 60 {
		t.Errorf("WeeklyShowAt should be 60, got %d", cfg.WeeklyShowAt)
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected Config
	}{
		{
			name: "all env vars set",
			envVars: map[string]string{
				"SHOW_COST":        "false",
				"SHOW_WEEKLY":      "false",
				"SHOW_TOKENS":      "false",
				"SHOW_GIT":         "false",
				"SHOW_GIT_DIRTY":   "false",
				"BAR_SIZE":         "15",
				"LIMIT_WARN":       "50",
				"LIMIT_CRIT":       "90",
				"CTX_WARN":         "50",
				"CTX_CRIT":         "90",
				"WEEKLY_SHOW_AT":   "70",
			},
			expected: Config{
				ShowCost:     false,
				ShowWeekly:   false,
				ShowTokens:   false,
				ShowGit:      false,
				ShowGitDirty: false,
				BarSize:      15,
				LimitWarn:    50,
				LimitCrit:    90,
				CtxWarn:      50,
				CtxCrit:      90,
				WeeklyShowAt: 70,
			},
		},
		{
			name:     "partial env vars",
			envVars:  map[string]string{"BAR_SIZE": "20", "SHOW_COST": "false"},
			expected: Config{ShowCost: false, BarSize: 20, ShowWeekly: true, ShowTokens: true, ShowGit: true, ShowGitDirty: true, LimitWarn: 60, LimitCrit: 85, CtxWarn: 60, CtxCrit: 85, WeeklyShowAt: 60},
		},
		{
			name:     "empty env vars use defaults",
			envVars:  map[string]string{},
			expected: DefaultConfig(),
		},
		{
			name:     "invalid bool defaults to false",
			envVars:  map[string]string{"SHOW_COST": "invalid"},
			expected: Config{ShowCost: false, ShowWeekly: true, ShowTokens: true, ShowGit: true, ShowGitDirty: true, BarSize: 10, LimitWarn: 60, LimitCrit: 85, CtxWarn: 60, CtxCrit: 85, WeeklyShowAt: 60},
		},
		{
			name:     "invalid int defaults to zero then default",
			envVars:  map[string]string{"BAR_SIZE": "invalid"},
			expected: DefaultConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env
			oldEnv := make(map[string]string)
			for k := range tt.envVars {
				oldEnv[k] = os.Getenv(k)
			}

			// Set test env
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			// Clear env vars not in test to avoid interference
			for k := range tt.envVars {
				if _, ok := tt.envVars[k]; !ok {
					os.Unsetenv(k)
				}
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if cfg != tt.expected {
				t.Errorf("got %+v, want %+v", cfg, tt.expected)
			}

			// Restore env
			for k, v := range oldEnv {
				if v == "" {
					os.Unsetenv(k)
				} else {
					os.Setenv(k, v)
				}
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		val      string
		def      bool
		expected bool
	}{
		{"true string", "TEST_BOOL", "true", false, true},
		{"false string", "TEST_BOOL", "false", true, false},
		{"empty string uses default", "TEST_BOOL", "", true, true},
		{"invalid string uses default", "TEST_BOOL", "invalid", true, true},
		{"1 as true", "TEST_BOOL", "1", false, true},
		{"0 as false", "TEST_BOOL", "0", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.key, tt.val)
			defer os.Unsetenv(tt.key)

			result := getEnvBool(tt.key, tt.def)
			if result != tt.expected {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", tt.key, tt.def, result, tt.expected)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		val      string
		def      int
		expected int
	}{
		{"valid int", "TEST_INT", "42", 0, 42},
		{"negative int", "TEST_INT", "-10", 0, -10},
		{"empty string uses default", "TEST_INT", "", 100, 100},
		{"invalid string uses default", "TEST_INT", "invalid", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.key, tt.val)
			defer os.Unsetenv(tt.key)

			result := getEnvInt(tt.key, tt.def)
			if result != tt.expected {
				t.Errorf("getEnvInt(%q, %v) = %v, want %v", tt.key, tt.def, result, tt.expected)
			}
		})
	}
}