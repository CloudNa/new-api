package profit

import "github.com/QuantumNous/new-api/common"

const (
	ObservationVersion = 1

	DefaultObserveGroup = "proxy-test"

	KeyObserveVersion                 = "profit_observe_version"
	KeyCostStatus                     = "profit_cost_status"
	KeyBillablePromptTokens           = "billable_prompt_tokens"
	KeyBillableCompletionTokens       = "billable_completion_tokens"
	KeyUpstreamActualPromptTokens     = "upstream_actual_prompt_tokens"
	KeyUpstreamActualCompletionTokens = "upstream_actual_completion_tokens"
	KeyEstimatedRevenueUSD            = "estimated_revenue_usd"
	KeyEstimatedUpstreamCostUSD       = "estimated_upstream_cost_usd"
	KeyGrossMarginUSD                 = "gross_margin_usd"
	KeyGrossMarginPct                 = "gross_margin_pct"
	KeyCompressionSavedTokens         = "compression_saved_tokens"
	KeyCacheSavedUSD                  = "cache_saved_usd"
	KeyRetryCostUSD                   = "retry_cost_usd"
	CostStatusMissingCostProfile      = "missing_cost_profile"
)

var userHiddenKeys = []string{
	KeyObserveVersion,
	KeyCostStatus,
	KeyBillablePromptTokens,
	KeyBillableCompletionTokens,
	KeyUpstreamActualPromptTokens,
	KeyUpstreamActualCompletionTokens,
	KeyEstimatedRevenueUSD,
	KeyEstimatedUpstreamCostUSD,
	KeyGrossMarginUSD,
	KeyGrossMarginPct,
	KeyCompressionSavedTokens,
	KeyCacheSavedUSD,
	KeyRetryCostUSD,
}

type ObservationInput struct {
	Group                          string
	BillablePromptTokens           int
	BillableCompletionTokens       int
	UpstreamActualPromptTokens     int
	UpstreamActualCompletionTokens int
	UserQuota                      int
	CompressionSavedTokens         int
}

func EnabledForGroup(group string) bool {
	return group == DefaultObserveGroup
}

func AppendObservation(other map[string]interface{}, input ObservationInput) {
	if other == nil || !EnabledForGroup(input.Group) {
		return
	}

	other[KeyObserveVersion] = ObservationVersion
	other[KeyCostStatus] = CostStatusMissingCostProfile
	other[KeyBillablePromptTokens] = positiveInt(input.BillablePromptTokens)
	other[KeyBillableCompletionTokens] = positiveInt(input.BillableCompletionTokens)
	other[KeyUpstreamActualPromptTokens] = positiveInt(input.UpstreamActualPromptTokens)
	other[KeyUpstreamActualCompletionTokens] = positiveInt(input.UpstreamActualCompletionTokens)
	other[KeyEstimatedRevenueUSD] = quotaToUSD(input.UserQuota)
	other[KeyEstimatedUpstreamCostUSD] = nil
	other[KeyGrossMarginUSD] = nil
	other[KeyGrossMarginPct] = nil
	other[KeyCompressionSavedTokens] = positiveInt(input.CompressionSavedTokens)
	other[KeyCacheSavedUSD] = nil
	other[KeyRetryCostUSD] = nil
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

func quotaToUSD(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}
