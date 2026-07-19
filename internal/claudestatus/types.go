package claudestatus

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
		Weekly   RateLimit `json:"weekly"`
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
	ResetsAt       string  `json:"resets_at"`
}