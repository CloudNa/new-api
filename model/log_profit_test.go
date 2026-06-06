package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

func TestFormatUserLogsStripsProfitFields(t *testing.T) {
	logs := []*Log{
		{
			Other: `{"profit_observe_version":1,"billable_prompt_tokens":100,"upstream_actual_completion_tokens":20,"output_policy_mode":"cap","output_policy_would_cap":true,"model_ratio":1.5}`,
		},
	}

	formatUserLogs(logs, 0)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, other, "profit_observe_version")
	require.NotContains(t, other, "billable_prompt_tokens")
	require.NotContains(t, other, "upstream_actual_completion_tokens")
	require.NotContains(t, other, "output_policy_mode")
	require.NotContains(t, other, "output_policy_would_cap")
	require.Equal(t, 1.5, other["model_ratio"])
}
