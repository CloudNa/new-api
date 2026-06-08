package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/QuantumNous/new-api/pkg/promptcompress"

	"github.com/stretchr/testify/require"
)

func resetProfitTestData(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM logs").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM abilities")
	})
}

func insertProfitTestLog(t *testing.T, log *Log, other map[string]interface{}) {
	t.Helper()
	log.Type = LogTypeConsume
	log.Other = common.MapToJsonStr(other)
	require.NoError(t, DB.Create(log).Error)
}

func TestGetProfitEventsFiltersObservedProxyTestLogs(t *testing.T) {
	resetProfitTestData(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     501,
		Name:   "gpt-load",
		Key:    "test-key",
		Group:  profit.DefaultObserveGroup,
		Models: "gpt-4o-mini",
	}).Error)

	insertProfitTestLog(t, &Log{
		CreatedAt:        100,
		UserId:           1,
		Username:         "alice",
		TokenName:        "proxy-token",
		ModelName:        "gpt-4o-mini",
		ChannelId:        501,
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     100,
		CompletionTokens: 20,
		Quota:            500000,
		RequestId:        "req-profit",
	}, map[string]interface{}{
		profit.KeyObserveVersion:                 profit.ObservationVersion,
		profit.KeyCostStatus:                     profit.CostStatusMissingCostProfile,
		profit.KeyBillablePromptTokens:           100,
		profit.KeyBillableCompletionTokens:       20,
		profit.KeyUpstreamActualPromptTokens:     90,
		profit.KeyUpstreamActualCompletionTokens: 20,
		profit.KeyEstimatedRevenueUSD:            1.0,
		profit.KeyEstimatedUpstreamCostUSD:       nil,
		profit.KeyGrossMarginUSD:                 nil,
		profit.KeyGrossMarginPct:                 nil,
		profit.KeyCompressionSavedTokens:         10,
		profit.KeyCompressionMode:                "stacked",
		profit.KeyCompressionEngine:              "stacked",
		profit.KeyCompressionTimestamp:           int64(1710000000123),
		profit.KeyCompressionFallbackApplied:     true,
		profit.KeyCompressionSavingsPercent:      11.5,
		profit.KeyCompressionBypassed:            false,
		profit.KeyCompressionRulesVersion:        "omniroute-style-go-v1",
		profit.KeyCompressionRulesApplied:        []string{"rtk:truncate", "caveman:pleasantries"},
		profit.KeyCompressionPreservedBlocks:     2,
		profit.KeyCompressionRedactedSecrets:     1,
		profit.KeyCompressionValidationWarnings:  []string{"ultra_slm_model_path_ignored_go_native_heuristic_used"},
		profit.KeyCompressionValidationErrors:    []string{"preserved_block_mismatch"},
		profit.KeyCompressionEngineBreakdown: []promptcompress.EngineBreakdownItem{
			{
				Engine:           "rtk",
				OriginalTokens:   100,
				CompressedTokens: 80,
				SavingsPercent:   20,
				TechniquesUsed:   []string{"rtk-filter"},
				RulesApplied:     []string{"rtk:go-test"},
				DurationMs:       3,
			},
		},
		profit.KeyRouteMode:                  profit.ModeObserve,
		profit.KeyRouteCandidateCount:        2,
		profit.KeyRouteSelectedChannelID:     501,
		profit.KeyRouteSelectedMarginRank:    2,
		profit.KeyRouteBestChannelID:         502,
		profit.KeyRouteBestChannelName:       "cheap",
		profit.KeyRouteBestCostProfileID:     "cheap-profile",
		profit.KeyRouteBestExpectedMarginUSD: 0.88,
		profit.KeyRouteWouldPreferDifferent:  true,
		profit.KeyRouteCandidates: []profit.RouteDecisionCandidate{
			{ChannelID: 501, ChannelName: "gpt-load", Selected: true, MarginRank: 2},
			{ChannelID: 502, ChannelName: "cheap", WouldPrefer: true, MarginRank: 1},
		},
		profit.KeyOutputPolicyMode:               profit.ModeCap,
		profit.KeyOutputPolicyID:                 "cap-gemini",
		profit.KeyOutputPolicyName:               "Cap Gemini",
		profit.KeyOutputPolicyCompletionTokens:   2500,
		profit.KeyOutputPolicyDefaultMaxTokens:   1000,
		profit.KeyOutputPolicyHardMaxTokens:      2000,
		profit.KeyOutputPolicyExceededDefault:    true,
		profit.KeyOutputPolicyExceededHard:       true,
		profit.KeyOutputPolicyRewriteOverLimit:   true,
		profit.KeyOutputPolicyWouldCap:           true,
		profit.KeyOutputPolicyObserveOnly:        true,
		profit.KeyOutputPolicyLiveEnforced:       false,
		profit.KeyRiskMode:                       profit.RiskModeAlert,
		profit.KeyRiskAlert:                      true,
		profit.KeyRiskReasons:                    []string{profit.RiskReasonLossMakingRequest},
		profit.KeyRiskMinGrossMarginUSD:          0.01,
		profit.KeyRiskMinExpectedMarginUSD:       0.01,
		profit.KeyRiskObserveOnly:                true,
		profit.KeyRiskLiveEnforced:               false,
		profit.KeyLongContextMode:                profit.ModeObserve,
		profit.KeyLongContextPolicyID:            "proxy-test-long-context-premium",
		profit.KeyLongContextPolicyName:          "proxy-test long context premium observe",
		profit.KeyLongContextTierID:              "32k-128k",
		profit.KeyLongContextTierName:            "32k-128k",
		profit.KeyLongContextTokens:              64000,
		profit.KeyLongContextMinTokens:           32001,
		profit.KeyLongContextMaxTokens:           128000,
		profit.KeyLongContextInputMultiplier:     1.25,
		profit.KeyLongContextInputRevenueUSD:     0.8,
		profit.KeyLongContextSuggestedExtraUSD:   0.2,
		profit.KeyLongContextSuggestedRevenueUSD: 1.2,
		profit.KeyLongContextPremiumRequired:     false,
		profit.KeyLongContextPremiumGroup:        "premium",
		profit.KeyLongContextObserveOnly:         true,
		profit.KeyLongContextLiveEnforced:        false,
		profit.KeyCacheSavedUSD:                  nil,
		profit.KeyRetryCostUSD:                   nil,
	})
	insertProfitTestLog(t, &Log{
		CreatedAt: 90,
		Username:  "bob",
		ModelName: "gpt-4o-mini",
		Group:     profit.DefaultObserveGroup,
	}, map[string]interface{}{
		"model_ratio": 1.5,
	})

	events, total, err := GetProfitEvents(ProfitLogFilter{Group: profit.DefaultObserveGroup}, 0, 10)

	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, events, 1)
	require.Equal(t, "alice", events[0].Username)
	require.Equal(t, "gpt-load", events[0].ChannelName)
	require.Equal(t, int64(100), events[0].BillablePromptTokens)
	require.Equal(t, int64(90), events[0].UpstreamActualPromptTokens)
	require.Nil(t, events[0].EstimatedUpstreamCostUSD)
	require.False(t, events[0].CostKnown)
	require.Equal(t, "stacked", events[0].CompressionMode)
	require.NotNil(t, events[0].CompressionSavingsPercent)
	require.InDelta(t, 11.5, *events[0].CompressionSavingsPercent, 0.0001)
	require.False(t, events[0].CompressionBypassed)
	require.Equal(t, "stacked", events[0].CompressionEngine)
	require.Equal(t, int64(1710000000123), events[0].CompressionTimestamp)
	require.True(t, events[0].CompressionFallbackApplied)
	require.Equal(t, "omniroute-style-go-v1", events[0].CompressionRulesVersion)
	require.Equal(t, []string{"rtk:truncate", "caveman:pleasantries"}, events[0].CompressionRulesApplied)
	require.Equal(t, int64(2), events[0].CompressionPreservedBlocks)
	require.Equal(t, int64(1), events[0].CompressionRedactedSecrets)
	require.Equal(t, []string{"ultra_slm_model_path_ignored_go_native_heuristic_used"}, events[0].CompressionValidationWarnings)
	require.Equal(t, []string{"preserved_block_mismatch"}, events[0].CompressionValidationErrors)
	require.Len(t, events[0].CompressionEngineBreakdown, 1)
	require.Equal(t, "rtk", events[0].CompressionEngineBreakdown[0].Engine)
	require.Equal(t, 80, events[0].CompressionEngineBreakdown[0].CompressedTokens)
	require.Equal(t, []string{"rtk:go-test"}, events[0].CompressionEngineBreakdown[0].RulesApplied)
	require.Equal(t, profit.ModeObserve, events[0].RouteMode)
	require.Equal(t, int64(2), events[0].RouteCandidateCount)
	require.Equal(t, 501, events[0].RouteSelectedChannelID)
	require.Equal(t, int64(2), events[0].RouteSelectedMarginRank)
	require.Equal(t, 502, events[0].RouteBestChannelID)
	require.Equal(t, "cheap", events[0].RouteBestChannelName)
	require.Equal(t, "cheap-profile", events[0].RouteBestCostProfileID)
	require.NotNil(t, events[0].RouteBestExpectedMarginUSD)
	require.InDelta(t, 0.88, *events[0].RouteBestExpectedMarginUSD, 0.0001)
	require.True(t, events[0].RouteWouldPreferDifferent)
	require.Len(t, events[0].RouteCandidates, 2)
	require.True(t, events[0].RouteCandidates[0].Selected)
	require.Equal(t, profit.ModeCap, events[0].OutputPolicyMode)
	require.Equal(t, "cap-gemini", events[0].OutputPolicyID)
	require.Equal(t, "Cap Gemini", events[0].OutputPolicyName)
	require.Equal(t, int64(2500), events[0].OutputPolicyCompletionTokens)
	require.Equal(t, int64(1000), events[0].OutputPolicyDefaultMaxTokens)
	require.Equal(t, int64(2000), events[0].OutputPolicyHardMaxTokens)
	require.True(t, events[0].OutputPolicyExceededDefault)
	require.True(t, events[0].OutputPolicyExceededHard)
	require.True(t, events[0].OutputPolicyRewriteOverLimit)
	require.True(t, events[0].OutputPolicyWouldCap)
	require.True(t, events[0].OutputPolicyObserveOnly)
	require.False(t, events[0].OutputPolicyLiveEnforced)
	require.Equal(t, profit.RiskModeAlert, events[0].RiskMode)
	require.True(t, events[0].RiskAlert)
	require.Equal(t, []string{profit.RiskReasonLossMakingRequest}, events[0].RiskReasons)
	require.InDelta(t, 0.01, events[0].RiskMinGrossMarginUSD, 0.0001)
	require.InDelta(t, 0.01, events[0].RiskMinExpectedMarginUSD, 0.0001)
	require.True(t, events[0].RiskObserveOnly)
	require.False(t, events[0].RiskLiveEnforced)
	require.Equal(t, profit.ModeObserve, events[0].LongContextMode)
	require.Equal(t, "proxy-test-long-context-premium", events[0].LongContextPolicyID)
	require.Equal(t, "proxy-test long context premium observe", events[0].LongContextPolicyName)
	require.Equal(t, "32k-128k", events[0].LongContextTierID)
	require.Equal(t, "32k-128k", events[0].LongContextTierName)
	require.Equal(t, int64(64000), events[0].LongContextTokens)
	require.Equal(t, int64(32001), events[0].LongContextMinTokens)
	require.Equal(t, int64(128000), events[0].LongContextMaxTokens)
	require.InDelta(t, 1.25, events[0].LongContextInputMultiplier, 0.0001)
	require.InDelta(t, 0.8, events[0].LongContextInputRevenueUSD, 0.0001)
	require.InDelta(t, 0.2, events[0].LongContextSuggestedExtraUSD, 0.0001)
	require.InDelta(t, 1.2, events[0].LongContextSuggestedRevenueUSD, 0.0001)
	require.False(t, events[0].LongContextPremiumRequired)
	require.Equal(t, "premium", events[0].LongContextPremiumGroup)
	require.True(t, events[0].LongContextObserveOnly)
	require.False(t, events[0].LongContextLiveEnforced)
}

