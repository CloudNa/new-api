package profit

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/require"
)

func TestBuildResponseCacheDecisionRequiresPublicStaticProxyTestRule(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeEnforce
	settings.ObserveOnly = false
	settings.ResponseCacheRules = []ResponseCacheRule{{
		ID:           "public-help",
		Enabled:      true,
		Group:        DefaultObserveGroup,
		ModelName:    "gpt-5*",
		Mode:         ModeEnforce,
		Scope:        ResponseCacheScopeGlobal,
		PublicStatic: true,
	}}

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-5.5",
		Messages: []dto.Message{{
			Role:    "user",
			Content: "public static help",
		}},
	}
	decision := BuildResponseCacheDecision(settings, ResponseCacheInput{
		Group:     DefaultObserveGroup,
		ModelName: "gpt-5.5",
		Request:   request,
	})

	require.NotNil(t, decision)
	require.True(t, decision.Eligible)
	require.Equal(t, ModeEnforce, decision.Mode)
	require.Equal(t, "public-help", decision.RuleID)
	require.NotEmpty(t, decision.KeyHash)

	repeat := BuildResponseCacheDecision(settings, ResponseCacheInput{
		Group:     DefaultObserveGroup,
		ModelName: "gpt-5.5",
		Request:   request,
	})
	require.Equal(t, decision.KeyHash, repeat.KeyHash)
}

func TestBuildResponseCacheDecisionBypassesUnsafeRequests(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeObserve
	settings.ResponseCacheRules = []ResponseCacheRule{{
		ID:           "public-help",
		Enabled:      true,
		Group:        DefaultObserveGroup,
		ModelName:    "*",
		Scope:        ResponseCacheScopeGlobal,
		PublicStatic: true,
	}}

	decision := BuildResponseCacheDecision(settings, ResponseCacheInput{
		Group: DefaultObserveGroup,
		Request: &dto.GeneralOpenAIRequest{
			Model: "gpt-5.5",
			Messages: []dto.Message{{
				Role:    "user",
				Content: []any{map[string]any{"type": dto.ContentTypeImageURL, "image_url": map[string]any{"url": "https://example.test/a.png"}}},
			}},
		},
	})

	require.NotNil(t, decision)
	require.False(t, decision.Eligible)
	require.Equal(t, "multimodal_not_supported", decision.BypassReason)
}

func TestBuildResponseCacheDecisionDowngradesEnforceWhenObserveOnly(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeEnforce
	settings.ObserveOnly = true
	settings.ResponseCacheRules = []ResponseCacheRule{{
		ID:           "public-help",
		Enabled:      true,
		Group:        DefaultObserveGroup,
		ModelName:    "*",
		Mode:         ModeEnforce,
		Scope:        ResponseCacheScopeGlobal,
		PublicStatic: true,
	}}

	decision := BuildResponseCacheDecision(settings, ResponseCacheInput{
		Group: DefaultObserveGroup,
		Request: &dto.GeneralOpenAIRequest{
			Model: "gpt-5.5",
			Messages: []dto.Message{{
				Role:    "user",
				Content: "public static help",
			}},
		},
	})

	require.NotNil(t, decision)
	require.True(t, decision.Eligible)
	require.Equal(t, ModeObserve, decision.Mode)
	require.Equal(t, "observe_only", decision.BypassReason)
}

func TestResponseCachePutGetUsesMemoryWhenRedisDisabled(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	ClearResponseCacheMemoryForTest()
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		ClearResponseCacheMemoryForTest()
	})

	decision := &ResponseCacheDecision{
		Eligible:     true,
		Mode:         ModeEnforce,
		RuleID:       "public-help",
		KeyHash:      "abc123",
		TTLSeconds:   60,
		MaxBodyBytes: 1024,
	}
	usage := &dto.Usage{
		PromptTokens:     12,
		CompletionTokens: 3,
		TotalTokens:      15,
	}

	require.NoError(t, PutResponseCache(decision, []byte(`{"id":"cached"}`), usage, "gpt-5.5"))
	entry, found, err := GetResponseCache(decision)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte(`{"id":"cached"}`), entry.Body)
	require.Equal(t, 12, entry.Usage.PromptTokens)
	require.Equal(t, 3, entry.Usage.CompletionTokens)
}

func TestResponseCacheSessionScopeRequiresExplicitSession(t *testing.T) {
	settings := DefaultSettings()
	settings.CacheMode = ModeObserve
	settings.ResponseCacheRules = []ResponseCacheRule{{
		ID:           "session-rule",
		Enabled:      true,
		Group:        DefaultObserveGroup,
		ModelName:    "*",
		Scope:        ResponseCacheScopeSession,
		PublicStatic: true,
	}}

	decision := BuildResponseCacheDecision(settings, ResponseCacheInput{
		Group: DefaultObserveGroup,
		Request: &dto.GeneralOpenAIRequest{
			Model: "gpt-5.5",
			Messages: []dto.Message{{
				Role:    "user",
				Content: "public static help",
			}},
		},
	})

	require.NotNil(t, decision)
	require.False(t, decision.Eligible)
	require.Equal(t, "missing_session_scope", decision.BypassReason)
}
