package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildLongContextDecisionSkipsWhenModeOff(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeOff

	decision := BuildLongContextDecisionWithSettings(LongContextInput{
		Group:                DefaultObserveGroup,
		ModelName:            "gpt-test",
		BillablePromptTokens: 64000,
		EstimatedRevenueUSD:  1,
	}, settings)

	require.Nil(t, decision)
}

func TestBuildLongContextDecisionUsesDefaultTiers(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeObserve

	decision := BuildLongContextDecisionWithSettings(LongContextInput{
		Group:                    DefaultObserveGroup,
		ModelName:                "gpt-test",
		BillablePromptTokens:     64000,
		BillableCompletionTokens: 16000,
		EstimatedRevenueUSD:      1,
	}, settings)

	require.NotNil(t, decision)
	require.Equal(t, ModeObserve, decision.Mode)
	require.Equal(t, DefaultLongContextPolicyID, decision.PolicyID)
	require.Equal(t, "32k-128k", decision.TierID)
	require.Equal(t, 64000, decision.ContextTokens)
	require.InDelta(t, 1.25, decision.InputMultiplier, 0.0001)
	require.InDelta(t, 0.8, decision.EstimatedInputRevenueUSD, 0.0001)
	require.InDelta(t, 0.2, decision.SuggestedExtraRevenueUSD, 0.0001)
	require.InDelta(t, 1.2, decision.SuggestedRevenueUSD, 0.0001)
	require.True(t, decision.ObserveOnly)
	require.False(t, decision.LiveEnforced)
}

func TestBuildLongContextDecisionSkipsBaseTier(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeObserve

	decision := BuildLongContextDecisionWithSettings(LongContextInput{
		Group:                DefaultObserveGroup,
		ModelName:            "gpt-test",
		BillablePromptTokens: 32000,
		EstimatedRevenueUSD:  1,
	}, settings)

	require.Nil(t, decision)
}

func TestBuildLongContextDecisionMarksPremiumTier(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeObserve

	decision := BuildLongContextDecisionWithSettings(LongContextInput{
		Group:                DefaultObserveGroup,
		ModelName:            "gpt-test",
		BillablePromptTokens: 600000,
		EstimatedRevenueUSD:  2,
	}, settings)

	require.NotNil(t, decision)
	require.Equal(t, "512k-plus", decision.TierID)
	require.InDelta(t, 2.5, decision.InputMultiplier, 0.0001)
	require.True(t, decision.PremiumRequired)
}

func TestMatchLongContextPolicyPrefersSpecificPolicy(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeObserve
	settings.LongContextPolicies = []LongContextPolicy{
		{
			ID:        "group",
			Enabled:   true,
			Group:     DefaultObserveGroup,
			ModelName: "*",
			Tiers: []LongContextTier{{
				ID:               "group-tier",
				MinContextTokens: 32001,
				InputMultiplier:  1.25,
			}},
		},
		{
			ID:        "specific",
			Enabled:   true,
			Group:     DefaultObserveGroup,
			ChannelID: 7,
			ModelName: "gpt-5*",
			Tiers: []LongContextTier{{
				ID:               "specific-tier",
				MinContextTokens: 32001,
				InputMultiplier:  1.75,
			}},
		},
	}

	policy, ok := MatchLongContextPolicy(settings, LongContextInput{
		Group:     DefaultObserveGroup,
		ModelName: "gpt-5.5",
		ChannelID: 7,
	})

	require.True(t, ok)
	require.Equal(t, "specific", policy.ID)
}
