package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/QuantumNous/new-api/pkg/promptcompress"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withPromptCompressionOptionMap(t *testing.T, settings promptcompress.Settings) {
	t.Helper()
	payload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	original := common.OptionMap
	common.OptionMap = map[string]string{
		promptcompress.SettingsOptionKey: string(payload),
	}
	t.Cleanup(func() {
		common.OptionMap = original
	})
}

func newPromptCompressionTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	return c
}

func newPromptCompressionRelayInfo(group string) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		UsingGroup:      group,
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		OriginModelName: "glart-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 7,
		},
	}
	info.SetEstimatePromptTokens(120)
	return info
}

func promptCompressionToolCallsRaw(t *testing.T, toolCall dto.ToolCallRequest) []byte {
	t.Helper()
	raw, err := common.Marshal([]dto.ToolCallRequest{toolCall})
	require.NoError(t, err)
	return raw
}

func TestApplyPromptCompressionForRelayProxyTestOnly(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeStandard,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Caveman: promptcompress.CavemanSettings{
			CompressRoles:    []string{"user"},
			MinMessageLength: 1,
		},
	})
	c := newPromptCompressionTestContext()
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "system", Content: "Please keep this exact system prompt."},
			{Role: "user", Content: "Please explain in detail what I need to do, and please provide a detailed explanation with repeated repeated repeated wording."},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed)
	require.Greater(t, info.PromptCompressionStats.CompressionSavedTokens, 0)
	require.Equal(t, "Please keep this exact system prompt.", request.Messages[0].Content)
	require.NotContains(t, request.Messages[1].Content.(string), "Please explain in detail")

	otherGroupRequest := &dto.GeneralOpenAIRequest{
		Model:    "glart-test",
		Messages: []dto.Message{{Role: "user", Content: "Please explain in detail what I need to do."}},
	}
	otherGroupInfo := newPromptCompressionRelayInfo("default")
	ApplyPromptCompressionForRelay(c, otherGroupInfo, otherGroupRequest, false)
	require.Nil(t, otherGroupInfo.PromptCompressionStats)
	require.Contains(t, otherGroupRequest.Messages[0].Content.(string), "Please explain in detail")
}

func TestApplyPromptCompressionForRelayStackedMatchesOmniRouteRoleSemantics(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeStacked,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Caveman: promptcompress.CavemanSettings{
			CompressRoles:    []string{"user"},
			MinMessageLength: 1,
		},
	})
	c := newPromptCompressionTestContext()
	toolCalls := promptCompressionToolCallsRaw(t, dto.ToolCallRequest{
		ID:   "call_1",
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      "bash",
			Arguments: `{"command":"docker build"}`,
		},
	})
	toolOutput := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Step 2/3 : COPY . /app",
		"Step 3/3 : RUN npm install",
		" ---> Running in abc123",
		" ---> def456",
		"Removing intermediate container abc123",
		"Successfully built def456",
		"Successfully tagged myapp:latest",
	}, "\n")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "system", Content: "Please keep this exact system prompt."},
			{Role: "assistant", Content: "", ToolCalls: toolCalls},
			{Role: "tool", ToolCallId: "call_1", Content: toolOutput},
			{Role: "user", Content: "Please explain in detail why the build failed, and please provide a detailed summary."},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed)
	require.Equal(t, "Please keep this exact system prompt.", request.Messages[0].Content)
	require.Contains(t, request.Messages[2].Content.(string), "Successfully built def456")
	require.NotContains(t, request.Messages[2].Content.(string), "Removing intermediate container")
	require.NotContains(t, strings.ToLower(request.Messages[3].Content.(string)), "please")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "rtk-filter")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "caveman")
	require.Contains(t, info.PromptCompressionStats.RulesApplied, "docker-build:strip")
}

