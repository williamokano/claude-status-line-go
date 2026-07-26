package config

import (
	"os"
	"reflect"
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
	// All env vars we test
	allEnvVars := []string{
		"CSL_SHOW_COST", "CSL_SHOW_WEEKLY", "CSL_SHOW_TOKENS", "CSL_SHOW_GIT", "CSL_SHOW_GIT_DIRTY",
		"CSL_BAR_SIZE", "CSL_LIMIT_WARN", "CSL_LIMIT_CRIT", "CSL_CTX_WARN", "CSL_CTX_CRIT", "CSL_WEEKLY_SHOW_AT",
	}

	tests := []struct {
		name     string
		envVars  map[string]string
		expected Config
	}{
		{
			name: "all env vars set",
			envVars: map[string]string{
				"CSL_SHOW_COST":      "false",
				"CSL_SHOW_WEEKLY":    "false",
				"CSL_SHOW_TOKENS":    "false",
				"CSL_SHOW_GIT":       "false",
				"CSL_SHOW_GIT_DIRTY": "false",
				"CSL_BAR_SIZE":       "15",
				"CSL_LIMIT_WARN":     "50",
				"CSL_LIMIT_CRIT":     "90",
				"CSL_CTX_WARN":       "50",
				"CSL_CTX_CRIT":       "90",
				"CSL_WEEKLY_SHOW_AT": "70",
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
			envVars:  map[string]string{"CSL_BAR_SIZE": "20", "CSL_SHOW_COST": "false"},
			expected: Config{ShowCost: false, BarSize: 20, ShowWeekly: true, ShowTokens: true, ShowGit: true, ShowGitDirty: true, LimitWarn: 60, LimitCrit: 85, CtxWarn: 60, CtxCrit: 85, WeeklyShowAt: 60},
		},
		{
			name:     "empty env vars use defaults",
			envVars:  map[string]string{},
			expected: DefaultConfig(),
		},
		{
			name:     "invalid bool uses default",
			envVars:  map[string]string{"CSL_SHOW_COST": "invalid"},
			expected: Config{ShowCost: true, ShowWeekly: true, ShowTokens: true, ShowGit: true, ShowGitDirty: true, BarSize: 10, LimitWarn: 60, LimitCrit: 85, CtxWarn: 60, CtxCrit: 85, WeeklyShowAt: 60},
		},
		{
			name:     "invalid int defaults to zero then default",
			envVars:  map[string]string{"CSL_BAR_SIZE": "invalid"},
			expected: DefaultConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load reads a config file now, so point it at an empty dir —
			// otherwise these assertions depend on the machine running them.
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())

			// Save and clear ALL relevant env vars
			oldEnv := make(map[string]string)
			for _, k := range allEnvVars {
				oldEnv[k] = os.Getenv(k)
				os.Unsetenv(k)
			}

			// Set test env
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if !reflect.DeepEqual(cfg, tt.expected) {
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

// TestLoad_IgnoresUnprefixedEnvVars guards the CSL_ namespace. Load used to
// read the bare names, so every documented CSL_* variable was a no-op and an
// unrelated SHOW_GIT or BAR_SIZE already in the user's environment would
// silently reshape the status line.
func TestLoad_IgnoresUnprefixedEnvVars(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("SHOW_COST", "false")
	t.Setenv("SHOW_WEEKLY", "false")
	t.Setenv("SHOW_GIT", "false")
	t.Setenv("BAR_SIZE", "99")
	t.Setenv("WEEKLY_SHOW_AT", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Errorf("unprefixed env vars leaked into config: got %+v, want defaults %+v", cfg, DefaultConfig())
	}
}

// TestLoad_ReadsPrefixedEnvVars is the other half of the contract: the
// documented CSL_* names must actually take effect.
func TestLoad_ReadsPrefixedEnvVars(t *testing.T) {
	t.Setenv("CSL_SHOW_WEEKLY", "false")
	t.Setenv("CSL_WEEKLY_SHOW_AT", "80")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ShowWeekly {
		t.Error("CSL_SHOW_WEEKLY=false should disable the weekly window")
	}
	if cfg.WeeklyShowAt != 80 {
		t.Errorf("CSL_WEEKLY_SHOW_AT = %d, want 80", cfg.WeeklyShowAt)
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
