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
	t.Cleanup(func() {
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM channels")
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
	require.Equal(t, "omniroute-style-go-v1", events[0].CompressionRulesVersion)
	require.Equal(t, []string{"rtk:truncate", "caveman:pleasantries"}, events[0].CompressionRulesApplied)
	require.Equal(t, int64(2), events[0].CompressionPreservedBlocks)
	require.Equal(t, int64(1), events[0].CompressionRedactedSecrets)
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
	require.Nil(t, analytics.CacheSavedUSD)
	require.Nil(t, analytics.RetryCostUSD)
}