func TestApplyPromptCompressionForRelayStackedCompressesExplicitUserTerminalOutput(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeStacked,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Caveman: promptcompress.CavemanSettings{
			CompressRoles:    []string{"user"},
			MinMessageLength: 1,
		},
	})
	c := newPromptCompressionTestContext()
	output := strings.Join([]string{
		"Here is terminal output from `go test ./...`:",
		"```text",
		"=== RUN TestA",
		"=== RUN TestB",
		"--- FAIL: TestB (0.00s)",
		"    a_test.go:1: boom",
		"=== RUN TestC",
		"FAIL\t./pkg\t0.1s",
		"```",
		"Please explain in detail what failed and please provide a detailed fix plan.",
	}, "\n")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "system", Content: "Keep this system prompt."},
			{Role: "user", Content: output},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed, "%+v", info.PromptCompressionStats)
	content := request.Messages[1].Content.(string)
	require.Contains(t, content, "--- FAIL: TestB")
	require.Contains(t, content, "a_test.go:1: boom")
	require.Contains(t, content, "FAIL\t./pkg\t0.1s")
	require.NotContains(t, content, "=== RUN TestA")
	require.NotContains(t, content, "please provide a detailed")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "rtk-filter")
	require.Contains(t, info.PromptCompressionStats.RulesApplied, "test-go:keep")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "caveman")
}

func TestApplyPromptCompressionForRelayRTKSkipsNonTerminalToolFilters(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeRTK,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
	})
	c := newPromptCompressionTestContext()
	toolCalls := promptCompressionToolCallsRaw(t, dto.ToolCallRequest{
		ID:   "call_1",
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      "read_file",
			Arguments: `{"path":"Dockerfile"}`,
		},
	})
	toolOutput := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Removing intermediate container abc123",
		"Successfully built def456",
	}, "\n")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "assistant", Content: "", ToolCalls: toolCalls},
			{Role: "tool", ToolCallId: "call_1", Content: toolOutput},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.True(t, info.PromptCompressionStats.Bypassed)
	require.Equal(t, "no_savings", info.PromptCompressionStats.BypassReason)
	require.Equal(t, toolOutput, request.Messages[1].Content)
	require.NotContains(t, info.PromptCompressionStats.RulesApplied, "docker-build:strip")
}

func TestApplyPromptCompressionForRelayRTKAssistantMessagesConfig(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeRTK,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Rtk: promptcompress.RtkSettings{
			MaxLines:                 120,
			MaxChars:                 12000,
			DeduplicateThreshold:     3,
			ApplyToToolResults:       true,
			ApplyToAssistantMessages: true,
		},
	})
	c := newPromptCompressionTestContext()
	output := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Step 2/3 : COPY . /app",
		" ---> Running in abc123",
		"Removing intermediate container abc123",
		"Successfully built def456",
	}, "\n")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "assistant", Content: output},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed)
	require.Contains(t, request.Messages[0].Content.(string), "Successfully built def456")
	require.NotContains(t, request.Messages[0].Content.(string), "Removing intermediate container")
	require.Contains(t, info.PromptCompressionStats.RulesApplied, "docker-build:strip")
}

func TestApplyPromptCompressionForRelayRTKCodeBlocksOnlyConfig(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeRTK,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Rtk: promptcompress.RtkSettings{
			MaxLines:             120,
			MaxChars:             12000,
			DeduplicateThreshold: 3,
			ApplyToCodeBlocks:    true,
		},
	})
	c := newPromptCompressionTestContext()
	content := strings.Join([]string{
		"before",
		"```go",
		"",
		"func main() {",
		"    println(\"x\")   ",
		"",
		"}",
		"",
		"```",
		"after",
	}, "\n")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "assistant", Content: content},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed)
	require.Contains(t, request.Messages[0].Content.(string), "before\n```go\nfunc main() {\n    println(\"x\")\n}\n```\nafter")
	require.NotContains(t, request.Messages[0].Content.(string), "println(\"x\")   ")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "rtk-code-strip")
	require.Contains(t, info.PromptCompressionStats.RulesApplied, "rtk:code-strip")
}

func TestApplyPromptCompressionForRelayPreservesCompositeContentShape(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeStandard,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Caveman: promptcompress.CavemanSettings{
			CompressRoles:    []string{"user"},
			MinMessageLength: 1,
		},
	})
	c := newPromptCompressionTestContext()
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "Please provide a detailed explanation of this.",
					},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "https://example.com/image.png",
						},
					},
					map[string]any{
						"type": "input_text",
						"text": "What I'm trying to do is deploy this.",
					},
				},
			},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed)
	parts, ok := request.Messages[0].Content.([]any)
	require.True(t, ok)
	require.Len(t, parts, 3)
	require.Equal(t, "Explain this.", parts[0].(map[string]any)["text"])
	require.Equal(t, "https://example.com/image.png", parts[1].(map[string]any)["image_url"].(map[string]any)["url"])
	require.Equal(t, "Goal:deploy this.", parts[2].(map[string]any)["text"])
}

