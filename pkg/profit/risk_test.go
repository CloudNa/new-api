package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRiskDecisionAlertsOnLowMargin(t *testing.T) {
	gross := -0.01
	grossPct := -10.0
	expected := -0.02
	expectedPct := -20.0
	settings := DefaultSettings()
	settings.RiskEnforcement = RiskModeAlert

	decision := BuildRiskDecision(settings, CostEstimate{
		CostKnown:         true,
		GrossMarginUSD:    &gross,
		GrossMarginPct:    &grossPct,
		ExpectedMarginUSD: &expected,
		ExpectedMarginPct: &expectedPct,
	})

	require.NotNil(t, decision)
	require.Equal(t, RiskModeAlert, decision.Mode)
	require.True(t, decision.Alert)
	require.Contains(t, decision.Reasons, RiskReasonGrossMarginBelowMinimum)
	require.Contains(t, decision.Reasons, RiskReasonExpectedMarginBelowMinimum)
	require.Contains(t, decision.Reasons, RiskReasonLossMakingRequest)
	require.True(t, decision.ObserveOnly)
	require.False(t, decision.LiveEnforced)
}

func TestBuildRiskDecisionSkipsWhenRiskOff(t *testing.T) {
	gross := -0.01
	settings := DefaultSettings()
	settings.RiskEnforcement = ModeOff

	decision := BuildRiskDecision(settings, CostEstimate{
		CostKnown:      true,
		GrossMarginUSD: &gross,
	})

	require.Nil(t, decision)
}
