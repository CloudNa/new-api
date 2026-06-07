package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOutputPolicyDecisionSkipsWhenModeOff(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModeOff

	decision := BuildOutputPolicyDecisionWithSettings(OutputPolicyInput{
		Group:            DefaultObserveGroup,
		ModelName:        "gpt-test",
		CompletionTokens: 1000,
	}, settings)

	require.Nil(t, decision)
}

func TestBuildOutputPolicyDecisionObservesProxyTestOnly(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModeObserve

	proxyDecision := BuildOutputPolicyDecisionWithSettings(OutputPolicyInput{
		Group:            DefaultObserveGroup,
		ModelName:        "gpt-test",
		CompletionTokens: 1000,
	}, settings)
	defaultDecision := BuildOutputPolicyDecisionWithSettings(OutputPolicyInput{
		Group:            "default",
		ModelName:        "gpt-test",
		CompletionTokens: 1000,
	}, settings)

	require.NotNil(t, proxyDecision)
	require.Equal(t, ModeObserve, proxyDecision.Mode)
	require.True(t, proxyDecision.ObserveOnly)
	require.False(t, proxyDecision.LiveEnforced)
	require.Nil(t, defaultDecision)
}

func TestMatchOutputPolicyPrefersSpecificPolicy(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModeObserve
	settings.OutputPolicies = []OutputPolicy{
		{
			ID:               "group",
			Enabled:          true,
			Group:            DefaultObserveGroup,
			ModelName:        "*",
			DefaultMaxTokens: 8000,
		},
		{
			ID:               "specific-channel-model",
			Enabled:          true,
			Group:            DefaultObserveGroup,
			ChannelID:        7,
			ModelName:        "gemini-2.5*",
			DefaultMaxTokens: 1200,
			HardMaxTokens:    2000,
		},
	}

	policy, ok := MatchOutputPolicy(settings, OutputPolicyInput{
		Group:       DefaultObserveGroup,
		ModelName:   "gemini-2.5-flash",
		ChannelID:   7,
		ChannelName: "cliproxyapi",
	})

	require.True(t, ok)
	require.Equal(t, "specific-channel-model", policy.ID)
	require.Equal(t, 1200, policy.DefaultMaxTokens)
	require.Equal(t, 2000, policy.HardMaxTokens)
}

func TestBuildOutputPolicyDecisionMarksThresholdsAndWouldCap(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModeCap
	settings.OutputPolicies = []OutputPolicy{
		{
			ID:               "cap-gemini",
			Name:             "Cap Gemini",
			Enabled:          true,
			Group:            DefaultObserveGroup,
			ModelName:        "gemini*",
			Mode:             ModeCap,
			DefaultMaxTokens: 1000,
			HardMaxTokens:    2000,
			RewriteOverLimit: true,
		},
	}

	decision := BuildOutputPolicyDecisionWithSettings(OutputPolicyInput{
		Group:            DefaultObserveGroup,
		ModelName:        "gemini-2.5-pro",
		CompletionTokens: 2500,
	}, settings)

	require.NotNil(t, decision)
	require.Equal(t, ModeCap, decision.Mode)
	require.Equal(t, "cap-gemini", decision.PolicyID)
	require.Equal(t, "Cap Gemini", decision.PolicyName)
	require.Equal(t, 2500, decision.CompletionTokens)
	require.Equal(t, 1000, decision.DefaultMaxTokens)
	require.Equal(t, 2000, decision.HardMaxTokens)
	require.True(t, decision.ExceededDefault)
	require.True(t, decision.ExceededHard)
	require.True(t, decision.RewriteOverLimit)
	require.True(t, decision.WouldCap)
	require.True(t, decision.ObserveOnly)
	require.False(t, decision.LiveEnforced)
}

func TestBuildOutputPolicyDecisionMarksPremiumRequired(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModePremiumRequired
	settings.OutputPolicies = []OutputPolicy{
		{
			ID:               "premium-long-output",
			Enabled:          true,
			Group:            DefaultObserveGroup,
			ModelName:        "glart-long",
			Mode:             ModePremiumRequired,
			DefaultMaxTokens: 4096,
			PremiumGroup:     "premium",
		},
	}

	decision := BuildOutputPolicyDecisionWithSettings(OutputPolicyInput{
		Group:            DefaultObserveGroup,
		ModelName:        "glart-long",
		CompletionTokens: 5000,
	}, settings)

	require.NotNil(t, decision)
	require.Equal(t, ModePremiumRequired, decision.Mode)
	require.True(t, decision.ExceededDefault)
	require.True(t, decision.PremiumRequired)
	require.Equal(t, "premium", decision.PremiumGroup)
	require.False(t, decision.LiveEnforced)
}
