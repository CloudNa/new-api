package service

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
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
	content any
}

type promptCompressionBuildResult struct {
	updates []promptCompressionUpdate
	stats   promptcompress.Stats
}

type promptCompressionContentState struct {
	original     any
	current      any
	originalText string
}

type promptCompressionToolMeta struct {
	toolName string
	command  string
}

type promptCompressionTextTransform func(text string) (next string, eligible bool, ok bool)

var promptCompressionTerminalToolPattern = regexp.MustCompile(`(?i)\b(bash|shell|terminal|run_command|execute_command|exec|command)\b`)

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
		request.Messages[update.index].SetContent(update.content)
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

func PreviewPromptCompressionMessages(settings promptcompress.Settings, mode promptcompress.Mode, messages []promptcompress.Message) ([]promptcompress.Message, promptcompress.Stats) {
	settings = settings.Normalize()
	if mode == "" {
		mode = settings.DefaultMode
	}
	request := &dto.GeneralOpenAIRequest{
		Messages: make([]dto.Message, len(messages)),
	}
	for i, message := range messages {
		request.Messages[i] = dto.Message{
			Role:    message.Role,
			Content: message.Content,
		}
	}
	config := settings.ToConfig(mode)
	buildResult := buildPromptCompressionUpdates(request, settings, config)
	for _, update := range buildResult.updates {
		request.Messages[update.index].SetContent(update.content)
	}
	out := make([]promptcompress.Message, len(request.Messages))
	for i, message := range request.Messages {
		out[i] = promptcompress.Message{
			Role:    message.Role,
			Content: message.StringContent(),
		}
	}
	return out, buildResult.stats
}

func PreviewPromptCompressionText(settings promptcompress.Settings, mode promptcompress.Mode, text string) promptcompress.Result {
	messages, stats := PreviewPromptCompressionMessages(settings, mode, []promptcompress.Message{
		{Role: "user", Content: text},
	})
	if len(messages) == 0 {
		return promptcompress.Result{Text: text, Compressed: false, Stats: stats}
	}
	return promptcompress.Result{
		Text:       messages[0].Content,
		Compressed: messages[0].Content != text,
		Stats:      stats,
	}
}

