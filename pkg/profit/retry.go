package profit

type RetryAttemptObservation struct {
	Index                    int      `json:"index"`
	ChannelID                int      `json:"channel_id"`
	ChannelName              string   `json:"channel_name,omitempty"`
	ModelName                string   `json:"model_name,omitempty"`
	PromptTokens             int      `json:"prompt_tokens,omitempty"`
	CompletionTokens         int      `json:"completion_tokens,omitempty"`
	StatusCode               int      `json:"status_code,omitempty"`
	ErrorType                string   `json:"error_type,omitempty"`
	ErrorCode                string   `json:"error_code,omitempty"`
	CostKnown                bool     `json:"cost_known"`
	CostStatus               string   `json:"profit_cost_status"`
	CostProfileID            string   `json:"cost_profile_id,omitempty"`
	CostProfileName          string   `json:"cost_profile_name,omitempty"`
	EstimatedUpstreamCostUSD *float64 `json:"estimated_upstream_cost_usd,omitempty"`
	ExpectedRetryCostUSD     *float64 `json:"expected_retry_cost_usd,omitempty"`
	WillRetry                bool     `json:"will_retry"`
	PlatformBorne            bool     `json:"platform_borne"`
}
