package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveModelAliasRequiresModeAndProxyTest(t *testing.T) {
	settings := DefaultSettings()
	settings.ModelAliasMode = ModeOff

	decision := ResolveModelAlias(settings, ModelAliasInput{
		Group: DefaultObserveGroup,
		SKU:   "glart-fast",
	})

	require.False(t, decision.Applied)
	require.Equal(t, "mode_off", decision.BypassReason)

	settings.ModelAliasMode = ModeObserve
	decision = ResolveModelAlias(settings, ModelAliasInput{
		Group: "default",
		SKU:   "glart-fast",
	})

	require.False(t, decision.Applied)
	require.Equal(t, "group_not_enabled", decision.BypassReason)
}

func TestResolveModelAliasSelectsHighestPriorityTarget(t *testing.T) {
	settings := DefaultSettings()
	settings.ModelAliasMode = ModeObserve
	settings.ModelAliases = []ModelAlias{
		{
			ID:       "low",
			Enabled:  true,
			Priority: 1,
			Group:    DefaultObserveGroup,
			SKU:      "glart-coder",
			Mode:     ModeObserve,
			Targets: []ModelAliasTarget{
				{ModelName: "gpt-5.4-mini", Priority: 100},
			},
		},
		{
			ID:       "high",
			Name:     "Coder SKU",
			Enabled:  true,
			Priority: 10,
			Group:    DefaultObserveGroup,
			SKU:      "glart-coder",
			Mode:     ModeObserve,
			Targets: []ModelAliasTarget{
				{ModelName: "gpt-5.4-mini", Priority: 10},
				{ModelName: "gpt-5.5", Priority: 100, ChannelID: 4},
			},
		},
	}

	decision := ResolveModelAlias(settings, ModelAliasInput{
		Group: DefaultObserveGroup,
		SKU:   "glart-coder",
	})

	require.True(t, decision.Applied)
	require.Equal(t, "high", decision.AliasID)
	require.Equal(t, "Coder SKU", decision.AliasName)
	require.Equal(t, "glart-coder", decision.SKU)
	require.Equal(t, "gpt-5.5", decision.UpstreamModelName)
	require.Equal(t, 4, decision.TargetChannelID)
	require.Equal(t, 2, decision.CandidateCount)
	require.True(t, decision.ObserveOnly)
}

func TestProfitSettingsValidateModelAliases(t *testing.T) {
	settings := DefaultSettings()
	settings.ModelAliasMode = ModeObserve
	settings.ModelAliases = []ModelAlias{{
		ID:      "bad-group",
		Enabled: true,
		Group:   "default",
		SKU:     "glart-fast",
		Mode:    ModeObserve,
		Targets: []ModelAliasTarget{{
			ModelName: "gpt-5.5",
		}},
	}}
	require.ErrorContains(t, settings.Validate(), "model aliases")

	settings.ModelAliases[0].Group = DefaultObserveGroup
	settings.ModelAliases[0].SKU = "fast"
	require.ErrorContains(t, settings.Validate(), "glart-")

	settings.ModelAliases[0].SKU = "glart-fast"
	settings.ModelAliases[0].Targets = nil
	require.ErrorContains(t, settings.Validate(), "at least one target")
}
