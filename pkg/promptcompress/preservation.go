package promptcompress

import (
	"fmt"
	"regexp"
	"strings"
)

const placeholderPrefix = "\x00GLART_PROMPT_PRESERVE_"
const placeholderToken = "GLART_PROMPT_PRESERVE"

type preservedBlock struct {
	Placeholder string
	Content     string
	Kind        string
	MustKeep    bool
}

type protectedText struct {
	Text          string
	Blocks        []preservedBlock
	RedactedCount int
}

type preservePattern struct {
	kind     string
	pattern  *regexp.Regexp
	mustKeep bool
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|token|secret|password)\s*[:=]\s*['"]?[^\s'",;]{8,}`),
	regexp.MustCompile(`(?i)\b(?:sk-[A-Za-z0-9_-]{16,}|AIza[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}|AKIA[0-9A-Z]{16})\b`),
}

var builtInPreservePatterns = []preservePattern{
	{kind: "fenced_code", pattern: regexp.MustCompile("(?s)```.*?```"), mustKeep: true},
	{kind: "inline_code", pattern: regexp.MustCompile("`[^`\n]+`"), mustKeep: true},
	{kind: "url", pattern: regexp.MustCompile(`(?i)\bhttps?://[^\s)\]"'>]+`), mustKeep: true},
	{kind: "markdown_link", pattern: regexp.MustCompile(`\[[^\]\n]+\]\([^) \n]+(?:\s+"[^"]*")?\)`), mustKeep: true},
	{kind: "json_line", pattern: regexp.MustCompile(`(?m)^\s*(?:[{}\[\],]|"[^"\n]+"\s*:).*$`), mustKeep: true},
	{kind: "xml_line", pattern: regexp.MustCompile(`(?m)^\s*</?[A-Za-z][^>\n]*>.*$`), mustKeep: true},
	{kind: "env_var", pattern: regexp.MustCompile(`\b(?:process\.env\.[A-Za-z_][A-Za-z0-9_]*|\$[A-Z_][A-Z0-9_]*|[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+)\b`), mustKeep: true},
	{kind: "file_path", pattern: regexp.MustCompile(`(?:^|\s)(?:\.{0,2}/[A-Za-z0-9_@./-]+|[A-Za-z]:\\[A-Za-z0-9_.\\/-]+)`), mustKeep: true},
	{kind: "error_code", pattern: regexp.MustCompile(`\b(?:TS\d{4}|[A-Z][A-Z0-9_]+(?:Error|Exception|Denied|Timeout)|HTTP\s+[45]\d\d)\b`), mustKeep: true},
	{kind: "command", pattern: regexp.MustCompile(`(?m)^\s*(?:\$|>)\s*(?:git|go|npm|pnpm|yarn|docker|kubectl|terraform|tofu|pytest|python|cargo|make|curl|wget)\b.*$`), mustKeep: true},
}

func protect(input string) protectedText {
	redacted, redactedCount := redactSecrets(input)
	state := protectedText{Text: redacted, RedactedCount: redactedCount}
	for _, item := range builtInPreservePatterns {
		state.Text = replaceMatches(state.Text, item, &state.Blocks)
	}
	return state
}

func restore(input string, blocks []preservedBlock) string {
	result := input
	for _, block := range blocks {
		result = strings.ReplaceAll(result, block.Placeholder, block.Content)
	}
	return result
}

func verifyPreserved(output string, blocks []preservedBlock) bool {
	for _, block := range blocks {
		if block.MustKeep && !strings.Contains(output, block.Content) {
			return false
		}
	}
	return true
}

func replaceMatches(input string, item preservePattern, blocks *[]preservedBlock) string {
	return item.pattern.ReplaceAllStringFunc(input, func(match string) string {
		if match == "" || strings.Contains(match, placeholderPrefix) || strings.Contains(match, placeholderToken) {
			return match
		}
		placeholder := fmt.Sprintf("%s%d\x00", placeholderPrefix, len(*blocks))
		*blocks = append(*blocks, preservedBlock{
			Placeholder: placeholder,
			Content:     match,
			Kind:        item.kind,
			MustKeep:    item.mustKeep,
		})
		return placeholder
	})
}

func redactSecrets(input string) (string, int) {
	result := input
	count := 0
	for _, pattern := range secretPatterns {
		matches := pattern.FindAllStringIndex(result, -1)
		count += len(matches)
		result = pattern.ReplaceAllString(result, "[redacted_secret]")
	}
	return result, count
}
