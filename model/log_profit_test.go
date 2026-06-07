package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsStripsProfitFields(t *testing.T) {
	logs := []*Log{
		{
			Other: `{"profit_observe_version":1,"billable_prompt_tokens":100,"upstream_actual_completion_tokens":20,"compression_engine":"stacked","compression_validation_warnings":["warn"],"compression_engine_breakdown":[{"engine":"rtk"}],"output_policy_mode":"cap","output_policy_would_cap":true,"profit_risk_mode":"alert","profit_risk_alert":true,"model_ratio":1.5}`,
		},
	}

	formatUserLogs(logs, 0)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "profit_observe_version")
	require.NotContains(t, other, "billable_prompt_tokens")
	require.NotContains(t, other, "upstream_actual_completion_tokens")
	require.NotContains(t, other, "compression_engine")
	require.NotContains(t, other, "compression_validation_warnings")
	require.NotContains(t, other, "compression_engine_breakdown")
	require.NotContains(t, other, "output_policy_mode")
	require.NotContains(t, other, "output_policy_would_cap")
	require.NotContains(t, other, "profit_risk_mode")
	require.NotContains(t, other, "profit_risk_alert")
	require.Equal(t, 1.5, other["model_ratio"])
}

func TestFormatUserLogsStripsLongContextProfitFields(t *testing.T) {
	logs := []*Log{
		{
			Other: `{"profit_observe_version":1,"long_context_mode":"observe","long_context_tokens":64000,"long_context_suggested_extra_revenue_usd":0.2,"long_context_premium_required":true,"model_ratio":1.5}`,
		},
	}

	formatUserLogs(logs, 0)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "profit_observe_version")
	require.NotContains(t, other, "long_context_mode")
	require.NotContains(t, other, "long_context_tokens")
	require.NotContains(t, other, "long_context_suggested_extra_revenue_usd")
	require.NotContains(t, other, "long_context_premium_required")
	require.Equal(t, 1.5, other["model_ratio"])
}
