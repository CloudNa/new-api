package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const profitRetryAttemptsContextKey = "glart_profit_retry_attempts"

func RecordProfitRetryAttempt(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel, err *types.NewAPIError, willRetry bool) bool {
	if c == nil || info == nil || channel == nil || err == nil || !profit.EnabledForGroup(info.UsingGroup) {
		return willRetry
	}
	promptTokens := retryAttemptPromptTokens(info)
	settings := profit.CurrentSettings()
	estimate := profit.EstimateCost(profit.CostInput{
		Group:                    info.UsingGroup,
		Provider:                 channel.Name,
		ChannelID:                channel.Id,
		ChannelName:              channel.Name,
		ModelName:                info.OriginModelName,
		BillablePromptTokens:     info.GetEstimatePromptTokens(),
		UpstreamPromptTokens:     promptTokens,
		UpstreamCompletionTokens: 0,
		RevenueUSD:               0,
		FailureRate:              1,
	}, profit.CurrentCostProfiles())
	attempts := profitRetryAttemptsFromContext(c)
	retryCost := retryAttemptCost(&estimate)
	decision := profit.BuildRetryBudgetDecisionWithSettings(profit.RetryBudgetInput{
		BaseWillRetry:       willRetry,
		CurrentRetryCostUSD: currentRetryCostUSD(attempts),
		NextRetryCostUSD:    retryCost,
		CostEstimate:        estimate,
	}, settings)
	finalWillRetry := decision.FinalWillRetry(willRetry)
	attempt := profit.RetryAttemptObservation{
		Index:                    len(attempts) + 1,
		ChannelID:                channel.Id,
		ChannelName:              channel.Name,
		ModelName:                info.OriginModelName,
		PromptTokens:             promptTokens,
		CompletionTokens:         0,
		StatusCode:               err.StatusCode,
		ErrorType:                string(err.GetErrorType()),
		ErrorCode:                string(err.GetErrorCode()),
		CostKnown:                estimate.CostKnown,
		CostStatus:               estimate.CostStatus,
		CostProfileID:            estimate.CostProfileID,
		CostProfileName:          estimate.CostProfileName,
		EstimatedUpstreamCostUSD: estimate.EstimatedUpstreamCostUSD,
		ExpectedRetryCostUSD:     retryCost,
		BaseWillRetry:            willRetry,
		WillRetry:                finalWillRetry,
		PlatformBorne:            true,
		RetryBudgetMode:          decision.Mode,
		MaxRetryCostUSD:          decision.MaxRetryCostUSD,
		CurrentRetryCostUSD:      decision.CurrentRetryCostUSD,
		RetryBudgetExceeded:      decision.BudgetExceeded,
		RetryBudgetLowMargin:     decision.LowMargin,
		RetryBudgetWouldSkip:     decision.WouldSkipRetry,
		RetryBudgetObserveOnly:   decision.ObserveOnly,
		RetryBudgetLiveEnforced:  decision.LiveEnforced,
		RetryBudgetBypassReason:  decision.BypassReason,
		RetryBudgetReason:        decision.Reason,
	}
	attempts = append(attempts, attempt)
	c.Set(profitRetryAttemptsContextKey, attempts)
	return finalWillRetry
}

func ProfitRetryObservationFromContext(c *gin.Context) (*float64, int, []profit.RetryAttemptObservation) {
	attempts := profitRetryAttemptsFromContext(c)
	if len(attempts) == 0 {
		return nil, 0, nil
	}
	var total float64
	hasCost := false
	for _, attempt := range attempts {
		if attempt.ExpectedRetryCostUSD == nil {
			continue
		}
		total += *attempt.ExpectedRetryCostUSD
		hasCost = true
	}
	if !hasCost {
		return nil, len(attempts), attempts
	}
	return &total, len(attempts), attempts
}

func profitRetryAttemptsFromContext(c *gin.Context) []profit.RetryAttemptObservation {
	if c == nil {
		return nil
	}
	value, ok := c.Get(profitRetryAttemptsContextKey)
	if !ok {
		return nil
	}
	attempts, ok := value.([]profit.RetryAttemptObservation)
	if !ok {
		return nil
	}
	return attempts
}

func retryAttemptCost(estimate *profit.CostEstimate) *float64 {
	if estimate == nil {
		return nil
	}
	if estimate.ExpectedCostUSD != nil {
		return estimate.ExpectedCostUSD
	}
	return estimate.EstimatedUpstreamCostUSD
}

func currentRetryCostUSD(attempts []profit.RetryAttemptObservation) float64 {
	var total float64
	for _, attempt := range attempts {
		if attempt.ExpectedRetryCostUSD == nil {
			continue
		}
		total += *attempt.ExpectedRetryCostUSD
	}
	return total
}

func retryAttemptPromptTokens(info *relaycommon.RelayInfo) int {
	if info == nil {
		return 0
	}
	if stats := info.PromptCompressionStats; stats != nil && !stats.Bypassed && stats.CompressedTokens > 0 {
		return stats.CompressedTokens
	}
	return info.GetEstimatePromptTokens()
}
