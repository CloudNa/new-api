package model

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/profit"

	"gorm.io/gorm"
)

const profitAnalyticsScanLimit = 20000

type ProfitLogFilter struct {
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Username       string
	Channel        int
	Group          string
}

type ProfitEvent struct {
	Id                             int      `json:"id"`
	CreatedAt                      int64    `json:"created_at"`
	UserId                         int      `json:"user_id"`
	Username                       string   `json:"username"`
	TokenName                      string   `json:"token_name"`
	ModelName                      string   `json:"model_name"`
	ChannelId                      int      `json:"channel"`
	ChannelName                    string   `json:"channel_name"`
	Group                          string   `json:"group"`
	RequestId                      string   `json:"request_id,omitempty"`
	UpstreamRequestId              string   `json:"upstream_request_id,omitempty"`
	PromptTokens                   int      `json:"prompt_tokens"`
	CompletionTokens               int      `json:"completion_tokens"`
	Quota                          int      `json:"quota"`
	UseTime                        int      `json:"use_time"`
	IsStream                       bool     `json:"is_stream"`
	CostStatus                     string   `json:"profit_cost_status"`
	CostKnown                      bool     `json:"cost_known"`
	CostProfileID                  string   `json:"cost_profile_id,omitempty"`
	CostProfileName                string   `json:"cost_profile_name,omitempty"`
	BillablePromptTokens           int64    `json:"billable_prompt_tokens"`
	BillableCompletionTokens       int64    `json:"billable_completion_tokens"`
	UpstreamActualPromptTokens     int64    `json:"upstream_actual_prompt_tokens"`
	UpstreamActualCompletionTokens int64    `json:"upstream_actual_completion_tokens"`
	EstimatedRevenueUSD            float64  `json:"estimated_revenue_usd"`
	EstimatedUpstreamCostUSD       *float64 `json:"estimated_upstream_cost_usd"`
	GrossMarginUSD                 *float64 `json:"gross_margin_usd"`
	GrossMarginPct                 *float64 `json:"gross_margin_pct"`
	ExpectedCostUSD                *float64 `json:"expected_cost_usd"`
	ExpectedMarginUSD              *float64 `json:"expected_margin_usd"`
	ExpectedMarginPct              *float64 `json:"expected_margin_pct"`
	CompressionSavedTokens         int64    `json:"compression_saved_tokens"`
	CompressionMode                string   `json:"compression_mode,omitempty"`
	CompressionSavingsPercent      *float64 `json:"compression_savings_percent,omitempty"`
	CompressionBypassed            bool     `json:"compression_bypassed"`
	CompressionBypassReason        string   `json:"compression_bypass_reason,omitempty"`
	CompressionRulesVersion        string   `json:"compression_rules_version,omitempty"`
	CompressionRulesApplied        []string `json:"compression_rules_applied,omitempty"`
	CompressionPreservedBlocks     int64    `json:"compression_preserved_blocks"`
	CompressionRedactedSecrets     int64    `json:"compression_redacted_secrets"`
	CacheSavedUSD                  *float64 `json:"cache_saved_usd"`
	RetryCostUSD                   *float64 `json:"retry_cost_usd"`
}

type ProfitAnalytics struct {
	RequestCount                   int64    `json:"request_count"`
	ScannedEvents                  int64    `json:"scanned_events"`
	TotalMatchingLogs              int64    `json:"total_matching_logs"`
	IsPartial                      bool     `json:"is_partial"`
	ScanLimit                      int      `json:"scan_limit"`
	CostKnownCount                 int64    `json:"cost_known_count"`
	MissingCostProfileCount        int64    `json:"missing_cost_profile_count"`
	BillablePromptTokens           int64    `json:"billable_prompt_tokens"`
	BillableCompletionTokens       int64    `json:"billable_completion_tokens"`
	UpstreamActualPromptTokens     int64    `json:"upstream_actual_prompt_tokens"`
	UpstreamActualCompletionTokens int64    `json:"upstream_actual_completion_tokens"`
	EstimatedRevenueUSD            float64  `json:"estimated_revenue_usd"`
	EstimatedUpstreamCostUSD       *float64 `json:"estimated_upstream_cost_usd"`
	GrossMarginUSD                 *float64 `json:"gross_margin_usd"`
	GrossMarginPct                 *float64 `json:"gross_margin_pct"`
	ExpectedCostUSD                *float64 `json:"expected_cost_usd"`
	ExpectedMarginUSD              *float64 `json:"expected_margin_usd"`
	ExpectedMarginPct              *float64 `json:"expected_margin_pct"`
	CompressionSavedTokens         int64    `json:"compression_saved_tokens"`
	CacheSavedUSD                  *float64 `json:"cache_saved_usd"`
	RetryCostUSD                   *float64 `json:"retry_cost_usd"`
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

	return analytics, nil
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
		CompressionSavingsPercent:      optionalFloat(other, profit.KeyCompressionSavingsPercent),
		CompressionBypassed:            boolValue(other, profit.KeyCompressionBypassed),
		CompressionBypassReason:        stringValue(other, profit.KeyCompressionBypassReason),
		CompressionRulesVersion:        stringValue(other, profit.KeyCompressionRulesVersion),
		CompressionRulesApplied:        stringSliceValue(other, profit.KeyCompressionRulesApplied),
		CompressionPreservedBlocks:     int64Value(other, profit.KeyCompressionPreservedBlocks),
		CompressionRedactedSecrets:     int64Value(other, profit.KeyCompressionRedactedSecrets),
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