func TestGetSatisfiedChannelCandidatesForProfitObservationDB(t *testing.T) {
	resetProfitTestData(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCache
	})

	lowPriority := int64(1)
	highPriority := int64(10)
	lowWeight := uint(10)
	highWeight := uint(20)
	require.NoError(t, DB.Create(&[]Channel{
		{
			Id:     601,
			Name:   "low",
			Key:    "test-key-low",
			Status: common.ChannelStatusEnabled,
			Weight: &lowWeight,
		},
		{
			Id:           602,
			Name:         "high",
			Key:          "test-key-high",
			Status:       common.ChannelStatusEnabled,
			Weight:       &highWeight,
			ResponseTime: 150,
		},
		{
			Id:     603,
			Name:   "disabled",
			Key:    "test-key-disabled",
			Status: common.ChannelStatusManuallyDisabled,
			Weight: &highWeight,
		},
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: profit.DefaultObserveGroup, Model: "gemini-test", ChannelId: 601, Enabled: true, Priority: &lowPriority, Weight: lowWeight},
		{Group: profit.DefaultObserveGroup, Model: "gemini-test", ChannelId: 602, Enabled: true, Priority: &highPriority, Weight: highWeight},
		{Group: profit.DefaultObserveGroup, Model: "gemini-test", ChannelId: 603, Enabled: true, Priority: &highPriority, Weight: highWeight},
	}).Error)

	candidates, err := GetSatisfiedChannelCandidatesForProfitObservation(profit.DefaultObserveGroup, "gemini-test", 8)

	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, 602, candidates[0].ChannelID)
	require.Equal(t, "high", candidates[0].ChannelName)
	require.Equal(t, int64(10), candidates[0].Priority)
	require.Equal(t, 20, candidates[0].Weight)
	require.Equal(t, 150, candidates[0].ResponseTime)
	require.Equal(t, 601, candidates[1].ChannelID)
}

