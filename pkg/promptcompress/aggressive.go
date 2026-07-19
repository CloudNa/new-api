package promptcompress

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const compressedMarkerPrefix = "[COMPRESSED:"

var (
	aggressiveAnsiPattern        = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	aggressiveShellPromptPattern = regexp.MustCompile(`\$\s`)
	aggressiveJSONPrefixPattern  = regexp.MustCompile(`^\s*[\{\[]`)
	aggressiveIntentPattern      = regexp.MustCompile(`(?i)^(?:request|fix|implement|add|remove|update|refactor|create|delete|change|build)\s*:`)
	ultraWhitespacePattern       = regexp.MustCompile(`\s+`)
	ultraForcePreservePattern    = regexp.MustCompile("(?i)\\d|https?://|[._/\\\\]|Error:|Exception:|```")
)

var aggressiveFileExtensions = map[string]bool{
	"c": true, "cpp": true, "css": true, "go": true, "h": true, "hpp": true, "html": true,
	"java": true, "js": true, "json": true, "jsx": true, "md": true, "py": true, "rb": true,
	"rs": true, "sh": true, "sql": true, "ts": true, "tsx": true, "yaml": true, "yml": true,
}

var ultraStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "shall": true, "can": true, "need": true,
	"dare": true, "ought": true, "used": true, "i": true, "we": true, "you": true,
	"he": true, "she": true, "it": true, "they": true, "me": true, "us": true,
	"him": true, "her": true, "them": true, "my": true, "our": true, "your": true,
	"his": true, "its": true, "their": true, "this": true, "that": true, "these": true,
	"those": true, "and": true, "but": true, "or": true, "nor": true, "for": true,
	"yet": true, "so": true, "as": true, "at": true, "by": true, "in": true,
	"of": true, "on": true, "to": true, "up": true, "via": true, "with": true,
	"from": true, "into": true, "onto": true, "upon": true, "about": true, "just": true,
	"very": true, "really": true, "quite": true, "rather": true, "also": true, "too": true,
	"even": true, "still": true, "already": true, "always": true, "never": true, "often": true,
	"usually": true, "sometimes": true, "here": true, "there": true,
}

type aggressiveToolResult struct {
	compressed string
	strategy   string
	saved      int
}

