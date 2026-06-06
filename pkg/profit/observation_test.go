package profit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendObservationAddsProxyTestMetrics(t *testing.T) {
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                          "proxy-test",
		BillablePromptTokens:           100,
		BillableCompletionTokens:       25,
		UpstreamActualPromptTokens:     80,
		UpstreamActualCompletionTokens: 25,
		UserQuota:                      500000,
		CompressionSavedTokens:         20,
	})

	require.Equal(t, ObservationVersion, other["profit_observe_version"])
	require.Equal(t, "missing_cost_profile", other["profit_cost_status"])
	require.Equal(t, 100, other["billable_prompt_tokens"])
	require.Equal(t, 25, other["billable_completion_tokens"])
	require.Equal(t, 80, other["upstream_actual_prompt_tokens"])
	require.Equal(t, 25, other["upstream_actual_completion_tokens"])
	require.Equal(t, float64(1), other["estimated_revenue_usd"])
	require.Nil(t, other["estimated_upstream_cost_usd"])
	require.Nil(t, other["gross_margin_usd"])
	require.Nil(t, other["gross_margin_pct"])
	require.Equal(t, 20, other["compression_saved_tokens"])
	require.Nil(t, other["cache_saved_usd"])
	require.Nil(t, other["retry_cost_usd"])
}

func TestAppendObservationAddsRouteDecision(t *testing.T) {
	bestMargin := 0.42
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:     "proxy-test",
		UserQuota: 500000,
		RouteDecision: &RouteDecision{
			Mode:                  ModeObserve,
			CandidateCount:        2,
			SelectedChannelID:     1,
			SelectedMarginRank:    2,
			BestChannelID:         2,
			BestChannelName:       "cheap",
			BestCostProfileID:     "cheap-profile",
			BestExpectedMarginUSD: &bestMargin,
			WouldPreferDifferent:  true,
			Candidates: []RouteDecisionCandidate{
				{ChannelID: 1, ChannelName: "selected", Selected: true, MarginRank: 2},
				{ChannelID: 2, ChannelName: "cheap", WouldPrefer: true, MarginRank: 1},
			},
		},
	})

	require.Equal(t, ModeObserve, other[KeyRouteMode])
	require.Equal(t, 2, other[KeyRouteCandidateCount])
	require.Equal(t, 1, other[KeyRouteSelectedChannelID])
	require.Equal(t, 2, other[KeyRouteSelectedMarginRank])
	require.Equal(t, 2, other[KeyRouteBestChannelID])
	require.Equal(t, "cheap", other[KeyRouteBestChannelName])
	require.Equal(t, "cheap-profile", other[KeyRouteBestCostProfileID])
	require.Equal(t, &bestMargin, other[KeyRouteBestExpectedMarginUSD])
	require.Equal(t, true, other[KeyRouteWouldPreferDifferent])
	require.Len(t, other[KeyRouteCandidates], 2)
}

func TestAppendObservationAddsOutputPolicyDecision(t *testing.T) {
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:     "proxy-test",
		UserQuota: 500000,
		OutputPolicyDecision: &OutputPolicyDecision{
			Mode:             ModeCap,
			PolicyID:         "cap-gemini",
			PolicyName:       "Cap Gemini",
			CompletionTokens: 2500,
			DefaultMaxTokens: 1000,
			HardMaxTokens:    2000,
			ExceededDefault:  true,
			ExceededHard:     true,
			RewriteOverLimit: true,
			WouldCap:         true,
			ObserveOnly:      true,
			LiveEnforced:     false,
		},
	})

	require.Equal(t, ModeCap, other[KeyOutputPolicyMode])
	require.Equal(t, "cap-gemini", other[KeyOutputPolicyID])
	require.Equal(t, "Cap Gemini", other[KeyOutputPolicyName])
	require.Equal(t, 2500, other[KeyOutputPolicyCompletionTokens])
	require.Equal(t, 1000, other[KeyOutputPolicyDefaultMaxTokens])
	require.Equal(t, 2000, other[KeyOutputPolicyHardMaxTokens])
	require.Equal(t, true, other[KeyOutputPolicyExceededDefault])
	require.Equal(t, true, other[KeyOutputPolicyExceededHard])
	require.Equal(t, true, other[KeyOutputPolicyRewriteOverLimit])
	require.Equal(t, true, other[KeyOutputPolicyWouldCap])
	require.Equal(t, true, other[KeyOutputPolicyObserveOnly])
	require.Equal(t, false, other[KeyOutputPolicyLiveEnforced])
}

func TestAppendObservationSkipsNonProxyTestGroup(t *testing.T) {
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                "default",
		BillablePromptTokens: 100,
		UserQuota:            500000,
	})

	require.Empty(t, other)
}

func TestAppendObservationClampsNegativeValues(t *testing.T) {
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                          "proxy-test",
		BillablePromptTokens:           -1,
		BillableCompletionTokens:       -2,
		UpstreamActualPromptTokens:     -3,
		UpstreamActualCompletionTokens: -4,
		UserQuota:                      -5,
		CompressionSavedTokens:         -6,
	})

	require.Equal(t, 0, other["billable_prompt_tokens"])
	require.Equal(t, 0, other["billable_completion_tokens"])
	require.Equal(t, 0, other["upstream_actual_prompt_tokens"])
	require.Equal(t, 0, other["upstream_actual_completion_tokens"])
	require.Equal(t, float64(0), other["estimated_revenue_usd"])
	require.Equal(t, 0, other["compression_saved_tokens"])
}

func TestStripUserVisibleFields(t *testing.T) {
	other := map[string]interface{}{
		"profit_observe_version":            1,
		"billable_prompt_tokens":            100,
		"upstream_actual_completion_tokens": 20,
		"compression_mode":                  "stacked",
		"compression_bypassed":              false,
		"compression_bypass_reason":         "no_savings",
		"compression_rules_version":         "omniroute-style-go-v1",
		"profit_route_mode":                 "observe",
		"profit_route_candidates":           []RouteDecisionCandidate{{ChannelID: 1}},
		"output_policy_mode":                "cap",
		"output_policy_would_cap":           true,
		"output_policy_live_enforced":       false,
		"model_ratio":                       1.5,
	}

	StripUserVisibleFields(other)

	require.NotContains(t, other, "profit_observe_version")
	require.NotContains(t, other, "billable_prompt_tokens")
	require.NotContains(t, other, "upstream_actual_completion_tokens")
	require.NotContains(t, other, "compression_mode")
	require.NotContains(t, other, "compression_bypassed")
	require.NotContains(t, other, "compression_bypass_reason")
	require.NotContains(t, other, "compression_rules_version")
	require.NotContains(t, other, "profit_route_mode")
	require.NotContains(t, other, "profit_route_candidates")
	require.NotContains(t, other, "output_policy_mode")
	require.NotContains(t, other, "output_policy_would_cap")
	require.NotContains(t, other, "output_policy_live_enforced")
	require.Equal(t, 1.5, other["model_ratio"])
}
