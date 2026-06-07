package promptcompress

import (
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/dlclark/regexp2"
)

type cavemanRule struct {
	Name           string
	Context        string
	Category       string
	MinIntensity   string
	Replacement    string
	ReplacementMap map[string]string
	Pattern        string
	Flags          string
	re             *regexp2.Regexp
}

type cavemanRulePack struct {
	Language string             `json:"language"`
	Category string             `json:"category"`
	Rules    []cavemanRuleEntry `json:"rules"`
}

type cavemanRuleEntry struct {
	Name           string            `json:"name"`
	Pattern        string            `json:"pattern"`
	Replacement    string            `json:"replacement"`
	ReplacementMap map[string]string `json:"replacementMap"`
	Flags          string            `json:"flags"`
	Context        string            `json:"context"`
	Category       string            `json:"category"`
	MinIntensity   string            `json:"minIntensity"`
	Description    string            `json:"description"`
}

var (
	cavemanRulesMu    sync.Mutex
	cavemanRulesCache = map[string][]cavemanRule{}
	cavemanRulesErrs  = map[string]error{}
)

var cavemanRuleKeywords = map[string][]string{
	"redundant_phrasing":                {"make sure", "be sure"},
	"redundant_because":                 {"due to the fact", "the reason is because"},
	"redundant_directive":               {"it is important", "you should", "remember to"},
	"pleasantries":                      {"sure", "certainly", "of course", "happy to", "thanks", "thank you", "glad to help", "glad to", "no problem", "you're welcome", "youre welcome", "absolutely"},
	"polite_framing":                    {"please", "kindly", "could you please", "would you please", "can you please", "i would like you", "i want you", "i need you"},
	"hedging":                           {"it seems like", "it appears that", "i think that", "i believe that", "probably", "possibly", "maybe it"},
	"verbose_instructions":              {"provide a detailed", "give me a comprehensive", "write an in-depth", "create a thorough", "explain in detail"},
	"filler_adverbs":                    {"basically", "essentially", "actually", "literally", "simply", "currently"},
	"filler_phrases":                    {"i want to", "i need to", "i'd like to", "i'm looking for"},
	"redundant_openers":                 {"hi there", "hello", "good morning", "hey"},
	"verbose_requests":                  {"i was wondering", "would it be possible"},
	"leader_phrases":                    {"i'll", "i will", "i can", "i'd", "let me", "you can", "we will", "we can", "let's"},
	"self_reference":                    {"i am trying to", "i am working on", "i have been"},
	"excessive_gratitude":               {"thank you so much", "thanks in advance", "i really appreciate"},
	"qualifier_removal":                 {"a bit", "a little", "somewhat", "kind of", "sort of"},
	"softeners":                         {"if possible", "when you get a chance", "at your convenience", "just wondering"},
	"uncertainty_fillers":               {"i guess", "i suppose", "more or less", "in a way"},
	"assistant_fillers":                 {"here's", "below is", "this is"},
	"compound_collapse":                 {"and any potential"},
	"explanatory_prefix":                {"the function appears to be handling", "the code seems to", "the class is", "this module is"},
	"question_to_directive":             {"can you explain why", "could you show me how", "would you tell me", "can you tell me"},
	"context_setup":                     {"i have the following code", "here is my code", "below is the code"},
	"intent_clarification":              {"what i'm trying to do", "my objective is to", "what i need is", "i'm aiming to"},
	"background_removal":                {"as you may know", "as we discussed earlier"},
	"meta_commentary":                   {"note that", "keep in mind", "remember that"},
	"purpose_statement":                 {"for the purpose of", "with the goal of", "in an effort to", "for every"},
	"list_conjunction":                  {"and also", "as well as"},
	"purpose_phrases":                   {"in order to", "so as to"},
	"redundant_quantifiers":             {"each and every", "any and all"},
	"all_quantifier":                    {"any and all"},
	"verbose_connectors":                {"furthermore", "additionally", "moreover", "in addition"},
	"transition_removal":                {"on the other hand", "in contrast", "however"},
	"emphasis_removal":                  {"very", "really", "extremely", "highly", "quite"},
	"passive_voice":                     {"is being used", "is being called", "is being generated", "was created", "was generated", "was implemented"},
	"repeated_context":                  {"as we discussed earlier", "as mentioned before", "as previously stated", "as i said before"},
	"repeated_question":                 {"same question as before", "i asked this earlier", "this is the same question"},
	"reestablished_context":             {"going back to the code above", "referring back to", "returning to"},
	"summary_replacement":               {"to summarize", "in summary of our conversation", "to recap"},
	"ultra_abbreviations":               {"database"},
	"ultra_config_abbreviation":         {"configuration"},
	"ultra_function_abbreviation":       {"function"},
	"ultra_request_abbreviation":        {"request"},
	"ultra_response_abbreviation":       {"response"},
	"ultra_implementation_abbreviation": {"implementation"},
	"ultra_authentication_abbreviation": {"authentication"},
	"ultra_authorization_abbreviation":  {"authorization"},
	"ultra_application_abbreviation":    {"application"},
	"ultra_dependency_abbreviation":     {"dependency", "dependencies"},
	"ultra_common_abbreviations":        {"implementation", "authentication", "authorization", "application", "dependency", "dependencies"},
}

