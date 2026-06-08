package profit

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestAppendObservationAddsProxyTestMetrics(t *testing.T) {
	withProfitOptionMap(t, map[string]string{})
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
	require.Equal(t, 0, other["cache_read_tokens"])
	require.Equal(t, 0, other["cache_write_tokens"])
	require.Nil(t, other["cache_saved_usd"])
	require.Nil(t, other["retry_cost_usd"])
}

func TestAppendObservationAddsCacheSavingsWhenObserved(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeObserve
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                     "cache-profile",
			Name:                   "Cache profile",
			Enabled:                true,
			ChannelID:              9,
			ModelName:              "cached-model",
			InputUSDPerMillion:     10,
			OutputUSDPerMillion:    20,
			CacheReadUSDPerMillion: 1,
		},
	}}
	profilesPayload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{
		SettingsOptionKey:     string(settingsPayload),
		CostProfilesOptionKey: string(profilesPayload),
	})
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                          "proxy-test",
		ChannelID:                      9,
		ModelName:                      "cached-model",
		BillablePromptTokens:           200000,
		BillableCompletionTokens:       1000,
		UpstreamActualPromptTokens:     100000,
		UpstreamActualCompletionTokens: 1000,
		CacheReadTokens:                100000,
		CacheWriteTokens:               50000,
		UserQuota:                      500000,
	})

	require.Equal(t, CostStatusConfigured, other[KeyCostStatus])
	require.Equal(t, 100000, other[KeyCacheReadTokens])
	require.Equal(t, 50000, other[KeyCacheWriteTokens])
	require.NotNil(t, other[KeyCacheSavedUSD])
	require.InDelta(t, 0.9, *(other[KeyCacheSavedUSD].(*float64)), 0.0000001)
}

func TestAppendObservationAddsResponseCacheSavingsWithZeroActualUsage(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeEnforce
	settings.ObserveOnly = false
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "response-cache-profile",
			Name:                "Response cache profile",
			Enabled:             true,
			ChannelID:           9,
			ModelName:           "cached-model",
			InputUSDPerMillion:  10,
			OutputUSDPerMillion: 20,
			FixedRequestUSD:     0.3,
		},
	}}
	profilesPayload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{
		SettingsOptionKey:     string(settingsPayload),
		CostProfilesOptionKey: string(profilesPayload),
	})
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                    "proxy-test",
		ChannelID:                9,
		ModelName:                "cached-model",
		BillablePromptTokens:     100000,
		BillableCompletionTokens: 10000,
		UserQuota:                500000,
		ResponseCacheDecision: &ResponseCacheDecision{
			Mode:       ModeEnforce,
			Eligible:   true,
			Hit:        true,
			WouldHit:   true,
			LiveServed: true,
			RuleID:     "public-help",
			KeyHash:    "abc123",
			Scope:      ResponseCacheScopeGlobal,
		},
	})

	require.Equal(t, CostStatusConfigured, other[KeyCostStatus])
	require.Equal(t, 0, other[KeyUpstreamActualPromptTokens])
	require.Equal(t, 0, other[KeyUpstreamActualCompletionTokens])
	require.Equal(t, true, other[KeyResponseCacheHit])
	require.Equal(t, true, other[KeyResponseCacheLiveServed])
	require.Equal(t, "public-help", other[KeyResponseCacheRuleID])
	require.NotNil(t, other[KeyEstimatedUpstreamCostUSD])
	require.InDelta(t, 0, *(other[KeyEstimatedUpstreamCostUSD].(*float64)), 0.0000001)
	require.NotNil(t, other[KeyResponseCacheSavedUSD])
	require.InDelta(t, 1.5, *(other[KeyResponseCacheSavedUSD].(*float64)), 0.0000001)
}

