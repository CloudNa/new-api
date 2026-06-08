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
	KeyCompressionEngine              = "compression_engine"
	KeyCompressionTimestamp           = "compression_timestamp"
	KeyCompressionFallbackApplied     = "compression_fallback_applied"
	KeyCompressionValidationWarnings  = "compression_validation_warnings"
	KeyCompressionValidationErrors    = "compression_validation_errors"
	KeyCompressionEngineBreakdown     = "compression_engine_breakdown"
	KeyCacheReadTokens                = "cache_read_tokens"
	KeyCacheWriteTokens               = "cache_write_tokens"
	KeyCacheSavedUSD                  = "cache_saved_usd"
	KeyResponseCacheMode              = "response_cache_mode"
	KeyResponseCacheEligible          = "response_cache_eligible"
	KeyResponseCacheHit               = "response_cache_hit"
	KeyResponseCacheWouldHit          = "response_cache_would_hit"
	KeyResponseCacheLiveServed        = "response_cache_live_served"
	KeyResponseCacheStored            = "response_cache_stored"
	KeyResponseCacheRuleID            = "response_cache_rule_id"
	KeyResponseCacheRuleName          = "response_cache_rule_name"
	KeyResponseCacheKeyHash           = "response_cache_key_hash"
	KeyResponseCacheScope             = "response_cache_scope"
	KeyResponseCacheTTLSeconds        = "response_cache_ttl_seconds"
	KeyResponseCacheBypassReason      = "response_cache_bypass_reason"
	KeyResponseCacheObserveOnly       = "response_cache_observe_only"
	KeyResponseCacheSavedUSD          = "response_cache_saved_usd"
	KeyRetryCostUSD                   = "retry_cost_usd"
	KeyRetryAttemptCount              = "profit_retry_attempt_count"
	KeyRetryAttempts                  = "profit_retry_attempts"
	KeyLongContextMode                = "long_context_mode"
	KeyLongContextPolicyID            = "long_context_policy_id"
	KeyLongContextPolicyName          = "long_context_policy_name"
	KeyLongContextTierID              = "long_context_tier_id"
	KeyLongContextTierName            = "long_context_tier_name"
	KeyLongContextTokens              = "long_context_tokens"
	KeyLongContextMinTokens           = "long_context_min_tokens"
	KeyLongContextMaxTokens           = "long_context_max_tokens"
	KeyLongContextInputMultiplier     = "long_context_input_multiplier"
	KeyLongContextInputRevenueUSD     = "long_context_input_revenue_usd"
	KeyLongContextSuggestedExtraUSD   = "long_context_suggested_extra_revenue_usd"
	KeyLongContextSuggestedRevenueUSD = "long_context_suggested_revenue_usd"
	KeyLongContextPremiumRequired     = "long_context_premium_required"
	KeyLongContextPremiumGroup        = "long_context_premium_group"
	KeyLongContextObserveOnly         = "long_context_observe_only"
	KeyLongContextLiveEnforced        = "long_context_live_enforced"
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
	KeyRouteLiveRoutingUsed           = "profit_route_live_routing_used"
	KeyRouteBypassReason              = "profit_route_bypass_reason"
	KeyRouteHealthMinSamples          = "profit_route_health_min_samples"
	KeyRouteHealthMinSuccessRatePct   = "profit_route_health_min_success_rate_pct"
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
	KeyRiskMode                       = "profit_risk_mode"
	KeyRiskAlert                      = "profit_risk_alert"
	KeyRiskReasons                    = "profit_risk_reasons"
	KeyRiskMinGrossMarginUSD          = "profit_risk_min_gross_margin_usd"
	KeyRiskMinGrossMarginPct          = "profit_risk_min_gross_margin_pct"
	KeyRiskMinExpectedMarginUSD       = "profit_risk_min_expected_margin_usd"
	KeyRiskMinExpectedMarginPct       = "profit_risk_min_expected_margin_pct"
	KeyRiskObserveOnly                = "profit_risk_observe_only"
	KeyRiskLiveEnforced               = "profit_risk_live_enforced"
	KeyModelAliasApplied              = "sku_alias_applied"
	KeyModelAliasMode                 = "sku_alias_mode"
	KeyModelAliasID                   = "sku_alias_id"
	KeyModelAliasName                 = "sku_alias_name"
	KeyModelAliasSKU                  = "sku_alias_sku"
	KeyModelAliasUpstreamModel        = "sku_alias_upstream_model"
	KeyModelAliasTargetChannelID      = "sku_alias_target_channel_id"
	KeyModelAliasTargetChannelName    = "sku_alias_target_channel_name"
	KeyModelAliasCandidateCount       = "sku_alias_candidate_count"
	KeyModelAliasObserveOnly          = "sku_alias_observe_only"
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
	KeyCompressionEngine,
	KeyCompressionTimestamp,
	KeyCompressionFallbackApplied,
	KeyCompressionValidationWarnings,
	KeyCompressionValidationErrors,
	KeyCompressionEngineBreakdown,
	KeyCacheReadTokens,
	KeyCacheWriteTokens,
	KeyCacheSavedUSD,
	KeyResponseCacheMode,
	KeyResponseCacheEligible,
	KeyResponseCacheHit,
	KeyResponseCacheWouldHit,
	KeyResponseCacheLiveServed,
	KeyResponseCacheStored,
	KeyResponseCacheRuleID,
	KeyResponseCacheRuleName,
	KeyResponseCacheKeyHash,
	KeyResponseCacheScope,
	KeyResponseCacheTTLSeconds,
	KeyResponseCacheBypassReason,
	KeyResponseCacheObserveOnly,
	KeyResponseCacheSavedUSD,
	KeyRetryCostUSD,
	KeyRetryAttemptCount,
	KeyRetryAttempts,
	KeyLongContextMode,
	KeyLongContextPolicyID,
	KeyLongContextPolicyName,
	KeyLongContextTierID,
	KeyLongContextTierName,
	KeyLongContextTokens,
	KeyLongContextMinTokens,
	KeyLongContextMaxTokens,
	KeyLongContextInputMultiplier,
	KeyLongContextInputRevenueUSD,
	KeyLongContextSuggestedExtraUSD,
	KeyLongContextSuggestedRevenueUSD,
	KeyLongContextPremiumRequired,
	KeyLongContextPremiumGroup,
	KeyLongContextObserveOnly,
	KeyLongContextLiveEnforced,
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
	KeyRouteLiveRoutingUsed,
	KeyRouteBypassReason,
	KeyRouteHealthMinSamples,
	KeyRouteHealthMinSuccessRatePct,
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
	KeyRiskMode,
	KeyRiskAlert,
	KeyRiskReasons,
	KeyRiskMinGrossMarginUSD,
	KeyRiskMinGrossMarginPct,
	KeyRiskMinExpectedMarginUSD,
	KeyRiskMinExpectedMarginPct,
	KeyRiskObserveOnly,
	KeyRiskLiveEnforced,
	KeyModelAliasApplied,
	KeyModelAliasMode,
	KeyModelAliasID,
	KeyModelAliasName,
	KeyModelAliasSKU,
	KeyModelAliasUpstreamModel,
	KeyModelAliasTargetChannelID,
	KeyModelAliasTargetChannelName,
	KeyModelAliasCandidateCount,
	KeyModelAliasObserveOnly,
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
	LongContextDecision            *LongContextDecision
	RetryCostUSD                   *float64
	RetryAttemptCount              int
	RetryAttempts                  []RetryAttemptObservation
	ModelAliasDecision             *ModelAliasDecision
	ResponseCacheDecision          *ResponseCacheDecision
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
	settings := CurrentSettings()
	if other == nil || !settingsEnabledForGroup(settings, input.Group) {
		return
	}

	revenueUSD := quotaToUSD(input.UserQuota)
	costModelName := input.ModelName
	if input.ModelAliasDecision != nil && input.ModelAliasDecision.Applied && input.ModelAliasDecision.UpstreamModelName != "" {
		costModelName = input.ModelAliasDecision.UpstreamModelName
	}
	responseCacheLiveServed := input.ResponseCacheDecision != nil && input.ResponseCacheDecision.LiveServed
	costEstimate := EstimateCost(CostInput{
		Group:                    input.Group,
		Provider:                 input.Provider,
		ChannelID:                input.ChannelID,
		ChannelName:              input.ChannelName,
		ModelName:                costModelName,
		BillablePromptTokens:     input.BillablePromptTokens,
		BillableCompletionTokens: input.BillableCompletionTokens,
		UpstreamPromptTokens:     input.UpstreamActualPromptTokens,
		UpstreamCompletionTokens: input.UpstreamActualCompletionTokens,
		CacheReadTokens:          input.CacheReadTokens,
		CacheWriteTokens:         input.CacheWriteTokens,
		RevenueUSD:               revenueUSD,
		LatencyMs:                input.LatencyMs,
		ActualUsageKnown:         responseCacheLiveServed,
		NoUpstreamRequest:        responseCacheLiveServed,
	}, CurrentCostProfiles())
	riskDecision := BuildRiskDecision(settings, costEstimate)
	longContextDecision := input.LongContextDecision
	if longContextDecision == nil {
		longContextDecision = BuildLongContextDecisionWithSettings(LongContextInput{
			Group:                    input.Group,
			ModelName:                input.ModelName,
			ChannelID:                input.ChannelID,
			ChannelName:              input.ChannelName,
			BillablePromptTokens:     input.BillablePromptTokens,
			BillableCompletionTokens: input.BillableCompletionTokens,
			EstimatedRevenueUSD:      revenueUSD,
		}, settings)
	}

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
	other[KeyCacheReadTokens] = positiveInt(input.CacheReadTokens)
	other[KeyCacheWriteTokens] = positiveInt(input.CacheWriteTokens)
	if settings.CacheMode == ModeObserve {
		other[KeyCacheSavedUSD] = costEstimate.CacheSavedUSD
	} else {
		other[KeyCacheSavedUSD] = nil
	}
	appendResponseCacheDecision(other, input, costEstimate)
	if input.RetryCostUSD != nil {
		other[KeyRetryCostUSD] = input.RetryCostUSD
	} else {
		other[KeyRetryCostUSD] = nil
	}
	if input.RetryAttemptCount > 0 {
		other[KeyRetryAttemptCount] = positiveInt(input.RetryAttemptCount)
	}
	if len(input.RetryAttempts) > 0 {
		other[KeyRetryAttempts] = input.RetryAttempts
	}
	appendModelAliasDecision(other, input.ModelAliasDecision)
	appendLongContextDecision(other, longContextDecision)
	other[KeyExpectedCostUSD] = costEstimate.ExpectedCostUSD
	other[KeyExpectedMarginUSD] = costEstimate.ExpectedMarginUSD
	other[KeyExpectedMarginPct] = costEstimate.ExpectedMarginPct
	appendRouteDecision(other, input.RouteDecision)
	appendOutputPolicyDecision(other, input.OutputPolicyDecision)
	appendRiskDecision(other, riskDecision)
}