func buildPromptCompressionUpdates(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config) promptCompressionBuildResult {
	started := time.Now()
	stats := promptcompress.Stats{
		Mode:                    config.Mode,
		Engine:                  string(config.Mode),
		Timestamp:               started.UnixMilli(),
		OmniRouteCompatibleMode: string(config.Mode),
	}
	states := make(map[int]*promptCompressionContentState, len(request.Messages))
	eligibleMessages := make(map[int]struct{})

	for i, message := range request.Messages {
		content, ok := promptCompressionContentText(message.Content)
		if !ok {
			continue
		}
		states[i] = &promptCompressionContentState{
			original:     message.Content,
			current:      message.Content,
			originalText: content,
		}
		stats.OriginalTokens += promptcompress.EstimateTokens(content)
	}

	if len(states) == 0 {
		stats.Bypassed = true
		stats.BypassReason = "no_compressible_messages"
		stats.DurationMs = time.Since(started).Milliseconds()
		return promptCompressionBuildResult{stats: stats}
	}

	if !applyPromptCompressionPipeline(request, settings, config, states, eligibleMessages, &stats) {
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.DurationMs = time.Since(started).Milliseconds()
		return promptCompressionBuildResult{stats: stats}
	}

	var updates []promptCompressionUpdate
	for index, state := range states {
		currentText, ok := promptCompressionContentText(state.current)
		if !ok {
			currentText = state.originalText
		}
		stats.CompressedTokens += promptcompress.EstimateTokens(currentText)
		if !reflect.DeepEqual(state.current, state.original) {
			updates = append(updates, promptCompressionUpdate{index: index, content: state.current})
		}
	}

	if len(eligibleMessages) == 0 {
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

func applyPromptCompressionPipeline(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config, states map[int]*promptCompressionContentState, eligibleMessages map[int]struct{}, stats *promptcompress.Stats) bool {
	switch config.Mode {
	case promptcompress.ModeRTK:
		return applyPromptCompressionRTKStep(request, config, states, eligibleMessages, stats)
	case promptcompress.ModeStacked:
		return applyPromptCompressionStackedPipeline(request, settings, config, states, eligibleMessages, stats)
	case promptcompress.ModeLite:
		return applyPromptCompressionCavemanStep(request, settings, config, promptcompress.IntensityLite, states, eligibleMessages, stats)
	case promptcompress.ModeStandard:
		return applyPromptCompressionCavemanStep(request, settings, config, settings.Caveman.Intensity, states, eligibleMessages, stats)
	case promptcompress.ModeAggressive:
		return applyPromptCompressionMessageModeStep(request, config, states, eligibleMessages, stats, promptcompress.ModeAggressive)
	case promptcompress.ModeUltra:
		return applyPromptCompressionMessageModeStep(request, config, states, eligibleMessages, stats, promptcompress.ModeUltra)
	default:
		return true
	}
}

func applyPromptCompressionStackedPipeline(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config, states map[int]*promptCompressionContentState, eligibleMessages map[int]struct{}, stats *promptcompress.Stats) bool {
	for _, step := range promptcompress.NormalizeStackedPipeline(config.StackedPipeline) {
		stepConfig := config
		switch step.Engine {
		case "rtk":
			stepConfig.RtkIntensity = promptCompressionPipelineRtkIntensity(step.Intensity, config.RtkIntensity)
			if !applyPromptCompressionRTKStep(request, stepConfig, states, eligibleMessages, stats) {
				return false
			}
		case "lite":
			if !applyPromptCompressionCavemanStep(request, settings, stepConfig, promptcompress.IntensityLite, states, eligibleMessages, stats) {
				return false
			}
		case "aggressive":
			if !applyPromptCompressionMessageModeStep(request, stepConfig, states, eligibleMessages, stats, promptcompress.ModeAggressive) {
				return false
			}
		case "ultra":
			if !applyPromptCompressionMessageModeStep(request, stepConfig, states, eligibleMessages, stats, promptcompress.ModeUltra) {
				return false
			}
		default:
			intensity := promptCompressionPipelineCavemanIntensity(step.Intensity, settings.Caveman.Intensity)
			if !applyPromptCompressionCavemanStep(request, settings, stepConfig, intensity, states, eligibleMessages, stats) {
				return false
			}
		}
	}
	return true
}

func promptCompressionPipelineRtkIntensity(value string, fallback promptcompress.RtkIntensity) promptcompress.RtkIntensity {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return promptcompress.NormalizeRtkIntensity(fallback)
	}
	return promptcompress.NormalizeRtkIntensity(promptcompress.RtkIntensity(value))
}

func promptCompressionPipelineCavemanIntensity(value string, fallback promptcompress.Intensity) promptcompress.Intensity {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	switch promptcompress.Intensity(value) {
	case promptcompress.IntensityLite, promptcompress.IntensityFull, promptcompress.IntensityStandard, promptcompress.IntensityAggressive, promptcompress.IntensityUltra:
		return promptcompress.Intensity(value)
	default:
		return fallback
	}
}

func applyPromptCompressionRTKStep(request *dto.GeneralOpenAIRequest, config promptcompress.Config, states map[int]*promptCompressionContentState, eligibleMessages map[int]struct{}, stats *promptcompress.Stats) bool {
	toolLookup := buildPromptCompressionToolLookup(request.Messages)
	for i, message := range request.Messages {
		state, ok := states[i]
		if !ok || !shouldPromptCompressionRTKMessage(message, config) {
			continue
		}
		next, eligible, ok := promptCompressionMapTextContent(state.current, func(content string) (string, bool, bool) {
			if content == "" {
				return content, false, true
			}
			result := promptcompress.CompressRTKText(content, config, promptCompressionRTKOptions(message, toolLookup, config))
			return mergePromptCompressionTextResult(content, result, stats)
		})
		if !ok {
			return false
		}
		if eligible {
			eligibleMessages[i] = struct{}{}
		}
		state.current = next
	}
	return true
}

func applyPromptCompressionCavemanStep(request *dto.GeneralOpenAIRequest, settings promptcompress.Settings, config promptcompress.Config, intensity promptcompress.Intensity, states map[int]*promptCompressionContentState, eligibleMessages map[int]struct{}, stats *promptcompress.Stats) bool {
	allowedRoles := promptCompressionRoleSet(settings.Caveman.CompressRoles)
	for i, message := range request.Messages {
		state, ok := states[i]
		if !ok || !shouldPromptCompressionCavemanRole(request, config, message, allowedRoles) {
			continue
		}
		next, eligible, ok := promptCompressionMapTextContent(state.current, func(content string) (string, bool, bool) {
			if len(content) < settings.Caveman.MinMessageLength {
				return content, false, true
			}
			result := promptcompress.CompressCavemanText(content, config, intensity, message.Role)
			return mergePromptCompressionTextResult(content, result, stats)
		})
		if !ok {
			return false
		}
		if eligible {
			eligibleMessages[i] = struct{}{}
		}
		state.current = next
	}
	return true
}

func applyPromptCompressionMessageModeStep(request *dto.GeneralOpenAIRequest, config promptcompress.Config, states map[int]*promptCompressionContentState, eligibleMessages map[int]struct{}, stats *promptcompress.Stats, mode promptcompress.Mode) bool {
	indexes := make([]int, 0, len(request.Messages))
	messages := make([]promptcompress.Message, 0, len(request.Messages))
	for i, message := range request.Messages {
		state, ok := states[i]
		if !ok {
			continue
		}
		content, ok := promptCompressionContentText(state.current)
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		indexes = append(indexes, i)
		messages = append(messages, promptcompress.Message{
			Role:    message.Role,
			Content: content,
		})
		eligibleMessages[i] = struct{}{}
	}
	if len(messages) == 0 {
		return true
	}

	stepConfig := config
	stepConfig.Mode = mode
	var out []promptcompress.Message
	var stepStats promptcompress.Stats
	switch mode {
	case promptcompress.ModeAggressive:
		out, stepStats = promptcompress.CompressAggressiveMessages(messages, stepConfig)
	case promptcompress.ModeUltra:
		out, stepStats = promptcompress.CompressUltraMessages(messages, stepConfig)
	default:
		return true
	}
	if stepStats.Bypassed && isPromptCompressionHardBypass(stepStats.BypassReason) {
		stats.Bypassed = true
		stats.BypassReason = stepStats.BypassReason
		return false
	}
	mergePromptCompressionStatsMetadata(stats, stepStats)
	for i, message := range out {
		if i >= len(indexes) || message.Content == messages[i].Content {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			stats.Bypassed = true
			stats.BypassReason = "empty_output"
			return false
		}
		index := indexes[i]
		state := states[index]
		state.current = promptCompressionReplaceTextContent(state.current, message.Content)
	}
	return true
}

func mergePromptCompressionTextResult(original string, result promptcompress.Result, stats *promptcompress.Stats) (string, bool, bool) {
	if strings.TrimSpace(result.Text) == "" && strings.TrimSpace(original) != "" {
		stats.Bypassed = true
		stats.BypassReason = "empty_output"
		return original, true, false
	}
	if result.Stats.Bypassed && isPromptCompressionHardBypass(result.Stats.BypassReason) {
		stats.Bypassed = true
		stats.BypassReason = result.Stats.BypassReason
		return original, true, false
	}
	stats.PreservedBlockCount += result.Stats.PreservedBlockCount
	stats.RedactedSecretCount += result.Stats.RedactedSecretCount
	mergePromptCompressionStatsMetadata(stats, result.Stats)
	if result.Compressed {
		return result.Text, true, true
	}
	return original, true, true
}

func mergePromptCompressionStatsMetadata(stats *promptcompress.Stats, stepStats promptcompress.Stats) {
	stats.TechniquesUsed = append(stats.TechniquesUsed, stepStats.TechniquesUsed...)
	stats.RulesApplied = append(stats.RulesApplied, stepStats.RulesApplied...)
	stats.ValidationWarnings = uniquePromptCompressionStrings(append(stats.ValidationWarnings, stepStats.ValidationWarnings...))
	stats.ValidationErrors = uniquePromptCompressionStrings(append(stats.ValidationErrors, stepStats.ValidationErrors...))
	stats.FallbackApplied = stats.FallbackApplied || stepStats.FallbackApplied
	stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, stepStats.RtkRawOutputPointers...)
	stats.EngineBreakdown = append(stats.EngineBreakdown, stepStats.EngineBreakdown...)
	if stepStats.Engine != "" {
		stats.EngineBreakdown = append(stats.EngineBreakdown, promptcompress.EngineBreakdownItem{
			Engine:           stepStats.Engine,
			OriginalTokens:   stepStats.OriginalTokens,
			CompressedTokens: stepStats.CompressedTokens,
			SavingsPercent:   stepStats.SavingsPercent,
			TechniquesUsed:   append([]string(nil), stepStats.TechniquesUsed...),
			RulesApplied:     append([]string(nil), stepStats.RulesApplied...),
			DurationMs:       stepStats.DurationMs,
		})
	}
	if stepStats.Aggressive == nil {
		return
	}
	if stats.Aggressive == nil {
		copyStats := *stepStats.Aggressive
		stats.Aggressive = &copyStats
		return
	}
	stats.Aggressive.SummarizerSavings += stepStats.Aggressive.SummarizerSavings
	stats.Aggressive.ToolResultSavings += stepStats.Aggressive.ToolResultSavings
	stats.Aggressive.AgingSavings += stepStats.Aggressive.AgingSavings
}