func CompressAggressiveMessages(messages []Message, config Config) ([]Message, Stats) {
	started := time.Now()
	stats := Stats{
		Mode:                    ModeAggressive,
		Engine:                  "aggressive",
		Timestamp:               started.UnixMilli(),
		OmniRouteCompatibleMode: string(ModeAggressive),
	}
	out := append([]Message(nil), messages...)
	for _, message := range messages {
		stats.OriginalTokens += EstimateTokens(message.Content)
	}
	if len(out) == 0 {
		stats.DurationMs = time.Since(started).Milliseconds()
		stats.CompressedTokens = stats.OriginalTokens
		stats.Bypassed = true
		stats.BypassReason = "no_compressible_messages"
		return out, stats
	}

	thresholds := effectiveAggressiveThresholds(config)
	strategies := effectiveAggressiveToolStrategies(config)
	summarizerEnabled := effectiveAggressiveSummarizerEnabled(config)
	maxTokensPerMessage := effectiveAggressiveMaxTokensPerMessage(config)
	minSavingsThreshold := effectiveAggressiveMinSavingsRate(config)
	aggressiveStats := AggressiveStats{}
	for i, message := range out {
		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "tool" && role != "function" {
			continue
		}
		if message.Content == "" || strings.HasPrefix(message.Content, compressedMarkerPrefix) {
			continue
		}
		result := compressAggressiveToolResult(message.Content, strategies)
		if result.strategy == "none" || result.saved <= 0 {
			continue
		}
		aggressiveStats.ToolResultSavings += result.saved
		out[i].Content = result.compressed
	}

	aged, agingSaved := applyAggressiveAging(out, thresholds, config.PreserveSystemPrompt)
	out = aged
	aggressiveStats.AgingSavings = agingSaved

	if summarizerEnabled {
		for i, message := range out {
			if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
				continue
			}
			if message.Content == "" || strings.HasPrefix(message.Content, compressedMarkerPrefix) {
				continue
			}
			if len(message.Content) <= maxTokensPerMessage*4 {
				continue
			}
			summary := summarizeAggressiveMessages([]Message{message}, maxTokensPerMessage, true)
			if summary != "" && len(summary) < len(message.Content) {
				tagged := "[COMPRESSED:summary] " + summary
				aggressiveStats.SummarizerSavings += maxInt(0, EstimateTokens(message.Content)-EstimateTokens(tagged))
				out[i].Content = tagged
			}
		}
	}

	out, stats = validateMessageCompression(messages, out, stats)
	for _, message := range out {
		stats.CompressedTokens += EstimateTokens(message.Content)
	}
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}

	if stats.SavingsPercent < minSavingsThreshold*100 {
		out, stats = applyAggressiveFallbacks(messages, out, config, stats)
	}

	stats.Aggressive = &aggressiveStats
	if aggressiveStats.ToolResultSavings > 0 {
		stats.TechniquesUsed = append(stats.TechniquesUsed, "toolResult")
	}
	if aggressiveStats.AgingSavings > 0 {
		stats.TechniquesUsed = append(stats.TechniquesUsed, "aging")
	}
	if aggressiveStats.SummarizerSavings > 0 {
		stats.TechniquesUsed = append(stats.TechniquesUsed, "summarizer")
	}

	stats.TechniquesUsed = uniqueStrings(stats.TechniquesUsed)
	stats.RulesApplied = uniqueStrings(stats.RulesApplied)
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.CompressedTokens >= stats.OriginalTokens {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return messages, stats
	}
	return out, stats
}

func CompressUltraMessages(messages []Message, config Config) ([]Message, Stats) {
	started := time.Now()
	stats := Stats{
		Mode:                    ModeUltra,
		Engine:                  "ultra",
		Timestamp:               started.UnixMilli(),
		OmniRouteCompatibleMode: string(ModeUltra),
	}
	out := append([]Message(nil), messages...)
	compressionRate := effectiveUltraCompressionRate(config)
	minScoreThreshold := effectiveUltraMinScoreThreshold(config)
	maxTokensPerMessage := effectiveUltraMaxTokensPerMessage(config)
	compressedIndexes := make(map[int]struct{}, len(out))
	if strings.TrimSpace(config.UltraModelPath) != "" {
		stats.ValidationWarnings = append(stats.ValidationWarnings, "ultra_slm_model_path_ignored_go_native_heuristic_used")
	}
	for i, message := range out {
		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
			continue
		}
		if message.Content == "" || strings.HasPrefix(message.Content, compressedMarkerPrefix) {
			continue
		}
		originalTokens := EstimateTokens(message.Content)
		if maxTokensPerMessage > 0 && originalTokens <= maxTokensPerMessage {
			continue
		}
		stats.OriginalTokens += originalTokens
		pruned := pruneUltraByScore(message.Content, compressionRate, minScoreThreshold)
		out[i].Content = pruned
		compressedIndexes[i] = struct{}{}
	}
	out, stats = validateMessageCompression(messages, out, stats)
	stats.CompressedTokens = 0
	for index := range compressedIndexes {
		if index >= len(out) {
			continue
		}
		stats.CompressedTokens += EstimateTokens(out[index].Content)
	}
	stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
	if stats.OriginalTokens > 0 {
		stats.SavingsPercent = float64(stats.CompressionSavedTokens) / float64(stats.OriginalTokens) * 100
	}
	stats.TechniquesUsed = []string{"ultra-heuristic-pruning"}
	stats.DurationMs = time.Since(started).Milliseconds()
	if stats.OriginalTokens == 0 || stats.CompressedTokens >= stats.OriginalTokens {
		stats.Bypassed = true
		stats.BypassReason = "no_savings"
		stats.CompressedTokens = stats.OriginalTokens
		stats.CompressionSavedTokens = 0
		stats.SavingsPercent = 0
		return messages, stats
	}
	return out, stats
}

