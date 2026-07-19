package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetStreamDiagnosticsReturnsOnlyProblemStreams(t *testing.T) {
	resetProfitTestData(t)

	insertStreamDiagnosticTestLog(t, &Log{
		CreatedAt:        100,
		Type:             LogTypeConsume,
		Username:         "alice",
		TokenName:        "public-token",
		ModelName:        "gpt-5.5",
		ChannelId:        10,
		Group:            "default",
		RequestId:        "req-bad",
		UseTime:          3,
		IsStream:         true,
		PromptTokens:     100,
		CompletionTokens: 20,
		Quota:            500,
	}, map[string]interface{}{
		"stream_status": map[string]interface{}{
			"status":            "error",
			"end_reason":        "upstream_eof_without_terminal_event",
			"request_format":    "openai_responses",
			"group":             "default",
			"model":             "gpt-5.5",
			"channel":           10,
			"terminal_received": false,
			"chunk_count":       3,
			"byte_count":        4096,
			"upstream_eof":      true,
			"error_count":       1,
			"errors":            []string{"upstream EOF before terminal event"},
		},
	})

	insertStreamDiagnosticTestLog(t, &Log{
		CreatedAt: 101,
		Type:      LogTypeConsume,
		Username:  "bob",
		ModelName: "gpt-5.5",
		ChannelId: 11,
		Group:     "default",
		RequestId: "req-ok",
		IsStream:  true,
	}, map[string]interface{}{
		"stream_status": map[string]interface{}{
			"status":      "ok",
			"end_reason":  "done",
			"chunk_count": 5,
		},
	})

	items, err := GetStreamDiagnostics(StreamDiagnosticsFilter{
		Group: "default",
		Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "req-bad", items[0].RequestID)
	require.Equal(t, "upstream_eof_without_terminal_event", items[0].EndReason)
	require.Equal(t, int64(3), items[0].ChunkCount)
	require.True(t, items[0].UpstreamEOF)
	require.Equal(t, []string{"upstream EOF before terminal event"}, items[0].Errors)

	empty, err := GetStreamDiagnostics(StreamDiagnosticsFilter{
		Group: "proxy-test",
		Limit: 10,
	})
	require.NoError(t, err)
	require.Empty(t, empty)
}

func insertStreamDiagnosticTestLog(t *testing.T, log *Log, other map[string]interface{}) {
	t.Helper()
	log.Other = common.MapToJsonStr(other)
	require.NoError(t, DB.Create(log).Error)
}
