package profit

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestBuildRouteDecisionRanksExpectedMargin(t *testing.T) {
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "expensive",
			Name:                "Expensive channel",
			Enabled:             true,
			ChannelID:           1,
			ModelName:           "model-a",
			InputUSDPerMillion:  10,
			OutputUSDPerMillion: 10,
		},
		{
			ID:                  "cheap",
			Name:                "Cheap channel",
			Enabled:             true,
			ChannelID:           2,
			ModelName:           "model-a",
			InputUSDPerMillion:  1,
			OutputUSDPerMillion: 1,
		},
	}}
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	decision := BuildRouteDecision(RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		SelectedChannelID:              1,
		SelectedChannelName:            "expensive",
		ModelName:                      "model-a",
		BillablePromptTokens:           100000,
		BillableCompletionTokens:       100000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 100000,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "expensive", Priority: 20, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
			{ChannelID: 2, ChannelName: "cheap", Priority: 10, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
		},
	})

	require.NotNil(t, decision)
	require.Equal(t, ModeObserve, decision.Mode)
	require.Equal(t, 2, decision.CandidateCount)
	require.Equal(t, 1, decision.SelectedChannelID)
	require.Equal(t, 2, decision.SelectedMarginRank)
	require.Equal(t, 2, decision.BestChannelID)
	require.Equal(t, "cheap", decision.BestCostProfileID)
	require.True(t, decision.WouldPreferDifferent)
	require.Len(t, decision.Candidates, 2)
	require.True(t, decision.Candidates[0].Selected)
	require.Equal(t, 2, decision.Candidates[0].MarginRank)
	require.False(t, decision.Candidates[0].WouldPrefer)
	require.Equal(t, 1, decision.Candidates[1].MarginRank)
	require.True(t, decision.Candidates[1].WouldPrefer)
	require.NotNil(t, decision.BestExpectedMarginUSD)
	require.InDelta(t, 0.8, *decision.BestExpectedMarginUSD, 0.0001)
}

func TestBuildRouteDecisionPreferMarginPreviewRanksExpectedMargin(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "expensive",
			Name:                "Expensive channel",
			Enabled:             true,
			ChannelID:           1,
			ModelName:           "model-a",
			InputUSDPerMillion:  10,
			OutputUSDPerMillion: 10,
		},
		{
			ID:                  "cheap",
			Name:                "Cheap channel",
			Enabled:             true,
			ChannelID:           2,
			ModelName:           "model-a",
			InputUSDPerMillion:  1,
			OutputUSDPerMillion: 1,
		},
	}}
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	profilesPayload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{
		SettingsOptionKey:     string(settingsPayload),
		CostProfilesOptionKey: string(profilesPayload),
	})

	decision := BuildRouteDecision(RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		SelectedChannelID:              1,
		SelectedChannelName:            "expensive",
		ModelName:                      "model-a",
		BillablePromptTokens:           100000,
		BillableCompletionTokens:       100000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 100000,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "expensive", Priority: 20, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
			{ChannelID: 2, ChannelName: "cheap", Priority: 10, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
		},
	})

	require.NotNil(t, decision)
	require.Equal(t, ModePreferMargin, decision.Mode)
	require.Equal(t, 2, decision.BestChannelID)
	require.True(t, decision.WouldPreferDifferent)
	require.Equal(t, 2, decision.SelectedMarginRank)
}

func TestBuildRouteDecisionSkipsWhenCostRoutingOff(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModeOff
	payload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{SettingsOptionKey: string(payload)})

	decision := BuildRouteDecision(RouteDecisionInput{
		Group:             DefaultObserveGroup,
		SelectedChannelID: 1,
		ModelName:         "model-a",
		Candidates:        []RouteCandidateInput{{ChannelID: 1}},
	})

	require.Nil(t, decision)
}

func TestSelectPreferMarginRouteRespectsObserveOnly(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	settings.ObserveOnly = true

	selection := SelectPreferMarginRoute(settings, RouteDecisionInput{
		Group:     DefaultObserveGroup,
		ModelName: "model-a",
		Candidates: []RouteCandidateInput{
			{ChannelID: 1},
			{ChannelID: 2},
		},
	})

	require.False(t, selection.LiveRoutingUsed)
	require.Equal(t, "observe_only", selection.Reason)
}

func TestSelectPreferMarginRouteChoosesBestKnownMargin(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	settings.ObserveOnly = false
	profiles := CostProfilesDocument{Items: []CostProfile{
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
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	selection := SelectPreferMarginRoute(settings, RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		ModelName:                      "model-a",
		BillablePromptTokens:           100000,
		BillableCompletionTokens:       100000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 100000,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "expensive", Priority: 20, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
			{ChannelID: 2, ChannelName: "cheap", Priority: 10, Weight: 100, HealthRequestCount: 20, HealthSuccessRatePct: 100},
		},
	})

	require.True(t, selection.LiveRoutingUsed)
	require.Equal(t, 2, selection.ChannelID)
	require.NotNil(t, selection.Decision)
	require.Equal(t, ModePreferMargin, selection.Decision.Mode)
}

