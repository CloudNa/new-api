package model

import (
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/QuantumNous/new-api/pkg/promptcompress"

	"gorm.io/gorm"
)

const profitAnalyticsScanLimit = 20000
const outputPolicyRecommendationMinSamples = 20
const outputPolicyRecommendationTokenStep int64 = 64

const (
	profitGuardrailActionNone                 = "none"
	profitGuardrailActionKeepObserving        = "keep_observing"
	profitGuardrailActionKeepObservingOutput  = "keep_observing_output_tail"
	profitGuardrailActionCollectOutputSamples = "collect_more_output_samples"
	profitGuardrailActionEnableOutputCap      = "enable_output_cap_observe"
	profitGuardrailActionReviewPricingOrCost  = "review_pricing_or_cost"

	profitGuardrailReasonNoRequests                        = "no_requests"
	profitGuardrailReasonNoMarginRisk                      = "no_margin_risk"
	profitGuardrailReasonOutputTailWithoutMarginRisk       = "output_tail_without_margin_risk"
	profitGuardrailReasonLossWithOutputTail                = "loss_with_output_tail"
	profitGuardrailReasonLowMarginWithOutputTail           = "low_margin_with_output_tail"
	profitGuardrailReasonMarginRiskWithInsufficientSamples = "margin_risk_with_insufficient_output_samples"
	profitGuardrailReasonMarginRiskWithoutOutputSamples    = "margin_risk_without_output_samples"
)

type ProfitLogFilter struct {
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Username       string
	Channel        int
	Group          string
}