func CompressAggressiveText(text string, config Config) Result {
	messages, stats := CompressAggressiveMessages([]Message{{Role: "user", Content: text}}, config)
	if len(messages) == 0 {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	return Result{Text: messages[0].Content, Compressed: messages[0].Content != text && !stats.Bypassed, Stats: stats}
}

func CompressUltraText(text string, config Config) Result {
	messages, stats := CompressUltraMessages([]Message{{Role: "user", Content: text}}, config)
	if len(messages) == 0 {
		return Result{Text: text, Compressed: false, Stats: stats}
	}
	return Result{Text: messages[0].Content, Compressed: messages[0].Content != text && !stats.Bypassed, Stats: stats}
}

func compressAggressiveToolResult(content string, opts ToolStrategiesConfig) aggressiveToolResult {
	if opts.FileContent {
		if result := compressAggressiveFileContent(content); result != "" {
			return newAggressiveToolResult(content, result, "fileContent")
		}
	}
	if opts.GrepSearch {
		if result := compressAggressiveGrepSearch(content); result != "" {
			return newAggressiveToolResult(content, result, "grepSearch")
		}
	}
	if opts.ShellOutput {
		if result := compressAggressiveShellOutput(content); result != "" {
			return newAggressiveToolResult(content, result, "shellOutput")
		}
	}
	if opts.JSON {
		if result := compressAggressiveJSON(content); result != "" {
			return newAggressiveToolResult(content, result, "json")
		}
	}
	if opts.ErrorMessage {
		if result := compressAggressiveErrorMessage(content); result != "" {
			return newAggressiveToolResult(content, result, "errorMessage")
		}
	}
	return aggressiveToolResult{compressed: content, strategy: "none"}
}

func newAggressiveToolResult(original string, compressed string, strategy string) aggressiveToolResult {
	return aggressiveToolResult{
		compressed: compressed,
		strategy:   strategy,
		saved:      EstimateTokens(original) - EstimateTokens(compressed),
	}
}

func compressAggressiveFileContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return ""
	}
	hasCode := false
	for _, line := range lines {
		if isAggressiveCodeLikeLine(line) {
			hasCode = true
			break
		}
	}
	if !hasCode {
		return ""
	}
	keep := 20
	tail := 5
	if len(lines) <= keep+tail {
		return content
	}
	head := strings.Join(lines[:keep], "\n")
	tailLines := strings.Join(lines[len(lines)-tail:], "\n")
	return fmt.Sprintf("%s\n... [%d lines elided] ...\n%s", head, len(lines)-keep-tail, tailLines)
}

