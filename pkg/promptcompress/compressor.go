package promptcompress

import (
	"strings"
	"time"
)

func CompressText(text string, config Config) Result {
	started := time.Now()
	mode := normalizeMode(config.Mode)
	stats := newStats(mode, text, started)
	if text == "" || mode == ModeOff {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if config.MinTokens > 0 && EstimateTokens(text) < config.MinTokens {
		stats.Bypassed = true
		stats.BypassReason = "below_min_tokens"
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if mode == ModeRTK {
		return CompressRTKText(text, config, RtkTextOptions{})
	}
	if mode == ModeStacked {
		return CompressStackedText(text, config)
	}
	if mode == ModeAggressive {
		return CompressAggressiveText(text, config)
	}
	if mode == ModeUltra {
		return CompressUltraText(text, config)
	}

	protected := protect(text)
	compressed := protected.Text
	var techniques []string
	var rules []string

	switch mode {
	case ModeLite:
		compressed, rules = applyCavemanWithConfig(compressed, IntensityLite, "user", config)
		techniques = appendTechnique(techniques, "caveman")
	case ModeStandard:
		compressed, rules = applyCavemanWithConfig(compressed, normalizeCavemanIntensity(config.CavemanIntensity), "user", config)
		techniques = appendTechnique(techniques, "caveman")
	case ModeRTK:
		compressed, techniques, rules = applyRTK(compressed, config, "")
	}

	restored := restore(compressed, protected.Blocks)
	if !verifyPreserved(protected.ValidationText, restored, protected.Blocks) {
		stats.Bypassed = true
		stats.BypassReason = "preservation_check_failed"
		return Result{Text: text, Compressed: false, Stats: stats}
	}

	stats.PreservedBlockCount = len(protected.Blocks)
	stats.RedactedSecretCount = protected.RedactedCount
	stats.TechniquesUsed = uniqueStrings(techniques)
	stats.RulesApplied = uniqueStrings(rules)
	stats.CompressedTokens = EstimateTokens(restored)
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.CompressedTokens >= stats.OriginalTokens && len([]rune(restored)) >= len([]rune(text)) && protected.RedactedCount == 0 {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	return Result{Text: restored, Compressed: stats.CompressedTokens < stats.OriginalTokens || protected.RedactedCount > 0, Stats: stats}
}

func CompressRTKText(text string, config Config, options RtkTextOptions) Result {
	started := time.Now()
	mode := ModeRTK
	stats := newStats(mode, text, started)
	if text == "" {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if config.MinTokens > 0 && EstimateTokens(text) < config.MinTokens {
		stats.Bypassed = true
		stats.BypassReason = "below_min_tokens"
		return Result{Text: text, Compressed: false, Stats: stats}
	}

	var compressed string
	var techniques []string
	var rules []string
	if options.CodeBlocksOnly {
		compressed, techniques, rules = applyRTKCodeBlocksOnly(text, config)
	} else {
		compressed, techniques, rules = applyRTKWithOptions(text, config, rtkApplyOptions{
			command:     options.Command,
			skipFilters: options.SkipFilters,
		})
	}
	stats.TechniquesUsed = uniqueStrings(techniques)
	stats.RulesApplied = uniqueStrings(rules)
	stats.CompressedTokens = EstimateTokens(compressed)
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.CompressedTokens >= stats.OriginalTokens && len([]rune(compressed)) >= len([]rune(text)) {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if pointer, err := maybePersistRtkRawOutput(text, config, options.Command); err == nil && pointer != nil {
		stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, *pointer)
		stats.TechniquesUsed = uniqueStrings(append(stats.TechniquesUsed, "rtk-raw-output-retention"))
		stats.RulesApplied = uniqueStrings(append(stats.RulesApplied, "rtk:raw-output-retention"))
	}
	return Result{Text: compressed, Compressed: stats.CompressedTokens < stats.OriginalTokens, Stats: stats}
}

func CompressStackedText(text string, config Config) Result {
	started := time.Now()
	stats := newStats(ModeStacked, text, started)
	if text == "" {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if config.MinTokens > 0 && EstimateTokens(text) < config.MinTokens {
		stats.Bypassed = true
		stats.BypassReason = "below_min_tokens"
		return Result{Text: text, Compressed: false, Stats: stats}
	}

	current := text
	for _, step := range NormalizeStackedPipeline(config.StackedPipeline) {
		stepConfig := config
		var result Result
		switch step.Engine {
		case "rtk":
			stepConfig.RtkIntensity = pipelineRtkIntensity(step.Intensity, config.RtkIntensity)
			result = CompressRTKText(current, stepConfig, RtkTextOptions{})
		case "lite":
			result = CompressCavemanText(current, stepConfig, IntensityLite, "user")
		case "aggressive":
			rtkResult := CompressRTKText(current, aggressiveConfig(stepConfig), RtkTextOptions{})
			stats.TechniquesUsed = append(stats.TechniquesUsed, rtkResult.Stats.TechniquesUsed...)
			stats.RulesApplied = append(stats.RulesApplied, rtkResult.Stats.RulesApplied...)
			stats.PreservedBlockCount += rtkResult.Stats.PreservedBlockCount
			stats.RedactedSecretCount += rtkResult.Stats.RedactedSecretCount
			stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, rtkResult.Stats.RtkRawOutputPointers...)
			if rtkResult.Compressed {
				current = rtkResult.Text
			}
			result = CompressCavemanText(current, stepConfig, IntensityAggressive, "user")
		case "ultra":
			rtkResult := CompressRTKText(current, ultraConfig(stepConfig), RtkTextOptions{})
			stats.TechniquesUsed = append(stats.TechniquesUsed, rtkResult.Stats.TechniquesUsed...)
			stats.RulesApplied = append(stats.RulesApplied, rtkResult.Stats.RulesApplied...)
			stats.PreservedBlockCount += rtkResult.Stats.PreservedBlockCount
			stats.RedactedSecretCount += rtkResult.Stats.RedactedSecretCount
			stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, rtkResult.Stats.RtkRawOutputPointers...)
			if rtkResult.Compressed {
				current = rtkResult.Text
			}
			result = CompressCavemanText(current, stepConfig, IntensityUltra, "user")
		default:
			result = CompressCavemanText(current, stepConfig, pipelineCavemanIntensity(step.Intensity, config.CavemanIntensity), "user")
		}
		stats.TechniquesUsed = append(stats.TechniquesUsed, result.Stats.TechniquesUsed...)
		stats.RulesApplied = append(stats.RulesApplied, result.Stats.RulesApplied...)
		stats.PreservedBlockCount += result.Stats.PreservedBlockCount
		stats.RedactedSecretCount += result.Stats.RedactedSecretCount
		stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, result.Stats.RtkRawOutputPointers...)
		if result.Stats.Bypassed && isHardCompressionBypass(result.Stats.BypassReason) {
			stats.Bypassed = true
			stats.BypassReason = result.Stats.BypassReason
			stats.CompressedTokens = stats.OriginalTokens
			stats.CompressionSavedTokens = 0
			stats.SavingsPercent = 0
			stats.DurationMs = time.Since(started).Milliseconds()
			return Result{Text: text, Compressed: false, Stats: stats}
		}
		if result.Compressed {
			current = result.Text
		}
	}

	stats.TechniquesUsed = uniqueStrings(stats.TechniquesUsed)
	stats.RulesApplied = uniqueStrings(stats.RulesApplied)
	stats.CompressedTokens = EstimateTokens(current)
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.CompressedTokens >= stats.OriginalTokens && len([]rune(current)) >= len([]rune(text)) && stats.RedactedSecretCount == 0 {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	return Result{Text: current, Compressed: stats.CompressedTokens < stats.OriginalTokens || stats.RedactedSecretCount > 0, Stats: stats}
}

func CompressCavemanText(text string, config Config, intensity Intensity, role string) Result {
	started := time.Now()
	mode := ModeStandard
	stats := newStats(mode, text, started)
	if text == "" {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	if config.MinTokens > 0 && EstimateTokens(text) < config.MinTokens {
		stats.Bypassed = true
		stats.BypassReason = "below_min_tokens"
		return Result{Text: text, Compressed: false, Stats: stats}
	}

	protected := protectWithPatterns(text, config.CavemanPreservePatterns)
	compressed, rules := applyCavemanWithConfig(protected.Text, intensity, role, config)
	restored := restore(compressed, protected.Blocks)
	if !verifyPreserved(protected.ValidationText, restored, protected.Blocks) {
		stats.Bypassed = true
		stats.BypassReason = "preservation_check_failed"
		return Result{Text: text, Compressed: false, Stats: stats}
	}

	stats.PreservedBlockCount = len(protected.Blocks)
	stats.RedactedSecretCount = protected.RedactedCount
	stats.TechniquesUsed = []string{"caveman"}
	stats.RulesApplied = uniqueStrings(rules)
	stats.CompressedTokens = EstimateTokens(restored)
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.CompressedTokens >= stats.OriginalTokens && len([]rune(restored)) >= len([]rune(text)) && protected.RedactedCount == 0 {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	return Result{Text: restored, Compressed: stats.CompressedTokens < stats.OriginalTokens || protected.RedactedCount > 0, Stats: stats}
}

func isHardCompressionBypass(reason string) bool {
	switch reason {
	case "empty_output", "preservation_check_failed":
		return true
	default:
		return false
	}
}

func CompressMessages(messages []Message, config Config) ([]Message, Stats) {
	started := time.Now()
	mode := normalizeMode(config.Mode)
	stats := Stats{Mode: mode, OmniRouteCompatibleMode: string(mode)}
	if mode == ModeOff {
		return append([]Message(nil), messages...), stats
	}
	switch mode {
	case ModeAggressive:
		return CompressAggressiveMessages(messages, config)
	case ModeUltra:
		return CompressUltraMessages(messages, config)
	}
	out := make([]Message, len(messages))
	for i, message := range messages {
		out[i] = message
		stats.OriginalTokens += EstimateTokens(message.Content)
		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
			stats.CompressedTokens += EstimateTokens(message.Content)
			continue
		}
		result := CompressText(message.Content, config)
		out[i].Content = result.Text
		stats.CompressedTokens += result.Stats.CompressedTokens
		stats.TechniquesUsed = append(stats.TechniquesUsed, result.Stats.TechniquesUsed...)
		stats.RulesApplied = append(stats.RulesApplied, result.Stats.RulesApplied...)
		stats.PreservedBlockCount += result.Stats.PreservedBlockCount
		stats.RedactedSecretCount += result.Stats.RedactedSecretCount
		stats.RtkRawOutputPointers = append(stats.RtkRawOutputPointers, result.Stats.RtkRawOutputPointers...)
	}
	stats.TechniquesUsed = uniqueStrings(stats.TechniquesUsed)
	stats.RulesApplied = uniqueStrings(stats.RulesApplied)
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.DurationMs = time.Since(started).Milliseconds()
	return out, stats
}

func normalizeMode(mode Mode) Mode {
	switch mode {
	case ModeLite, ModeStandard, ModeAggressive, ModeUltra, ModeRTK, ModeStacked:
		return mode
	default:
		return ModeOff
	}
}

func normalizeCavemanIntensity(intensity Intensity) Intensity {
	if intensity == "" {
		return IntensityLite
	}
	if !validIntensity(intensity) {
		return IntensityLite
	}
	return intensity
}

func pipelineRtkIntensity(value string, fallback RtkIntensity) RtkIntensity {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return NormalizeRtkIntensity(fallback)
	}
	return NormalizeRtkIntensity(RtkIntensity(value))
}

func pipelineCavemanIntensity(value string, fallback Intensity) Intensity {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return normalizeCavemanIntensity(fallback)
	}
	switch Intensity(value) {
	case IntensityLite, IntensityFull, IntensityStandard, IntensityAggressive, IntensityUltra:
		return Intensity(value)
	default:
		return normalizeCavemanIntensity(fallback)
	}
}

func aggressiveConfig(config Config) Config {
	if config.MaxLines <= 0 {
		config.MaxLines = 90
	}
	if config.MaxChars <= 0 {
		config.MaxChars = 9000
	}
	return config
}

func ultraConfig(config Config) Config {
	if config.MaxLines <= 0 {
		config.MaxLines = 60
	}
	if config.MaxChars <= 0 {
		config.MaxChars = 6000
	}
	return config
}

func appendTechnique(items []string, item string) []string {
	if item == "" {
		return items
	}
	return append(items, item)
}

func uniqueStrings(items []string) []string {
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

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func stringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out[item] = true
	}
	return out
}