func TestAppendObservationAddsModelAliasDecision(t *testing.T) {
	withProfitOptionMap(t, map[string]string{})
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                "proxy-test",
		ModelName:            "glart-fast",
		BillablePromptTokens: 100,
		UserQuota:            500000,
		ModelAliasDecision: &ModelAliasDecision{
			Applied:           true,
			Mode:              ModeObserve,
			AliasID:           "proxy-test-glart-fast",
			AliasName:         "glart-fast",
			SKU:               "glart-fast",
			UpstreamModelName: "gpt-5.5",
			TargetChannelID:   4,
			TargetChannelName: "CLIProxyAPI proxy-test",
			CandidateCount:    2,
			ObserveOnly:       true,
		},
	})

	require.Equal(t, true, other[KeyModelAliasApplied])
	require.Equal(t, ModeObserve, other[KeyModelAliasMode])
	require.Equal(t, "proxy-test-glart-fast", other[KeyModelAliasID])
	require.Equal(t, "glart-fast", other[KeyModelAliasSKU])
	require.Equal(t, "gpt-5.5", other[KeyModelAliasUpstreamModel])
	require.Equal(t, 4, other[KeyModelAliasTargetChannelID])
	require.Equal(t, 2, other[KeyModelAliasCandidateCount])
	require.Equal(t, true, other[KeyModelAliasObserveOnly])
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

func TestAppendObservationAddsLongContextDecision(t *testing.T) {
	settings := DefaultSettings()
	settings.LongContextMode = ModeObserve
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{
		SettingsOptionKey: string(settingsPayload),
	})
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                    "proxy-test",
		ModelName:                "gpt-test",
		BillablePromptTokens:     64000,
		BillableCompletionTokens: 16000,
		UserQuota:                500000,
	})

	require.Equal(t, ModeObserve, other[KeyLongContextMode])
	require.Equal(t, DefaultLongContextPolicyID, other[KeyLongContextPolicyID])
	require.Equal(t, "32k-128k", other[KeyLongContextTierID])
	require.Equal(t, 64000, other[KeyLongContextTokens])
	require.Equal(t, 32001, other[KeyLongContextMinTokens])
	require.Equal(t, 128000, other[KeyLongContextMaxTokens])
	require.InDelta(t, 1.25, other[KeyLongContextInputMultiplier], 0.0001)
	require.InDelta(t, 0.8, other[KeyLongContextInputRevenueUSD], 0.0001)
	require.InDelta(t, 0.2, other[KeyLongContextSuggestedExtraUSD], 0.0001)
	require.InDelta(t, 1.2, other[KeyLongContextSuggestedRevenueUSD], 0.0001)
	require.Equal(t, false, other[KeyLongContextPremiumRequired])
	require.Equal(t, true, other[KeyLongContextObserveOnly])
	require.Equal(t, false, other[KeyLongContextLiveEnforced])
}

func TestAppendObservationAddsLowMarginRiskDecision(t *testing.T) {
	settings := DefaultSettings()
	settings.RiskEnforcement = RiskModeAlert
	settings.RiskMinGrossUSD = 0
	settings.RiskMinExpectUSD = 0
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	profiles := CostProfilesDocument{Items: []CostProfile{
		{
			ID:                  "loss-profile",
			Name:                "Loss profile",
			Enabled:             true,
			ChannelID:           7,
			ModelName:           "loss-model",
			InputUSDPerMillion:  1000,
			OutputUSDPerMillion: 1000,
		},
	}}
	profilesPayload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	withProfitOptionMap(t, map[string]string{
		SettingsOptionKey:     string(settingsPayload),
		CostProfilesOptionKey: string(profilesPayload),
	})
	other := map[string]interface{}{}

	AppendObservation(other, ObservationInput{
		Group:                          "proxy-test",
		ChannelID:                      7,
		ModelName:                      "loss-model",
		BillablePromptTokens:           1000,
		BillableCompletionTokens:       1000,
		UpstreamActualPromptTokens:     1000,
		UpstreamActualCompletionTokens: 1000,
		UserQuota:                      1,
	})

	require.Equal(t, RiskModeAlert, other[KeyRiskMode])
	require.Equal(t, true, other[KeyRiskAlert])
	require.Contains(t, other[KeyRiskReasons], RiskReasonGrossMarginBelowMinimum)
	require.Contains(t, other[KeyRiskReasons], RiskReasonExpectedMarginBelowMinimum)
	require.Contains(t, other[KeyRiskReasons], RiskReasonLossMakingRequest)
	require.Equal(t, float64(0), other[KeyRiskMinGrossMarginUSD])
	require.Equal(t, float64(0), other[KeyRiskMinExpectedMarginUSD])
	require.Equal(t, true, other[KeyRiskObserveOnly])
	require.Equal(t, false, other[KeyRiskLiveEnforced])
}