func isPromptCompressionHardBypass(reason string) bool {
	switch reason {
	case "empty_output", "preservation_check_failed":
		return true
	default:
		return false
	}
}

func promptCompressionReplaceTextContent(content any, text string) any {
	switch value := content.(type) {
	case string:
		return text
	case []any:
		next := make([]any, 0, len(value))
		replaced := false
		for _, item := range value {
			if _, ok := promptCompressionTextBlockText(item); !ok {
				next = append(next, item)
				continue
			}
			if replaced {
				continue
			}
			replacement, ok := promptCompressionSetTextBlockText(item, text)
			if !ok {
				next = append(next, item)
				continue
			}
			next = append(next, replacement)
			replaced = true
		}
		if !replaced {
			next = append([]any{map[string]any{"type": dto.ContentTypeText, "text": text}}, next...)
		}
		return next
	case []dto.MediaContent:
		next := make([]dto.MediaContent, 0, len(value))
		replaced := false
		for _, item := range value {
			if item.Type != dto.ContentTypeText {
				next = append(next, item)
				continue
			}
			if replaced {
				continue
			}
			item.Text = text
			next = append(next, item)
			replaced = true
		}
		if !replaced {
			next = append([]dto.MediaContent{{Type: dto.ContentTypeText, Text: text}}, next...)
		}
		return next
	default:
		return text
	}
}

