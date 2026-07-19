package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetryBudgetObserveWouldSkipWithoutLiveEnforcement(t *testing.T) {
	nextCost := 0.02
	settings := DefaultSettings()
	settings.RetryBudgetMode = ModeObserve
	settings.MaxRetryCostUSD = 0.01

	decision := BuildRetryBudgetDecisionWithSettings(RetryBudgetInput{
		BaseWillRetry:       true,
		CurrentRetryCostUSD: 0,
		NextRetryCostUSD:    &nextCost,
	}, settings)

	require.True(t, decision.BudgetExceeded)
	require.True(t, decision.WouldSkipRetry)
	require.True(t, decision.ObserveOnly)
	require.False(t, decision.LiveEnforced)
	require.Equal(t, RetryBudgetReasonBudgetExceeded, decision.Reason)
	require.True(t, decision.FinalWillRetry(true))
}

func TestRetryBudgetEnforceRequiresObserveOnlyDisabled(t *testing.T) {
	nextCost := 0.02
	settings := DefaultSettings()
	settings.ObserveOnly = false
	settings.RetryBudgetMode = ModeEnforce
	settings.MaxRetryCostUSD = 0.01

	decision := BuildRetryBudgetDecisionWithSettings(RetryBudgetInput{
		BaseWillRetry:       true,
		CurrentRetryCostUSD: 0,
		NextRetryCostUSD:    &nextCost,
	}, settings)

	require.True(t, decision.WouldSkipRetry)
	require.False(t, decision.ObserveOnly)
	require.True(t, decision.LiveEnforced)
	require.False(t, decision.FinalWillRetry(true))
}

func TestRetryBudgetLowMarginSkip(t *testing.T) {
	nextCost := 0.01
	expectedMargin := -0.01
	settings := DefaultSettings()
	settings.RetryBudgetMode = ModeObserve
	settings.RetryLowMarginSkip = true

	decision := BuildRetryBudgetDecisionWithSettings(RetryBudgetInput{
		BaseWillRetry:    true,
		NextRetryCostUSD: &nextCost,
		CostEstimate: CostEstimate{
			ExpectedMarginUSD: &expectedMargin,
		},
	}, settings)

	require.True(t, decision.LowMargin)
	require.True(t, decision.WouldSkipRetry)
	require.Equal(t, RetryBudgetReasonLowMargin, decision.Reason)
	require.True(t, decision.FinalWillRetry(true))
}

func TestRetryBudgetBypassesWhenCostUnknown(t *testing.T) {
	settings := DefaultSettings()
	settings.RetryBudgetMode = ModeObserve
	settings.MaxRetryCostUSD = 0.01

	decision := BuildRetryBudgetDecisionWithSettings(RetryBudgetInput{
		BaseWillRetry: true,
	}, settings)

	require.False(t, decision.WouldSkipRetry)
	require.Equal(t, RetryBudgetBypassCostUnknown, decision.BypassReason)
}
