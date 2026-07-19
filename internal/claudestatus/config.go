package claudestatus

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