type ProfitEvent struct {
	Id                             int                                  `json:"id"`
	CreatedAt                      int64                                `json:"created_at"`
	UserId                         int                                  `json:"user_id"`
	Username                       string                               `json:"username"`
	TokenName                      string                               `json:"token_name"`
	ModelName                      string                               `json:"model_name"`
	ChannelId                      int                                  `json:"channel"`
	ChannelName                    string                               `json:"channel_name"`
	Group                          string                               `json:"group"`
	RequestId                      string                               `json:"request_id,omitempty"`
	UpstreamRequestId              string                               `json:"upstream_request_id,omitempty"`
	PromptTokens                   int                                  `json:"prompt_tokens"`
	CompletionTokens               int                                  `json:"completion_tokens"`
	Quota                          int                                  `json:"quota"`
	UseTime                        int                                  `json:"use_time"`
	IsStream                       bool                                 `json:"is_stream"`
	CostStatus                     string                               `json:"profit_cost_status"`
	CostKnown                      bool                                 `json:"cost_known"`
	CostProfileID                  string                               `json:"cost_profile_id,omitempty"`
	CostProfileName                string                               `json:"cost_profile_name,omitempty"`
	BillablePromptTokens           int64                                `json:"billable_prompt_tokens"`
	BillableCompletionTokens       int64                                `json:"billable_completion_tokens"`
	UpstreamActualPromptTokens     int64                                `json:"upstream_actual_prompt_tokens"`
	UpstreamActualCompletionTokens int64                                `json:"upstream_actual_completion_tokens"`
	EstimatedRevenueUSD            float64                              `json:"estimated_revenue_usd"`
	EstimatedUpstreamCostUSD       *float64                             `json:"estimated_upstream_cost_usd"`
	GrossMarginUSD                 *float64                             `json:"gross_margin_usd"`
	GrossMarginPct                 *float64                             `json:"gross_margin_pct"`
	ExpectedCostUSD                *float64                             `json:"expected_cost_usd"`
	ExpectedMarginUSD              *float64                             `json:"expected_margin_usd"`
	ExpectedMarginPct              *float64                             `json:"expected_margin_pct"`
	CompressionSavedTokens         int64                                `json:"compression_saved_tokens"`
	CompressionMode                string                               `json:"compression_mode,omitempty"`
	CompressionEngine              string                               `json:"compression_engine,omitempty"`
	CompressionTimestamp           int64                                `json:"compression_timestamp,omitempty"`
	CompressionFallbackApplied     bool                                 `json:"compression_fallback_applied"`
	CompressionSavingsPercent      *float64                             `json:"compression_savings_percent,omitempty"`
	CompressionBypassed            bool                                 `json:"compression_bypassed"`
	CompressionBypassReason        string                               `json:"compression_bypass_reason,omitempty"`
	CompressionRulesVersion        string                               `json:"compression_rules_version,omitempty"`
	CompressionRulesApplied        []string                             `json:"compression_rules_applied,omitempty"`
	CompressionPreservedBlocks     int64                                `json:"compression_preserved_blocks"`
	CompressionRedactedSecrets     int64                                `json:"compression_redacted_secrets"`
	CompressionValidationWarnings  []string                             `json:"compression_validation_warnings,omitempty"`
	CompressionValidationErrors    []string                             `json:"compression_validation_errors,omitempty"`
	CompressionEngineBreakdown     []promptcompress.EngineBreakdownItem `json:"compression_engine_breakdown,omitempty"`
	RouteMode                      string                               `json:"profit_route_mode,omitempty"`
	RouteCandidateCount            int64                                `json:"profit_route_candidate_count"`
	RouteSelectedChannelID         int                                  `json:"profit_route_selected_channel_id"`
	RouteSelectedMarginRank        int64                                `json:"profit_route_selected_margin_rank"`
	RouteBestChannelID             int                                  `json:"profit_route_best_channel_id"`
	RouteBestChannelName           string                               `json:"profit_route_best_channel_name,omitempty"`
	RouteBestCostProfileID         string                               `json:"profit_route_best_cost_profile_id,omitempty"`
	RouteBestExpectedMarginUSD     *float64                             `json:"profit_route_best_expected_margin_usd,omitempty"`
	RouteWouldPreferDifferent      bool                                 `json:"profit_route_would_prefer_different"`
	RouteCandidates                []profit.RouteDecisionCandidate      `json:"profit_route_candidates,omitempty"`
	RouteLiveRoutingUsed           bool                                 `json:"profit_route_live_routing_used"`
	RouteBypassReason              string                               `json:"profit_route_bypass_reason,omitempty"`
	RouteHealthMinSamples          int64                                `json:"profit_route_health_min_samples"`
	RouteHealthMinSuccessRatePct   float64                              `json:"profit_route_health_min_success_rate_pct"`
	OutputPolicyMode               string                               `json:"output_policy_mode,omitempty"`
	OutputPolicyID                 string                               `json:"output_policy_id,omitempty"`
	OutputPolicyName               string                               `json:"output_policy_name,omitempty"`
	OutputPolicyCompletionTokens   int64                                `json:"output_policy_completion_tokens"`
	OutputPolicyDefaultMaxTokens   int64                                `json:"output_policy_default_max_tokens"`
	OutputPolicyHardMaxTokens      int64                                `json:"output_policy_hard_max_tokens"`
	OutputPolicyExceededDefault    bool                                 `json:"output_policy_exceeded_default"`
	OutputPolicyExceededHard       bool                                 `json:"output_policy_exceeded_hard"`
	OutputPolicyRewriteOverLimit   bool                                 `json:"output_policy_rewrite_over_limit"`
	OutputPolicyWouldCap           bool                                 `json:"output_policy_would_cap"`
	OutputPolicyPremiumRequired    bool                                 `json:"output_policy_premium_required"`
	OutputPolicyPremiumGroup       string                               `json:"output_policy_premium_group,omitempty"`
	OutputPolicyObserveOnly        bool                                 `json:"output_policy_observe_only"`
	OutputPolicyLiveEnforced       bool                                 `json:"output_policy_live_enforced"`
	RiskMode                       string                               `json:"profit_risk_mode,omitempty"`
	RiskAlert                      bool                                 `json:"profit_risk_alert"`
	RiskReasons                    []string                             `json:"profit_risk_reasons,omitempty"`
	RiskMinGrossMarginUSD          float64                              `json:"profit_risk_min_gross_margin_usd"`
	RiskMinGrossMarginPct          float64                              `json:"profit_risk_min_gross_margin_pct"`
	RiskMinExpectedMarginUSD       float64                              `json:"profit_risk_min_expected_margin_usd"`
	RiskMinExpectedMarginPct       float64                              `json:"profit_risk_min_expected_margin_pct"`
	RiskObserveOnly                bool                                 `json:"profit_risk_observe_only"`
	RiskLiveEnforced               bool                                 `json:"profit_risk_live_enforced"`
	LongContextMode                string                               `json:"long_context_mode,omitempty"`
	LongContextPolicyID            string                               `json:"long_context_policy_id,omitempty"`
	LongContextPolicyName          string                               `json:"long_context_policy_name,omitempty"`
	LongContextTierID              string                               `json:"long_context_tier_id,omitempty"`
	LongContextTierName            string                               `json:"long_context_tier_name,omitempty"`
	LongContextTokens              int64                                `json:"long_context_tokens"`
	LongContextMinTokens           int64                                `json:"long_context_min_tokens"`
	LongContextMaxTokens           int64                                `json:"long_context_max_tokens"`
	LongContextInputMultiplier     float64                              `json:"long_context_input_multiplier"`
	LongContextInputRevenueUSD     float64                              `json:"long_context_input_revenue_usd"`
	LongContextSuggestedExtraUSD   float64                              `json:"long_context_suggested_extra_revenue_usd"`
	LongContextSuggestedRevenueUSD float64                              `json:"long_context_suggested_revenue_usd"`
	LongContextPremiumRequired     bool                                 `json:"long_context_premium_required"`
	LongContextPremiumGroup        string                               `json:"long_context_premium_group,omitempty"`
	LongContextObserveOnly         bool                                 `json:"long_context_observe_only"`
	LongContextLiveEnforced        bool                                 `json:"long_context_live_enforced"`
	CacheSavedUSD                  *float64                             `json:"cache_saved_usd"`
	RetryCostUSD                   *float64                             `json:"retry_cost_usd"`
}