func isAggressiveCodeLikeLine(rawLine string) bool {
	line := strings.TrimLeft(rawLine, " \t")
	for _, prefix := range []string{"import ", "export ", "function ", "class ", "const ", "let ", "var ", "return ", "if(", "if (", "for(", "for (", "while(", "while ("} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func compressAggressiveGrepSearch(content string) string {
	lines := strings.Split(content, "\n")
	grepLines := make([]string, 0, len(lines))
	paths := make([]string, 0)
	seenPaths := map[string]bool{}
	for _, line := range lines {
		pathValue := parseAggressiveGrepLinePath(line)
		if pathValue == "" {
			continue
		}
		grepLines = append(grepLines, line)
		if !seenPaths[pathValue] {
			seenPaths[pathValue] = true
			paths = append(paths, pathValue)
		}
	}
	if len(grepLines) == 0 {
		return ""
	}
	top := grepLines
	if len(top) > 30 {
		top = top[:30]
	}
	result := strings.Join(top, "\n")
	if remaining := len(grepLines) - len(top); remaining > 0 {
		result += fmt.Sprintf("\n... [%d more matches]", remaining)
	}
	result += "\nFiles: " + strings.Join(paths, ", ")
	return result
}

func parseAggressiveGrepLinePath(line string) string {
	firstColon := strings.Index(line, ":")
	if firstColon <= 0 {
		return ""
	}
	secondColon := strings.Index(line[firstColon+1:], ":")
	if secondColon == -1 {
		return ""
	}
	secondColon += firstColon + 1
	lineNumber := line[firstColon+1 : secondColon]
	if lineNumber == "" {
		return ""
	}
	for _, r := range lineNumber {
		if r < '0' || r > '9' {
			return ""
		}
	}
	filePath := line[:firstColon]
	if filePath == "" || strings.ContainsAny(filePath, " \t\r\n") {
		return ""
	}
	return filePath
}

func compressAggressiveShellOutput(content string) string {
	hasAnsi := aggressiveAnsiPattern.MatchString(content)
	hasPrompt := aggressiveShellPromptPattern.MatchString(content)
	if !hasAnsi && !hasPrompt {
		return ""
	}
	cleaned := aggressiveAnsiPattern.ReplaceAllString(content, "")
	lines := strings.Split(cleaned, "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	deduped := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(deduped) == 0 || line != deduped[len(deduped)-1] {
			deduped = append(deduped, line)
		}
	}
	return strings.Join(deduped, "\n")
}

func compressAggressiveJSON(content string) string {
	if len(content) <= 2000 || !aggressiveJSONPrefixPattern.MatchString(content) {
		return ""
	}
	var parsed any
	if err := common.Unmarshal([]byte(content), &parsed); err != nil {
		return ""
	}
	switch value := parsed.(type) {
	case []any:
		if len(value) <= 7 {
			return content
		}
		summary := map[string]any{
			"type":   "array",
			"total":  len(value),
			"first5": value[:5],
			"last2":  value[len(value)-2:],
		}
		data, err := common.Marshal(summary)
		if err != nil {
			return ""
		}
		return string(data)
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		summary := map[string]any{}
		limit := minInt(20, len(keys))
		for _, key := range keys[:limit] {
			item := value[key]
			if nested, ok := item.(map[string]any); ok {
				summary[key] = fmt.Sprintf("{...%d keys}", len(nested))
			} else {
				summary[key] = item
			}
		}
		if len(keys) > 20 {
			summary[fmt.Sprintf("_remaining_%d_keys", len(keys)-20)] = true
		}
		data, err := common.Marshal(summary)
		if err != nil {
			return ""
		}
		return string(data)
	default:
		return ""
	}
}

func compressAggressiveErrorMessage(content string) string {
	if !hasAggressiveErrorLikeOutput(content) {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}
	errorLine := lines[0]
	stackLines := lines[1:]
	head := stackLines[:minInt(10, len(stackLines))]
	tail := []string{}
	if len(stackLines) > 10 {
		tail = stackLines[maxInt(0, len(stackLines)-3):]
	}
	middle := []string{}
	if len(stackLines) > 13 {
		middle = []string{fmt.Sprintf("... [%d frames elided] ...", len(stackLines)-13)}
	}
	return strings.Join(append(append([]string{errorLine}, head...), append(middle, tail...)...), "\n")
}

func hasAggressiveErrorLikeOutput(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "error:") ||
		strings.Contains(lower, "error ") ||
		strings.Contains(lower, "[error]") ||
		strings.Contains(lower, "exception:") ||
		strings.Contains(lower, "exception ") ||
		strings.Contains(lower, "[exception]") ||
		strings.Contains(lower, "traceback")
}