func TestGetProfitAnalyticsKeepsUnknownCostSeparate(t *testing.T) {
	resetProfitTestData(t)

	insertProfitTestLog(t, &Log{
		CreatedAt:        100,
		Username:         "alice",
		ModelName:        "gpt-4o-mini",
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     100,
		CompletionTokens: 20,
		Quota:            500000,
	}, map[string]interface{}{
		profit.KeyObserveVersion:                 profit.ObservationVersion,
		profit.KeyCostStatus:                     profit.CostStatusMissingCostProfile,
		profit.KeyBillablePromptTokens:           100,
		profit.KeyBillableCompletionTokens:       20,
		profit.KeyUpstreamActualPromptTokens:     90,
		profit.KeyUpstreamActualCompletionTokens: 20,
		profit.KeyEstimatedRevenueUSD:            1.0,
		profit.KeyEstimatedUpstreamCostUSD:       nil,
		profit.KeyGrossMarginUSD:                 nil,
		profit.KeyGrossMarginPct:                 nil,
		profit.KeyCompressionSavedTokens:         10,
		profit.KeyOutputPolicyMode:               profit.ModeCap,
		profit.KeyOutputPolicyCompletionTokens:   2500,
		profit.KeyOutputPolicyDefaultMaxTokens:   1000,
		profit.KeyOutputPolicyHardMaxTokens:      2000,
		profit.KeyOutputPolicyExceededDefault:    true,
		profit.KeyOutputPolicyExceededHard:       true,
		profit.KeyOutputPolicyWouldCap:           true,
		profit.KeyOutputPolicyPremiumRequired:    true,
		profit.KeyRiskMode:                       profit.RiskModeAlert,
		profit.KeyRiskAlert:                      true,
		profit.KeyRiskReasons: []string{
			profit.RiskReasonLossMakingRequest,
			profit.RiskReasonGrossMarginBelowMinimum,
			profit.RiskReasonExpectedMarginBelowMinimum,
		},
		profit.KeyLongContextMode:              profit.ModeObserve,
		profit.KeyLongContextTokens:            64000,
		profit.KeyLongContextSuggestedExtraUSD: 0.2,
		profit.KeyLongContextPremiumRequired:   true,
		profit.KeyCacheSavedUSD:                nil,
		profit.KeyRetryCostUSD:                 nil,
	})
	insertProfitTestLog(t, &Log{
		CreatedAt:        110,
		Username:         "alice",
		ModelName:        "gpt-4o-mini",
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     50,
		CompletionTokens: 10,
		Quota:            250000,
	}, map[string]interface{}{
		profit.KeyObserveVersion:                 profit.ObservationVersion,
		profit.KeyCostStatus:                     "configured",
		profit.KeyBillablePromptTokens:           50,
		profit.KeyBillableCompletionTokens:       10,
		profit.KeyUpstreamActualPromptTokens:     45,
		profit.KeyUpstreamActualCompletionTokens: 10,
		profit.KeyEstimatedRevenueUSD:            0.5,
		profit.KeyEstimatedUpstreamCostUSD:       0.2,
		profit.KeyCompressionSavedTokens:         5,
	})

	analytics, err := GetProfitAnalytics(ProfitLogFilter{Group: profit.DefaultObserveGroup})

	require.NoError(t, err)
	require.Equal(t, int64(2), analytics.RequestCount)
	require.Equal(t, int64(2), analytics.TotalMatchingLogs)
	require.Equal(t, int64(1), analytics.CostKnownCount)
	require.Equal(t, int64(1), analytics.MissingCostProfileCount)
	require.Equal(t, int64(150), analytics.BillablePromptTokens)
	require.Equal(t, int64(30), analytics.BillableCompletionTokens)
	require.Equal(t, int64(135), analytics.UpstreamActualPromptTokens)
	require.Equal(t, int64(30), analytics.UpstreamActualCompletionTokens)
	require.InDelta(t, 1.5, analytics.EstimatedRevenueUSD, 0.0001)
	require.NotNil(t, analytics.EstimatedUpstreamCostUSD)
	require.InDelta(t, 0.2, *analytics.EstimatedUpstreamCostUSD, 0.0001)
	require.NotNil(t, analytics.GrossMarginUSD)
	require.InDelta(t, 0.3, *analytics.GrossMarginUSD, 0.0001)
	require.NotNil(t, analytics.GrossMarginPct)
	require.InDelta(t, 60.0, *analytics.GrossMarginPct, 0.0001)
	require.Equal(t, int64(15), analytics.CompressionSavedTokens)
	require.Equal(t, int64(1), analytics.OutputPolicyObservedCount)
	require.Equal(t, int64(2500), analytics.OutputPolicyCompletionTokens)
	require.Equal(t, int64(1), analytics.OutputPolicyExceededDefaultCount)
	require.Equal(t, int64(1), analytics.OutputPolicyExceededHardCount)
	require.Equal(t, int64(1), analytics.OutputPolicyWouldCapCount)
	require.Equal(t, int64(1), analytics.OutputPolicyPremiumRequiredCount)
	require.Equal(t, int64(1), analytics.RiskObservedCount)
	require.Equal(t, int64(1), analytics.RiskAlertCount)
	require.Equal(t, int64(1), analytics.RiskLossMakingCount)
	require.Equal(t, int64(1), analytics.RiskLowGrossMarginCount)
	require.Equal(t, int64(1), analytics.RiskLowExpectedMarginCount)
	require.Equal(t, int64(1), analytics.LongContextObservedCount)
	require.Equal(t, int64(1), analytics.LongContextPremiumRequiredCount)
	require.Equal(t, int64(64000), analytics.LongContextTokens)
	require.InDelta(t, 0.2, analytics.LongContextSuggestedExtraUSD, 0.0001)
	require.Nil(t, analytics.CacheSavedUSD)
	require.Nil(t, analytics.RetryCostUSD)
}

