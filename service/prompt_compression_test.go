package service

import (
	"net/http"
	"net/http/httptest"
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
