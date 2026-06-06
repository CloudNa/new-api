package profit

import "github.com/QuantumNous/new-api/common"

const (
	ObservationVersion = 1

	DefaultObserveGroup = "proxy-test"

	KeyObserveVersion                 = "profit_observe_version"
	KeyCostStatus                     = "profit_cost_status"
	KeyCostProfileID                  = "profit_cost_profile_id"
	KeyCostProfileName                = "profit_cost_profile_name"
	KeyBillablePromptTokens           = "billable_prompt_tokens"
	KeyBillableCompletionTokens       = "billable_completion_tokens"
	KeyUpstreamActualPromptTokens     = "upstream_actual_prompt_tokens"
	KeyUpstreamActualCompletionTokens = "upstream_actual_completion_tokens"
	KeyEstimatedRevenueUSD            = "estimated_revenue_usd"
	KeyEstimatedUpstreamCostUSD       = "estimated_upstream_cost_usd"
	KeyGrossMarginUSD                 = "gross_margin_usd"
	KeyGrossMarginPct                 = "gross_margin_pct"
	KeyCompressionSavedTokens         = "compression_saved_tokens"
	KeyCompressionMode                = "compression_mode"
	KeyCompressionSavingsPercent      = "compression_savings_percent"
	KeyCompressionBypassed            = "compression_bypassed"
	KeyCompressionBypassReason        = "compression_bypass_reason"
	KeyCompressionRulesApplied        = "compression_rules_applied"
	KeyCompressionRulesVersion        = "compression_rules_version"
	KeyCompressionPreservedBlocks     = "compression_preserved_blocks"
	KeyCompressionRedactedSecrets     = "compression_redacted_secrets"
	KeyCacheSavedUSD                  = "cache_saved_usd"
	KeyRetryCostUSD                   = "retry_cost_usd"
	KeyExpectedCostUSD                = "expected_cost_usd"
	KeyExpectedMarginUSD              = "expected_margin_usd"
	KeyExpectedMarginPct              = "expected_margin_pct"
	KeyRouteMode                      = "profit_route_mode"
	KeyRouteCandidateCount            = "profit_route_candidate_count"
	KeyRouteSelectedChannelID         = "profit_route_selected_channel_id"
	KeyRouteSelectedMarginRank        = "profit_route_selected_margin_rank"
	KeyRouteBestChannelID             = "profit_route_best_channel_id"
	KeyRouteBestChannelName           = "profit_route_best_channel_name"
	KeyRouteBestCostProfileID         = "profit_route_best_cost_profile_id"
	KeyRouteBestExpectedMarginUSD     = "profit_route_best_expected_margin_usd"
	KeyRouteWouldPreferDifferent      = "profit_route_would_prefer_different"
	KeyRouteCandidates                = "profit_route_candidates"
	KeyOutputPolicyMode               = "output_policy_mode"
	KeyOutputPolicyID                 = "output_policy_id"
	KeyOutputPolicyName               = "output_policy_name"
	KeyOutputPolicyCompletionTokens   = "output_policy_completion_tokens"
	KeyOutputPolicyDefaultMaxTokens   = "output_policy_default_max_tokens"
	KeyOutputPolicyHardMaxTokens      = "output_policy_hard_max_tokens"
	KeyOutputPolicyExceededDefault    = "output_policy_exceeded_default"
	KeyOutputPolicyExceededHard       = "output_policy_exceeded_hard"
	KeyOutputPolicyRewriteOverLimit   = "output_policy_rewrite_over_limit"
	KeyOutputPolicyWouldCap           = "output_policy_would_cap"
	KeyOutputPolicyPremiumRequired    = "output_policy_premium_required"
	KeyOutputPolicyPremiumGroup       = "output_policy_premium_group"
	KeyOutputPolicyObserveOnly        = "output_policy_observe_only"
	KeyOutputPolicyLiveEnforced       = "output_policy_live_enforced"
	CostStatusMissingCostProfile      = "missing_cost_profile"
)

