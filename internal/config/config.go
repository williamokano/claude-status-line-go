package config

import (
	"os"
	"strconv"
)

type Config struct {
	ShowCost     bool
	ShowWeekly   bool
	ShowTokens   bool
	ShowGit      bool
	ShowGitDirty bool

	BarSize      int

	LimitWarn    int
	LimitCrit    int

	CtxWarn      int
	CtxCrit      int

	WeeklyShowAt int

	NoColor      bool
	Format       string
}

func DefaultConfig() Config {
	return Config{
		ShowCost:     true,
		ShowWeekly:   true,
		ShowTokens:   true,
		ShowGit:      true,
		ShowGitDirty: true,

		BarSize:      10,

		LimitWarn:    60,
		LimitCrit:    85,

		CtxWarn:      60,
		CtxCrit:      85,

		WeeklyShowAt: 60,
	}
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	// Every setting is namespaced under CSL_, as documented. Reading the bare
	// names would let an unrelated SHOW_GIT or BAR_SIZE already in the user's
	// environment silently reshape the status line.
	cfg.ShowCost = getEnvBool("CSL_SHOW_COST", cfg.ShowCost)
	cfg.ShowWeekly = getEnvBool("CSL_SHOW_WEEKLY", cfg.ShowWeekly)
	cfg.ShowTokens = getEnvBool("CSL_SHOW_TOKENS", cfg.ShowTokens)
	cfg.ShowGit = getEnvBool("CSL_SHOW_GIT", cfg.ShowGit)
	cfg.ShowGitDirty = getEnvBool("CSL_SHOW_GIT_DIRTY", cfg.ShowGitDirty)

	cfg.BarSize = getEnvInt("CSL_BAR_SIZE", cfg.BarSize)

	cfg.LimitWarn = getEnvInt("CSL_LIMIT_WARN", cfg.LimitWarn)
	cfg.LimitCrit = getEnvInt("CSL_LIMIT_CRIT", cfg.LimitCrit)

	cfg.CtxWarn = getEnvInt("CSL_CTX_WARN", cfg.CtxWarn)
	cfg.CtxCrit = getEnvInt("CSL_CTX_CRIT", cfg.CtxCrit)

	cfg.WeeklyShowAt = getEnvInt("CSL_WEEKLY_SHOW_AT", cfg.WeeklyShowAt)

	cfg.NoColor = getEnvBool("NO_COLOR", false)
	cfg.Format = os.Getenv("CSL_FORMAT")

	return cfg, nil
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