func applyAggressiveAging(messages []Message, thresholds AggressiveThresholds, preserveSystemPrompt bool) ([]Message, int) {
	out := append([]Message(nil), messages...)
	total := len(out)
	saved := 0
	for i, message := range out {
		text := message.Content
		if preserveSystemPrompt && strings.EqualFold(message.Role, "system") || strings.HasPrefix(text, compressedMarkerPrefix) {
			continue
		}
		distance := total - 1 - i
		switch {
		case distance <= thresholds.Verbatim:
			continue
		case distance <= thresholds.Light:
			compressed := normalizeAggressiveLiteText(text)
			tagged := "[COMPRESSED:aging:light] " + compressed
			if EstimateTokens(tagged) < EstimateTokens(text) {
				saved += EstimateTokens(text) - EstimateTokens(tagged)
				out[i].Content = tagged
			}
		case distance <= thresholds.Moderate:
			result := CompressCavemanText(text, DefaultConfig(), IntensityLite, message.Role)
			if result.Compressed {
				tagged := "[COMPRESSED:aging:moderate] " + result.Text
				if EstimateTokens(tagged) < EstimateTokens(text) {
					saved += EstimateTokens(text) - EstimateTokens(tagged)
					out[i].Content = tagged
				}
			}
		default:
			var tagged string
			switch strings.ToLower(strings.TrimSpace(message.Role)) {
			case "assistant":
				tagged = "[COMPRESSED:aging:fullSummary] " + summarizeAggressiveMessages([]Message{message}, 2000, true)
			case "user":
				firstLine := strings.TrimSpace(strings.Split(text, "\n")[0])
				if len([]rune(firstLine)) > 120 {
					firstLine = string([]rune(firstLine)[:120])
				}
				tagged = "[COMPRESSED:aging:fullSummary] " + firstLine
			default:
				continue
			}
			if EstimateTokens(tagged) < EstimateTokens(text) {
				saved += EstimateTokens(text) - EstimateTokens(tagged)
				out[i].Content = tagged
			}
		}
	}
	return out, maxInt(0, saved)
}

func applyAggressiveFallbacks(original []Message, current []Message, config Config, stats Stats) ([]Message, Stats) {
	cavemanMessages := append([]Message(nil), current...)
	cavemanTokens := 0
	cavemanChanged := false
	for i, message := range cavemanMessages {
		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
			cavemanTokens += EstimateTokens(message.Content)
			continue
		}
		result := CompressCavemanText(message.Content, config, IntensityLite, message.Role)
		if result.Compressed {
			cavemanMessages[i].Content = result.Text
			cavemanChanged = true
		}
		cavemanTokens += EstimateTokens(cavemanMessages[i].Content)
	}
	if cavemanChanged {
		cavemanSavings := 0.0
		if stats.OriginalTokens > 0 {
			cavemanSavings = float64(maxInt(0, stats.OriginalTokens-cavemanTokens)) / float64(stats.OriginalTokens) * 100
		}
		if cavemanSavings > stats.SavingsPercent {
			stats.CompressedTokens = cavemanTokens
			stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
			stats.SavingsPercent = cavemanSavings
			stats.FallbackApplied = true
			stats.TechniquesUsed = append(stats.TechniquesUsed, "caveman-fallback")
			return cavemanMessages, stats
		}
	}

	liteMessages := append([]Message(nil), current...)
	liteTokens := 0
	liteChanged := false
	for i, message := range liteMessages {
		if config.PreserveSystemPrompt && strings.EqualFold(message.Role, "system") {
			liteTokens += EstimateTokens(message.Content)
			continue
		}
		next := normalizeAggressiveLiteText(message.Content)
		if next != message.Content {
			liteMessages[i].Content = next
			liteChanged = true
		}
		liteTokens += EstimateTokens(liteMessages[i].Content)
	}
	if liteChanged {
		liteSavings := 0.0
		if stats.OriginalTokens > 0 {
			liteSavings = float64(maxInt(0, stats.OriginalTokens-liteTokens)) / float64(stats.OriginalTokens) * 100
		}
		if liteSavings > stats.SavingsPercent {
			stats.CompressedTokens = liteTokens
			stats.CompressionSavedTokens = maxInt(0, stats.OriginalTokens-stats.CompressedTokens)
			stats.SavingsPercent = liteSavings
			stats.FallbackApplied = true
			stats.TechniquesUsed = append(stats.TechniquesUsed, "lite-fallback")
			return liteMessages, stats
		}
	}
	return currentOrOriginal(original, current), stats
}

