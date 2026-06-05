package promptcompress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCavemanPreservesStructuredContent(t *testing.T) {
	input := "Hello, please provide a detailed explanation of this.\n```go\nfmt.Println(\"the code\")\n```\nSee https://example.com/docs.\n{\"path\":\"/tmp/app.go\",\"ok\":true}"

	result := CompressText(input, Config{Mode: ModeStandard})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.NotContains(t, strings.ToLower(result.Text), "please")
	require.Contains(t, result.Text, "```go\nfmt.Println(\"the code\")\n```")
	require.Contains(t, result.Text, "https://example.com/docs")
	require.Contains(t, result.Text, "{\"path\":\"/tmp/app.go\",\"ok\":true}")
	require.GreaterOrEqual(t, result.Stats.PreservedBlockCount, 3)
	require.Contains(t, result.Stats.TechniquesUsed, "caveman")
}

func TestSecretLikeTextIsRedacted(t *testing.T) {
	input := "please use api_key=test-value-redacted-123456 for this request"

	result := CompressText(input, Config{Mode: ModeLite})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.NotContains(t, result.Text, "test-value-redacted-123456")
	require.Contains(t, result.Text, "[redacted_secret]")
	require.Equal(t, 1, result.Stats.RedactedSecretCount)
}

func TestRTKCompressesDockerBuildOutput(t *testing.T) {
	input := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Step 2/3 : COPY . /app",
		"Step 3/3 : RUN npm install",
		" ---> Running in abc123",
		" ---> def456",
		"Removing intermediate container abc123",
		"Successfully built def456",
		"Successfully tagged myapp:latest",
	}, "\n")

	result := CompressText(input, Config{Mode: ModeRTK})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "Step 1/3 : FROM node:20")
	require.Contains(t, result.Text, "Successfully built def456")
	require.NotContains(t, result.Text, "Removing intermediate container")
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, result.Stats.RulesApplied, "docker-build:strip")
}

func TestStackedRunsRTKThenCaveman(t *testing.T) {
	input := strings.Join([]string{
		"Sure, I can help.",
		"INFO started",
		"INFO started",
		"INFO started",
		"ERROR failed to connect",
		"Please explain in detail why it failed.",
	}, "\n")

	result := CompressText(input, Config{Mode: ModeStacked})

	require.True(t, result.Compressed)
	require.Equal(t, 1, strings.Count(result.Text, "INFO started"))
	require.Contains(t, result.Text, "ERROR failed to connect")
	require.NotContains(t, strings.ToLower(result.Text), "please")
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, result.Stats.RulesApplied, "docker-logs:collapse")
	require.Contains(t, result.Stats.TechniquesUsed, "caveman")
}

func TestCompressMessagesPreservesSystemPrompt(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "Please keep the system prompt exactly as written."},
		{Role: "user", Content: "Please explain in detail what I need to do."},
	}

	out, stats := CompressMessages(messages, Config{Mode: ModeStandard, PreserveSystemPrompt: true})

	require.Equal(t, messages[0].Content, out[0].Content)
	require.NotEqual(t, messages[1].Content, out[1].Content)
	require.Contains(t, stats.TechniquesUsed, "caveman")
}

func TestBypassesWhenPreservationWouldDropProtectedContent(t *testing.T) {
	input := "vite v5.0.0 building\nhttps://example.com/must-keep\nbuilt in 10ms"

	result := CompressText(input, Config{Mode: ModeRTK})

	require.False(t, result.Compressed)
	require.True(t, result.Stats.Bypassed)
	require.Equal(t, "preservation_check_failed", result.Stats.BypassReason)
	require.Equal(t, input, result.Text)
}