func TestGetProfitAnalyticsOutputPolicyPercentiles(t *testing.T) {
	resetProfitTestData(t)

	for i := 1; i <= 20; i++ {
		insertProfitTestLog(t, &Log{
			CreatedAt:        int64(100 + i),
			Username:         "alice",
			ModelName:        "gpt-5.5",
			Group:            profit.DefaultObserveGroup,
			PromptTokens:     10,
			CompletionTokens: i * 100,
			Quota:            1000,
		}, map[string]interface{}{
			profit.KeyObserveVersion:               profit.ObservationVersion,
			profit.KeyOutputPolicyMode:             profit.ModeCap,
			profit.KeyOutputPolicyCompletionTokens: i * 100,
		})
	}
	insertProfitTestLog(t, &Log{
		CreatedAt:        200,
		Username:         "alice",
		ModelName:        "gpt-5.5",
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     10,
		CompletionTokens: 0,
		Quota:            1000,
	}, map[string]interface{}{
		profit.KeyObserveVersion:               profit.ObservationVersion,
		profit.KeyOutputPolicyMode:             profit.ModeCap,
		profit.KeyOutputPolicyCompletionTokens: 0,
	})

	analytics, err := GetProfitAnalytics(ProfitLogFilter{Group: profit.DefaultObserveGroup})

	require.NoError(t, err)
	require.Equal(t, int64(21), analytics.OutputPolicyObservedCount)
	require.Equal(t, int64(20), analytics.OutputPolicyCompletionSampleCount)
	require.Equal(t, int64(21000), analytics.OutputPolicyCompletionTokens)
	require.InDelta(t, 1050, analytics.OutputPolicyCompletionAvgTokens, 0.0001)
	require.Equal(t, int64(1900), analytics.OutputPolicyCompletionP95Tokens)
	require.Equal(t, int64(2000), analytics.OutputPolicyCompletionP99Tokens)
	require.Equal(t, int64(2000), analytics.OutputPolicyCompletionMaxTokens)
	require.Equal(t, int64(1920), analytics.OutputPolicyRecommendedDefaultMax)
	require.Equal(t, int64(2048), analytics.OutputPolicyRecommendedHardMax)
	require.Equal(t, "medium", analytics.OutputPolicyRecommendationConfidence)
	require.Equal(t, "p95_p99_observed", analytics.OutputPolicyRecommendationReason)
}

