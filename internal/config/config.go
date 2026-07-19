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

	cfg.ShowCost = getEnvBool("SHOW_COST", cfg.ShowCost)
	cfg.ShowWeekly = getEnvBool("SHOW_WEEKLY", cfg.ShowWeekly)
	cfg.ShowTokens = getEnvBool("SHOW_TOKENS", cfg.ShowTokens)
	cfg.ShowGit = getEnvBool("SHOW_GIT", cfg.ShowGit)
	cfg.ShowGitDirty = getEnvBool("SHOW_GIT_DIRTY", cfg.ShowGitDirty)

	cfg.BarSize = getEnvInt("BAR_SIZE", cfg.BarSize)

	cfg.LimitWarn = getEnvInt("LIMIT_WARN", cfg.LimitWarn)
	cfg.LimitCrit = getEnvInt("LIMIT_CRIT", cfg.LimitCrit)

	cfg.CtxWarn = getEnvInt("CTX_WARN", cfg.CtxWarn)
	cfg.CtxCrit = getEnvInt("CTX_CRIT", cfg.CtxCrit)

	cfg.WeeklyShowAt = getEnvInt("WEEKLY_SHOW_AT", cfg.WeeklyShowAt)

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