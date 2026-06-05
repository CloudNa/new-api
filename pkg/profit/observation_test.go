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
	require.Equal(t, 1.5, other["model_ratio"])
}