func TestApplyPromptCompressionForRelayAggressiveCompressesNonTerminalToolResult(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeAggressive,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
	})
	c := newPromptCompressionTestContext()
	toolCalls := promptCompressionToolCallsRaw(t, dto.ToolCallRequest{
		ID:   "call_1",
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      "read_file",
			Arguments: `{"path":"pkg/promptcompress/aggressive.go"}`,
		},
	})
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "system", Content: "Keep this system prompt exactly as written."},
			{Role: "assistant", Content: "", ToolCalls: toolCalls},
			{Role: "tool", ToolCallId: "call_1", Content: promptCompressionLongCodeFileContent()},
			{Role: "user", Content: "Newest prompt should remain readable."},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed, "%+v", info.PromptCompressionStats)
	require.Equal(t, "Keep this system prompt exactly as written.", request.Messages[0].Content)
	toolContent := request.Messages[2].Content.(string)
	require.Contains(t, toolContent, "... [10 lines elided] ...")
	require.Contains(t, toolContent, "const value00 = 0")
	require.Contains(t, toolContent, "const value34 = 34")
	require.NotContains(t, toolContent, "const value24 = 24")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "toolResult")
	require.NotNil(t, info.PromptCompressionStats.Aggressive)
	require.Greater(t, info.PromptCompressionStats.Aggressive.ToolResultSavings, 0)
}

func TestApplyPromptCompressionForRelayUltraPrunesUserText(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeUltra,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
	})
	c := newPromptCompressionTestContext()
	systemPrompt := "Keep this system prompt exactly as written."
	userPrompt := strings.Join([]string{
		"the and a an is are was were to of in on with from",
		"Critical Error: preserve https://example.com/docs and src/service/prompt_compression.go",
		"AlphaBeta importantContext should remain because it carries signal",
	}, " ")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed, "%+v", info.PromptCompressionStats)
	require.Equal(t, systemPrompt, request.Messages[0].Content)
	pruned := request.Messages[1].Content.(string)
	require.Contains(t, pruned, "Error:")
	require.Contains(t, pruned, "https://example.com/docs")
	require.Contains(t, pruned, "src/service/prompt_compression.go")
	require.NotContains(t, pruned, "the and a an")
	require.Contains(t, info.PromptCompressionStats.TechniquesUsed, "ultra-heuristic-pruning")
}

func TestApplyPromptCompressionForRelayWritesDiagnostics(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:              true,
		DefaultMode:          promptcompress.ModeUltra,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{profit.DefaultObserveGroup},
		Ultra: promptcompress.UltraSettings{
			ModelPath: "/models/slm.onnx",
		},
	})
	c := newPromptCompressionTestContext()
	userPrompt := strings.Join([]string{
		"the and a an is are was were to of in on with from",
		"Critical Error: preserve https://example.com/docs and src/service/prompt_compression.go",
		"AlphaBeta importantContext should remain because it carries signal",
	}, " ")
	request := &dto.GeneralOpenAIRequest{
		Model: "glart-test",
		Messages: []dto.Message{
			{Role: "user", Content: userPrompt},
		},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, false)

	require.NotNil(t, info.PromptCompressionStats)
	require.False(t, info.PromptCompressionStats.Bypassed, "%+v", info.PromptCompressionStats)
	require.Equal(t, "ultra", info.PromptCompressionStats.Engine)
	require.Greater(t, info.PromptCompressionStats.Timestamp, int64(0))
	require.Contains(t, info.PromptCompressionStats.ValidationWarnings, "ultra_slm_model_path_ignored_go_native_heuristic_used")
	require.NotEmpty(t, info.PromptCompressionStats.EngineBreakdown)
	require.Equal(t, "ultra", info.PromptCompressionStats.EngineBreakdown[0].Engine)

	other := map[string]interface{}{}
	saved := injectPromptCompressionOther(other, info)

	require.Greater(t, saved, 0)
	require.Equal(t, "ultra", other[profit.KeyCompressionMode])
	require.Equal(t, "ultra", other[profit.KeyCompressionEngine])
	require.Equal(t, info.PromptCompressionStats.Timestamp, other[profit.KeyCompressionTimestamp])
	require.Equal(t, false, other[profit.KeyCompressionFallbackApplied])
	require.Equal(t, info.PromptCompressionStats.ValidationWarnings, other[profit.KeyCompressionValidationWarnings])
	require.Equal(t, info.PromptCompressionStats.EngineBreakdown, other[profit.KeyCompressionEngineBreakdown])
}