type ProfitAnalytics struct {
	RequestCount                         int64                `json:"request_count"`
	ScannedEvents                        int64                `json:"scanned_events"`
	TotalMatchingLogs                    int64                `json:"total_matching_logs"`
	IsPartial                            bool                 `json:"is_partial"`
	ScanLimit                            int                  `json:"scan_limit"`
	CostKnownCount                       int64                `json:"cost_known_count"`
	MissingCostProfileCount              int64                `json:"missing_cost_profile_count"`
	BillablePromptTokens                 int64                `json:"billable_prompt_tokens"`
	BillableCompletionTokens             int64                `json:"billable_completion_tokens"`
	UpstreamActualPromptTokens           int64                `json:"upstream_actual_prompt_tokens"`
	UpstreamActualCompletionTokens       int64                `json:"upstream_actual_completion_tokens"`
	EstimatedRevenueUSD                  float64              `json:"estimated_revenue_usd"`
	EstimatedUpstreamCostUSD             *float64             `json:"estimated_upstream_cost_usd"`
	GrossMarginUSD                       *float64             `json:"gross_margin_usd"`
	GrossMarginPct                       *float64             `json:"gross_margin_pct"`
	ExpectedCostUSD                      *float64             `json:"expected_cost_usd"`
	ExpectedMarginUSD                    *float64             `json:"expected_margin_usd"`
	ExpectedMarginPct                    *float64             `json:"expected_margin_pct"`
	CompressionSavedTokens               int64                `json:"compression_saved_tokens"`
	OutputPolicyObservedCount            int64                `json:"output_policy_observed_count"`
	OutputPolicyExceededDefaultCount     int64                `json:"output_policy_exceeded_default_count"`
	OutputPolicyExceededHardCount        int64                `json:"output_policy_exceeded_hard_count"`
	OutputPolicyWouldCapCount            int64                `json:"output_policy_would_cap_count"`
	OutputPolicyPremiumRequiredCount     int64                `json:"output_policy_premium_required_count"`
	OutputPolicyCompletionTokens         int64                `json:"output_policy_completion_tokens"`
	OutputPolicyCompletionSampleCount    int64                `json:"output_policy_completion_sample_count"`
	OutputPolicyCompletionAvgTokens      float64              `json:"output_policy_completion_avg_tokens"`
	OutputPolicyCompletionP95Tokens      int64                `json:"output_policy_completion_p95_tokens"`
	OutputPolicyCompletionP99Tokens      int64                `json:"output_policy_completion_p99_tokens"`
	OutputPolicyCompletionMaxTokens      int64                `json:"output_policy_completion_max_tokens"`
	OutputPolicyRecommendedDefaultMax    int64                `json:"output_policy_recommended_default_max_tokens"`
	OutputPolicyRecommendedHardMax       int64                `json:"output_policy_recommended_hard_max_tokens"`
	OutputPolicyRecommendationConfidence string               `json:"output_policy_recommendation_confidence"`
	OutputPolicyRecommendationReason     string               `json:"output_policy_recommendation_reason"`
	ProfitGuardrailAction                string               `json:"profit_guardrail_action"`
	ProfitGuardrailReason                string               `json:"profit_guardrail_reason"`
	ProfitGuardrailConfidence            string               `json:"profit_guardrail_confidence"`
	ProfitGuardrailOutputMode            string               `json:"profit_guardrail_output_mode"`
	ProfitGuardrailDefaultMaxTokens      int64                `json:"profit_guardrail_default_max_tokens"`
	ProfitGuardrailHardMaxTokens         int64                `json:"profit_guardrail_hard_max_tokens"`
	ProfitGuardrailPolicyTemplate        *profit.OutputPolicy `json:"profit_guardrail_policy_template,omitempty"`
	ProfitGuardrailPolicyTemplateJSON    string               `json:"profit_guardrail_policy_template_json,omitempty"`
	RiskObservedCount                    int64                `json:"profit_risk_observed_count"`
	RiskAlertCount                       int64                `json:"profit_risk_alert_count"`
	RiskLossMakingCount                  int64                `json:"profit_risk_loss_making_count"`
	RiskLowGrossMarginCount              int64                `json:"profit_risk_low_gross_margin_count"`
	RiskLowExpectedMarginCount           int64                `json:"profit_risk_low_expected_margin_count"`
	LongContextObservedCount             int64                `json:"long_context_observed_count"`
	LongContextPremiumRequiredCount      int64                `json:"long_context_premium_required_count"`
	LongContextTokens                    int64                `json:"long_context_tokens"`
	LongContextSuggestedExtraUSD         float64              `json:"long_context_suggested_extra_revenue_usd"`
	CacheSavedUSD                        *float64             `json:"cache_saved_usd"`
	RetryCostUSD                         *float64             `json:"retry_cost_usd"`
}