var cavemanArticleHintPattern = regexp.MustCompile(`\b(?:a|an|the)\b`)

func applyCaveman(text string, intensity Intensity) (string, []string) {
	return applyCavemanForRole(text, intensity, "user")
}

func applyCavemanForRole(text string, intensity Intensity, role string) (string, []string) {
	return applyCavemanWithConfig(text, intensity, role, DefaultConfig())
}

func applyCavemanWithConfig(text string, intensity Intensity, role string, config Config) (string, []string) {
	language := selectCavemanLanguage(text, config)
	rules, err := loadCavemanRulesForLanguage(language)
	if err != nil || len(rules) == 0 {
		rules, err = loadCavemanRules()
	}
	if err != nil {
		return cleanupCavemanArtifacts(text), []string{"caveman:load_error"}
	}
	result := text
	lowerText := strings.ToLower(text)
	applied := make([]string, 0)
	role = normalizeCavemanRole(role)
	skipRules := stringSet(config.CavemanSkipRules)
	for _, rule := range rules {
		if skipRules[rule.Name] {
			continue
		}
		if !cavemanRuleMatches(rule, intensity, role) {
			continue
		}
		if !shouldAttemptCavemanRule(rule.Name, lowerText) {
			continue
		}
		before := result
		next, err := applyCavemanRule(result, rule)
		if err != nil {
			continue
		}
		result = next
		if result != before {
			applied = append(applied, "caveman:"+rule.Name)
		}
	}
	result = recapitalizeCavemanSentences(cleanupCavemanArtifacts(result))
	return result, applied
}

func loadCavemanRules() ([]cavemanRule, error) {
	return loadCavemanRulesForLanguage("en")
}

func loadCavemanRulesForLanguage(language string) ([]cavemanRule, error) {
	language = strings.TrimSpace(language)
	if language == "" {
		language = "en"
	}
	cavemanRulesMu.Lock()
	if rules, ok := cavemanRulesCache[language]; ok {
		err := cavemanRulesErrs[language]
		cavemanRulesMu.Unlock()
		return rules, err
	}
	cavemanRulesMu.Unlock()

	rules, err := readCavemanRules(language)

	cavemanRulesMu.Lock()
	cavemanRulesCache[language] = rules
	cavemanRulesErrs[language] = err
	cavemanRulesMu.Unlock()
	return rules, err
}

func readCavemanRules(language string) ([]cavemanRule, error) {
	const root = "omniroute/caveman_rules"
	dir := path.Join(root, language)
	files, err := fs.ReadDir(omniRouteFS, dir)
	if err != nil {
		return []cavemanRule{}, nil
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			names = append(names, file.Name())
		}
	}
	sort.Strings(names)
	var rules []cavemanRule
	for _, name := range names {
		data, err := omniRouteFS.ReadFile(path.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var pack cavemanRulePack
		if err := common.Unmarshal(data, &pack); err != nil {
			return nil, err
		}
		for _, entry := range pack.Rules {
			flags := entry.Flags
			if flags == "" {
				flags = "gi"
			}
			compiled, err := regexp2.Compile(entry.Pattern, cavemanRegexOptions(flags))
			if err != nil {
				return nil, err
			}
			compiled.MatchTimeout = 50 * time.Millisecond
			context := entry.Context
			if context == "" {
				context = "all"
			}
			minIntensity := entry.MinIntensity
			if minIntensity == "" {
				minIntensity = "lite"
			}
			category := entry.Category
			if category == "" {
				category = pack.Category
			}
			rules = append(rules, cavemanRule{
				Name:           entry.Name,
				Context:        context,
				Category:       category,
				MinIntensity:   minIntensity,
				Replacement:    entry.Replacement,
				ReplacementMap: entry.ReplacementMap,
				Pattern:        entry.Pattern,
				Flags:          flags,
				re:             compiled,
			})
		}
	}
	return rules, nil
}

