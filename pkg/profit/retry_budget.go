package profit

const (
	RetryBudgetBypassOff            = "off"
	RetryBudgetBypassBaseRetryFalse = "base_retry_false"
	RetryBudgetBypassNotConfigured  = "not_configured"
	RetryBudgetBypassCostUnknown    = "cost_unknown"
	RetryBudgetReasonBudgetExceeded = "budget_exceeded"
	RetryBudgetReasonLowMargin      = "low_margin_retry"
)

type RetryBudgetInput struct {
	BaseWillRetry       bool
	CurrentRetryCostUSD float64
	NextRetryCostUSD    *float64
	CostEstimate        CostEstimate
}

type RetryBudgetDecision struct {
	Mode                string   `json:"mode,omitempty"`
	MaxRetryCostUSD     float64  `json:"max_retry_cost_usd,omitempty"`
	CurrentRetryCostUSD float64  `json:"current_retry_cost_usd,omitempty"`
	NextRetryCostUSD    *float64 `json:"next_retry_cost_usd,omitempty"`
	BudgetExceeded      bool     `json:"budget_exceeded"`
	LowMargin           bool     `json:"low_margin"`
	WouldSkipRetry      bool     `json:"would_skip_retry"`
	ObserveOnly         bool     `json:"observe_only"`
	LiveEnforced        bool     `json:"live_enforced"`
	BypassReason        string   `json:"bypass_reason,omitempty"`
	Reason              string   `json:"reason,omitempty"`
}

func BuildRetryBudgetDecision(input RetryBudgetInput) RetryBudgetDecision {
	return BuildRetryBudgetDecisionWithSettings(input, CurrentSettings())
}

func BuildRetryBudgetDecisionWithSettings(input RetryBudgetInput, settings Settings) RetryBudgetDecision {
	settings = settings.Normalize()
	decision := RetryBudgetDecision{
		Mode:                settings.RetryBudgetMode,
		MaxRetryCostUSD:     settings.MaxRetryCostUSD,
		CurrentRetryCostUSD: nonNegativeFloat(input.CurrentRetryCostUSD),
		NextRetryCostUSD:    input.NextRetryCostUSD,
		ObserveOnly:         settings.ObserveOnly || settings.RetryBudgetMode != ModeEnforce,
	}
	if settings.RetryBudgetMode == "" || settings.RetryBudgetMode == ModeOff {
		decision.BypassReason = RetryBudgetBypassOff
		return decision
	}
	if !input.BaseWillRetry {
		decision.BypassReason = RetryBudgetBypassBaseRetryFalse
		return decision
	}
	if settings.MaxRetryCostUSD <= 0 && !settings.RetryLowMarginSkip {
		decision.BypassReason = RetryBudgetBypassNotConfigured
		return decision
	}
	if input.NextRetryCostUSD == nil {
		decision.BypassReason = RetryBudgetBypassCostUnknown
		return decision
	}
	nextCost := nonNegativeFloat(*input.NextRetryCostUSD)
	decision.NextRetryCostUSD = float64Ptr(nextCost)
	if settings.MaxRetryCostUSD > 0 && decision.CurrentRetryCostUSD+nextCost > settings.MaxRetryCostUSD {
		decision.BudgetExceeded = true
		decision.WouldSkipRetry = true
		decision.Reason = RetryBudgetReasonBudgetExceeded
	}
	if settings.RetryLowMarginSkip && retryEstimateLowMargin(input.CostEstimate) {
		decision.LowMargin = true
		decision.WouldSkipRetry = true
		if decision.Reason == "" {
			decision.Reason = RetryBudgetReasonLowMargin
		}
	}
	decision.LiveEnforced = decision.WouldSkipRetry && settings.RetryBudgetMode == ModeEnforce && !settings.ObserveOnly
	return decision
}

func (decision RetryBudgetDecision) FinalWillRetry(baseWillRetry bool) bool {
	return baseWillRetry && !decision.LiveEnforced
}

func retryEstimateLowMargin(estimate CostEstimate) bool {
	if estimate.ExpectedMarginUSD != nil && *estimate.ExpectedMarginUSD < 0 {
		return true
	}
	if estimate.GrossMarginUSD != nil && *estimate.GrossMarginUSD < 0 {
		return true
	}
	return false
}

func nonNegativeFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func float64Ptr(value float64) *float64 {
	return &value
}