var userHiddenKeys = []string{
	KeyObserveVersion,
	KeyCostStatus,
	KeyCostProfileID,
	KeyCostProfileName,
	KeyBillablePromptTokens,
	KeyBillableCompletionTokens,
	KeyUpstreamActualPromptTokens,
	KeyUpstreamActualCompletionTokens,
	KeyEstimatedRevenueUSD,
	KeyEstimatedUpstreamCostUSD,
	KeyGrossMarginUSD,
	KeyGrossMarginPct,
	KeyCompressionSavedTokens,
	KeyCompressionMode,
	KeyCompressionSavingsPercent,
	KeyCompressionBypassed,
	KeyCompressionBypassReason,
	KeyCompressionRulesApplied,
	KeyCompressionRulesVersion,
	KeyCompressionPreservedBlocks,
	KeyCompressionRedactedSecrets,
	KeyCacheSavedUSD,
	KeyRetryCostUSD,
	KeyExpectedCostUSD,
	KeyExpectedMarginUSD,
	KeyExpectedMarginPct,
	KeyRouteMode,
	KeyRouteCandidateCount,
	KeyRouteSelectedChannelID,
	KeyRouteSelectedMarginRank,
	KeyRouteBestChannelID,
	KeyRouteBestChannelName,
	KeyRouteBestCostProfileID,
	KeyRouteBestExpectedMarginUSD,
	KeyRouteWouldPreferDifferent,
	KeyRouteCandidates,
	KeyOutputPolicyMode,
	KeyOutputPolicyID,
	KeyOutputPolicyName,
	KeyOutputPolicyCompletionTokens,
	KeyOutputPolicyDefaultMaxTokens,
	KeyOutputPolicyHardMaxTokens,
	KeyOutputPolicyExceededDefault,
	KeyOutputPolicyExceededHard,
	KeyOutputPolicyRewriteOverLimit,
	KeyOutputPolicyWouldCap,
	KeyOutputPolicyPremiumRequired,
	KeyOutputPolicyPremiumGroup,
	KeyOutputPolicyObserveOnly,
	KeyOutputPolicyLiveEnforced,
}

type ObservationInput struct {
	Group                          string
	Provider                       string
	ChannelID                      int
	ChannelName                    string
	ModelName                      string
	BillablePromptTokens           int
	BillableCompletionTokens       int
	UpstreamActualPromptTokens     int
	UpstreamActualCompletionTokens int
	CacheReadTokens                int
	CacheWriteTokens               int
	UserQuota                      int
	CompressionSavedTokens         int
	LatencyMs                      int
	RouteDecision                  *RouteDecision
	OutputPolicyDecision           *OutputPolicyDecision
}

func EnabledForGroup(group string) bool {
	settings := CurrentSettings()
	return settingsEnabledForGroup(settings, group)
}

func settingsEnabledForGroup(settings Settings, group string) bool {
	settings = settings.Normalize()
	if !settings.Enabled || settings.GlobalKillSwitch || group != DefaultObserveGroup {
		return false
	}
	for _, enabledGroup := range settings.ObserveGroups {
		if enabledGroup == group {
			return true
		}
	}
	return false
}

func AppendObservation(other map[string]interface{}, input ObservationInput) {
	if other == nil || !EnabledForGroup(input.Group) {
		return
	}

	revenueUSD := quotaToUSD(input.UserQuota)
	costEstimate := EstimateCost(CostInput{
		Group:                    input.Group,
		Provider:                 input.Provider,
		ChannelID:                input.ChannelID,
		ChannelName:              input.ChannelName,
		ModelName:                input.ModelName,
		BillablePromptTokens:     input.BillablePromptTokens,
		BillableCompletionTokens: input.BillableCompletionTokens,
		UpstreamPromptTokens:     input.UpstreamActualPromptTokens,
		UpstreamCompletionTokens: input.UpstreamActualCompletionTokens,
		CacheReadTokens:          input.CacheReadTokens,
		CacheWriteTokens:         input.CacheWriteTokens,
		RevenueUSD:               revenueUSD,
		LatencyMs:                input.LatencyMs,
	}, CurrentCostProfiles())

	other[KeyObserveVersion] = ObservationVersion
	other[KeyCostStatus] = costEstimate.CostStatus
	if costEstimate.CostProfileID != "" {
		other[KeyCostProfileID] = costEstimate.CostProfileID
	}
	if costEstimate.CostProfileName != "" {
		other[KeyCostProfileName] = costEstimate.CostProfileName
	}
	other[KeyBillablePromptTokens] = positiveInt(input.BillablePromptTokens)
	other[KeyBillableCompletionTokens] = positiveInt(input.BillableCompletionTokens)
	other[KeyUpstreamActualPromptTokens] = positiveInt(input.UpstreamActualPromptTokens)
	other[KeyUpstreamActualCompletionTokens] = positiveInt(input.UpstreamActualCompletionTokens)
	other[KeyEstimatedRevenueUSD] = revenueUSD
	other[KeyEstimatedUpstreamCostUSD] = costEstimate.EstimatedUpstreamCostUSD
	other[KeyGrossMarginUSD] = costEstimate.GrossMarginUSD
	other[KeyGrossMarginPct] = costEstimate.GrossMarginPct
	other[KeyCompressionSavedTokens] = positiveInt(input.CompressionSavedTokens)
	other[KeyCacheSavedUSD] = nil
	other[KeyRetryCostUSD] = nil
	other[KeyExpectedCostUSD] = costEstimate.ExpectedCostUSD
	other[KeyExpectedMarginUSD] = costEstimate.ExpectedMarginUSD
	other[KeyExpectedMarginPct] = costEstimate.ExpectedMarginPct
	appendRouteDecision(other, input.RouteDecision)
	appendOutputPolicyDecision(other, input.OutputPolicyDecision)
}