func GetProfitEvents(filter ProfitLogFilter, startIdx int, num int) (events []*ProfitEvent, total int64, err error) {
	tx, err := buildProfitLogQuery(filter)
	if err != nil {
		return nil, 0, err
	}
	if err = tx.Model(&Log{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []*Log
	if err = tx.Order("logs.created_at desc, logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	events = make([]*ProfitEvent, 0, len(logs))
	for _, log := range logs {
		event, ok := profitEventFromLog(log)
		if !ok {
			continue
		}
		events = append(events, event)
	}
	if err = attachProfitEventChannelNames(events); err != nil {
		return events, total, err
	}
	return events, total, nil
}

func GetProfitAnalytics(filter ProfitLogFilter) (ProfitAnalytics, error) {
	analytics := ProfitAnalytics{ScanLimit: profitAnalyticsScanLimit}
	tx, err := buildProfitLogQuery(filter)
	if err != nil {
		return analytics, err
	}
	if err = tx.Model(&Log{}).Count(&analytics.TotalMatchingLogs).Error; err != nil {
		return analytics, err
	}

	var logs []*Log
	if err = tx.Order("logs.created_at desc, logs.id desc").Limit(profitAnalyticsScanLimit).Find(&logs).Error; err != nil {
		return analytics, err
	}
	analytics.ScannedEvents = int64(len(logs))
	analytics.IsPartial = analytics.TotalMatchingLogs > analytics.ScannedEvents

	var upstreamCostSum float64
	var grossMarginSum float64
	var grossMarginRevenueBase float64
	var expectedCostSum float64
	var expectedMarginSum float64
	var expectedMarginRevenueBase float64
	var cacheSavedSum float64
	var retryCostSum float64
	var hasExpectedCost bool
	var hasCacheSaved bool
	var hasRetryCost bool
	var outputCompletionSamples []int64

	for _, log := range logs {
		event, ok := profitEventFromLog(log)
		if !ok {
			continue
		}
		analytics.RequestCount++
		analytics.BillablePromptTokens += event.BillablePromptTokens
		analytics.BillableCompletionTokens += event.BillableCompletionTokens
		analytics.UpstreamActualPromptTokens += event.UpstreamActualPromptTokens
		analytics.UpstreamActualCompletionTokens += event.UpstreamActualCompletionTokens
		analytics.EstimatedRevenueUSD += event.EstimatedRevenueUSD
		analytics.CompressionSavedTokens += event.CompressionSavedTokens
		if event.OutputPolicyMode != "" {
			analytics.OutputPolicyObservedCount++
			analytics.OutputPolicyCompletionTokens += event.OutputPolicyCompletionTokens
			if event.OutputPolicyCompletionTokens > 0 {
				outputCompletionSamples = append(outputCompletionSamples, event.OutputPolicyCompletionTokens)
			}
			if event.OutputPolicyExceededDefault {
				analytics.OutputPolicyExceededDefaultCount++
			}
			if event.OutputPolicyExceededHard {
				analytics.OutputPolicyExceededHardCount++
			}
			if event.OutputPolicyWouldCap {
				analytics.OutputPolicyWouldCapCount++
			}
			if event.OutputPolicyPremiumRequired {
				analytics.OutputPolicyPremiumRequiredCount++
			}
		}
		if event.RiskMode != "" {
			analytics.RiskObservedCount++
			if event.RiskAlert {
				analytics.RiskAlertCount++
			}
			if stringSliceContains(event.RiskReasons, profit.RiskReasonLossMakingRequest) {
				analytics.RiskLossMakingCount++
			}
			if stringSliceContains(event.RiskReasons, profit.RiskReasonGrossMarginBelowMinimum) {
				analytics.RiskLowGrossMarginCount++
			}
			if stringSliceContains(event.RiskReasons, profit.RiskReasonExpectedMarginBelowMinimum) {
				analytics.RiskLowExpectedMarginCount++
			}
		}
		if event.LongContextMode != "" {
			analytics.LongContextObservedCount++
			analytics.LongContextTokens += event.LongContextTokens
			analytics.LongContextSuggestedExtraUSD += event.LongContextSuggestedExtraUSD
			if event.LongContextPremiumRequired {
				analytics.LongContextPremiumRequiredCount++
			}
		}
		if event.CostStatus == profit.CostStatusMissingCostProfile {
			analytics.MissingCostProfileCount++
		}
		if event.EstimatedUpstreamCostUSD != nil {
			analytics.CostKnownCount++
			upstreamCostSum += *event.EstimatedUpstreamCostUSD
			grossMarginRevenueBase += event.EstimatedRevenueUSD
			if event.GrossMarginUSD != nil {
				grossMarginSum += *event.GrossMarginUSD
			} else {
				grossMarginSum += event.EstimatedRevenueUSD - *event.EstimatedUpstreamCostUSD
			}
		}
		if event.ExpectedCostUSD != nil {
			hasExpectedCost = true
			expectedCostSum += *event.ExpectedCostUSD
			expectedMarginRevenueBase += event.EstimatedRevenueUSD
			if event.ExpectedMarginUSD != nil {
				expectedMarginSum += *event.ExpectedMarginUSD
			} else {
				expectedMarginSum += event.EstimatedRevenueUSD - *event.ExpectedCostUSD
			}
		}
		if event.CacheSavedUSD != nil {
			hasCacheSaved = true
			cacheSavedSum += *event.CacheSavedUSD
		}
		if event.RetryCostUSD != nil {
			hasRetryCost = true
			retryCostSum += *event.RetryCostUSD
		}
	}

	if analytics.CostKnownCount > 0 {
		analytics.EstimatedUpstreamCostUSD = floatPtr(upstreamCostSum)
		analytics.GrossMarginUSD = floatPtr(grossMarginSum)
		if grossMarginRevenueBase > 0 {
			analytics.GrossMarginPct = floatPtr(grossMarginSum / grossMarginRevenueBase * 100)
		}
	}
	if hasExpectedCost {
		analytics.ExpectedCostUSD = floatPtr(expectedCostSum)
		analytics.ExpectedMarginUSD = floatPtr(expectedMarginSum)
		if expectedMarginRevenueBase > 0 {
			analytics.ExpectedMarginPct = floatPtr(expectedMarginSum / expectedMarginRevenueBase * 100)
		}
	}
	if hasCacheSaved {
		analytics.CacheSavedUSD = floatPtr(cacheSavedSum)
	}
	if hasRetryCost {
		analytics.RetryCostUSD = floatPtr(retryCostSum)
	}
	if len(outputCompletionSamples) > 0 {
		sort.Slice(outputCompletionSamples, func(i, j int) bool {
			return outputCompletionSamples[i] < outputCompletionSamples[j]
		})
		analytics.OutputPolicyCompletionSampleCount = int64(len(outputCompletionSamples))
		analytics.OutputPolicyCompletionAvgTokens = float64(analytics.OutputPolicyCompletionTokens) / float64(len(outputCompletionSamples))
		analytics.OutputPolicyCompletionP95Tokens = nearestRankPercentile(outputCompletionSamples, 95)
		analytics.OutputPolicyCompletionP99Tokens = nearestRankPercentile(outputCompletionSamples, 99)
		analytics.OutputPolicyCompletionMaxTokens = outputCompletionSamples[len(outputCompletionSamples)-1]
		analytics.OutputPolicyRecommendedDefaultMax = roundUpToMultiple(analytics.OutputPolicyCompletionP95Tokens, outputPolicyRecommendationTokenStep)
		analytics.OutputPolicyRecommendedHardMax = roundUpToMultiple(analytics.OutputPolicyCompletionP99Tokens, outputPolicyRecommendationTokenStep)
		if analytics.OutputPolicyRecommendedHardMax < analytics.OutputPolicyRecommendedDefaultMax {
			analytics.OutputPolicyRecommendedHardMax = analytics.OutputPolicyRecommendedDefaultMax
		}
		if analytics.OutputPolicyCompletionSampleCount < outputPolicyRecommendationMinSamples {
			analytics.OutputPolicyRecommendationConfidence = "low"
			analytics.OutputPolicyRecommendationReason = "insufficient_samples"
		} else if analytics.OutputPolicyCompletionSampleCount < 100 {
			analytics.OutputPolicyRecommendationConfidence = "medium"
			analytics.OutputPolicyRecommendationReason = "p95_p99_observed"
		} else {
			analytics.OutputPolicyRecommendationConfidence = "high"
			analytics.OutputPolicyRecommendationReason = "p95_p99_observed"
		}
	} else {
		analytics.OutputPolicyRecommendationConfidence = "none"
		analytics.OutputPolicyRecommendationReason = "no_samples"
	}
	applyProfitGuardrailRecommendation(&analytics)
	applyProfitGuardrailPolicyTemplate(&analytics, filter)

	return analytics, nil
}

func applyProfitGuardrailRecommendation(analytics *ProfitAnalytics) {
	if analytics == nil {
		return
	}
	analytics.ProfitGuardrailAction = profitGuardrailActionKeepObserving
	analytics.ProfitGuardrailReason = profitGuardrailReasonNoMarginRisk
	analytics.ProfitGuardrailConfidence = "none"
	if analytics.RequestCount == 0 {
		analytics.ProfitGuardrailAction = profitGuardrailActionNone
		analytics.ProfitGuardrailReason = profitGuardrailReasonNoRequests
		return
	}
	hasMarginRisk := analytics.RiskLossMakingCount > 0 ||
		analytics.RiskLowGrossMarginCount > 0 ||
		analytics.RiskLowExpectedMarginCount > 0
	if !hasMarginRisk {
		if analytics.OutputPolicyWouldCapCount > 0 {
			analytics.ProfitGuardrailAction = profitGuardrailActionKeepObservingOutput
			analytics.ProfitGuardrailReason = profitGuardrailReasonOutputTailWithoutMarginRisk
			analytics.ProfitGuardrailConfidence = analytics.OutputPolicyRecommendationConfidence
		}
		return
	}
	if analytics.OutputPolicyCompletionSampleCount >= outputPolicyRecommendationMinSamples &&
		analytics.OutputPolicyRecommendedHardMax > 0 {
		analytics.ProfitGuardrailAction = profitGuardrailActionEnableOutputCap
		if analytics.RiskLossMakingCount > 0 {
			analytics.ProfitGuardrailReason = profitGuardrailReasonLossWithOutputTail
		} else {
			analytics.ProfitGuardrailReason = profitGuardrailReasonLowMarginWithOutputTail
		}
		analytics.ProfitGuardrailConfidence = analytics.OutputPolicyRecommendationConfidence
		analytics.ProfitGuardrailOutputMode = profit.ModeCap
		analytics.ProfitGuardrailDefaultMaxTokens = analytics.OutputPolicyRecommendedDefaultMax
		analytics.ProfitGuardrailHardMaxTokens = analytics.OutputPolicyRecommendedHardMax
		return
	}
	if analytics.OutputPolicyCompletionSampleCount > 0 {
		analytics.ProfitGuardrailAction = profitGuardrailActionCollectOutputSamples
		analytics.ProfitGuardrailReason = profitGuardrailReasonMarginRiskWithInsufficientSamples
		analytics.ProfitGuardrailConfidence = analytics.OutputPolicyRecommendationConfidence
		return
	}
	analytics.ProfitGuardrailAction = profitGuardrailActionReviewPricingOrCost
	analytics.ProfitGuardrailReason = profitGuardrailReasonMarginRiskWithoutOutputSamples
	analytics.ProfitGuardrailConfidence = "low"
}

func applyProfitGuardrailPolicyTemplate(analytics *ProfitAnalytics, filter ProfitLogFilter) {
	if analytics == nil ||
		analytics.ProfitGuardrailAction != profitGuardrailActionEnableOutputCap ||
		analytics.ProfitGuardrailDefaultMaxTokens <= 0 ||
		analytics.ProfitGuardrailHardMaxTokens <= 0 {
		return
	}
	group := strings.TrimSpace(filter.Group)
	if group != profit.DefaultObserveGroup {
		return
	}
	modelName := strings.TrimSpace(filter.ModelName)
	id := group + "-output-cap-recommended"
	if safeModel := safePolicyIDPart(modelName); safeModel != "" {
		id += "-" + safeModel
	}
	if filter.Channel > 0 {
		id += "-ch-" + strconv.Itoa(filter.Channel)
	}
	template := profit.OutputPolicy{
		ID:               id,
		Name:             group + " output cap observe",
		Enabled:          true,
		Priority:         90,
		Group:            group,
		ModelName:        modelName,
		ChannelID:        filter.Channel,
		Mode:             profit.ModeCap,
		DefaultMaxTokens: int64ToInt(analytics.ProfitGuardrailDefaultMaxTokens),
		HardMaxTokens:    int64ToInt(analytics.ProfitGuardrailHardMaxTokens),
		RewriteOverLimit: true,
		PremiumRequired:  false,
		PremiumGroup:     "premium",
		Notes:            "Generated from profit guardrail observe analytics; keep global observe_only=true before live enforcement.",
	}
	payload, err := common.Marshal([]profit.OutputPolicy{template})
	if err != nil {
		return
	}
	analytics.ProfitGuardrailPolicyTemplate = &template
	analytics.ProfitGuardrailPolicyTemplateJSON = string(payload)
}

func safePolicyIDPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func int64ToInt(value int64) int {
	if value <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}

func nearestRankPercentile(sortedValues []int64, percentile int) int64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedValues[0]
	}
	if percentile >= 100 {
		return sortedValues[len(sortedValues)-1]
	}
	rank := (percentile*len(sortedValues) + 99) / 100
	return sortedValues[rank-1]
}

func roundUpToMultiple(value int64, step int64) int64 {
	if value <= 0 || step <= 0 {
		return 0
	}
	return ((value + step - 1) / step) * step
}

func buildProfitLogQuery(filter ProfitLogFilter) (*gorm.DB, error) {
	tx := LOG_DB.Model(&Log{}).
		Where("logs.type = ?", LogTypeConsume).
		Where("logs.other LIKE ?", "%"+profit.KeyObserveVersion+"%")

	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", filter.ModelName); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", filter.Username); err != nil {
		return nil, err
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", filter.EndTimestamp)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", filter.Group)
	}
	return tx, nil
}