func TestGetProfitAnalyticsOutputPolicyRecommendationNeedsSamples(t *testing.T) {
	resetProfitTestData(t)

	insertProfitTestLog(t, &Log{
		CreatedAt:        100,
		Username:         "alice",
		ModelName:        "gpt-5.5",
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     10,
		CompletionTokens: 200,
		Quota:            1000,
	}, map[string]interface{}{
		profit.KeyObserveVersion:               profit.ObservationVersion,
		profit.KeyOutputPolicyMode:             profit.ModeCap,
		profit.KeyOutputPolicyCompletionTokens: 200,
	})

	analytics, err := GetProfitAnalytics(ProfitLogFilter{Group: profit.DefaultObserveGroup})

	require.NoError(t, err)
	require.Equal(t, int64(1), analytics.OutputPolicyCompletionSampleCount)
	require.Equal(t, int64(256), analytics.OutputPolicyRecommendedDefaultMax)
	require.Equal(t, int64(256), analytics.OutputPolicyRecommendedHardMax)
	require.Equal(t, "low", analytics.OutputPolicyRecommendationConfidence)
	require.Equal(t, "insufficient_samples", analytics.OutputPolicyRecommendationReason)
}

func TestGetProfitAnalyticsGuardrailSuggestsOutputCapForMarginRisk(t *testing.T) {
	resetProfitTestData(t)

	for i := 1; i <= 20; i++ {
		insertProfitTestLog(t, &Log{
			CreatedAt:        int64(100 + i),
			Username:         "alice",
			ModelName:        "gpt-5.5",
			Group:            profit.DefaultObserveGroup,
			PromptTokens:     10,
			CompletionTokens: i * 100,
			Quota:            1000,
		}, map[string]interface{}{
			profit.KeyObserveVersion:               profit.ObservationVersion,
			profit.KeyOutputPolicyMode:             profit.ModeCap,
			profit.KeyOutputPolicyCompletionTokens: i * 100,
			profit.KeyRiskMode:                     profit.RiskModeAlert,
			profit.KeyRiskAlert:                    true,
			profit.KeyRiskReasons: []string{
				profit.RiskReasonExpectedMarginBelowMinimum,
			},
		})
	}

	analytics, err := GetProfitAnalytics(ProfitLogFilter{Group: profit.DefaultObserveGroup})

	require.NoError(t, err)
	require.Equal(t, int64(20), analytics.RiskLowExpectedMarginCount)
	require.Equal(t, "enable_output_cap_observe", analytics.ProfitGuardrailAction)
	require.Equal(t, "low_margin_with_output_tail", analytics.ProfitGuardrailReason)
	require.Equal(t, "medium", analytics.ProfitGuardrailConfidence)
	require.Equal(t, profit.ModeCap, analytics.ProfitGuardrailOutputMode)
	require.Equal(t, int64(1920), analytics.ProfitGuardrailDefaultMaxTokens)
	require.Equal(t, int64(2048), analytics.ProfitGuardrailHardMaxTokens)
}