func selectCavemanLanguage(text string, config Config) string {
	language := strings.TrimSpace(config.CavemanLanguage)
	if language == "" {
		language = "en"
	}
	if config.CavemanAutoDetectLanguage {
		language = detectCavemanLanguage(text)
	}
	enabledPacks := config.CavemanEnabledLanguagePacks
	if len(enabledPacks) == 0 {
		enabledPacks = []string{"en", language}
	}
	if containsString(enabledPacks, language) {
		return language
	}
	if containsString(enabledPacks, "en") {
		return "en"
	}
	return language
}

func detectCavemanLanguage(text string) string {
	for _, item := range []struct {
		language string
		pattern  *regexp.Regexp
	}{
		{"pt-BR", regexp.MustCompile(`(?i)\b(?:voce|você|preciso|arquivo|codigo|código|erro|falha|obrigado)\b`)},
		{"es", regexp.MustCompile(`(?i)\b(?:necesito|archivo|codigo|código|error|fallo|gracias|puedes)\b`)},
		{"de", regexp.MustCompile(`(?i)\b(?:ich|datei|fehler|bitte|kannst|konfiguration|danke)\b`)},
		{"fr", regexp.MustCompile(`(?i)\b(?:fichier|erreur|merci|peux|configuration|besoin)\b`)},
		{"ja", regexp.MustCompile(`[\x{3040}-\x{30ff}]`)},
	} {
		if item.pattern.MatchString(text) {
			return item.language
		}
	}
	return "en"
}

func applyCavemanRule(text string, rule cavemanRule) (string, error) {
	if len(rule.ReplacementMap) == 0 {
		return rule.re.Replace(text, rule.Replacement, -1, -1)
	}
	return rule.re.ReplaceFunc(text, func(match regexp2.Match) string {
		normalized := normalizeReplacementKey(match.String())
		if replacement, ok := rule.ReplacementMap[normalized]; ok {
			return replacement
		}
		if rule.Replacement != "" {
			return rule.Replacement
		}
		return match.String()
	}, -1, -1)
}

func cavemanRuleMatches(rule cavemanRule, active Intensity, role string) bool {
	if rule.Context != "all" && rule.Context != role {
		return false
	}
	return cavemanIntensityRank(string(active)) >= cavemanIntensityRank(rule.MinIntensity)
}

func shouldAttemptCavemanRule(ruleName string, lowerText string) bool {
	if ruleName == "articles" {
		return cavemanArticleHintPattern.MatchString(lowerText)
	}
	keywords, ok := cavemanRuleKeywords[ruleName]
	if !ok {
		return true
	}
	for _, keyword := range keywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}
	return false
}

func cavemanRegexOptions(flags string) regexp2.RegexOptions {
	var options regexp2.RegexOptions = regexp2.ECMAScript
	if strings.Contains(flags, "i") {
		options |= regexp2.IgnoreCase
	}
	if strings.Contains(flags, "m") {
		options |= regexp2.Multiline
	}
	if strings.Contains(flags, "s") {
		options |= regexp2.Singleline
	}
	return options
}

func normalizeReplacementKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func normalizeCavemanRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "system", "assistant", "user":
		return role
	default:
		return "user"
	}
}

func intensityAllows(active Intensity, required Intensity) bool {
	return cavemanIntensityRank(string(active)) >= cavemanIntensityRank(string(required))
}

func intensityRank(value Intensity) int {
	return cavemanIntensityRank(string(value))
}

func cavemanIntensityRank(value string) int {
	switch value {
	case "lite":
		return 1
	case "standard", "full", "aggressive":
		return 2
	case "ultra":
		return 3
	default:
		return 2
	}
}

func cleanupCavemanArtifacts(text string) string {
	result := text
	if hasRepeatedHorizontalWhitespace(result) {
		result = collapseHorizontalWhitespaceRuns(result)
	}
	result = removeHorizontalWhitespaceBeforePunctuation(result)
	result = collapseRepeatedSentencePunctuation(result)
	if strings.Contains(result, " \n") || strings.Contains(result, "\t\n") {
		result = stripLineTrailingHorizontalWhitespace(result)
	}
	if strings.HasSuffix(result, " ") || strings.HasSuffix(result, "\t") {
		result = trimEndHorizontalWhitespace(result)
	}
	if strings.Contains(result, "\n\n\n") {
		result = collapseExcessNewlines(result)
	}
	if strings.HasPrefix(result, "\n") {
		result = trimLeadingNewlines(result)
	}
	if strings.HasSuffix(result, "\n") {
		result = trimTrailingNewlines(result)
	}
	return result
}

