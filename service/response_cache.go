package service

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const (
	profitResponseCacheDecisionKey = "profit_response_cache_decision"
	profitResponseCacheBodyKey     = "profit_response_cache_body"
	responseCacheSessionHeader     = "X-Glart-Cache-Session"
)

func PrepareResponseCacheForRelay(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, passThroughBody bool) (*profit.ResponseCacheDecision, *profit.ResponseCacheEntry) {
	if c == nil || info == nil || request == nil {
		return nil, nil
	}
	settings := profit.CurrentSettings()
	decision := profit.BuildResponseCacheDecision(settings, profit.ResponseCacheInput{
		Group:           info.UsingGroup,
		ModelName:       request.Model,
		ChannelID:       info.ChannelId,
		ChannelName:     c.GetString("channel_name"),
		UserID:          info.UserId,
		TokenID:         info.TokenId,
		SessionKey:      responseCacheSessionKey(c, request),
		IsStream:        info.IsStream,
		PassThroughBody: passThroughBody,
		Request:         request,
	})
	if decision == nil {
		return nil, nil
	}
	c.Set(profitResponseCacheDecisionKey, decision)
	if !decision.Eligible {
		return decision, nil
	}
	entry, found, err := profit.GetResponseCache(decision)
	if err != nil {
		decision.BypassReason = "cache_read_failed"
		logger.LogWarn(c, "response cache read failed: "+err.Error())
		return decision, nil
	}
	if !found {
		return decision, nil
	}
	decision.Hit = true
	decision.WouldHit = true
	decision.SavedPrompt = entry.Usage.PromptTokens
	decision.SavedCompletion = entry.Usage.CompletionTokens
	if decision.Mode != profit.ModeEnforce {
		return decision, nil
	}
	decision.LiveServed = true
	return decision, &entry
}

func ServeResponseCacheHit(c *gin.Context, entry *profit.ResponseCacheEntry) *dto.Usage {
	if c == nil || entry == nil || len(entry.Body) == 0 {
		return nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	IOCopyBytesGracefully(c, &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}, entry.Body)
	usage := entry.Usage
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return &usage
}

func StoreResponseCacheForRelay(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, usage *dto.Usage) {
	if c == nil || info == nil || request == nil || usage == nil {
		return
	}
	decision := ProfitResponseCacheObservationFromContext(c)
	if decision == nil || !decision.Eligible || decision.LiveServed || decision.Hit {
		return
	}
	body, ok := ResponseCacheCapturedBodyFromContext(c)
	if !ok || len(body) == 0 {
		decision.BypassReason = "response_body_not_captured"
		return
	}
	if err := profit.PutResponseCache(decision, body, usage, request.Model); err != nil {
		decision.BypassReason = "cache_store_failed"
		logger.LogWarn(c, "response cache store failed: "+err.Error())
		return
	}
	decision.Stored = true
}

func CaptureResponseCacheBody(c *gin.Context, body []byte) {
	if c == nil || len(body) == 0 {
		return
	}
	c.Set(profitResponseCacheBodyKey, append([]byte(nil), body...))
}

func ResponseCacheCapturedBodyFromContext(c *gin.Context) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(profitResponseCacheBodyKey)
	if !ok {
		return nil, false
	}
	body, ok := value.([]byte)
	if !ok || len(body) == 0 {
		return nil, false
	}
	return append([]byte(nil), body...), true
}

func ProfitResponseCacheObservationFromContext(c *gin.Context) *profit.ResponseCacheDecision {
	if c == nil {
		return nil
	}
	value, ok := c.Get(profitResponseCacheDecisionKey)
	if !ok {
		return nil
	}
	decision, ok := value.(*profit.ResponseCacheDecision)
	if !ok {
		return nil
	}
	return decision
}

func responseCacheSessionKey(c *gin.Context, request *dto.GeneralOpenAIRequest) string {
	if c != nil {
		if header := strings.TrimSpace(c.GetHeader(responseCacheSessionHeader)); header != "" {
			return header
		}
	}
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.PromptCacheKey)
}
