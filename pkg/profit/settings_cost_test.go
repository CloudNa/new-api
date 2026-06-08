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

func TestProfitSettingsAllowsPreferMarginCostRoutingPreview(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin

	require.NoError(t, settings.Validate())

	normalized := settings.Normalize()
	require.Equal(t, ModePreferMargin, normalized.CostRoutingMode)
	require.True(t, normalized.ObserveOnly)
}

func TestProfitSettingsValidateLimitsObserveScopeToProxyTest(t *testing.T) {
	settings := DefaultSettings()
	settings.ObserveGroups = []string{"default"}
	require.ErrorContains(t, settings.Validate(), "observe_groups")

	settings = DefaultSettings()
	settings.LongContextPolicies = []LongContextPolicy{{
		ID:      "default-long-context",
		Enabled: true,
		Group:   "default",
		Mode:    ModeObserve,
		Tiers: []LongContextTier{{
			ID:              "32k",
			InputMultiplier: 1.25,
		}},
	}}
	require.ErrorContains(t, settings.Validate(), "long context policies")

	settings = DefaultSettings()
	settings.OutputPolicies = []OutputPolicy{{
		ID:               "global-output-cap",
		Enabled:          true,
		Mode:             ModeCap,
		DefaultMaxTokens: 1024,
		HardMaxTokens:    2048,
	}}
	require.ErrorContains(t, settings.Validate(), "output policies")

	settings = DefaultSettings()
	settings.OutputPolicies = []OutputPolicy{{
		ID:               "disabled-default-output-cap",
		Enabled:          false,
		Group:            "default",
		Mode:             ModeCap,
		DefaultMaxTokens: 1024,
		HardMaxTokens:    2048,
	}}
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

func TestDefaultCostProfilesProvideGenericFallbacks(t *testing.T) {
	withProfitOptionMap(t, map[string]string{})

	doc, err := LoadCostProfiles()
	require.NoError(t, err)

	gptEstimate := EstimateCost(CostInput{
		ChannelID:                99,
		Provider:                 "new-sidecar",
		ModelName:                "gpt-5.5",
		UpstreamPromptTokens:     762,
		UpstreamCompletionTokens: 33,
		RevenueUSD:               0.012074,
	}, doc)

	require.True(t, gptEstimate.CostKnown)
	require.Equal(t, CostStatusConfigured, gptEstimate.CostStatus)
	require.Equal(t, "generic-gpt-5-premium", gptEstimate.CostProfileID)
	require.NotNil(t, gptEstimate.EstimatedUpstreamCostUSD)
	require.InDelta(t, 0.0012825, *gptEstimate.EstimatedUpstreamCostUSD, 0.0000001)
	require.NotNil(t, gptEstimate.GrossMarginUSD)

	deepseekEstimate := EstimateCost(CostInput{
		ChannelID:                100,
		ModelName:                "deepseek-chat",
		UpstreamPromptTokens:     1000,
		UpstreamCompletionTokens: 100,
		RevenueUSD:               0.01,
	}, doc)

	require.True(t, deepseekEstimate.CostKnown)
	require.Equal(t, "generic-deepseek-family", deepseekEstimate.CostProfileID)

	kiroEstimate := EstimateCost(CostInput{
		ChannelID:                101,
		ModelName:                "kiro-coder",
		UpstreamPromptTokens:     1000,
		UpstreamCompletionTokens: 100,
		RevenueUSD:               0.01,
	}, doc)

	require.True(t, kiroEstimate.CostKnown)
	require.Equal(t, "generic-kiro-family", kiroEstimate.CostProfileID)

	unknownEstimate := EstimateCost(CostInput{
		ChannelID:                102,
		ModelName:                "future-model-1",
		UpstreamPromptTokens:     1000,
		UpstreamCompletionTokens: 100,
		RevenueUSD:               0.01,
	}, doc)

	require.True(t, unknownEstimate.CostKnown)
	require.Equal(t, "generic-any-model", unknownEstimate.CostProfileID)
}

func TestStoredCostProfilesCanDisableGenericFallback(t *testing.T) {
	stored := CostProfilesDocument{Items: []CostProfile{
		{
			ID:        "generic-gpt-5-premium",
			Name:      "Disable GPT-5 fallback",
			Enabled:   false,
			ModelName: "gpt-5*",
		},
		{
			ID:        "generic-any-model",
			Name:      "Disable universal fallback",
			Enabled:   false,
			ModelName: "*",
		},
	}}
	payload, err := common.Marshal(stored.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	doc, err := LoadCostProfiles()
	require.NoError(t, err)

	estimate := EstimateCost(CostInput{
		ChannelID:                99,
		ModelName:                "gpt-5.5",
		UpstreamPromptTokens:     762,
		UpstreamCompletionTokens: 33,
		RevenueUSD:               0.012074,
	}, doc)

	require.False(t, estimate.CostKnown)
	require.Equal(t, CostStatusMissingCostProfile, estimate.CostStatus)
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

func TestPreviewRoutePreferMarginRemainsObserveOnly(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
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

	preview := PreviewRoute(settings, doc, RoutePreviewRequest{
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
	require.Equal(t, ModePreferMargin, preview.RoutingMode)
	require.Contains(t, preview.Message, "prefer_margin preview")
	require.Equal(t, 1, preview.SelectedIndex)
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
