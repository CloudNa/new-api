package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/profit"

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
		profit.KeyCompressionSavingsPercent:      11.5,
		profit.KeyCompressionBypassed:            false,
		profit.KeyCompressionRulesVersion:        "omniroute-style-go-v1",
		profit.KeyCompressionRulesApplied:        []string{"rtk:truncate", "caveman:pleasantries"},
		profit.KeyCompressionPreservedBlocks:     2,
		profit.KeyCompressionRedactedSecrets:     1,
		profit.KeyRouteMode:                      profit.ModeObserve,
		profit.KeyRouteCandidateCount:            2,
		profit.KeyRouteSelectedChannelID:         501,
		profit.KeyRouteSelectedMarginRank:        2,
		profit.KeyRouteBestChannelID:             502,
		profit.KeyRouteBestChannelName:           "cheap",
		profit.KeyRouteBestCostProfileID:         "cheap-profile",
		profit.KeyRouteBestExpectedMarginUSD:     0.88,
		profit.KeyRouteWouldPreferDifferent:      true,
		profit.KeyRouteCandidates: []profit.RouteDecisionCandidate{
			{ChannelID: 501, ChannelName: "gpt-load", Selected: true, MarginRank: 2},
			{ChannelID: 502, ChannelName: "cheap", WouldPrefer: true, MarginRank: 1},
		},
		profit.KeyOutputPolicyMode:             profit.ModeCap,
		profit.KeyOutputPolicyID:               "cap-gemini",
		profit.KeyOutputPolicyName:             "Cap Gemini",
		profit.KeyOutputPolicyCompletionTokens: 2500,
		profit.KeyOutputPolicyDefaultMaxTokens: 1000,
		profit.KeyOutputPolicyHardMaxTokens:    2000,
		profit.KeyOutputPolicyExceededDefault:  true,
		profit.KeyOutputPolicyExceededHard:     true,
		profit.KeyOutputPolicyRewriteOverLimit: true,
		profit.KeyOutputPolicyWouldCap:         true,
		profit.KeyOutputPolicyObserveOnly:      true,
		profit.KeyOutputPolicyLiveEnforced:     false,
		profit.KeyCacheSavedUSD:                nil,
		profit.KeyRetryCostUSD:                 nil,
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
	require.Equal(t, "omniroute-style-go-v1", events[0].CompressionRulesVersion)
	require.Equal(t, []string{"rtk:truncate", "caveman:pleasantries"}, events[0].CompressionRulesApplied)
	require.Equal(t, int64(2), events[0].CompressionPreservedBlocks)
	require.Equal(t, int64(1), events[0].CompressionRedactedSecrets)
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
		profit.KeyCacheSavedUSD:                  nil,
		profit.KeyRetryCostUSD:                   nil,
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
	require.Nil(t, analytics.CacheSavedUSD)
	require.Nil(t, analytics.RetryCostUSD)
}