func currentOrOriginal(original []Message, current []Message) []Message {
	if len(current) == 0 {
		return original
	}
	return current
}

func effectiveAggressiveThresholds(config Config) AggressiveThresholds {
	return normalizeAggressiveThresholds(config.AggressiveThresholds, DefaultAggressiveThresholds())
}

func effectiveAggressiveToolStrategies(config Config) ToolStrategiesConfig {
	if !config.AggressiveToolConfigured {
		return DefaultToolStrategiesConfig()
	}
	return normalizeToolStrategies(config.AggressiveToolStrategies, DefaultToolStrategiesConfig())
}

func effectiveAggressiveSummarizerEnabled(config Config) bool {
	if !config.AggressiveSummarizerSet {
		return true
	}
	return config.AggressiveSummarizerEnabled
}

func effectiveAggressiveMaxTokensPerMessage(config Config) int {
	if config.AggressiveMaxTokensPerMsg <= 0 {
		return 2048
	}
	return config.AggressiveMaxTokensPerMsg
}

func effectiveAggressiveMinSavingsRate(config Config) float64 {
	if !config.AggressiveMinSavingsRateSet && config.AggressiveMinSavingsRate == 0 {
		return 0.05
	}
	if config.AggressiveMinSavingsRate < 0 || config.AggressiveMinSavingsRate > 1 {
		return 0.05
	}
	return config.AggressiveMinSavingsRate
}

func effectiveUltraCompressionRate(config Config) float64 {
	if !config.UltraCompressionRateSet && config.UltraCompressionRate == 0 {
		return 0.5
	}
	if config.UltraCompressionRate < 0 || config.UltraCompressionRate > 1 {
		return 0.5
	}
	return config.UltraCompressionRate
}

func effectiveUltraMinScoreThreshold(config Config) float64 {
	if !config.UltraMinScoreThresholdSet && config.UltraMinScoreThreshold == 0 {
		return 0.3
	}
	if config.UltraMinScoreThreshold < 0 || config.UltraMinScoreThreshold > 1 {
		return 0.3
	}
	return config.UltraMinScoreThreshold
}

func effectiveUltraMaxTokensPerMessage(config Config) int {
	if config.UltraMaxTokensPerMessage < 0 {
		return 0
	}
	return config.UltraMaxTokensPerMessage
}

func validateMessageCompression(original []Message, compressed []Message, stats Stats) ([]Message, Stats) {
	out := append([]Message(nil), compressed...)
	for i, message := range out {
		if i >= len(original) || message.Content == original[i].Content {
			continue
		}
		validation := validateCompression(original[i].Content, message.Content)
		stats.ValidationWarnings = append(stats.ValidationWarnings, validation.Warnings...)
		if validation.Valid {
			continue
		}
		stats.ValidationErrors = append(stats.ValidationErrors, validation.Errors...)
		stats.FallbackApplied = true
		out[i] = original[i]
	}
	stats.ValidationWarnings = uniqueStrings(stats.ValidationWarnings)
	stats.ValidationErrors = uniqueStrings(stats.ValidationErrors)
	return out, stats
}

func normalizeAggressiveLiteText(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
}

