package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/QuantumNous/new-api/pkg/promptcompress"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const promptCompressionTimeout = 2 * time.Second

type promptCompressionUpdate struct {
	index   int
	content string
}

type promptCompressionBuildResult struct {
	updates []promptCompressionUpdate
	stats   promptcompress.Stats
}

// LoadPromptCompressionSettings reads the stored compression settings from the
// in-memory option map. It intentionally does not import model, so callers can
// use it on hot relay paths without pulling DB code into the request hook.
func LoadPromptCompressionSettings() (promptcompress.Settings, error) {
	settings := promptcompress.DefaultSettings()
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[promptcompress.SettingsOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(raw) == "" {
		return settings, nil
	}
	if err := common.UnmarshalJsonStr(raw, &settings); err != nil {
		return promptcompress.Settings{}, errors.New("invalid stored compression settings")
	}
	return settings.Normalize(), nil
}

// ApplyPromptCompressionForRelay is the v1 request hook. It only supports
// OpenAI-compatible chat messages, only for proxy-test, and always fails open
// to the original request when compression is unsafe or unavailable.
func ApplyPromptCompressionForRelay(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, passThroughBody bool) {
	if info == nil || request == nil {
		return
	}
	settings, err := LoadPromptCompressionSettings()
	if err != nil {
		if info.UsingGroup == profit.DefaultObserveGroup {
			setPromptCompressionBypass(info, promptcompress.ModeOff, "invalid_settings", 0)
			logger.LogError(c, "prompt compression settings invalid: "+err.Error())
		}
		return
	}
	if !settings.Enabled || settings.GlobalKillSwitch {
		return
	}
	if !isPromptCompressionV1Group(settings, info.UsingGroup) {
		return
	}

	mode := resolvePromptCompressionMode(c, settings, info, request)
	if mode == promptcompress.ModeOff {
		return
	}
	if info.RelayFormat != types.RelayFormatOpenAI || info.RelayMode != relayconstant.RelayModeChatCompletions {
		setPromptCompressionBypass(info, mode, "unsupported_relay_mode", info.GetEstimatePromptTokens())
		return
	}
	if passThroughBody {
		setPromptCompressionBypass(info, mode, "pass_through_body", info.GetEstimatePromptTokens())
		return
	}
	if len(request.Messages) == 0 {
		setPromptCompressionBypass(info, mode, "unsupported_request_shape", info.GetEstimatePromptTokens())
		return
	}
	if settings.MinTokens > 0 && info.GetEstimatePromptTokens() > 0 && info.GetEstimatePromptTokens() < settings.MinTokens {
		setPromptCompressionBypass(info, mode, "below_min_tokens", info.GetEstimatePromptTokens())
		return
	}

	config := settings.ToConfig(mode)
	buildResult, err := buildPromptCompressionUpdatesWithTimeout(request, settings, config)
	if err != nil {
		setPromptCompressionBypass(info, mode, err.Error(), info.GetEstimatePromptTokens())
		logger.LogWarn(c, "prompt compression bypassed: "+err.Error())
		return
	}
	for _, update := range buildResult.updates {
		request.Messages[update.index].SetStringContent(update.content)
	}
	info.PromptCompressionStats = &buildResult.stats
}

func buildPromptCompressionUpdatesWithTimeout(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config) (promptCompressionBuildResult, error) {
	resultCh := make(chan promptCompressionBuildResult, 1)
	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				errCh <- fmt.Errorf("compressor_panic")
			}
		}()
		resultCh <- buildPromptCompressionUpdates(request, settings, config)
	}()

	select {
	case result := <-resultCh:
		if result.stats.OriginalTokens > 0 && result.stats.CompressedTokens <= 0 {
			return promptCompressionBuildResult{}, errors.New("empty_output")
		}
		if result.stats.Bypassed {
			return result, nil
		}
		if len(result.updates) == 0 {
			result.stats.Bypassed = true
			result.stats.BypassReason = "no_savings"
		}
		return result, nil
	case err := <-errCh:
		return promptCompressionBuildResult{}, err
	case <-time.After(promptCompressionTimeout):
		return promptCompressionBuildResult{}, errors.New("compressor_timeout")
	}
}

