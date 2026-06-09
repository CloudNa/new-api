package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func BuildProfitRiskPrecheckDecision(info *relaycommon.RelayInfo, promptTokens int, maxCompletionTokens int, preConsumedQuota int) (*profit.RiskDecision, profit.CostEstimate) {
	estimate := profit.CostEstimate{CostStatus: profit.CostStatusMissingCostProfile}
	if info == nil || !profit.EnabledForGroup(info.UsingGroup) {
		return nil, estimate
	}
	settings := profit.CurrentSettings()
	settings = settings.Normalize()
	costModelName := profitRoutingModelName(info, info.OriginModelName)
	promptTokens = max(0, promptTokens)
	maxCompletionTokens = max(0, maxCompletionTokens)
	estimate = profit.EstimateCost(profit.CostInput{
		Group:                    info.UsingGroup,
		ModelName:                costModelName,
		BillablePromptTokens:     promptTokens,
		BillableCompletionTokens: maxCompletionTokens,
		UpstreamPromptTokens:     promptTokens,
		UpstreamCompletionTokens: maxCompletionTokens,
		RevenueUSD:               profit.QuotaToUSD(preConsumedQuota),
	}, profit.CurrentCostProfiles())
	return profit.BuildRiskDecision(settings, estimate), estimate
}

func EnforceProfitRiskBeforeRelay(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, maxCompletionTokens int, preConsumedQuota int) *types.NewAPIError {
	decision, estimate := BuildProfitRiskPrecheckDecision(info, promptTokens, maxCompletionTokens, preConsumedQuota)
	if decision == nil || !decision.LiveEnforced {
		return nil
	}
	recordProfitRiskPrecheckLog(c, info, promptTokens, maxCompletionTokens, preConsumedQuota, decision, estimate)
	return types.NewErrorWithStatusCode(
		fmt.Errorf("请求未通过平台收益风控，请降低输出上限或切换模型后重试"),
		types.ErrorCodeProfitRiskGuardrail,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	)
}

func recordProfitRiskPrecheckLog(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, maxCompletionTokens int, preConsumedQuota int, decision *profit.RiskDecision, estimate profit.CostEstimate) {
	if c == nil || info == nil || decision == nil {
		return
	}
	costModelName := profitRoutingModelName(info, info.OriginModelName)
	other := map[string]interface{}{}
	profit.AppendObservation(other, profit.ObservationInput{
		Group:                          info.UsingGroup,
		ModelName:                      costModelName,
		BillablePromptTokens:           promptTokens,
		BillableCompletionTokens:       maxCompletionTokens,
		UpstreamActualPromptTokens:     promptTokens,
		UpstreamActualCompletionTokens: maxCompletionTokens,
		UserQuota:                      preConsumedQuota,
	})
	if estimate.CostProfileID != "" {
		other[profit.KeyCostProfileID] = estimate.CostProfileID
	}
	if estimate.CostProfileName != "" {
		other[profit.KeyCostProfileName] = estimate.CostProfileName
	}
	content := "收益风控拦截"
	if len(decision.Reasons) > 0 {
		content += ": " + strings.Join(decision.Reasons, ",")
	}
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId:        0,
		PromptTokens:     promptTokens,
		CompletionTokens: maxCompletionTokens,
		ModelName:        costModelName,
		TokenName:        c.GetString("token_name"),
		Quota:            0,
		Content:          content,
		TokenId:          info.TokenId,
		UseTimeSeconds:   0,
		IsStream:         info.IsStream,
		Group:            info.UsingGroup,
		Other:            other,
	})
}
