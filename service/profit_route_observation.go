package service

import (
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/pkg/profit"

	"github.com/gin-gonic/gin"
)

type profitRouteDecisionInput struct {
	Group                          string
	Provider                       string
	SelectedChannelID              int
	SelectedChannelName            string
	ModelName                      string
	BillablePromptTokens           int
	BillableCompletionTokens       int
	UpstreamActualPromptTokens     int
	UpstreamActualCompletionTokens int
	CacheReadTokens                int
	CacheWriteTokens               int
	UserQuota                      int
	LatencyMs                      int
}

func buildProfitRouteDecision(ctx *gin.Context, input profitRouteDecisionInput) *profit.RouteDecision {
	if !profit.EnabledForGroup(input.Group) {
		return nil
	}
	settings := profit.CurrentSettings()
	settings = settings.Normalize()
	if settings.CostRoutingMode != profit.ModeObserve && settings.CostRoutingMode != profit.ModePreferMargin {
		return nil
	}

	channelCandidates, err := model.GetSatisfiedChannelCandidatesForProfitObservation(
		input.Group,
		input.ModelName,
		profit.DefaultRouteCandidateLimit,
	)
	if err != nil {
		logger.LogWarn(ctx, "profit route observation skipped: "+err.Error())
		return nil
	}

	channelIDs := make([]int, 0, len(channelCandidates))
	for _, channel := range channelCandidates {
		channelIDs = append(channelIDs, channel.ChannelID)
	}
	channelHealth, healthErr := perfmetrics.QueryChannelHealth(perfmetrics.ChannelHealthParams{
		Model:      input.ModelName,
		Group:      input.Group,
		ChannelIDs: channelIDs,
		Hours:      settings.CostRoutingHealthWindowHours,
	})
	if healthErr != nil {
		logger.LogWarn(ctx, "profit channel health observation skipped: "+healthErr.Error())
		channelHealth = nil
	}

	candidates := make([]profit.RouteCandidateInput, 0, len(channelCandidates))
	for _, channel := range channelCandidates {
		latencyMs := channel.ResponseTime
		failureRate := 0.0
		if channel.ChannelID == input.SelectedChannelID && input.LatencyMs > 0 {
			latencyMs = input.LatencyMs
		} else if health, ok := channelHealth[channel.ChannelID]; ok && health.AvgLatencyMs > 0 {
			latencyMs = int(health.AvgLatencyMs)
		}
		if health, ok := channelHealth[channel.ChannelID]; ok && health.RequestCount > 0 {
			failureRate = health.FailureRate / 100
		}
		candidates = append(candidates, profit.RouteCandidateInput{
			Provider:                       channel.ChannelName,
			ChannelID:                      channel.ChannelID,
			ChannelName:                    channel.ChannelName,
			ModelName:                      input.ModelName,
			BillablePromptTokens:           input.BillablePromptTokens,
			BillableCompletionTokens:       input.BillableCompletionTokens,
			UpstreamActualPromptTokens:     input.UpstreamActualPromptTokens,
			UpstreamActualCompletionTokens: input.UpstreamActualCompletionTokens,
			CacheReadTokens:                input.CacheReadTokens,
			CacheWriteTokens:               input.CacheWriteTokens,
			UserQuota:                      input.UserQuota,
			LatencyMs:                      latencyMs,
			FailureRate:                    failureRate,
			HealthRequestCount:             channelHealth[channel.ChannelID].RequestCount,
			HealthSuccessRatePct:           channelHealth[channel.ChannelID].SuccessRate,
			Priority:                       channel.Priority,
			Weight:                         channel.Weight,
		})
	}

	decision := profit.BuildRouteDecisionWithSettings(settings, profit.RouteDecisionInput{
		Group:                          input.Group,
		Provider:                       input.Provider,
		SelectedChannelID:              input.SelectedChannelID,
		SelectedChannelName:            input.SelectedChannelName,
		ModelName:                      input.ModelName,
		BillablePromptTokens:           input.BillablePromptTokens,
		BillableCompletionTokens:       input.BillableCompletionTokens,
		UpstreamActualPromptTokens:     input.UpstreamActualPromptTokens,
		UpstreamActualCompletionTokens: input.UpstreamActualCompletionTokens,
		CacheReadTokens:                input.CacheReadTokens,
		CacheWriteTokens:               input.CacheWriteTokens,
		UserQuota:                      input.UserQuota,
		LatencyMs:                      input.LatencyMs,
		Candidates:                     candidates,
	})
	if decision != nil {
		decision.LiveRoutingUsed = ctx.GetBool(profit.KeyRouteLiveRoutingUsed)
		decision.BypassReason = ctx.GetString(profit.KeyRouteBypassReason)
		if !decision.LiveRoutingUsed && decision.BypassReason == "" {
			decision.BypassReason = profit.InferRouteBypassReason(settings, decision)
		}
	}
	return decision
}