func promptCompressionContentText(content any) (string, bool) {
	switch value := content.(type) {
	case string:
		return value, true
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := promptCompressionTextBlockText(item); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n"), len(parts) > 0
	case []dto.MediaContent:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if item.Type == dto.ContentTypeText && item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.Join(parts, "\n"), len(parts) > 0
	default:
		return "", false
	}
}

func promptCompressionMapTextContent(content any, transform promptCompressionTextTransform) (any, bool, bool) {
	switch value := content.(type) {
	case string:
		next, eligible, ok := transform(value)
		return next, eligible, ok
	case []any:
		next := make([]any, len(value))
		copy(next, value)
		changed := false
		eligible := false
		for i, item := range value {
			text, ok := promptCompressionTextBlockText(item)
			if !ok {
				continue
			}
			nextText, textEligible, transformOK := transform(text)
			if !transformOK {
				return content, true, false
			}
			eligible = eligible || textEligible
			if nextText == text {
				continue
			}
			replaced, replaceOK := promptCompressionSetTextBlockText(item, nextText)
			if !replaceOK {
				continue
			}
			next[i] = replaced
			changed = true
		}
		if !changed {
			return content, eligible, true
		}
		return next, eligible, true
	case []dto.MediaContent:
		next := make([]dto.MediaContent, len(value))
		copy(next, value)
		changed := false
		eligible := false
		for i, item := range value {
			if item.Type != dto.ContentTypeText {
				continue
			}
			nextText, textEligible, transformOK := transform(item.Text)
			if !transformOK {
				return content, true, false
			}
			eligible = eligible || textEligible
			if nextText == item.Text {
				continue
			}
			next[i].Text = nextText
			changed = true
		}
		if !changed {
			return content, eligible, true
		}
		return next, eligible, true
	default:
		return content, false, true
	}
}