func buildPromptCompressionUpdates(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config) promptCompressionBuildResult {
	started := time.Now()
	stats := promptcompress.Stats{
		Mode:                    config.Mode,
		OmniRouteCompatibleMode: string(config.Mode),
	}
	allowedRoles := promptCompressionRoleSet(settings.Caveman.CompressRoles)
	var updates []promptCompressionUpdate
	compressibleMessages := 0

	for i, message := range request.Messages {
		content, ok := message.Content.(string)
		if !ok {
			continue
		}
		tokens := promptcompress.EstimateTokens(content)
		stats.OriginalTokens += tokens

		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, request.GetSystemRoleName()) {
			stats.CompressedTokens += tokens
			continue
		}
		if !allowedRoles[strings.ToLower(strings.TrimSpace(message.Role))] {
			stats.CompressedTokens += tokens
			continue
		}

		compressibleMessages++
		result := promptcompress.CompressText(content, config)
		if strings.TrimSpace(result.Text) == "" && strings.TrimSpace(content) != "" {
			stats.Bypassed = true
			stats.BypassReason = "empty_output"
			stats.CompressedTokens = stats.OriginalTokens
			stats.CompressionSavedTokens = 0
			stats.DurationMs = time.Since(started).Milliseconds()
			return promptCompressionBuildResult{stats: stats}
		}

		stats.CompressedTokens += result.Stats.CompressedTokens
		stats.TechniquesUsed = append(stats.TechniquesUsed, result.Stats.TechniquesUsed...)
		stats.RulesApplied = append(stats.RulesApplied, result.Stats.RulesApplied...)
		stats.PreservedBlockCount += result.Stats.PreservedBlockCount
		stats.RedactedSecretCount += result.Stats.RedactedSecretCount
		if result.Compressed {
			updates = append(updates, promptCompressionUpdate{index: i, content: result.Text})
		}
	}

	if compressibleMessages == 0 {
		stats.Bypassed = true
		stats.BypassReason = "no_compressible_messages"
	}
	stats.TechniquesUsed = uniquePromptCompressionStrings(stats.TechniquesUsed)
	stats.RulesApplied = uniquePromptCompressionStrings(stats.RulesApplied)
	stats.CompressionSavedTokens = maxPromptCompressionInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	if len(updates) == 0 && !stats.Bypassed {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
	}
	return promptCompressionBuildResult{updates: updates, stats: stats}
}

func isPromptCompressionV1Group(settings promptcompress.Settings, group string) bool {
	if group != profit.DefaultObserveGroup {
		return false
	}
	for _, allowed := range settings.AllowedGroups {
		if allowed == group {
			return true
		}
	}
	return false
}

func resolvePromptCompressionMode(c *gin.Context, settings promptcompress.Settings, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) promptcompress.Mode {
	mode := settings.DefaultMode
	if groupMode, ok := settings.GroupModes[info.UsingGroup]; ok {
		mode = groupMode
	}
	for _, modelName := range []string{info.OriginModelName, info.UpstreamModelName, request.Model} {
		if modelMode, ok := settings.ModelModes[modelName]; ok {
			mode = modelMode
		}
	}
	for _, channelKey := range []string{strconv.Itoa(info.ChannelId), c.GetString("channel_name")} {
		if channelMode, ok := settings.ChannelModes[channelKey]; ok {
			mode = channelMode
		}
	}
	if mode == promptcompress.ModeOff && settings.AutoTriggerTokens > 0 && info.GetEstimatePromptTokens() >= settings.AutoTriggerTokens {
		mode = settings.AutoTriggerMode
	}
	return settings.ToConfig(mode).Mode
}

func setPromptCompressionBypass(info *relaycommon.RelayInfo, mode promptcompress.Mode, reason string, originalTokens int) {
	if info == nil {
		return
	}
	stats := &promptcompress.Stats{
		OriginalTokens:          maxPromptCompressionInt(0, originalTokens),
		CompressedTokens:        maxPromptCompressionInt(0, originalTokens),
		Mode:                    mode,
		OmniRouteCompatibleMode: string(mode),
		Bypassed:                true,
		BypassReason:            reason,
	}
	info.PromptCompressionStats = stats
}

func promptCompressionRoleSet(roles []string) map[string]bool {
	out := make(map[string]bool, len(roles))
	for _, role := range roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "" {
			continue
		}
		out[role] = true
	}
	return out
}

func activePromptCompressionStats(info *relaycommon.RelayInfo) *promptcompress.Stats {
	if info == nil || info.PromptCompressionStats == nil || info.PromptCompressionStats.Bypassed {
		return nil
	}
	return info.PromptCompressionStats
}

func billingUsageForPromptCompression(info *relaycommon.RelayInfo, usage *dto.Usage) *dto.Usage {
	stats := activePromptCompressionStats(info)
	if stats == nil || usage == nil || info.GetEstimatePromptTokens() <= 0 {
		return usage
	}
	billingUsage := *usage
	billingUsage.PromptTokens = info.GetEstimatePromptTokens()
	billingUsage.TotalTokens = billingUsage.PromptTokens + billingUsage.CompletionTokens
	return &billingUsage
}

func injectPromptCompressionOther(other map[string]interface{}, info *relaycommon.RelayInfo) int {
	if other == nil || info == nil || info.PromptCompressionStats == nil {
		return 0
	}
	stats := info.PromptCompressionStats
	other[profit.KeyCompressionMode] = string(stats.Mode)
	other[profit.KeyCompressionSavingsPercent] = stats.SavingsPercent
	other[profit.KeyCompressionBypassed] = stats.Bypassed
	if stats.BypassReason != "" {
		other[profit.KeyCompressionBypassReason] = stats.BypassReason
	}
	if len(stats.RulesApplied) > 0 {
		other[profit.KeyCompressionRulesApplied] = stats.RulesApplied
	}
	other[profit.KeyCompressionRulesVersion] = promptcompress.RuleVersion
	other[profit.KeyCompressionPreservedBlocks] = stats.PreservedBlockCount
	other[profit.KeyCompressionRedactedSecrets] = stats.RedactedSecretCount
	if stats.Bypassed {
		return 0
	}
	return stats.CompressionSavedTokens
}

func uniquePromptCompressionStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func maxPromptCompressionInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
