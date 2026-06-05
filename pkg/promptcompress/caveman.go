package promptcompress

import (
	"regexp"
	"strings"
)

type cavemanRule struct {
	name         string
	minIntensity Intensity
	pattern      *regexp.Regexp
	replacement  string
}

var cavemanRules = []cavemanRule{
	{name: "pleasantries", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:i would be happy to|i'd be happy to|happy to|glad to help|thank you|thanks|no problem|you're welcome|absolutely|certainly|of course|sure)\b[,.!?\s]*`), replacement: ""},
	{name: "polite_framing", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:please|kindly|could you please|would you please|can you please|i would like you to|i want you to|i need you to)\b\s*`), replacement: ""},
	{name: "hedging", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:it seems like|it appears that|i think that|i believe that|probably|possibly|maybe)\b\s*`), replacement: ""},
	{name: "verbose_instructions", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:provide a detailed explanation of|give me a comprehensive explanation of|write an in-depth explanation of|create a thorough explanation of|explain in detail)\b\s*`), replacement: "explain "},
	{name: "filler_adverbs", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:basically|essentially|actually|literally|simply|currently)\b\s*`), replacement: ""},
	{name: "filler_openers", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?im)^(?:hi there|hello|good morning|hey)\s*[,.!?\s]*`), replacement: ""},
	{name: "intent_clarification", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:what i'm trying to do is|my objective is to|what i need is|i'm aiming to)\b\s*`), replacement: "Goal: "},
	{name: "context_setup", minIntensity: IntensityLite, pattern: regexp.MustCompile(`(?i)\b(?:i have the following code|here is my code|below is the code)\b\s*[:.]?\s*`), replacement: "Code: "},
	{name: "redundant_because", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:due to the fact that|the reason is because)\b\s*`), replacement: "because "},
	{name: "redundant_directive", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:it is important to|you should|remember to)\b\s*`), replacement: ""},
	{name: "leader_phrases", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?im)^(?:i'll|i will|i can|i'd|let me|you can|we will|we can|let's)\s+([a-z])`), replacement: "$1"},
	{name: "verbose_connectors", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:furthermore|additionally|moreover|in addition)\b[,.]?\s*`), replacement: ""},
	{name: "purpose_phrases", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:in order to|so as to)\b\s*`), replacement: "to "},
	{name: "redundant_quantifiers", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:each and every|any and all)\b\s*`), replacement: "all "},
	{name: "emphasis_removal", minIntensity: IntensityStandard, pattern: regexp.MustCompile(`(?i)\b(?:very|really|extremely|highly|quite)\b\s*`), replacement: ""},
	{name: "article_removal", minIntensity: IntensityAggressive, pattern: regexp.MustCompile(`\b(?:[Aa]n|[Aa]|[Tt]he)\s+([a-z])`), replacement: "$1"},
	{name: "ultra_database", minIntensity: IntensityUltra, pattern: regexp.MustCompile(`(?i)\bdatabase\b`), replacement: "db"},
	{name: "ultra_configuration", minIntensity: IntensityUltra, pattern: regexp.MustCompile(`(?i)\bconfiguration\b`), replacement: "config"},
	{name: "ultra_implementation", minIntensity: IntensityUltra, pattern: regexp.MustCompile(`(?i)\bimplementation\b`), replacement: "impl"},
	{name: "ultra_authentication", minIntensity: IntensityUltra, pattern: regexp.MustCompile(`(?i)\bauthentication\b`), replacement: "authn"},
	{name: "ultra_authorization", minIntensity: IntensityUltra, pattern: regexp.MustCompile(`(?i)\bauthorization\b`), replacement: "authz"},
}

func applyCaveman(text string, intensity Intensity) (string, []string) {
	result := text
	applied := make([]string, 0)
	for _, rule := range cavemanRules {
		if !intensityAllows(intensity, rule.minIntensity) {
			continue
		}
		before := result
		result = rule.pattern.ReplaceAllString(result, rule.replacement)
		if result != before {
			applied = append(applied, "caveman:"+rule.name)
		}
	}
	result = cleanupCavemanArtifacts(result)
	return result, applied
}

func intensityAllows(active Intensity, required Intensity) bool {
	return intensityRank(active) >= intensityRank(required)
}

func intensityRank(value Intensity) int {
	switch value {
	case IntensityLite:
		return 1
	case IntensityStandard:
		return 2
	case IntensityAggressive:
		return 3
	case IntensityUltra:
		return 4
	default:
		return 2
	}
}

func cleanupCavemanArtifacts(text string) string {
	result := text
	result = regexp.MustCompile(`[ \t]{2,}`).ReplaceAllString(result, " ")
	result = regexp.MustCompile(`[ \t]+([,.;:!?])`).ReplaceAllString(result, "$1")
	result = regexp.MustCompile(`([.!?]){2,}`).ReplaceAllString(result, "$1")
	result = regexp.MustCompile(`(?m)[ \t]+$`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}