func TestGetProfitAnalyticsGuardrailCollectsSamplesBeforeCap(t *testing.T) {
	resetProfitTestData(t)

	insertProfitTestLog(t, &Log{
		CreatedAt:        100,
		Username:         "alice",
		ModelName:        "gpt-5.5",
		Group:            profit.DefaultObserveGroup,
		PromptTokens:     10,
		CompletionTokens: 200,
		Quota:            1000,
	}, map[string]interface{}{
		profit.KeyObserveVersion:               profit.ObservationVersion,
		profit.KeyOutputPolicyMode:             profit.ModeCap,
		profit.KeyOutputPolicyCompletionTokens: 200,
		profit.KeyRiskMode:                     profit.RiskModeAlert,
		profit.KeyRiskAlert:                    true,
		profit.KeyRiskReasons: []string{
			profit.RiskReasonLossMakingRequest,
		},
	})

	analytics, err := GetProfitAnalytics(ProfitLogFilter{Group: profit.DefaultObserveGroup})

	require.NoError(t, err)
	require.Equal(t, int64(1), analytics.RiskLossMakingCount)
	require.Equal(t, "collect_more_output_samples", analytics.ProfitGuardrailAction)
	require.Equal(t, "margin_risk_with_insufficient_output_samples", analytics.ProfitGuardrailReason)
	require.Equal(t, "low", analytics.ProfitGuardrailConfidence)
	require.Empty(t, analytics.ProfitGuardrailOutputMode)
	require.Zero(t, analytics.ProfitGuardrailDefaultMaxTokens)
	require.Zero(t, analytics.ProfitGuardrailHardMaxTokens)
}
