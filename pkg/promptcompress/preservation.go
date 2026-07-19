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
	Text           string
	ValidationText string
	Blocks         []preservedBlock
	RedactedCount  int
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

var preInlineMathPreservePatterns = []preservePattern{
	{kind: "math_block", pattern: regexp.MustCompile(`(?s)\$\$.*?\$\$`), mustKeep: true},
	{kind: "math_block", pattern: regexp.MustCompile(`(?s)\\\[.*?\\\]`), mustKeep: true},
}

var postInlineMathPreservePatterns = []preservePattern{
	{kind: "latex_block", pattern: regexp.MustCompile(`(?s)\\begin\{[A-Za-z*]+\}.*?\\end\{[A-Za-z*]+\}`), mustKeep: true},
	{kind: "markdown_heading", pattern: regexp.MustCompile(`(?m)^#{1,6}\s+.+$`), mustKeep: true},
	{kind: "markdown_table", pattern: regexp.MustCompile(`(?m)^\s*\|.*\|\s*$`), mustKeep: true},
	{kind: "markdown_table", pattern: regexp.MustCompile(`(?m)^\s*\|?\s*:?-{3,}:?\s*(?:\|\s*:?-{3,}:?\s*)+\|?\s*$`), mustKeep: true},
	{kind: "typst_directive", pattern: regexp.MustCompile(`(?m)^\s*#(?:set|show|let|import|include)\b.+$`), mustKeep: true},
	{kind: "inline_code", pattern: regexp.MustCompile("`[^`\n]+`"), mustKeep: true},
	{kind: "markdown_link", pattern: regexp.MustCompile(`\[[^\]\n]+\]\([^) \n]+(?:\s+"[^"]*")?\)`), mustKeep: true},
	{kind: "url", pattern: regexp.MustCompile(`(?i)\bhttps?://[^\s)\]"'>]+`), mustKeep: true},
	{kind: "const_case", pattern: regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`), mustKeep: true},
	{kind: "env_var", pattern: regexp.MustCompile(`\bprocess\.env\.[A-Za-z_][A-Za-z0-9_]*\b`), mustKeep: true},
	{kind: "env_var", pattern: regexp.MustCompile(`\$[A-Z_][A-Z0-9_]*\b`), mustKeep: true},
	{kind: "version", pattern: regexp.MustCompile(`\b\d+(?:\.\d+){1,3}(?:[-+][A-Za-z0-9.-]+)?\b`), mustKeep: true},
	{kind: "dotted_identifier", pattern: regexp.MustCompile(`\b[a-zA-Z_$][\w$]*(?:\.[a-zA-Z_$][\w$]*)+\(\)?`), mustKeep: true},
	{kind: "function_call", pattern: regexp.MustCompile(`\b[A-Za-z_$][\w$]*\s*\([^()\n]*\)`), mustKeep: true},
	{kind: "json_line", pattern: regexp.MustCompile(`(?m)^\s*(?:[{}\[\],]|"[^"\n]+"\s*:).*$`), mustKeep: true},
	{kind: "xml_line", pattern: regexp.MustCompile(`(?m)^\s*</?[A-Za-z][^>\n]*>.*$`), mustKeep: true},
	{kind: "file_path", pattern: regexp.MustCompile(`(?:^|\s)(?:\.{0,2}/[A-Za-z0-9_@./-]+|[A-Za-z]:\\[A-Za-z0-9_.\\/-]+)`), mustKeep: true},
	{kind: "error_code", pattern: regexp.MustCompile(`\b(?:TS\d{4}|[A-Z][A-Z0-9_]+(?:Error|Exception|Denied|Timeout)|HTTP\s+[45]\d\d)\b`), mustKeep: true},
	{kind: "error_message", pattern: regexp.MustCompile(`\b(?:TypeError|ReferenceError|SyntaxError|RangeError|URIError|EvalError|Error|Exception):[^\n]+`), mustKeep: true},
	{kind: "command", pattern: regexp.MustCompile(`(?m)^\s*(?:\$|>)\s*(?:git|go|npm|pnpm|yarn|docker|kubectl|terraform|tofu|pytest|python|cargo|make|curl|wget)\b.*$`), mustKeep: true},
}

var (
	fencedCodeOpenPattern  = regexp.MustCompile("^([ \\t]{0,3})(`{3,}|~{3,})[^\\n]*(?:\\n|$)")
	fencedCodeClosePattern = regexp.MustCompile("^([ \\t]{0,3})(`{3,}|~{3,})\\s*(?:\\n|$)")
)

func protect(input string) protectedText {
	return protectWithPatterns(input, nil)
}

func protectWithPatterns(input string, patterns []string) protectedText {
	redacted, redactedCount := redactSecrets(input)
	state := protectedText{Text: redacted, ValidationText: redacted, RedactedCount: redactedCount}
	state.Text = extractFrontmatterForProtect(state.Text, &state.Blocks)
	state.Text = extractFencedCodeForProtect(state.Text, &state.Blocks)
	for _, item := range preInlineMathPreservePatterns {
		state.Text = replaceMatches(state.Text, item, &state.Blocks)
	}
	state.Text = replaceCollectedMatches(state.Text, collectInlineMath(state.Text), "math_inline", &state.Blocks)
	for _, item := range postInlineMathPreservePatterns {
		state.Text = replaceMatches(state.Text, item, &state.Blocks)
	}
	for _, item := range compileCustomPreservePatterns(patterns) {
		state.Text = replaceMatches(state.Text, item, &state.Blocks)
	}
	return state
}

func compileCustomPreservePatterns(patterns []string) []preservePattern {
	out := make([]preservePattern, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		out = append(out, preservePattern{kind: "custom", pattern: compiled, mustKeep: true})
	}
	return out
}

func restore(input string, blocks []preservedBlock) string {
	result := input
	for _, block := range blocks {
		result = strings.ReplaceAll(result, block.Placeholder, block.Content)
	}
	return result
}

func verifyPreserved(original string, output string, blocks []preservedBlock) bool {
	for _, block := range blocks {
		if block.MustKeep && !strings.Contains(output, block.Content) {
			return false
		}
	}
	return validateCompression(original, output).Valid
}

func replaceMatches(input string, item preservePattern, blocks *[]preservedBlock) string {
	return item.pattern.ReplaceAllStringFunc(input, func(match string) string {
		if match == "" || strings.Contains(match, placeholderPrefix) || strings.Contains(match, placeholderToken) {
			return match
		}
		return addPreservedBlock(match, item.kind, item.mustKeep, blocks)
	})
}

func replaceCollectedMatches(input string, matches []string, kind string, blocks *[]preservedBlock) string {
	result := input
	for _, match := range matches {
		if match == "" || strings.Contains(match, placeholderPrefix) || strings.Contains(match, placeholderToken) {
			continue
		}
		placeholder := addPreservedBlock(match, kind, true, blocks)
		result = strings.Replace(result, match, placeholder, 1)
	}
	return result
}

func addPreservedBlock(content string, kind string, mustKeep bool, blocks *[]preservedBlock) string {
	placeholder := fmt.Sprintf("%s%d\x00", placeholderPrefix, len(*blocks))
	*blocks = append(*blocks, preservedBlock{
		Placeholder: placeholder,
		Content:     content,
		Kind:        kind,
		MustKeep:    mustKeep,
	})
	return placeholder
}

func extractFrontmatterForProtect(input string, blocks *[]preservedBlock) string {
	if !strings.HasPrefix(input, "---\n") {
		return input
	}
	closeOffset := strings.Index(input[4:], "\n---")
	if closeOffset == -1 {
		return input
	}
	closeIndex := closeOffset + 4
	closeEndOffset := strings.Index(input[closeIndex+4:], "\n")
	end := len(input)
	if closeEndOffset != -1 {
		end = closeIndex + 4 + closeEndOffset + 1
	}
	return addPreservedBlock(input[:end], "frontmatter", true, blocks) + input[end:]
}

func extractFencedCodeForProtect(input string, blocks *[]preservedBlock) string {
	lines := splitLinesKeepingEndings(input)
	var output strings.Builder
	for i := 0; i < len(lines); {
		line := lines[i]
		if line == "" && i == len(lines)-1 {
			break
		}
		opening := fencedCodeOpenPattern.FindStringSubmatch(line)
		if opening == nil {
			output.WriteString(line)
			i++
			continue
		}
		fence := opening[2]
		fenceChar := fence[0]
		minLen := len(fence)
		block := line
		closed := false
		j := i + 1
		for ; j < len(lines); j++ {
			candidate := lines[j]
			block += candidate
			closing := fencedCodeClosePattern.FindStringSubmatch(candidate)
			if closing != nil && closing[2][0] == fenceChar && len(closing[2]) >= minLen {
				closed = true
				break
			}
		}
		if !closed {
			output.WriteString(line)
			i++
			continue
		}
		output.WriteString(addPreservedBlock(block, "fenced_code", true, blocks))
		i = j + 1
	}
	return output.String()
}

func splitLinesKeepingEndings(input string) []string {
	if input == "" {
		return []string{""}
	}
	lines := make([]string, 0, strings.Count(input, "\n")+1)
	start := 0
	for index := 0; index < len(input); index++ {
		if input[index] != '\n' {
			continue
		}
		lines = append(lines, input[start:index+1])
		start = index + 1
	}
	if start < len(input) {
		lines = append(lines, input[start:])
	} else {
		lines = append(lines, "")
	}
	return lines
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