func TestMergePromptCompressionStatsMetadataIncludesDiagnostics(t *testing.T) {
	stats := promptcompress.Stats{}
	stepStats := promptcompress.Stats{
		Engine:             "ultra",
		OriginalTokens:     100,
		CompressedTokens:   70,
		SavingsPercent:     30,
		TechniquesUsed:     []string{"ultra-heuristic-pruning"},
		RulesApplied:       []string{"rule-a"},
		ValidationWarnings: []string{"warning-a"},
		ValidationErrors:   []string{"error-a"},
		FallbackApplied:    true,
		DurationMs:         12,
	}

	mergePromptCompressionStatsMetadata(&stats, stepStats)

	require.Contains(t, stats.TechniquesUsed, "ultra-heuristic-pruning")
	require.Contains(t, stats.RulesApplied, "rule-a")
	require.Contains(t, stats.ValidationWarnings, "warning-a")
	require.Contains(t, stats.ValidationErrors, "error-a")
	require.True(t, stats.FallbackApplied)
	require.Len(t, stats.EngineBreakdown, 1)
	require.Equal(t, "ultra", stats.EngineBreakdown[0].Engine)
	require.Equal(t, 100, stats.EngineBreakdown[0].OriginalTokens)
	require.Equal(t, 70, stats.EngineBreakdown[0].CompressedTokens)
}

func TestApplyPromptCompressionForRelayBypassesPassThrough(t *testing.T) {
	withPromptCompressionOptionMap(t, promptcompress.Settings{
		Enabled:       true,
		DefaultMode:   promptcompress.ModeStandard,
		AllowedGroups: []string{profit.DefaultObserveGroup},
		Caveman: promptcompress.CavemanSettings{
			CompressRoles:    []string{"user"},
			MinMessageLength: 1,
		},
	})
	c := newPromptCompressionTestContext()
	request := &dto.GeneralOpenAIRequest{
		Model:    "glart-test",
		Messages: []dto.Message{{Role: "user", Content: "Please explain in detail what I need to do."}},
	}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)

	ApplyPromptCompressionForRelay(c, info, request, true)

	require.NotNil(t, info.PromptCompressionStats)
	require.True(t, info.PromptCompressionStats.Bypassed)
	require.Equal(t, "pass_through_body", info.PromptCompressionStats.BypassReason)
	require.Contains(t, request.Messages[0].Content.(string), "Please explain in detail")
}

func TestResolvePromptCompressionModeAutoTriggerOverridesDefault(t *testing.T) {
	c := newPromptCompressionTestContext()
	settings := promptcompress.DefaultSettings()
	settings.Enabled = true
	settings.DefaultMode = promptcompress.ModeStandard
	settings.AutoTriggerMode = promptcompress.ModeRTK
	settings.AutoTriggerTokens = 50
	settings.AllowedGroups = []string{profit.DefaultObserveGroup}
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)
	info.SetEstimatePromptTokens(120)
	request := &dto.GeneralOpenAIRequest{Model: "glart-test"}

	mode := resolvePromptCompressionMode(c, settings.Normalize(), info, request)

	require.Equal(t, promptcompress.ModeRTK, mode)
}

func TestBillingUsageForPromptCompressionKeepsOriginalPromptTokens(t *testing.T) {
	info := newPromptCompressionRelayInfo(profit.DefaultObserveGroup)
	info.SetEstimatePromptTokens(200)
	info.PromptCompressionStats = &promptcompress.Stats{
		OriginalTokens:         200,
		CompressedTokens:       120,
		CompressionSavedTokens: 80,
		Mode:                   promptcompress.ModeStacked,
	}
	usage := &dto.Usage{
		PromptTokens:     120,
		CompletionTokens: 30,
		TotalTokens:      150,
	}

	billingUsage := billingUsageForPromptCompression(info, usage)

	require.NotSame(t, usage, billingUsage)
	require.Equal(t, 200, billingUsage.PromptTokens)
	require.Equal(t, 230, billingUsage.TotalTokens)
	require.Equal(t, 120, usage.PromptTokens)
	require.Equal(t, 150, usage.TotalTokens)
}

func promptCompressionLongCodeFileContent() string {
	lines := make([]string, 0, 35)
	for i := 0; i < 35; i++ {
		lines = append(lines, fmt.Sprintf("const value%02d = %d", i, i))
	}
	return strings.Join(lines, "\n")
}
