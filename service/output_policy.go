package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const profitOutputPolicyDecisionKey = "profit_output_policy_decision"

func ApplyOutputPolicyForRelay(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, passThroughBody bool) {
	if c == nil || info == nil || request == nil || passThroughBody {
		return
	}
	hasMaxTokens := request.MaxCompletionTokens != nil || request.MaxTokens != nil
	requestedMaxTokens := int(request.GetMaxTokens())
	decision := profit.BuildOutputPolicyRequestDecision(profit.OutputPolicyInput{
		Group:               info.UsingGroup,
		ModelName:           request.Model,
		ChannelID:           info.ChannelId,
		ChannelName:         c.GetString("channel_name"),
		RequestedMaxTokens:  requestedMaxTokens,
		MaxTokensConfigured: hasMaxTokens,
	})
	if decision == nil {
		return
	}
	c.Set(profitOutputPolicyDecisionKey, decision)
	if !decision.LiveEnforced || decision.AppliedMaxTokens <= 0 {
		return
	}
	applied := uint(decision.AppliedMaxTokens)
	if request.MaxCompletionTokens != nil {
		request.MaxCompletionTokens = &applied
	}
	if request.MaxTokens != nil {
		request.MaxTokens = &applied
	}
	if request.MaxCompletionTokens == nil && request.MaxTokens == nil {
		request.MaxTokens = &applied
	}
}

func ProfitOutputPolicyDecisionFromContext(c *gin.Context) *profit.OutputPolicyDecision {
	if c == nil {
		return nil
	}
	value, ok := c.Get(profitOutputPolicyDecisionKey)
	if !ok {
		return nil
	}
	decision, ok := value.(*profit.OutputPolicyDecision)
	if !ok {
		return nil
	}
	return decision
}

func MergeOutputPolicyCompletion(decision *profit.OutputPolicyDecision, completionTokens int) *profit.OutputPolicyDecision {
	if decision == nil {
		return nil
	}
	next := *decision
	next.CompletionTokens = max(0, completionTokens)
	next.ExceededDefault = next.DefaultMaxTokens > 0 && next.CompletionTokens > next.DefaultMaxTokens
	next.ExceededHard = next.HardMaxTokens > 0 && next.CompletionTokens > next.HardMaxTokens
	next.EnforcedLimitExceeded = next.LiveEnforced && next.AppliedMaxTokens > 0 && next.CompletionTokens > next.AppliedMaxTokens
	if next.Mode == profit.ModeCap {
		next.WouldCap = next.WouldCap || next.ExceededHard || (next.RewriteOverLimit && next.ExceededDefault)
	}
	if next.Mode == profit.ModePremiumRequired && (next.ExceededDefault || next.ExceededHard) {
		next.PremiumRequired = true
	}
	return &next
}