func promptCompressionTextBlockText(item any) (string, bool) {
	switch value := item.(type) {
	case map[string]any:
		blockType, _ := value["type"].(string)
		if blockType != "" && blockType != dto.ContentTypeText && blockType != "input_text" {
			return "", false
		}
		text, ok := value["text"].(string)
		return text, ok
	case dto.MediaContent:
		if value.Type != dto.ContentTypeText {
			return "", false
		}
		return value.Text, true
	default:
		return "", false
	}
}

func promptCompressionSetTextBlockText(item any, text string) (any, bool) {
	switch value := item.(type) {
	case map[string]any:
		blockType, _ := value["type"].(string)
		if blockType != "" && blockType != dto.ContentTypeText && blockType != "input_text" {
			return item, false
		}
		next := make(map[string]any, len(value))
		for key, itemValue := range value {
			next[key] = itemValue
		}
		next["text"] = text
		return next, true
	case dto.MediaContent:
		if value.Type != dto.ContentTypeText {
			return item, false
		}
		value.Text = text
		return value, true
	default:
		return item, false
	}
}

func shouldPromptCompressionRTKMessage(message dto.Message, config promptcompress.Config) bool {
	role := strings.ToLower(strings.TrimSpace(message.Role))
	switch role {
	case "tool":
		return config.ApplyToToolResults || config.ApplyToCodeBlocks && promptCompressionContentHasCodeFence(message.Content)
	case "assistant":
		return config.ApplyToAssistantMessages || config.ApplyToCodeBlocks && promptCompressionContentHasCodeFence(message.Content)
	case "user":
		return promptCompressionContentHasRTKOutput(message.Content, config)
	default:
		return false
	}
}

func promptCompressionContentHasRTKOutput(content any, config promptcompress.Config) bool {
	switch value := content.(type) {
	case string:
		return promptcompress.ContainsRTKOutput(value, config)
	case []any:
		for _, item := range value {
			if text, ok := promptCompressionTextBlockText(item); ok && promptcompress.ContainsRTKOutput(text, config) {
				return true
			}
		}
	case []dto.MediaContent:
		for _, item := range value {
			if item.Type == dto.ContentTypeText && promptcompress.ContainsRTKOutput(item.Text, config) {
				return true
			}
		}
	}
	return false
}

func promptCompressionContentHasCodeFence(content any) bool {
	switch value := content.(type) {
	case string:
		return strings.Contains(value, "```")
	case []any:
		for _, item := range value {
			if text, ok := promptCompressionTextBlockText(item); ok && strings.Contains(text, "```") {
				return true
			}
		}
	case []dto.MediaContent:
		for _, item := range value {
			if item.Type == dto.ContentTypeText && strings.Contains(item.Text, "```") {
				return true
			}
		}
	}
	return false
}

func shouldPromptCompressionCavemanRole(request *dto.GeneralOpenAIRequest, config promptcompress.Config, message dto.Message, allowedRoles map[string]bool) bool {
	role := strings.ToLower(strings.TrimSpace(message.Role))
	if config.PreserveSystemPrompt && isPromptCompressionSystemRole(request, role) {
		return false
	}
	if !allowedRoles[role] {
		return false
	}
	return true
}

