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

	protected := protect(text)
	compressed := protected.Text
	var techniques []string
	var rules []string

	switch mode {
	case ModeLite:
		compressed, rules = applyCaveman(compressed, IntensityLite)
		techniques = appendTechnique(techniques, "caveman")
	case ModeStandard:
		compressed, rules = applyCaveman(compressed, IntensityStandard)
		techniques = appendTechnique(techniques, "caveman")
	case ModeAggressive:
		rtkText, rtkTechniques, rtkRules := applyRTK(compressed, aggressiveConfig(config), "")
		compressed = rtkText
		techniques = append(techniques, rtkTechniques...)
		rules = append(rules, rtkRules...)
		cavemanText, cavemanRules := applyCaveman(compressed, IntensityAggressive)
		compressed = cavemanText
		techniques = appendTechnique(techniques, "caveman")
		rules = append(rules, cavemanRules...)
	case ModeUltra:
		rtkText, rtkTechniques, rtkRules := applyRTK(compressed, ultraConfig(config), "")
		compressed = rtkText
		techniques = append(techniques, rtkTechniques...)
		rules = append(rules, rtkRules...)
		cavemanText, cavemanRules := applyCaveman(compressed, IntensityUltra)
		compressed = cavemanText
		techniques = appendTechnique(techniques, "caveman")
		rules = append(rules, cavemanRules...)
	case ModeRTK:
		compressed, techniques, rules = applyRTK(compressed, config, "")
	case ModeStacked:
		rtkText, rtkTechniques, rtkRules := applyRTK(compressed, config, "")
		compressed = rtkText
		techniques = append(techniques, rtkTechniques...)
		rules = append(rules, rtkRules...)
		cavemanText, cavemanRules := applyCaveman(compressed, IntensityStandard)
		compressed = cavemanText
		techniques = appendTechnique(techniques, "caveman")
		rules = append(rules, cavemanRules...)
	}

	restored := restore(compressed, protected.Blocks)
	if !verifyPreserved(restored, protected.Blocks) {
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

func CompressMessages(messages []Message, config Config) ([]Message, Stats) {
	started := time.Now()
	mode := normalizeMode(config.Mode)
	stats := Stats{Mode: mode, OmniRouteCompatibleMode: string(mode)}
	if mode == ModeOff {
		return append([]Message(nil), messages...), stats
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