func StripUserVisibleFields(other map[string]interface{}) {
	if other == nil {
		return
	}
	for _, key := range userHiddenKeys {
		delete(other, key)
	}
}

func IsObservation(other map[string]interface{}) bool {
	if other == nil {
		return false
	}
	_, ok := other[KeyObserveVersion]
	return ok
}

func positiveInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func appendRouteDecision(other map[string]interface{}, decision *RouteDecision) {
	if other == nil || decision == nil {
		return
	}
	other[KeyRouteMode] = decision.Mode
	other[KeyRouteCandidateCount] = positiveInt(decision.CandidateCount)
	other[KeyRouteSelectedChannelID] = positiveInt(decision.SelectedChannelID)
	other[KeyRouteSelectedMarginRank] = decision.SelectedMarginRank
	other[KeyRouteBestChannelID] = positiveInt(decision.BestChannelID)
	if decision.BestChannelName != "" {
		other[KeyRouteBestChannelName] = decision.BestChannelName
	}
	if decision.BestCostProfileID != "" {
		other[KeyRouteBestCostProfileID] = decision.BestCostProfileID
	}
	other[KeyRouteBestExpectedMarginUSD] = decision.BestExpectedMarginUSD
	other[KeyRouteWouldPreferDifferent] = decision.WouldPreferDifferent
	if len(decision.Candidates) > 0 {
		other[KeyRouteCandidates] = decision.Candidates
	}
}

func appendOutputPolicyDecision(other map[string]interface{}, decision *OutputPolicyDecision) {
	if other == nil || decision == nil {
		return
	}
	other[KeyOutputPolicyMode] = decision.Mode
	if decision.PolicyID != "" {
		other[KeyOutputPolicyID] = decision.PolicyID
	}
	if decision.PolicyName != "" {
		other[KeyOutputPolicyName] = decision.PolicyName
	}
	other[KeyOutputPolicyCompletionTokens] = positiveInt(decision.CompletionTokens)
	other[KeyOutputPolicyDefaultMaxTokens] = positiveInt(decision.DefaultMaxTokens)
	other[KeyOutputPolicyHardMaxTokens] = positiveInt(decision.HardMaxTokens)
	other[KeyOutputPolicyExceededDefault] = decision.ExceededDefault
	other[KeyOutputPolicyExceededHard] = decision.ExceededHard
	other[KeyOutputPolicyRewriteOverLimit] = decision.RewriteOverLimit
	other[KeyOutputPolicyWouldCap] = decision.WouldCap
	other[KeyOutputPolicyPremiumRequired] = decision.PremiumRequired
	if decision.PremiumGroup != "" {
		other[KeyOutputPolicyPremiumGroup] = decision.PremiumGroup
	}
	other[KeyOutputPolicyObserveOnly] = decision.ObserveOnly
	other[KeyOutputPolicyLiveEnforced] = decision.LiveEnforced
}

func quotaToUSD(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}