func recapitalizeCavemanSentences(text string) string {
	return regexp.MustCompile(`(^|[.!?][ \t]|\n[ \t]*)([a-z])`).ReplaceAllStringFunc(text, func(match string) string {
		runes := []rune(match)
		if len(runes) == 0 {
			return match
		}
		runes[len(runes)-1] = []rune(strings.ToUpper(string(runes[len(runes)-1])))[0]
		return string(runes)
	})
}

func isHorizontalWhitespace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isSentencePunctuation(r rune) bool {
	return r == '.' || r == '!' || r == '?'
}

func isCleanupPunctuation(r rune) bool {
	switch r {
	case ',', '.', ';', ':', '!', '?':
		return true
	default:
		return false
	}
}

func hasRepeatedHorizontalWhitespace(text string) bool {
	previousWasWhitespace := false
	for _, r := range text {
		currentIsWhitespace := isHorizontalWhitespace(r)
		if currentIsWhitespace && previousWasWhitespace {
			return true
		}
		previousWasWhitespace = currentIsWhitespace
	}
	return false
}

func collapseHorizontalWhitespaceRuns(text string) string {
	runes := []rune(text)
	var output strings.Builder
	changed := false
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if !isHorizontalWhitespace(r) {
			output.WriteRune(r)
			continue
		}
		start := index
		for index+1 < len(runes) && isHorizontalWhitespace(runes[index+1]) {
			index++
		}
		if index > start {
			output.WriteRune(' ')
			changed = true
		} else {
			output.WriteRune(r)
		}
	}
	if !changed {
		return text
	}
	return output.String()
}

func removeHorizontalWhitespaceBeforePunctuation(text string) string {
	runes := []rune(text)
	var output strings.Builder
	changed := false
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if !isHorizontalWhitespace(r) {
			output.WriteRune(r)
			continue
		}
		start := index
		for index+1 < len(runes) && isHorizontalWhitespace(runes[index+1]) {
			index++
		}
		if index+1 < len(runes) && isCleanupPunctuation(runes[index+1]) {
			changed = true
			continue
		}
		output.WriteString(string(runes[start : index+1]))
	}
	if !changed {
		return text
	}
	return output.String()
}

func collapseRepeatedSentencePunctuation(text string) string {
	runes := []rune(text)
	var output strings.Builder
	changed := false
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if !isSentencePunctuation(r) {
			output.WriteRune(r)
			continue
		}
		lastPunctuation := r
		start := index
		for index+1 < len(runes) && isSentencePunctuation(runes[index+1]) {
			index++
			lastPunctuation = runes[index]
		}
		if index > start {
			changed = true
		}
		output.WriteRune(lastPunctuation)
	}
	if !changed {
		return text
	}
	return output.String()
}

func trimEndHorizontalWhitespace(text string) string {
	runes := []rune(text)
	end := len(runes)
	for end > 0 && isHorizontalWhitespace(runes[end-1]) {
		end--
	}
	if end == len(runes) {
		return text
	}
	return string(runes[:end])
}

func stripLineTrailingHorizontalWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	changed := false
	for index, line := range lines {
		cleaned := trimEndHorizontalWhitespace(line)
		if cleaned != line {
			changed = true
			lines[index] = cleaned
		}
	}
	if !changed {
		return text
	}
	return strings.Join(lines, "\n")
}

func collapseExcessNewlines(text string) string {
	runes := []rune(text)
	var output strings.Builder
	changed := false
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if r != '\n' {
			output.WriteRune(r)
			continue
		}
		start := index
		for index+1 < len(runes) && runes[index+1] == '\n' {
			index++
		}
		newlineCount := index - start + 1
		if newlineCount > 2 {
			output.WriteString("\n\n")
			changed = true
		} else {
			output.WriteString(string(runes[start : index+1]))
		}
	}
	if !changed {
		return text
	}
	return output.String()
}

func trimLeadingNewlines(text string) string {
	runes := []rune(text)
	start := 0
	for start < len(runes) && runes[start] == '\n' {
		start++
	}
	if start == 0 {
		return text
	}
	return string(runes[start:])
}

func trimTrailingNewlines(text string) string {
	runes := []rune(text)
	end := len(runes)
	for end > 0 && runes[end-1] == '\n' {
		end--
	}
	if end == len(runes) {
		return text
	}
	return string(runes[:end])
}
