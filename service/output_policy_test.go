package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withOutputPolicyOptionMap(t *testing.T, settings profit.Settings) {
	t.Helper()
	payload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		profit.SettingsOptionKey: string(payload),
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
	})
}

func newOutputPolicyTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	c.Set("channel_name", "test-channel")
	return c
}

func newOutputPolicyRelayInfo(group string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UsingGroup: group,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 7,
		},
	}
}

func outputPolicyCapSettings() profit.Settings {
	settings := profit.DefaultSettings()
	settings.ObserveOnly = false
	settings.OutputCapMode = profit.ModeCap
	settings.OutputPolicies = []profit.OutputPolicy{
		{
			ID:               "cap-gpt",
			Enabled:          true,
			Group:            profit.DefaultObserveGroup,
			ModelName:        "gpt-5*",
			Mode:             profit.ModeCap,
			DefaultMaxTokens: 128,
			HardMaxTokens:    256,
			RewriteOverLimit: true,
		},
	}
	return settings
}

func TestApplyOutputPolicyForRelayCapsConfiguredMaxTokens(t *testing.T) {
	withOutputPolicyOptionMap(t, outputPolicyCapSettings())
	c := newOutputPolicyTestContext()
	requested := uint(1024)
	request := &dto.GeneralOpenAIRequest{
		Model:     "gpt-5.5",
		MaxTokens: &requested,
	}

	ApplyOutputPolicyForRelay(c, newOutputPolicyRelayInfo(profit.DefaultObserveGroup), request, false)

	require.NotNil(t, request.MaxTokens)
	require.Equal(t, uint(256), *request.MaxTokens)
	decision := ProfitOutputPolicyDecisionFromContext(c)
	require.NotNil(t, decision)
	require.Equal(t, 1024, decision.RequestedMaxTokens)
	require.Equal(t, 256, decision.AppliedMaxTokens)
	require.True(t, decision.LiveEnforced)

	merged := MergeOutputPolicyCompletion(decision, 42)
	require.Equal(t, 42, merged.CompletionTokens)
	require.True(t, merged.LiveEnforced)
}

func TestApplyOutputPolicyForRelayCapsBothOpenAIMaxTokenFields(t *testing.T) {
	withOutputPolicyOptionMap(t, outputPolicyCapSettings())
	c := newOutputPolicyTestContext()
	maxTokens := uint(1024)
	maxCompletionTokens := uint(2048)
	request := &dto.GeneralOpenAIRequest{
		Model:               "gpt-5.5",
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
	}

	ApplyOutputPolicyForRelay(c, newOutputPolicyRelayInfo(profit.DefaultObserveGroup), request, false)

	require.NotNil(t, request.MaxTokens)
	require.NotNil(t, request.MaxCompletionTokens)
	require.Equal(t, uint(256), *request.MaxTokens)
	require.Equal(t, uint(256), *request.MaxCompletionTokens)
}

func TestApplyOutputPolicyForRelayInjectsDefaultMaxTokensWhenAbsent(t *testing.T) {
	withOutputPolicyOptionMap(t, outputPolicyCapSettings())
	c := newOutputPolicyTestContext()
	request := &dto.GeneralOpenAIRequest{Model: "gpt-5.5"}

	ApplyOutputPolicyForRelay(c, newOutputPolicyRelayInfo(profit.DefaultObserveGroup), request, false)

	require.NotNil(t, request.MaxTokens)
	require.Equal(t, uint(128), *request.MaxTokens)
	decision := ProfitOutputPolicyDecisionFromContext(c)
	require.NotNil(t, decision)
	require.Equal(t, 0, decision.RequestedMaxTokens)
	require.Equal(t, 128, decision.AppliedMaxTokens)
	require.True(t, decision.LiveEnforced)
}

func TestApplyOutputPolicyForRelayPreservesExplicitZeroMaxTokens(t *testing.T) {
	withOutputPolicyOptionMap(t, outputPolicyCapSettings())
	c := newOutputPolicyTestContext()
	zero := uint(0)
	request := &dto.GeneralOpenAIRequest{
		Model:     "gpt-5.5",
		MaxTokens: &zero,
	}

	ApplyOutputPolicyForRelay(c, newOutputPolicyRelayInfo(profit.DefaultObserveGroup), request, false)

	require.NotNil(t, request.MaxTokens)
	require.Equal(t, uint(0), *request.MaxTokens)
	decision := ProfitOutputPolicyDecisionFromContext(c)
	require.NotNil(t, decision)
	require.Equal(t, 0, decision.AppliedMaxTokens)
	require.False(t, decision.LiveEnforced)
}

func TestApplyOutputPolicyForRelaySkipsOutsideProxyTest(t *testing.T) {
	withOutputPolicyOptionMap(t, outputPolicyCapSettings())
	c := newOutputPolicyTestContext()
	requested := uint(1024)
	request := &dto.GeneralOpenAIRequest{
		Model:     "gpt-5.5",
		MaxTokens: &requested,
	}

	ApplyOutputPolicyForRelay(c, newOutputPolicyRelayInfo("default"), request, false)

	require.NotNil(t, request.MaxTokens)
	require.Equal(t, uint(1024), *request.MaxTokens)
	require.Nil(t, ProfitOutputPolicyDecisionFromContext(c))
}