func summarizeAggressiveMessages(messages []Message, maxLen int, preserveCode bool) string {
	filtered := make([]Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" || strings.HasPrefix(message.Content, compressedMarkerPrefix) {
			continue
		}
		filtered = append(filtered, message)
	}
	if len(filtered) == 0 {
		return ""
	}
	parts := []string{"[COMPRESSED:summary]"}
	if intents := aggressiveSummaryIntents(filtered); len(intents) > 0 {
		parts = append(parts, "Intents: "+strings.Join(intents, "; ")+".")
	}
	if files := aggressiveSummaryFilePaths(filtered); len(files) > 0 {
		parts = append(parts, "Files touched: "+strings.Join(files, ", ")+".")
	}
	if errors := aggressiveSummaryErrors(filtered); len(errors) > 0 {
		parts = append(parts, "Errors: "+strings.Join(errors, "; ")+".")
	}
	if decision := aggressiveSummaryLastAssistant(filtered); decision != "" {
		if preserveCode {
			decision = trimAggressiveCodeFences(decision)
		}
		runes := []rune(decision)
		if len(runes) > 200 {
			decision = string(runes[:200])
		}
		parts = append(parts, "Last decision: "+decision+".")
	}
	result := strings.Join(parts, " ")
	if maxLen > 0 && len([]rune(result)) > maxLen {
		runes := []rune(result)
		if maxLen <= 3 {
			return string(runes[:maxLen])
		}
		return string(runes[:maxLen-3]) + "..."
	}
	return result
}

func aggressiveSummaryIntents(messages []Message) []string {
	intents := []string{}
	for _, message := range messages {
		if !strings.EqualFold(message.Role, "user") {
			continue
		}
		firstLine := strings.TrimSpace(strings.Split(message.Content, "\n")[0])
		if firstLine == "" {
			continue
		}
		if aggressiveIntentPattern.MatchString(firstLine) {
			intents = append(intents, truncateRunes(firstLine, 120))
		} else if len(intents) == 0 {
			intents = append(intents, truncateRunes(firstLine, 120))
		}
	}
	return intents
}

func aggressiveSummaryFilePaths(messages []Message) []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, message := range messages {
		for _, token := range strings.Fields(message.Content) {
			filePath := aggressiveKnownFilePathToken(token)
			if filePath == "" || seen[filePath] {
				continue
			}
			seen[filePath] = true
			paths = append(paths, filePath)
			if len(paths) >= 20 {
				return paths
			}
		}
	}
	return paths
}

func aggressiveSummaryErrors(messages []Message) []string {
	out := []string{}
	for _, message := range messages {
		segments := regexp.MustCompile(`[.\n]`).Split(message.Content, -1)
		for _, segment := range segments {
			trimmed := strings.TrimSpace(segment)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			if strings.Contains(trimmed, "TypeError:") ||
				strings.Contains(trimmed, "ReferenceError:") ||
				strings.Contains(trimmed, "SyntaxError:") ||
				strings.Contains(trimmed, "RangeError:") ||
				strings.Contains(trimmed, "URIError:") ||
				strings.Contains(trimmed, "EvalError:") ||
				strings.Contains(trimmed, "Error:") ||
				strings.Contains(trimmed, "Exception:") ||
				strings.Contains(lower, "error ts") {
				out = append(out, truncateRunes(trimmed, 150))
				if len(out) >= 10 {
					return out
				}
			}
		}
	}
	return out
}

func aggressiveSummaryLastAssistant(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "assistant") && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i].Content
		}
	}
	return ""
}

func aggressiveKnownFilePathToken(token string) string {
	clean := strings.Trim(token, "'\"`()[]{},;:")
	lastDot := strings.LastIndex(clean, ".")
	if lastDot <= 0 || lastDot == len(clean)-1 {
		return ""
	}
	extension := strings.ToLower(clean[lastDot+1:])
	if !aggressiveFileExtensions[extension] {
		return ""
	}
	if !strings.Contains(clean, "/") && !strings.Contains(clean, ".") {
		return ""
	}
	return clean
}

