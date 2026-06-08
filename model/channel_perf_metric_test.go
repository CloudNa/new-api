package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelPerfMetricUpsertAndSummary(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM channel_perf_metrics").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channel_perf_metrics")
	})

	require.NoError(t, UpsertChannelPerfMetric(&ChannelPerfMetric{
		ModelName:      "gpt-5.5",
		Group:          "proxy-test",
		ChannelID:      4,
		BucketTs:       100,
		RequestCount:   2,
		SuccessCount:   1,
		TotalLatencyMs: 3000,
		OutputTokens:   100,
		GenerationMs:   2000,
	}))
	require.NoError(t, UpsertChannelPerfMetric(&ChannelPerfMetric{
		ModelName:      "gpt-5.5",
		Group:          "proxy-test",
		ChannelID:      4,
		BucketTs:       100,
		RequestCount:   3,
		SuccessCount:   3,
		TotalLatencyMs: 6000,
		OutputTokens:   200,
		GenerationMs:   4000,
	}))

	summaries, err := GetChannelPerfMetricSummaries("gpt-5.5", "proxy-test", []int{4}, 0, 200)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, 4, summaries[0].ChannelID)
	require.EqualValues(t, 5, summaries[0].RequestCount)
	require.EqualValues(t, 4, summaries[0].SuccessCount)
	require.EqualValues(t, 9000, summaries[0].TotalLatencyMs)
	require.EqualValues(t, 300, summaries[0].OutputTokens)
	require.EqualValues(t, 6000, summaries[0].GenerationMs)
}
