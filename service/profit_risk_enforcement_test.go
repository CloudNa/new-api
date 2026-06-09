package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func profitRiskEnforceSettings() profit.Settings {
	settings := profit.DefaultSettings()
	settings.ObserveOnly = false
	settings.RiskEnforcement = profit.ModeEnforce
	settings.RiskMinExpectUSD = 0.01
	return settings
}

func profitRiskCostProfiles() profit.CostProfilesDocument {
	return profit.CostProfilesDocument{Items: []profit.CostProfile{
		{
			ID:                  "loss-profile",
			Name:                "Loss profile",
			Enabled:             true,
			ModelName:           "loss-model",
			InputUSDPerMillion:  20,
			OutputUSDPerMillion: 20,
			RiskPenaltyUSD:      0.01,
		},
	}}
}

func TestBuildProfitRiskPrecheckDecisionEnforcesLowExpectedMargin(t *testing.T) {
	withProfitRetrySettingsOptionMap(t, profitRiskEnforceSettings(), profitRiskCostProfiles())
	info := &relaycommon.RelayInfo{
		UsingGroup:      profit.DefaultObserveGroup,
		OriginModelName: "loss-model",
	}

	decision, estimate := BuildProfitRiskPrecheckDecision(info, 1000, 1000, 1)

	require.True(t, estimate.CostKnown)
	require.NotNil(t, decision)
	require.True(t, decision.Alert)
	require.True(t, decision.LiveEnforced)
	require.Contains(t, decision.Reasons, profit.RiskReasonExpectedMarginBelowMinimum)
}

func TestBuildProfitRiskPrecheckDecisionSkipsUnknownCost(t *testing.T) {
	withProfitRetrySettingsOptionMap(t, profitRiskEnforceSettings(), profit.CostProfilesDocument{Items: []profit.CostProfile{
		{ID: "generic-any-model", Name: "Generic any-model fallback estimate", Enabled: false, ModelName: "*"},
	}})
	info := &relaycommon.RelayInfo{
		UsingGroup:      profit.DefaultObserveGroup,
		OriginModelName: "loss-model",
	}

	decision, estimate := BuildProfitRiskPrecheckDecision(info, 1000, 1000, 1)

	require.False(t, estimate.CostKnown)
	require.Nil(t, decision)
}

func TestEnforceProfitRiskBeforeRelayReturnsGuardrailError(t *testing.T) {
	withProfitRetrySettingsOptionMap(t, profitRiskEnforceSettings(), profitRiskCostProfiles())
	originalLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = originalLogConsumeEnabled
	})
	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         2,
		UsingGroup:      profit.DefaultObserveGroup,
		OriginModelName: "loss-model",
	}

	err := EnforceProfitRiskBeforeRelay(c, info, 1000, 1000, 1)

	require.NotNil(t, err)
	require.Equal(t, types.ErrorCodeProfitRiskGuardrail, err.GetErrorCode())
	require.True(t, types.IsSkipRetryError(err))
}