func appendResponseCacheDecision(other map[string]interface{}, input ObservationInput, costEstimate CostEstimate) {
	if other == nil || input.ResponseCacheDecision == nil {
		return
	}
	decision := input.ResponseCacheDecision
	other[KeyResponseCacheMode] = decision.Mode
	other[KeyResponseCacheEligible] = decision.Eligible
	other[KeyResponseCacheHit] = decision.Hit
	other[KeyResponseCacheWouldHit] = decision.WouldHit
	other[KeyResponseCacheLiveServed] = decision.LiveServed
	other[KeyResponseCacheStored] = decision.Stored
	if decision.RuleID != "" {
		other[KeyResponseCacheRuleID] = decision.RuleID
	}
	if decision.RuleName != "" {
		other[KeyResponseCacheRuleName] = decision.RuleName
	}
	if decision.KeyHash != "" {
		other[KeyResponseCacheKeyHash] = decision.KeyHash
	}
	if decision.Scope != "" {
		other[KeyResponseCacheScope] = decision.Scope
	}
	if decision.TTLSeconds > 0 {
		other[KeyResponseCacheTTLSeconds] = decision.TTLSeconds
	}
	if decision.BypassReason != "" {
		other[KeyResponseCacheBypassReason] = decision.BypassReason
	}
	other[KeyResponseCacheObserveOnly] = decision.ObserveOnly
	if !decision.LiveServed {
		return
	}
	withoutCacheEstimate := EstimateCost(CostInput{
		Group:                    input.Group,
		Provider:                 input.Provider,
		ChannelID:                input.ChannelID,
		ChannelName:              input.ChannelName,
		ModelName:                input.ModelName,
		BillablePromptTokens:     input.BillablePromptTokens,
		BillableCompletionTokens: input.BillableCompletionTokens,
		UpstreamPromptTokens:     input.BillablePromptTokens,
		UpstreamCompletionTokens: input.BillableCompletionTokens,
		RevenueUSD:               costEstimate.EstimatedRevenueUSD,
		LatencyMs:                input.LatencyMs,
		ActualUsageKnown:         true,
	}, CurrentCostProfiles())
	if withoutCacheEstimate.EstimatedUpstreamCostUSD == nil || costEstimate.EstimatedUpstreamCostUSD == nil {
		return
	}
	savedUSD := *withoutCacheEstimate.EstimatedUpstreamCostUSD - *costEstimate.EstimatedUpstreamCostUSD
	if savedUSD < 0 {
		savedUSD = 0
	}
	other[KeyResponseCacheSavedUSD] = &savedUSD
}