func TestSelectPreferMarginRouteBypassesWhenBestHasInsufficientSamples(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	settings.ObserveOnly = false
	profiles := CostProfilesDocument{Items: []CostProfile{
		{ID: "expensive", Enabled: true, ChannelID: 1, ModelName: "model-a", InputUSDPerMillion: 10, OutputUSDPerMillion: 10},
		{ID: "cheap", Enabled: true, ChannelID: 2, ModelName: "model-a", InputUSDPerMillion: 1, OutputUSDPerMillion: 1},
	}}
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	selection := SelectPreferMarginRoute(settings, RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		ModelName:                      "model-a",
		BillablePromptTokens:           100000,
		BillableCompletionTokens:       100000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 100000,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "expensive", HealthRequestCount: 20, HealthSuccessRatePct: 100},
			{ChannelID: 2, ChannelName: "cheap", HealthRequestCount: 3, HealthSuccessRatePct: 100},
		},
	})

	require.False(t, selection.LiveRoutingUsed)
	require.Equal(t, "insufficient_health_samples", selection.Reason)
	require.NotNil(t, selection.Decision)
	require.Equal(t, 2, selection.Decision.BestChannelID)
	require.Equal(t, "insufficient_health_samples", selection.Decision.BypassReason)
	require.Equal(t, "insufficient_samples", selection.Decision.Candidates[1].HealthStatus)
}

func TestSelectPreferMarginRouteBypassesWhenBestSuccessRateIsLow(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	settings.ObserveOnly = false
	profiles := CostProfilesDocument{Items: []CostProfile{
		{ID: "expensive", Enabled: true, ChannelID: 1, ModelName: "model-a", InputUSDPerMillion: 10, OutputUSDPerMillion: 10},
		{ID: "cheap", Enabled: true, ChannelID: 2, ModelName: "model-a", InputUSDPerMillion: 1, OutputUSDPerMillion: 1},
	}}
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	selection := SelectPreferMarginRoute(settings, RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		ModelName:                      "model-a",
		BillablePromptTokens:           100000,
		BillableCompletionTokens:       100000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 100000,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "expensive", HealthRequestCount: 20, HealthSuccessRatePct: 100},
			{ChannelID: 2, ChannelName: "cheap", HealthRequestCount: 20, HealthSuccessRatePct: 80},
		},
	})

	require.False(t, selection.LiveRoutingUsed)
	require.Equal(t, "below_success_rate", selection.Reason)
	require.NotNil(t, selection.Decision)
	require.Equal(t, 2, selection.Decision.BestChannelID)
	require.Equal(t, "below_success_rate", selection.Decision.BypassReason)
	require.Equal(t, "below_success_rate", selection.Decision.Candidates[1].HealthStatus)
}

func TestSelectPreferMarginRouteRequiresMultipleCandidates(t *testing.T) {
	settings := DefaultSettings()
	settings.CostRoutingMode = ModePreferMargin
	settings.ObserveOnly = false

	selection := SelectPreferMarginRoute(settings, RouteDecisionInput{
		Group:      DefaultObserveGroup,
		ModelName:  "model-a",
		Candidates: []RouteCandidateInput{{ChannelID: 1}},
	})

	require.False(t, selection.LiveRoutingUsed)
	require.Equal(t, "single_candidate", selection.Reason)
}

func TestBuildRouteDecisionUsesFailureRatePenalty(t *testing.T) {
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                         "same-cost-a",
			Enabled:                    true,
			ChannelID:                  1,
			ModelName:                  "model-a",
			InputUSDPerMillion:         1,
			OutputUSDPerMillion:        1,
			FailurePenaltyUSD:          0.1,
			LatencyPenaltyUSDPerSecond: 0,
			RiskPenaltyUSD:             0,
		},
		{
			ID:                         "same-cost-b",
			Enabled:                    true,
			ChannelID:                  2,
			ModelName:                  "model-a",
			InputUSDPerMillion:         1,
			OutputUSDPerMillion:        1,
			FailurePenaltyUSD:          0.1,
			LatencyPenaltyUSDPerSecond: 0,
			RiskPenaltyUSD:             0,
		},
	}}
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{CostProfilesOptionKey: string(payload)})

	decision := BuildRouteDecision(RouteDecisionInput{
		Group:                          DefaultObserveGroup,
		SelectedChannelID:              1,
		ModelName:                      "model-a",
		BillablePromptTokens:           1000,
		BillableCompletionTokens:       0,
		UpstreamActualPromptTokens:     1000,
		UpstreamActualCompletionTokens: 0,
		UserQuota:                      500000,
		Candidates: []RouteCandidateInput{
			{ChannelID: 1, ChannelName: "flaky", FailureRate: 0.9},
			{ChannelID: 2, ChannelName: "healthy", FailureRate: 0.0},
		},
	})

	require.NotNil(t, decision)
	require.Equal(t, 2, decision.BestChannelID)
	require.True(t, decision.WouldPreferDifferent)
	require.Equal(t, 0.9, decision.Candidates[0].FailureRate)
}
