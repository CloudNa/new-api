package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSanitizeLogTextRedactsMediaPayloads(t *testing.T) {
	payload := strings.Repeat("A", 320)
	content := "生成失败 data:image/png;base64," + payload + ` path=C:\Users\CloudNa\AppData\Local\KuaiyidianAI\frame.png`

	sanitized := sanitizeLogText(content)

	require.Contains(t, sanitized, redactedDataURLLogValue)
	require.Contains(t, sanitized, redactedLocalPathLogValue)
	require.NotContains(t, sanitized, payload)
	require.NotContains(t, sanitized, `C:\Users\CloudNa`)
}

func TestSanitizeLogOtherRedactsNestedMediaValues(t *testing.T) {
	payload := strings.Repeat("B", 320)
	other := map[string]interface{}{
		"request_path": "/v1/images/generations",
		"image_url":    "https://cdn.example.com/uploads/private-frame.png?signature=secret",
		"nested": map[string]interface{}{
			"response_body": "data:video/mp4;base64," + payload,
			"small":         "keep-me",
		},
		"items": []interface{}{
			map[string]interface{}{
				"audio_url": "file:///C:/Users/CloudNa/AppData/Local/KuaiyidianAI/voice.wav",
			},
			"safe text",
		},
		"b64_json": payload,
		"count":    2,
	}

	sanitized := sanitizeLogOther(other)
	serialized := common.MapToJsonStr(sanitized)

	require.Equal(t, "/v1/images/generations", sanitized["request_path"])
	require.Equal(t, redactedMediaURLLogValue, sanitized["image_url"])
	require.Contains(t, serialized, "keep-me")
	require.Contains(t, serialized, redactedDataURLLogValue)
	require.Contains(t, serialized, redactedBase64LogValue)
	require.Contains(t, serialized, redactedMediaURLLogValue)
	require.NotContains(t, serialized, payload)
	require.NotContains(t, serialized, "private-frame.png")
	require.NotContains(t, serialized, "signature=secret")
	require.NotContains(t, serialized, "voice.wav")
}