func appendModelAliasDecision(other map[string]interface{}, decision *ModelAliasDecision) {
	if other == nil || decision == nil || !decision.Applied {
		return
	}
	other[KeyModelAliasApplied] = true
	other[KeyModelAliasMode] = decision.Mode
	if decision.AliasID != "" {
		other[KeyModelAliasID] = decision.AliasID
	}
	if decision.AliasName != "" {
		other[KeyModelAliasName] = decision.AliasName
	}
	if decision.SKU != "" {
		other[KeyModelAliasSKU] = decision.SKU
	}
	if decision.UpstreamModelName != "" {
		other[KeyModelAliasUpstreamModel] = decision.UpstreamModelName
	}
	other[KeyModelAliasTargetChannelID] = positiveInt(decision.TargetChannelID)
	if decision.TargetChannelName != "" {
		other[KeyModelAliasTargetChannelName] = decision.TargetChannelName
	}
	other[KeyModelAliasCandidateCount] = positiveInt(decision.CandidateCount)
	other[KeyModelAliasObserveOnly] = decision.ObserveOnly
}

func appendLongContextDecision(other map[string]interface{}, decision *LongContextDecision) {
	if other == nil || decision == nil {
		return
	}
	other[KeyLongContextMode] = decision.Mode
	if decision.PolicyID != "" {
		other[KeyLongContextPolicyID] = decision.PolicyID
	}
	if decision.PolicyName != "" {
		other[KeyLongContextPolicyName] = decision.PolicyName
	}
	if decision.TierID != "" {
		other[KeyLongContextTierID] = decision.TierID
	}
	if decision.TierName != "" {
		other[KeyLongContextTierName] = decision.TierName
	}
	other[KeyLongContextTokens] = positiveInt(decision.ContextTokens)
	other[KeyLongContextMinTokens] = positiveInt(decision.MinContextTokens)
	other[KeyLongContextMaxTokens] = positiveInt(decision.MaxContextTokens)
	other[KeyLongContextInputMultiplier] = decision.InputMultiplier
	other[KeyLongContextInputRevenueUSD] = decision.EstimatedInputRevenueUSD
	other[KeyLongContextSuggestedExtraUSD] = decision.SuggestedExtraRevenueUSD
	other[KeyLongContextSuggestedRevenueUSD] = decision.SuggestedRevenueUSD
	other[KeyLongContextPremiumRequired] = decision.PremiumRequired
	if decision.PremiumGroup != "" {
		other[KeyLongContextPremiumGroup] = decision.PremiumGroup
	}
	other[KeyLongContextObserveOnly] = decision.ObserveOnly
	other[KeyLongContextLiveEnforced] = decision.LiveEnforced
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
	other[KeyRouteLiveRoutingUsed] = decision.LiveRoutingUsed
	if decision.BypassReason != "" {
		other[KeyRouteBypassReason] = decision.BypassReason
	}
	if decision.HealthMinSamples > 0 {
		other[KeyRouteHealthMinSamples] = decision.HealthMinSamples
	}
	if decision.HealthMinSuccessRate > 0 {
		other[KeyRouteHealthMinSuccessRatePct] = decision.HealthMinSuccessRate
	}
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

func appendRiskDecision(other map[string]interface{}, decision *RiskDecision) {
	if other == nil || decision == nil {
		return
	}
	other[KeyRiskMode] = decision.Mode
	other[KeyRiskAlert] = decision.Alert
	if len(decision.Reasons) > 0 {
		other[KeyRiskReasons] = decision.Reasons
	}
	other[KeyRiskMinGrossMarginUSD] = decision.MinGrossMarginUSD
	other[KeyRiskMinGrossMarginPct] = decision.MinGrossMarginPct
	other[KeyRiskMinExpectedMarginUSD] = decision.MinExpectedMarginUSD
	other[KeyRiskMinExpectedMarginPct] = decision.MinExpectedMarginPct
	other[KeyRiskObserveOnly] = decision.ObserveOnly
	other[KeyRiskLiveEnforced] = decision.LiveEnforced
}

func quotaToUSD(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}
