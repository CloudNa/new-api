package profit

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	ResponseCacheScopeGlobal  = "global"
	ResponseCacheScopeUser    = "user"
	ResponseCacheScopeSession = "session"

	DefaultResponseCacheTTLSeconds   = 600
	DefaultResponseCacheMaxBodyBytes = 1 << 20
	responseCacheRedisPrefix         = "profit:response_cache:"
)

type ResponseCacheInput struct {
	Group           string
	ModelName       string
	ChannelID       int
	ChannelName     string
	UserID          int
	TokenID         int
	SessionKey      string
	IsStream        bool
	PassThroughBody bool
	Request         *dto.GeneralOpenAIRequest
}

type ResponseCacheDecision struct {
	Mode            string `json:"mode"`
	Eligible        bool   `json:"eligible"`
	Hit             bool   `json:"hit"`
	WouldHit        bool   `json:"would_hit,omitempty"`
	LiveServed      bool   `json:"live_served,omitempty"`
	Stored          bool   `json:"stored,omitempty"`
	RuleID          string `json:"rule_id,omitempty"`
	RuleName        string `json:"rule_name,omitempty"`
	KeyHash         string `json:"key_hash,omitempty"`
	Scope           string `json:"scope,omitempty"`
	TTLSeconds      int    `json:"ttl_seconds,omitempty"`
	MaxBodyBytes    int    `json:"max_body_bytes,omitempty"`
	BypassReason    string `json:"bypass_reason,omitempty"`
	ObserveOnly     bool   `json:"observe_only,omitempty"`
	SavedPrompt     int    `json:"saved_prompt_tokens,omitempty"`
	SavedCompletion int    `json:"saved_completion_tokens,omitempty"`
}

type ResponseCacheEntry struct {
	Version    int       `json:"version"`
	RuleID     string    `json:"rule_id"`
	KeyHash    string    `json:"key_hash"`
	Body       []byte    `json:"body"`
	Usage      dto.Usage `json:"usage"`
	CreatedAt  int64     `json:"created_at"`
	ExpiresAt  int64     `json:"expires_at"`
	BodyBytes  int       `json:"body_bytes"`
	ModelName  string    `json:"model_name"`
	StoredMode string    `json:"stored_mode"`
}

type responseCacheMemoryItem struct {
	entry     ResponseCacheEntry
	expiresAt time.Time
}

var responseCacheMemory = struct {
	sync.RWMutex
	items map[string]responseCacheMemoryItem
}{
	items: map[string]responseCacheMemoryItem{},
}

func BuildResponseCacheDecision(settings Settings, input ResponseCacheInput) *ResponseCacheDecision {
	settings = settings.Normalize()
	if !settings.Enabled || settings.GlobalKillSwitch || settings.CacheMode == ModeOff || !settingsEnabledForGroup(settings, input.Group) {
		return nil
	}
	decision := &ResponseCacheDecision{
		Mode:        settings.CacheMode,
		ObserveOnly: settings.ObserveOnly,
	}
	if input.Request == nil {
		decision.BypassReason = "missing_request"
		return decision
	}
	if input.PassThroughBody {
		decision.BypassReason = "pass_through_body"
		return decision
	}
	if input.IsStream || input.Request.IsStream(nil) {
		decision.BypassReason = "stream_not_supported"
		return decision
	}
	if reason := responseCacheUnsafeRequestReason(input.Request); reason != "" {
		decision.BypassReason = reason
		return decision
	}

	rule, ok := matchResponseCacheRule(settings.ResponseCacheRules, input)
	if !ok {
		decision.BypassReason = "no_matching_rule"
		return decision
	}
	decision.Mode = responseCacheEffectiveMode(settings, rule)
	decision.ObserveOnly = settings.ObserveOnly
	decision.RuleID = rule.ID
	decision.RuleName = rule.Name
	decision.Scope = rule.Scope
	decision.TTLSeconds = responseCacheTTLSeconds(rule)
	decision.MaxBodyBytes = responseCacheMaxBodyBytes(rule)
	if decision.Mode == ModeOff {
		decision.BypassReason = "rule_off"
		return decision
	}
	if settings.ObserveOnly && decision.Mode == ModeEnforce {
		decision.Mode = ModeObserve
		decision.BypassReason = "observe_only"
	}
	scopeKey, reason := responseCacheScopeKey(rule.Scope, input)
	if reason != "" {
		decision.BypassReason = reason
		return decision
	}
	keyHash, err := responseCacheKeyHash(rule, input, scopeKey)
	if err != nil {
		decision.BypassReason = "key_build_failed"
		return decision
	}
	decision.KeyHash = keyHash
	decision.Eligible = true
	return decision
}

