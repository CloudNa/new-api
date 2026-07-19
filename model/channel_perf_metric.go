package model

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelPerfMetric stores channel-level relay health for guarded profit routing.
type ChannelPerfMetric struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_channel_perf_model_group_channel_bucket,priority:1"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_channel_perf_model_group_channel_bucket,priority:2"`
	ChannelID      int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_perf_model_group_channel_bucket,priority:3;index"`
	BucketTs       int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_channel_perf_model_group_channel_bucket,priority:4;index:idx_channel_perf_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
}

func (ChannelPerfMetric) TableName() string {
	return "channel_perf_metrics"
}

func UpsertChannelPerfMetric(metric *ChannelPerfMetric) error {
	if metric == nil || metric.RequestCount == 0 || metric.ChannelID <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "model_name"},
			{Name: "group"},
			{Name: "channel_id"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":    gorm.Expr("channel_perf_metrics.request_count + ?", metric.RequestCount),
			"success_count":    gorm.Expr("channel_perf_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms": gorm.Expr("channel_perf_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
			"output_tokens":    gorm.Expr("channel_perf_metrics.output_tokens + ?", metric.OutputTokens),
			"generation_ms":    gorm.Expr("channel_perf_metrics.generation_ms + ?", metric.GenerationMs),
		}),
	}).Create(metric).Error
}

type ChannelPerfMetricSummary struct {
	ChannelID      int   `json:"channel_id"`
	RequestCount   int64 `json:"request_count"`
	SuccessCount   int64 `json:"success_count"`
	TotalLatencyMs int64 `json:"total_latency_ms"`
	OutputTokens   int64 `json:"output_tokens"`
	GenerationMs   int64 `json:"generation_ms"`
}

func GetChannelPerfMetricSummaries(modelName string, group string, channelIDs []int, startTs int64, endTs int64) ([]ChannelPerfMetricSummary, error) {
	var summaries []ChannelPerfMetricSummary
	if modelName == "" || group == "" || len(channelIDs) == 0 {
		return summaries, nil
	}
	err := DB.Model(&ChannelPerfMetric{}).
		Select("channel_id, SUM(request_count) as request_count, SUM(success_count) as success_count, SUM(total_latency_ms) as total_latency_ms, SUM(output_tokens) as output_tokens, SUM(generation_ms) as generation_ms").
		Where("model_name = ? AND "+commonGroupCol+" = ? AND channel_id IN ? AND bucket_ts >= ? AND bucket_ts <= ?", modelName, group, channelIDs, startTs, endTs).
		Group("channel_id").
		Having("SUM(request_count) > 0").
		Find(&summaries).Error
	return summaries, err
}

func DeleteChannelPerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&ChannelPerfMetric{}).Error
}

func ChannelPerfMetricStartTime(hours int) int64 {
	if hours <= 0 {
		hours = 24
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
}