func TestAppendObservationAddsRetryObservation(t *testing.T) {
	other := map[string]interface{}{}
	retryCost := 0.0123

	AppendObservation(other, ObservationInput{
		Group:             "proxy-test",
		UserQuota:         500000,
		RetryCostUSD:      &retryCost,
		RetryAttemptCount: 1,
		RetryAttempts: []RetryAttemptObservation{
			{
				Index:                1,
				ChannelID:            7,
				ChannelName:          "retry-channel",
				ModelName:            "gpt-test",
				PromptTokens:         1200,
				StatusCode:           502,
				ErrorType:            "upstream_error",
				ErrorCode:            "bad_response",
				CostKnown:            true,
				CostStatus:           CostStatusConfigured,
				ExpectedRetryCostUSD: &retryCost,
				WillRetry:            true,
				PlatformBorne:        true,
			},
		},
	})

	require.Equal(t, &retryCost, other[KeyRetryCostUSD])
	require.Equal(t, 1, other[KeyRetryAttemptCount])
	require.Len(t, other[KeyRetryAttempts], 1)
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
		"profit_observe_version":                   1,
		"billable_prompt_tokens":                   100,
		"upstream_actual_completion_tokens":        20,
		"compression_mode":                         "stacked",
		"compression_bypassed":                     false,
		"compression_bypass_reason":                "no_savings",
		"compression_rules_version":                "omniroute-style-go-v1",
		"profit_route_mode":                        "observe",
		"profit_route_candidates":                  []RouteDecisionCandidate{{ChannelID: 1}},
		"output_policy_mode":                       "cap",
		"output_policy_would_cap":                  true,
		"output_policy_live_enforced":              false,
		"profit_risk_mode":                         "alert",
		"profit_risk_alert":                        true,
		"profit_risk_reasons":                      []string{RiskReasonLossMakingRequest},
		"long_context_mode":                        "observe",
		"long_context_suggested_extra_revenue_usd": 0.2,
		"retry_cost_usd":                           0.01,
		"profit_retry_attempt_count":               1,
		"profit_retry_attempts":                    []RetryAttemptObservation{{ChannelID: 1}},
		"response_cache_mode":                      "enforce",
		"response_cache_hit":                       true,
		"response_cache_saved_usd":                 0.12,
		"model_ratio":                              1.5,
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
	require.NotContains(t, other, "profit_risk_mode")
	require.NotContains(t, other, "profit_risk_alert")
	require.NotContains(t, other, "profit_risk_reasons")
	require.NotContains(t, other, "long_context_mode")
	require.NotContains(t, other, "long_context_suggested_extra_revenue_usd")
	require.NotContains(t, other, "retry_cost_usd")
	require.NotContains(t, other, "profit_retry_attempt_count")
	require.NotContains(t, other, "profit_retry_attempts")
	require.NotContains(t, other, "response_cache_mode")
	require.NotContains(t, other, "response_cache_hit")
	require.NotContains(t, other, "response_cache_saved_usd")
	require.Equal(t, 1.5, other["model_ratio"])
}
