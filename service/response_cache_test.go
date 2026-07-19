package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStoreResponseCacheSkipsObservedHit(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	profit.ClearResponseCacheMemoryForTest()
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		profit.ClearResponseCacheMemoryForTest()
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	decision := &profit.ResponseCacheDecision{
		Eligible:   true,
		Hit:        true,
		Mode:       profit.ModeObserve,
		RuleID:     "public-help",
		KeyHash:    "observed-hit",
		TTLSeconds: 60,
	}
	ctx.Set(profitResponseCacheDecisionKey, decision)
	CaptureResponseCacheBody(ctx, []byte(`{"id":"cached"}`))

	StoreResponseCacheForRelay(ctx, &relaycommon.RelayInfo{}, &dto.GeneralOpenAIRequest{Model: "gpt-5.5"}, &dto.Usage{
		PromptTokens:     10,
		CompletionTokens: 2,
		TotalTokens:      12,
	})

	require.False(t, decision.Stored)
	_, found, err := profit.GetResponseCache(decision)
	require.NoError(t, err)
	require.False(t, found)
}