func GetResponseCache(decision *ResponseCacheDecision) (ResponseCacheEntry, bool, error) {
	if decision == nil || !decision.Eligible || decision.KeyHash == "" {
		return ResponseCacheEntry{}, false, nil
	}
	key := responseCacheStorageKey(decision.KeyHash)
	if common.RedisEnabled && common.RDB != nil {
		raw, err := common.RedisGet(key)
		if err == nil && strings.TrimSpace(raw) != "" {
			var entry ResponseCacheEntry
			if err := common.UnmarshalJsonStr(raw, &entry); err != nil {
				return ResponseCacheEntry{}, false, err
			}
			if responseCacheEntryExpired(entry) {
				_ = common.RedisDel(key)
				return ResponseCacheEntry{}, false, nil
			}
			return entry, true, nil
		}
	}

	responseCacheMemory.RLock()
	item, found := responseCacheMemory.items[key]
	responseCacheMemory.RUnlock()
	if !found {
		return ResponseCacheEntry{}, false, nil
	}
	if time.Now().After(item.expiresAt) || responseCacheEntryExpired(item.entry) {
		responseCacheMemory.Lock()
		delete(responseCacheMemory.items, key)
		responseCacheMemory.Unlock()
		return ResponseCacheEntry{}, false, nil
	}
	return item.entry, true, nil
}

func PutResponseCache(decision *ResponseCacheDecision, body []byte, usage *dto.Usage, modelName string) error {
	if decision == nil || !decision.Eligible || decision.KeyHash == "" {
		return nil
	}
	if len(body) == 0 {
		return errors.New("empty_body")
	}
	if decision.MaxBodyBytes > 0 && len(body) > decision.MaxBodyBytes {
		return errors.New("body_too_large")
	}
	if usage == nil || usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return errors.New("invalid_usage")
	}
	now := time.Now()
	ttl := time.Duration(decision.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = time.Duration(DefaultResponseCacheTTLSeconds) * time.Second
	}
	entry := ResponseCacheEntry{
		Version:    ObservationVersion,
		RuleID:     decision.RuleID,
		KeyHash:    decision.KeyHash,
		Body:       append([]byte(nil), body...),
		Usage:      *usage,
		CreatedAt:  now.Unix(),
		ExpiresAt:  now.Add(ttl).Unix(),
		BodyBytes:  len(body),
		ModelName:  strings.TrimSpace(modelName),
		StoredMode: decision.Mode,
	}
	payload, err := common.Marshal(entry)
	if err != nil {
		return err
	}
	key := responseCacheStorageKey(decision.KeyHash)
	if common.RedisEnabled && common.RDB != nil {
		if err := common.RedisSet(key, string(payload), ttl); err == nil {
			return nil
		}
	}
	responseCacheMemory.Lock()
	responseCacheMemory.items[key] = responseCacheMemoryItem{entry: entry, expiresAt: now.Add(ttl)}
	responseCacheMemory.Unlock()
	return nil
}

func ClearResponseCacheMemoryForTest() {
	responseCacheMemory.Lock()
	responseCacheMemory.items = map[string]responseCacheMemoryItem{}
	responseCacheMemory.Unlock()
}

func responseCacheEffectiveMode(settings Settings, rule ResponseCacheRule) string {
	mode := strings.TrimSpace(rule.Mode)
	if mode == "" {
		mode = settings.CacheMode
	}
	if mode == "" {
		mode = ModeObserve
	}
	if mode == ModeEnforce && settings.CacheMode != ModeEnforce {
		return ModeObserve
	}
	if !validResponseCacheMode(mode) {
		return ModeObserve
	}
	return mode
}

func responseCacheTTLSeconds(rule ResponseCacheRule) int {
	if rule.TTLSeconds > 0 {
		return rule.TTLSeconds
	}
	return DefaultResponseCacheTTLSeconds
}

func responseCacheMaxBodyBytes(rule ResponseCacheRule) int {
	if rule.MaxBodyBytes > 0 {
		return rule.MaxBodyBytes
	}
	return DefaultResponseCacheMaxBodyBytes
}

func matchResponseCacheRule(rules []ResponseCacheRule, input ResponseCacheInput) (ResponseCacheRule, bool) {
	normalized := normalizeResponseCacheRules(rules)
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].Priority > normalized[j].Priority
	})
	for _, rule := range normalized {
		if !rule.Enabled || !rule.PublicStatic {
			continue
		}
		if rule.Group != "" && rule.Group != input.Group {
			continue
		}
		if rule.ChannelID > 0 && rule.ChannelID != input.ChannelID {
			continue
		}
		if rule.ChannelName != "" && !equalFoldTrim(rule.ChannelName, input.ChannelName) {
			continue
		}
		modelName := input.ModelName
		if modelName == "" && input.Request != nil {
			modelName = input.Request.Model
		}
		if rule.ModelName != "" {
			if _, ok := modelPatternScore(rule.ModelName, modelName); !ok {
				continue
			}
		}
		return rule, true
	}
	return ResponseCacheRule{}, false
}

