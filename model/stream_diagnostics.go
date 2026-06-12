package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const defaultStreamDiagnosticsLimit = 100
const maxStreamDiagnosticsLimit = 500

type StreamDiagnosticsFilter struct {
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Username       string
	Channel        int
	Group          string
	Limit          int
}

type StreamDiagnostic struct {
	LogID             int      `json:"log_id"`
	CreatedAt         int64    `json:"created_at"`
	Type              int      `json:"type"`
	Username          string   `json:"username"`
	TokenName         string   `json:"token_name"`
	ModelName         string   `json:"model_name"`
	Group             string   `json:"group"`
	ChannelID         int      `json:"channel"`
	ChannelName       string   `json:"channel_name"`
	RequestID         string   `json:"request_id,omitempty"`
	UpstreamRequestID string   `json:"upstream_request_id,omitempty"`
	UseTime           int      `json:"use_time"`
	Status            string   `json:"status"`
	EndReason         string   `json:"end_reason"`
	EndError          string   `json:"end_error,omitempty"`
	RequestFormat     string   `json:"request_format,omitempty"`
	TerminalEvent     string   `json:"terminal_event,omitempty"`
	TerminalReceived  bool     `json:"terminal_received"`
	ChunkCount        int64    `json:"chunk_count"`
	ByteCount         int64    `json:"byte_count"`
	StartedAt         int64    `json:"started_at,omitempty"`
	FirstChunkAt      int64    `json:"first_chunk_at,omitempty"`
	LastChunkAt       int64    `json:"last_chunk_at,omitempty"`
	UpstreamEOF       bool     `json:"upstream_eof"`
	ClientGone        bool     `json:"client_gone"`
	ReadError         string   `json:"read_error,omitempty"`
	WriteError        string   `json:"write_error,omitempty"`
	ErrorCount        int64    `json:"error_count"`
	Errors            []string `json:"errors,omitempty"`
	PromptTokens      int      `json:"prompt_tokens"`
	CompletionTokens  int      `json:"completion_tokens"`
	Quota             int      `json:"quota"`
}

func GetStreamDiagnostics(filter StreamDiagnosticsFilter) ([]StreamDiagnostic, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultStreamDiagnosticsLimit
	}
	if limit > maxStreamDiagnosticsLimit {
		limit = maxStreamDiagnosticsLimit
	}

	tx := LOG_DB.Model(&Log{}).Where("is_stream = ?", true)
	if filter.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", filter.EndTimestamp)
	}
	if strings.TrimSpace(filter.ModelName) != "" {
		tx = tx.Where("model_name = ?", strings.TrimSpace(filter.ModelName))
	}
	if strings.TrimSpace(filter.Username) != "" {
		tx = tx.Where("username = ?", strings.TrimSpace(filter.Username))
	}
	if filter.Channel > 0 {
		tx = tx.Where("channel_id = ?", filter.Channel)
	}
	if strings.TrimSpace(filter.Group) != "" {
		tx = tx.Where(logGroupCol+" = ?", strings.TrimSpace(filter.Group))
	}

	scanLimit := limit * 5
	if scanLimit < limit {
		scanLimit = limit
	}
	if scanLimit > maxStreamDiagnosticsLimit*5 {
		scanLimit = maxStreamDiagnosticsLimit * 5
	}

	var logs []*Log
	if err := tx.Order("id desc").Limit(scanLimit).Find(&logs).Error; err != nil {
		return nil, err
	}

	items := make([]StreamDiagnostic, 0, limit)
	for _, log := range logs {
		if log == nil || strings.TrimSpace(log.Other) == "" {
			continue
		}
		otherMap, err := parseLogOther(log.Other)
		if err != nil {
			continue
		}
		statusMap, ok := nestedMap(otherMap, "stream_status")
		if !ok {
			continue
		}
		item := streamDiagnosticFromLog(log, statusMap)
		if item.Status == "ok" && item.EndReason == "done" && item.ErrorCount == 0 {
			continue
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func parseLogOther(raw string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := commonUnmarshalLogOther(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func commonUnmarshalLogOther(raw string, target *map[string]interface{}) error {
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	return common.UnmarshalJsonStr(raw, target)
}

func streamDiagnosticFromLog(log *Log, statusMap map[string]interface{}) StreamDiagnostic {
	return StreamDiagnostic{
		LogID:             log.Id,
		CreatedAt:         log.CreatedAt,
		Type:              log.Type,
		Username:          log.Username,
		TokenName:         log.TokenName,
		ModelName:         log.ModelName,
		Group:             log.Group,
		ChannelID:         log.ChannelId,
		ChannelName:       log.ChannelName,
		RequestID:         log.RequestId,
		UpstreamRequestID: log.UpstreamRequestId,
		UseTime:           log.UseTime,
		Status:            mapString(statusMap, "status"),
		EndReason:         mapString(statusMap, "end_reason"),
		EndError:          mapString(statusMap, "end_error"),
		RequestFormat:     mapString(statusMap, "request_format"),
		TerminalEvent:     mapString(statusMap, "terminal_event"),
		TerminalReceived:  mapBool(statusMap, "terminal_received"),
		ChunkCount:        mapInt64(statusMap, "chunk_count"),
		ByteCount:         mapInt64(statusMap, "byte_count"),
		StartedAt:         mapInt64(statusMap, "started_at"),
		FirstChunkAt:      mapInt64(statusMap, "first_chunk_at"),
		LastChunkAt:       mapInt64(statusMap, "last_chunk_at"),
		UpstreamEOF:       mapBool(statusMap, "upstream_eof"),
		ClientGone:        mapBool(statusMap, "client_gone"),
		ReadError:         mapString(statusMap, "read_error"),
		WriteError:        mapString(statusMap, "write_error"),
		ErrorCount:        mapInt64(statusMap, "error_count"),
		Errors:            mapStringSlice(statusMap, "errors"),
		PromptTokens:      log.PromptTokens,
		CompletionTokens:  log.CompletionTokens,
		Quota:             log.Quota,
	}
}

func nestedMap(src map[string]interface{}, key string) (map[string]interface{}, bool) {
	value, ok := src[key]
	if !ok || value == nil {
		return nil, false
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed, true
	}
	return nil, false
}

func mapString(src map[string]interface{}, key string) string {
	value, ok := src[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func mapBool(src map[string]interface{}, key string) bool {
	value, ok := src[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		result, _ := strconv.ParseBool(typed)
		return result
	default:
		return fmt.Sprint(typed) == "true"
	}
}

func mapInt64(src map[string]interface{}, key string) int64 {
	value, ok := src[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		result, _ := strconv.ParseInt(typed, 10, 64)
		return result
	default:
		result, _ := strconv.ParseInt(fmt.Sprint(typed), 10, 64)
		return result
	}
}

func mapStringSlice(src map[string]interface{}, key string) []string {
	value, ok := src[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			items = append(items, fmt.Sprint(item))
		}
		return items
	default:
		return []string{fmt.Sprint(typed)}
	}
}