func isPromptCompressionSystemRole(request *dto.GeneralOpenAIRequest, role string) bool {
	if role == "system" || role == "developer" {
		return true
	}
	return request != nil && strings.EqualFold(role, request.GetSystemRoleName())
}

func buildPromptCompressionToolLookup(messages []dto.Message) map[string]promptCompressionToolMeta {
	lookup := make(map[string]promptCompressionToolMeta)
	for _, message := range messages {
		if !strings.EqualFold(message.Role, "assistant") {
			continue
		}
		for _, toolCall := range message.ParseToolCalls() {
			if toolCall.ID == "" {
				continue
			}
			lookup[toolCall.ID] = promptCompressionToolMeta{
				toolName: toolCall.Function.Name,
				command:  promptCompressionCommandFromArguments(toolCall.Function.Arguments),
			}
		}
	}
	return lookup
}

func promptCompressionCommandFromArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return ""
	}
	var args map[string]any
	if err := common.UnmarshalJsonStr(arguments, &args); err != nil {
		return ""
	}
	if command, ok := args["command"].(string); ok {
		return command
	}
	if command, ok := args["cmd"].(string); ok {
		return command
	}
	return ""
}

func promptCompressionRTKOptions(message dto.Message, lookup map[string]promptCompressionToolMeta, config promptcompress.Config) promptcompress.RtkTextOptions {
	options := promptcompress.RtkTextOptions{
		CodeBlocksOnly: config.ApplyToCodeBlocks && !config.ApplyToToolResults && !config.ApplyToAssistantMessages,
	}
	if strings.EqualFold(message.Role, "user") {
		options.CodeBlocksOnly = false
		options.EmbeddedOutputsOnly = true
		return options
	}
	meta, ok := lookup[message.ToolCallId]
	if !ok {
		return options
	}
	if promptCompressionTerminalToolPattern.MatchString(meta.toolName) {
		options.Command = meta.command
		return options
	}
	options.SkipFilters = true
	return options
}

func aggressivePromptCompressionConfig(config promptcompress.Config) promptcompress.Config {
	if config.MaxLines <= 0 || config.MaxLines > 90 {
		config.MaxLines = 90
	}
	if config.MaxChars <= 0 || config.MaxChars > 9000 {
		config.MaxChars = 9000
	}
	return config
}

func ultraPromptCompressionConfig(config promptcompress.Config) promptcompress.Config {
	if config.MaxLines <= 0 || config.MaxLines > 60 {
		config.MaxLines = 60
	}
	if config.MaxChars <= 0 || config.MaxChars > 6000 {
		config.MaxChars = 6000
	}
	return config
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
	if settings.AutoTriggerTokens > 0 && info.GetEstimatePromptTokens() >= settings.AutoTriggerTokens {
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
		Engine:                  string(mode),
		Timestamp:               time.Now().UnixMilli(),
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
	if stats.Engine != "" {
		other[profit.KeyCompressionEngine] = stats.Engine
	}
	if stats.Timestamp > 0 {
		other[profit.KeyCompressionTimestamp] = stats.Timestamp
	}
	other[profit.KeyCompressionSavingsPercent] = stats.SavingsPercent
	other[profit.KeyCompressionBypassed] = stats.Bypassed
	other[profit.KeyCompressionFallbackApplied] = stats.FallbackApplied
	if stats.BypassReason != "" {
		other[profit.KeyCompressionBypassReason] = stats.BypassReason
	}
	if len(stats.RulesApplied) > 0 {
		other[profit.KeyCompressionRulesApplied] = stats.RulesApplied
	}
	if len(stats.ValidationWarnings) > 0 {
		other[profit.KeyCompressionValidationWarnings] = stats.ValidationWarnings
	}
	if len(stats.ValidationErrors) > 0 {
		other[profit.KeyCompressionValidationErrors] = stats.ValidationErrors
	}
	if len(stats.EngineBreakdown) > 0 {
		other[profit.KeyCompressionEngineBreakdown] = stats.EngineBreakdown
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