func profitEventFromLog(log *Log) (*ProfitEvent, bool) {
	other, err := common.StrToMap(log.Other)
	if err != nil || !profit.IsObservation(other) {
		return nil, false
	}

	upstreamCost := optionalFloat(other, profit.KeyEstimatedUpstreamCostUSD)
	event := &ProfitEvent{
		Id:                             log.Id,
		CreatedAt:                      log.CreatedAt,
		UserId:                         log.UserId,
		Username:                       log.Username,
		TokenName:                      log.TokenName,
		ModelName:                      log.ModelName,
		ChannelId:                      log.ChannelId,
		Group:                          log.Group,
		RequestId:                      log.RequestId,
		UpstreamRequestId:              log.UpstreamRequestId,
		PromptTokens:                   log.PromptTokens,
		CompletionTokens:               log.CompletionTokens,
		Quota:                          log.Quota,
		UseTime:                        log.UseTime,
		IsStream:                       log.IsStream,
		CostStatus:                     stringValue(other, profit.KeyCostStatus),
		CostKnown:                      upstreamCost != nil,
		CostProfileID:                  stringValue(other, profit.KeyCostProfileID),
		CostProfileName:                stringValue(other, profit.KeyCostProfileName),
		BillablePromptTokens:           int64Value(other, profit.KeyBillablePromptTokens),
		BillableCompletionTokens:       int64Value(other, profit.KeyBillableCompletionTokens),
		UpstreamActualPromptTokens:     int64Value(other, profit.KeyUpstreamActualPromptTokens),
		UpstreamActualCompletionTokens: int64Value(other, profit.KeyUpstreamActualCompletionTokens),
		EstimatedRevenueUSD:            floatValue(other, profit.KeyEstimatedRevenueUSD),
		EstimatedUpstreamCostUSD:       upstreamCost,
		GrossMarginUSD:                 optionalFloat(other, profit.KeyGrossMarginUSD),
		GrossMarginPct:                 optionalFloat(other, profit.KeyGrossMarginPct),
		ExpectedCostUSD:                optionalFloat(other, profit.KeyExpectedCostUSD),
		ExpectedMarginUSD:              optionalFloat(other, profit.KeyExpectedMarginUSD),
		ExpectedMarginPct:              optionalFloat(other, profit.KeyExpectedMarginPct),
		CompressionSavedTokens:         int64Value(other, profit.KeyCompressionSavedTokens),
		CompressionMode:                stringValue(other, profit.KeyCompressionMode),
		CompressionEngine:              stringValue(other, profit.KeyCompressionEngine),
		CompressionTimestamp:           int64Value(other, profit.KeyCompressionTimestamp),
		CompressionFallbackApplied:     boolValue(other, profit.KeyCompressionFallbackApplied),
		CompressionSavingsPercent:      optionalFloat(other, profit.KeyCompressionSavingsPercent),
		CompressionBypassed:            boolValue(other, profit.KeyCompressionBypassed),
		CompressionBypassReason:        stringValue(other, profit.KeyCompressionBypassReason),
		CompressionRulesVersion:        stringValue(other, profit.KeyCompressionRulesVersion),
		CompressionRulesApplied:        stringSliceValue(other, profit.KeyCompressionRulesApplied),
		CompressionPreservedBlocks:     int64Value(other, profit.KeyCompressionPreservedBlocks),
		CompressionRedactedSecrets:     int64Value(other, profit.KeyCompressionRedactedSecrets),
		CompressionValidationWarnings:  stringSliceValue(other, profit.KeyCompressionValidationWarnings),
		CompressionValidationErrors:    stringSliceValue(other, profit.KeyCompressionValidationErrors),
		CompressionEngineBreakdown:     compressionEngineBreakdownValue(other, profit.KeyCompressionEngineBreakdown),
		RouteMode:                      stringValue(other, profit.KeyRouteMode),
		RouteCandidateCount:            int64Value(other, profit.KeyRouteCandidateCount),
		RouteSelectedChannelID:         int(int64Value(other, profit.KeyRouteSelectedChannelID)),
		RouteSelectedMarginRank:        int64Value(other, profit.KeyRouteSelectedMarginRank),
		RouteBestChannelID:             int(int64Value(other, profit.KeyRouteBestChannelID)),
		RouteBestChannelName:           stringValue(other, profit.KeyRouteBestChannelName),
		RouteBestCostProfileID:         stringValue(other, profit.KeyRouteBestCostProfileID),
		RouteBestExpectedMarginUSD:     optionalFloat(other, profit.KeyRouteBestExpectedMarginUSD),
		RouteWouldPreferDifferent:      boolValue(other, profit.KeyRouteWouldPreferDifferent),
		RouteCandidates:                routeCandidatesValue(other, profit.KeyRouteCandidates),
		RouteLiveRoutingUsed:           boolValue(other, profit.KeyRouteLiveRoutingUsed),
		RouteBypassReason:              stringValue(other, profit.KeyRouteBypassReason),
		RouteHealthMinSamples:          int64Value(other, profit.KeyRouteHealthMinSamples),
		RouteHealthMinSuccessRatePct:   floatValue(other, profit.KeyRouteHealthMinSuccessRatePct),
		OutputPolicyMode:               stringValue(other, profit.KeyOutputPolicyMode),
		OutputPolicyID:                 stringValue(other, profit.KeyOutputPolicyID),
		OutputPolicyName:               stringValue(other, profit.KeyOutputPolicyName),
		OutputPolicyCompletionTokens:   int64Value(other, profit.KeyOutputPolicyCompletionTokens),
		OutputPolicyDefaultMaxTokens:   int64Value(other, profit.KeyOutputPolicyDefaultMaxTokens),
		OutputPolicyHardMaxTokens:      int64Value(other, profit.KeyOutputPolicyHardMaxTokens),
		OutputPolicyExceededDefault:    boolValue(other, profit.KeyOutputPolicyExceededDefault),
		OutputPolicyExceededHard:       boolValue(other, profit.KeyOutputPolicyExceededHard),
		OutputPolicyRewriteOverLimit:   boolValue(other, profit.KeyOutputPolicyRewriteOverLimit),
		OutputPolicyWouldCap:           boolValue(other, profit.KeyOutputPolicyWouldCap),
		OutputPolicyPremiumRequired:    boolValue(other, profit.KeyOutputPolicyPremiumRequired),
		OutputPolicyPremiumGroup:       stringValue(other, profit.KeyOutputPolicyPremiumGroup),
		OutputPolicyObserveOnly:        boolValue(other, profit.KeyOutputPolicyObserveOnly),
		OutputPolicyLiveEnforced:       boolValue(other, profit.KeyOutputPolicyLiveEnforced),
		RiskMode:                       stringValue(other, profit.KeyRiskMode),
		RiskAlert:                      boolValue(other, profit.KeyRiskAlert),
		RiskReasons:                    stringSliceValue(other, profit.KeyRiskReasons),
		RiskMinGrossMarginUSD:          floatValue(other, profit.KeyRiskMinGrossMarginUSD),
		RiskMinGrossMarginPct:          floatValue(other, profit.KeyRiskMinGrossMarginPct),
		RiskMinExpectedMarginUSD:       floatValue(other, profit.KeyRiskMinExpectedMarginUSD),
		RiskMinExpectedMarginPct:       floatValue(other, profit.KeyRiskMinExpectedMarginPct),
		RiskObserveOnly:                boolValue(other, profit.KeyRiskObserveOnly),
		RiskLiveEnforced:               boolValue(other, profit.KeyRiskLiveEnforced),
		LongContextMode:                stringValue(other, profit.KeyLongContextMode),
		LongContextPolicyID:            stringValue(other, profit.KeyLongContextPolicyID),
		LongContextPolicyName:          stringValue(other, profit.KeyLongContextPolicyName),
		LongContextTierID:              stringValue(other, profit.KeyLongContextTierID),
		LongContextTierName:            stringValue(other, profit.KeyLongContextTierName),
		LongContextTokens:              int64Value(other, profit.KeyLongContextTokens),
		LongContextMinTokens:           int64Value(other, profit.KeyLongContextMinTokens),
		LongContextMaxTokens:           int64Value(other, profit.KeyLongContextMaxTokens),
		LongContextInputMultiplier:     floatValue(other, profit.KeyLongContextInputMultiplier),
		LongContextInputRevenueUSD:     floatValue(other, profit.KeyLongContextInputRevenueUSD),
		LongContextSuggestedExtraUSD:   floatValue(other, profit.KeyLongContextSuggestedExtraUSD),
		LongContextSuggestedRevenueUSD: floatValue(other, profit.KeyLongContextSuggestedRevenueUSD),
		LongContextPremiumRequired:     boolValue(other, profit.KeyLongContextPremiumRequired),
		LongContextPremiumGroup:        stringValue(other, profit.KeyLongContextPremiumGroup),
		LongContextObserveOnly:         boolValue(other, profit.KeyLongContextObserveOnly),
		LongContextLiveEnforced:        boolValue(other, profit.KeyLongContextLiveEnforced),
		CacheSavedUSD:                  optionalFloat(other, profit.KeyCacheSavedUSD),
		RetryCostUSD:                   optionalFloat(other, profit.KeyRetryCostUSD),
	}
	return event, true
}