func responseCacheScopeKey(scope string, input ResponseCacheInput) (string, string) {
	switch scope {
	case ResponseCacheScopeGlobal:
		return "global", ""
	case ResponseCacheScopeUser:
		if input.UserID <= 0 {
			return "", "missing_user_scope"
		}
		return "user:" + strconv.Itoa(input.UserID), ""
	case ResponseCacheScopeSession:
		sessionKey := strings.TrimSpace(input.SessionKey)
		if sessionKey == "" && input.Request != nil {
			sessionKey = strings.TrimSpace(input.Request.PromptCacheKey)
		}
		if sessionKey == "" {
			return "", "missing_session_scope"
		}
		return "session:" + hex.EncodeToString(common.Sha256Raw([]byte(sessionKey))), ""
	default:
		return "", "invalid_scope"
	}
}

func responseCacheKeyHash(rule ResponseCacheRule, input ResponseCacheInput, scopeKey string) (string, error) {
	request := input.Request
	if request == nil {
		return "", errors.New("missing request")
	}
	payload := map[string]any{
		"rule_id":      rule.ID,
		"scope":        rule.Scope,
		"scope_key":    scopeKey,
		"group":        input.Group,
		"channel_id":   input.ChannelID,
		"channel_name": strings.ToLower(strings.TrimSpace(input.ChannelName)),
		"model":        request.Model,
		"request":      request,
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(common.Sha256Raw(data)), nil
}

func responseCacheStorageKey(hash string) string {
	return responseCacheRedisPrefix + hash
}

func responseCacheEntryExpired(entry ResponseCacheEntry) bool {
	return entry.ExpiresAt > 0 && time.Now().Unix() > entry.ExpiresAt
}

func responseCacheUnsafeRequestReason(request *dto.GeneralOpenAIRequest) string {
	if request == nil {
		return "missing_request"
	}
	if len(request.Messages) == 0 {
		return "unsupported_request_shape"
	}
	if len(request.Tools) > 0 || rawPresent(request.FunctionCall) || request.ToolChoice != nil || rawPresent(request.Functions) {
		return "tools_not_supported"
	}
	if request.WebSearchOptions != nil || rawPresent(request.WebSearch) || rawPresent(request.SearchParameters) ||
		rawPresent(request.SearchDomainFilter) || rawPresent(request.SearchRecencyFilter) || rawPresent(request.SearchMode) ||
		request.ReturnImages != nil || request.ReturnRelatedQuestions != nil {
		return "search_not_supported"
	}
	if rawPresent(request.Modalities) || rawPresent(request.Audio) || rawPresent(request.ExtraBody) || request.Input != nil || request.Prompt != nil {
		return "unsupported_request_shape"
	}
	for _, message := range request.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "tool" || message.ToolCallId != "" || rawPresent(message.ToolCalls) {
			return "tools_not_supported"
		}
		if !responseCacheTextOnlyContent(message.Content) {
			return "multimodal_not_supported"
		}
	}
	return ""
}

func responseCacheTextOnlyContent(content any) bool {
	switch value := content.(type) {
	case nil:
		return true
	case string:
		return true
	case []any:
		for _, item := range value {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return false
			}
			blockType, _ := itemMap["type"].(string)
			if blockType != "" && blockType != dto.ContentTypeText && blockType != "input_text" {
				return false
			}
			if _, ok := itemMap["text"].(string); !ok {
				return false
			}
		}
		return true
	case []dto.MediaContent:
		for _, item := range value {
			if item.Type != "" && item.Type != dto.ContentTypeText {
				return false
			}
			if rawPresent(item.CacheControl) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func rawPresent(raw []byte) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func DescribeResponseCacheDecision(decision *ResponseCacheDecision) string {
	if decision == nil {
		return ""
	}
	if decision.LiveServed {
		return fmt.Sprintf("response_cache_hit rule=%s key=%s", decision.RuleID, decision.KeyHash)
	}
	if decision.Stored {
		return fmt.Sprintf("response_cache_store rule=%s key=%s", decision.RuleID, decision.KeyHash)
	}
	if decision.BypassReason != "" {
		return "response_cache_bypass=" + decision.BypassReason
	}
	return ""
}