func trimAggressiveCodeFences(text string) string {
	var builder strings.Builder
	cursor := 0
	for {
		startRel := strings.Index(text[cursor:], "```")
		if startRel == -1 {
			builder.WriteString(text[cursor:])
			return builder.String()
		}
		start := cursor + startRel
		builder.WriteString(text[cursor:start])
		openEndRel := strings.Index(text[start+3:], "\n")
		if openEndRel == -1 {
			builder.WriteString(text[start:])
			return builder.String()
		}
		openEnd := start + 3 + openEndRel
		closeStartRel := strings.Index(text[openEnd+1:], "\n```")
		if closeStartRel == -1 {
			builder.WriteString(text[start:])
			return builder.String()
		}
		closeStart := openEnd + 1 + closeStartRel
		closeEnd := closeStart + 4
		code := text[openEnd+1 : closeStart]
		lines := strings.Split(code, "\n")
		if len(lines) <= 4 {
			builder.WriteString(text[start:closeEnd])
			cursor = closeEnd
			continue
		}
		builder.WriteString(text[start:openEnd])
		builder.WriteString("\n")
		builder.WriteString(strings.Join(lines[:3], "\n"))
		builder.WriteString("\n...\n")
		builder.WriteString(lines[len(lines)-1])
		builder.WriteString("\n```")
		cursor = closeEnd
	}
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func pruneUltraByScore(text string, keepRate float64, minScore float64) string {
	if text == "" || keepRate >= 1 {
		return text
	}
	tokens := splitUltraTokens(text)
	type scoredToken struct {
		wordIndex int
		score     float64
	}
	scored := []scoredToken{}
	wordCount := 0
	for _, token := range tokens {
		if ultraWhitespacePattern.MatchString(token) && strings.TrimSpace(token) == "" {
			continue
		}
		scored = append(scored, scoredToken{wordIndex: wordCount, score: scoreUltraToken(token)})
		wordCount++
	}
	if wordCount == 0 {
		return text
	}
	targetKeep := intCeil(float64(wordCount) * keepRate)
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})
	toPrune := map[int]bool{}
	for _, item := range scored {
		if len(toPrune) >= wordCount-targetKeep {
			break
		}
		if item.score < minScore {
			toPrune[item.wordIndex] = true
		}
	}
	var builder strings.Builder
	wordIndex := 0
	for _, token := range tokens {
		if ultraWhitespacePattern.MatchString(token) && strings.TrimSpace(token) == "" {
			builder.WriteString(token)
			continue
		}
		if !toPrune[wordIndex] {
			builder.WriteString(token)
		}
		wordIndex++
	}
	return strings.TrimSpace(regexp.MustCompile(`\s{2,}`).ReplaceAllString(builder.String(), " "))
}

func splitUltraTokens(text string) []string {
	matches := ultraWhitespacePattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}
	tokens := []string{}
	cursor := 0
	for _, match := range matches {
		if match[0] > cursor {
			tokens = append(tokens, text[cursor:match[0]])
		}
		tokens = append(tokens, text[match[0]:match[1]])
		cursor = match[1]
	}
	if cursor < len(text) {
		tokens = append(tokens, text[cursor:])
	}
	return tokens
}

func scoreUltraToken(token string) float64 {
	if ultraForcePreservePattern.MatchString(token) {
		return 1.0
	}
	lower := strings.ToLower(token)
	if ultraStopwords[lower] {
		return 0.1
	}
	if len([]rune(token)) <= 2 {
		return 0.2
	}
	first := []rune(token)[0]
	if first >= 'A' && first <= 'Z' {
		return 0.8
	}
	if len([]rune(token)) >= 6 {
		return 0.7
	}
	return 0.5
}

func intCeil(value float64) int {
	asInt := int(value)
	if float64(asInt) == value {
		return asInt
	}
	return asInt + 1
}
