package service

import (
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
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

	candidates := make([]profit.RouteCandidateInput, 0, len(channelCandidates))
	for _, channel := range channelCandidates {
		latencyMs := channel.ResponseTime
		if channel.ChannelID == input.SelectedChannelID && input.LatencyMs > 0 {
			latencyMs = input.LatencyMs
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
			Priority:                       channel.Priority,
			Weight:                         channel.Weight,
		})
	}

	return profit.BuildRouteDecision(profit.RouteDecisionInput{
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
}