func attachProfitEventChannelNames(events []*ProfitEvent) error {
	if len(events) == 0 || DB == nil {
		return nil
	}
	seen := make(map[int]struct{})
	channelIds := make([]int, 0)
	for _, event := range events {
		if event.ChannelId == 0 {
			continue
		}
		if _, ok := seen[event.ChannelId]; ok {
			continue
		}
		seen[event.ChannelId] = struct{}{}
		channelIds = append(channelIds, event.ChannelId)
	}
	if len(channelIds) == 0 {
		return nil
	}

	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIds).Find(&channels).Error; err != nil {
		return err
	}
	channelMap := make(map[int]string, len(channels))
	for _, channel := range channels {
		channelMap[channel.Id] = channel.Name
	}
	for _, event := range events {
		event.ChannelName = channelMap[event.ChannelId]
	}
	return nil
}

func stringValue(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func int64Value(data map[string]interface{}, key string) int64 {
	return int64(floatValue(data, key))
}

func boolValue(data map[string]interface{}, key string) bool {
	value, ok := data[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	default:
		return false
	}
}

func stringSliceValue(data map[string]interface{}, key string) []string {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	switch items := value.(type) {
	case []string:
		return items
	case []interface{}:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func routeCandidatesValue(data map[string]interface{}, key string) []profit.RouteDecisionCandidate {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	if candidates, ok := value.([]profit.RouteDecisionCandidate); ok {
		return candidates
	}
	payload, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	var candidates []profit.RouteDecisionCandidate
	if err = common.Unmarshal(payload, &candidates); err != nil {
		return nil
	}
	return candidates
}

func compressionEngineBreakdownValue(data map[string]interface{}, key string) []promptcompress.EngineBreakdownItem {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	if items, ok := value.([]promptcompress.EngineBreakdownItem); ok {
		return items
	}
	payload, err := common.Marshal(value)
	if err != nil {
		return nil
	}
	var items []promptcompress.EngineBreakdownItem
	if err = common.Unmarshal(payload, &items); err != nil {
		return nil
	}
	return items
}

func optionalFloat(data map[string]interface{}, key string) *float64 {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	number, ok := numericValue(value)
	if !ok {
		return nil
	}
	return floatPtr(number)
}

func floatValue(data map[string]interface{}, key string) float64 {
	number, _ := numericValue(data[key])
	return number
}

func numericValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case string:
		number, err := strconv.ParseFloat(v, 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
