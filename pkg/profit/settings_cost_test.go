package profit

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func withProfitOptionMap(t *testing.T, options map[string]string) {
	t.Helper()
	original := common.OptionMap
	common.OptionMap = options
	t.Cleanup(func() {
		common.OptionMap = original
	})
}

func TestProfitSettingsDefaultsObserveProxyTest(t *testing.T) {
	withProfitOptionMap(t, map[string]string{})

	settings, err := LoadSettings()

	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.True(t, settings.ObserveOnly)
	require.Equal(t, []string{DefaultObserveGroup}, settings.ObserveGroups)
	require.Equal(t, ModeObserve, settings.CostRoutingMode)
	require.True(t, EnabledForGroup(DefaultObserveGroup))
	require.False(t, EnabledForGroup("default"))
}

func TestProfitSettingsNormalizeOutputPolicies(t *testing.T) {
	settings := DefaultSettings()
	settings.OutputCapMode = ModeCap
	settings.OutputPolicies = []OutputPolicy{
		{
			ID:               "bad-mode",
			Enabled:          true,
			Group:            " proxy-test ",
			ModelName:        " gemini* ",
			Mode:             "bad",
			DefaultMaxTokens: -1,
			HardMaxTokens:    -2,
		},
		{
			Enabled:   true,
			Group:     " ",
			ModelName: " ",
		},
	}

	normalized := settings.Normalize()

	require.Equal(t, ModeCap, normalized.OutputCapMode)
	require.Len(t, normalized.OutputPolicies, 1)
	require.Equal(t, "bad-mode", normalized.OutputPolicies[0].ID)
	require.Equal(t, DefaultObserveGroup, normalized.OutputPolicies[0].Group)
	require.Equal(t, "gemini*", normalized.OutputPolicies[0].ModelName)
	require.Equal(t, ModeObserve, normalized.OutputPolicies[0].Mode)
	require.Equal(t, 0, normalized.OutputPolicies[0].DefaultMaxTokens)
	require.Equal(t, 0, normalized.OutputPolicies[0].HardMaxTokens)
}

func TestProfitSettingsValidateRiskThresholds(t *testing.T) {
	settings := DefaultSettings()
	settings.RiskEnforcement = RiskModeAlert
	settings.RiskMinGrossUSD = -1

	require.Error(t, settings.Validate())

	settings.RiskMinGrossUSD = 0
	settings.RiskMinExpectPct = 10
	require.NoError(t, settings.Validate())
}

func TestEstimateCostUsesMatchingProfile(t *testing.T) {
	doc := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                         "gemini-flash",
			Name:                       "Gemini Flash upstream",
			Enabled:                    true,
			ChannelID:                  12,
			ModelName:                  "gemini-2.5-flash*",
			InputUSDPerMillion:         0.3,
			OutputUSDPerMillion:        2.5,
			CacheReadUSDPerMillion:     0.03,
			CacheWriteUSDPerMillion:    0.3,
			FixedRequestUSD:            0.001,
			LatencyPenaltyUSDPerSecond: 0.0001,
			RiskPenaltyUSD:             0.002,
		},
	}}

	estimate := EstimateCost(CostInput{
		ChannelID:                12,
		ModelName:                "gemini-2.5-flash-preview",
		UpstreamPromptTokens:     1000000,
		UpstreamCompletionTokens: 100000,
		CacheReadTokens:          500000,
		CacheWriteTokens:         100000,
		RevenueUSD:               1.0,
		LatencyMs:                2000,
	}, doc)

	require.True(t, estimate.CostKnown)
	require.Equal(t, CostStatusConfigured, estimate.CostStatus)
	require.Equal(t, "gemini-flash", estimate.CostProfileID)
	require.NotNil(t, estimate.EstimatedUpstreamCostUSD)
	require.InDelta(t, 0.596, *estimate.EstimatedUpstreamCostUSD, 0.0001)
	require.NotNil(t, estimate.ExpectedCostUSD)
	require.InDelta(t, 0.5982, *estimate.ExpectedCostUSD, 0.0001)
	require.NotNil(t, estimate.GrossMarginUSD)
	require.InDelta(t, 0.404, *estimate.GrossMarginUSD, 0.0001)
}

func TestPreviewRouteIsObserveOnlyAndSelectsBestMargin(t *testing.T) {
	doc := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "expensive",
			Enabled:             true,
			ChannelID:           1,
			ModelName:           "model-a",
			InputUSDPerMillion:  10,
			OutputUSDPerMillion: 10,
		},
		{
			ID:                  "cheap",
			Enabled:             true,
			ChannelID:           2,
			ModelName:           "model-a",
			InputUSDPerMillion:  1,
			OutputUSDPerMillion: 1,
		},
	}}

	preview := PreviewRoute(DefaultSettings(), doc, RoutePreviewRequest{
		Group:      DefaultObserveGroup,
		ModelName:  "model-a",
		RevenueUSD: 1,
		Candidates: []CostInput{
			{ChannelID: 1, ModelName: "model-a", UpstreamPromptTokens: 100000, UpstreamCompletionTokens: 100000, RevenueUSD: 1},
			{ChannelID: 2, ModelName: "model-a", UpstreamPromptTokens: 100000, UpstreamCompletionTokens: 100000, RevenueUSD: 1},
		},
	})

	require.True(t, preview.ObserveOnly)
	require.False(t, preview.LiveRoutingUsed)
	require.Equal(t, 1, preview.SelectedIndex)
	require.False(t, preview.Candidates[0].WouldPrefer)
	require.True(t, preview.Candidates[1].WouldPrefer)
}

func TestAppendObservationAppliesStoredCostProfile(t *testing.T) {
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "channel-7",
			Name:                "Channel 7",
			Enabled:             true,
			ChannelID:           7,
			ModelName:           "gpt-test",
			InputUSDPerMillion:  0.5,
			OutputUSDPerMillion: 1,
		},
	}}
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	other := map[string]interface{}{}
	AppendObservation(other, ObservationInput{
		Group:                          DefaultObserveGroup,
		ChannelID:                      7,
		ModelName:                      "gpt-test",
		BillablePromptTokens:           1000,
		BillableCompletionTokens:       100,
		UpstreamActualPromptTokens:     1000,
		UpstreamActualCompletionTokens: 100,
		UserQuota:                      500000,
	})

	require.Equal(t, CostStatusConfigured, other[KeyCostStatus])
	require.Equal(t, "channel-7", other[KeyCostProfileID])
	require.NotNil(t, other[KeyEstimatedUpstreamCostUSD])
	require.NotNil(t, other[KeyGrossMarginUSD])
}
