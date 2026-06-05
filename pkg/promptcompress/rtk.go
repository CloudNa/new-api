package promptcompress

import (
	"fmt"
	"regexp"
	"strings"
)

type rtkFilter struct {
	id               string
	commandPatterns  []*regexp.Regexp
	outputPatterns   []*regexp.Regexp
	stripPatterns    []*regexp.Regexp
	keepPatterns     []*regexp.Regexp
	collapsePatterns []*regexp.Regexp
	matchOutput      []rtkMatchOutput
	onEmpty          string
	maxLines         int
	headLines        int
	tailLines        int
}

type rtkMatchOutput struct {
	pattern *regexp.Regexp
	unless  *regexp.Regexp
	message string
}

var rtkFilters = []rtkFilter{
	{
		id:              "git-diff",
		commandPatterns: regexps(`(?i)^git\s+(?:diff|show)\b`),
		outputPatterns:  regexps(`(?m)^diff --git `, `(?m)^@@\s+-\d+`),
		keepPatterns:    regexps(`(?m)^(?:diff --git|@@|[+-])`),
		stripPatterns:   regexps(`(?m)^\s*$`, `(?m)^index [0-9a-f]`),
		maxLines:        160,
		headLines:       40,
		tailLines:       80,
	},
	{
		id:              "git-status",
		commandPatterns: regexps(`(?i)^git\s+status\b`),
		outputPatterns:  regexps(`(?m)^On branch `, `(?m)^Changes (?:not staged|to be committed)`, `(?m)^Untracked files:`),
		stripPatterns:   regexps(`(?m)^\s*$`, `(?m)^\s+\(use "git .*"\)`),
		maxLines:        100,
		headLines:       35,
		tailLines:       45,
	},
	{
		id:              "go-test",
		commandPatterns: regexps(`(?i)^go\s+test\b`),
		outputPatterns:  regexps(`(?m)^(?:ok|FAIL)\s+[\w./-]+`, `(?m)^--- FAIL: `, `(?m)^panic: `),
		keepPatterns:    regexps(`(?m)^(?:ok|FAIL)\s+`, `(?m)^--- FAIL: `, `(?m)^\s+.*_test\.go:\d+`, `(?m)^panic: `, `(?i)error|failed|exception`),
		stripPatterns:   regexps(`(?m)^\s*$`),
		maxLines:        140,
		headLines:       30,
		tailLines:       70,
	},
	{
		id:              "typescript",
		commandPatterns: regexps(`(?i)^(?:tsc|npm\s+run\s+typecheck)\b`),
		outputPatterns:  regexps(`TS\d{4}:`, `(?i)error TS\d{4}`),
		keepPatterns:    regexps(`TS\d{4}:`, `(?i)error TS\d{4}`, `(?m)^Found \d+ errors?`),
		stripPatterns:   regexps(`(?m)^\s*$`),
		maxLines:        120,
		headLines:       20,
		tailLines:       60,
	},
	{
		id:              "docker-build",
		commandPatterns: regexps(`(?i)^docker\s+(?:build|buildx\s+build)\b`, `(?i)^(?:docker\s+compose|docker-compose)\s+build\b`),
		outputPatterns:  regexps(`Successfully built \w+`, `Successfully tagged \S+`, `ERROR \[`, `exit code: \d+`),
		keepPatterns:    regexps(`ERROR`, `ERR!`, `exit code: \d+`, `failed to solve`, `Successfully built`, `Successfully tagged`, `writing image sha256:`, `DONE`, `^Step \d+/\d+ :`, `WARN\s*\[`),
		stripPatterns:   regexps(`(?m)^#\d+\s+sha256:`, `(?m)^\s*$`, `(?m)^\s+\d+\.\d+s$`, `(?m)^\s*-+>\s*Running in`, `(?m)^\s*Removing intermediate container`),
		maxLines:        100,
		headLines:       15,
		tailLines:       50,
	},
	{
		id:               "docker-logs",
		commandPatterns:  regexps(`(?i)^docker\s+(?:compose\s+)?logs\b`),
		outputPatterns:   regexps(`\b(?:ERROR|WARN|INFO)\b`, `(?m)^Attaching to `),
		keepPatterns:     regexps(`ERROR`, `WARN`, `Exception`, `Traceback`, `failed`, `listening`, `started`),
		stripPatterns:    regexps(`(?m)^Attaching to `, `(?m)^\s*$`),
		collapsePatterns: regexps(`INFO`),
		maxLines:         120,
		headLines:        20,
		tailLines:        60,
	},
	{
		id:              "kubectl",
		commandPatterns: regexps(`(?i)^kubectl\b`),
		outputPatterns:  regexps(`(?m)^NAME\s+`, `(?i)Error from server`, `(?m)^\w+/\S+`),
		keepPatterns:    regexps(`(?m)^NAME\s+`, `(?i)error|failed|warning`, `(?m)^\w+/\S+`, `(?m)^\S+\s+(?:Running|Pending|Failed|CrashLoopBackOff)`),
		stripPatterns:   regexps(`(?m)^\s*$`),
		maxLines:        140,
		headLines:       30,
		tailLines:       70,
	},
	{
		id:              "terraform-plan",
		commandPatterns: regexps(`(?i)^(?:terraform|tofu|opentofu)\s+plan\b`),
		outputPatterns:  regexps(`Terraform will perform the following actions:`, `OpenTofu will perform the following actions:`, `(?i)Plan: \d+ to add`),
		keepPatterns:    regexps(`(?i)Plan: \d+ to add`, `(?m)^\s*[+#~-]\s`, `(?i)Error:`, `(?i)Warning:`),
		stripPatterns:   regexps(`(?m)^\s*$`, `(?m)^Refreshing state`),
		maxLines:        180,
		headLines:       40,
		tailLines:       90,
	},
	{
		id:             "vite",
		outputPatterns: regexps(`(?i)vite v[\d.]+`, `built in`),
		matchOutput: []rtkMatchOutput{
			{pattern: regexp.MustCompile(`(?i)built in`), unless: regexp.MustCompile(`(?i)error|failed`), message: "vite: build ok"},
		},
		keepPatterns:  regexps(`(?i)error`, `(?i)failed`, `built in`, `dist/`, `rendering chunks`),
		stripPatterns: regexps(`(?m)^transforming `, `(?m)^✓ \d+ modules transformed`, `(?m)^\s*$`),
		maxLines:      120,
		headLines:     30,
		tailLines:     45,
	},
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func applyRTK(text string, config Config, command string) (string, []string, []string) {
	filter := matchRTKFilter(text, command)
	if filter == nil {
		result, deduped := deduplicateRepeatedLines(text, normalizedDedupThreshold(config))
		techniques := []string{}
		rules := []string{}
		if deduped > 0 {
			techniques = append(techniques, "rtk-dedup")
			rules = append(rules, "rtk:dedup")
		}
		truncated, ok := smartTruncate(result, config, 24, 24)
		if ok {
			result = truncated
			techniques = append(techniques, "rtk-truncate")
			rules = append(rules, "rtk:truncate")
		}
		return result, techniques, rules
	}
	result, rules := applyRTKFilter(text, *filter)
	techniques := []string{}
	if len(rules) > 0 {
		techniques = append(techniques, "rtk-filter")
	}
	deduped, collapsed := deduplicateRepeatedLines(result, normalizedDedupThreshold(config))
	if collapsed > 0 {
		result = deduped
		techniques = append(techniques, "rtk-dedup")
		rules = append(rules, "rtk:dedup")
	}
	truncated, ok := smartTruncate(result, config, filter.headLines, filter.tailLines)
	if ok {
		result = truncated
		techniques = append(techniques, "rtk-truncate")
		rules = append(rules, "rtk:truncate")
	}
	return result, uniqueStrings(techniques), uniqueStrings(rules)
}

func matchRTKFilter(text string, command string) *rtkFilter {
	for i := range rtkFilters {
		filter := &rtkFilters[i]
		if command != "" && anyRegexpMatches(filter.commandPatterns, command) {
			return filter
		}
		if anyRegexpMatches(filter.outputPatterns, text) {
			return filter
		}
	}
	return nil
}

func applyRTKFilter(text string, filter rtkFilter) (string, []string) {
	applied := make([]string, 0)
	lines := splitLines(text)
	strippedAnsi := false
	for i := range lines {
		next := ansiPattern.ReplaceAllString(lines[i], "")
		if next != lines[i] {
			strippedAnsi = true
			lines[i] = next
		}
		lines[i] = normalizeStderrPrefix(lines[i])
	}
	if strippedAnsi {
		applied = append(applied, filter.id+":strip-ansi")
	}

	if len(filter.matchOutput) > 0 {
		blob := strings.Join(lines, "\n")
		for _, rule := range filter.matchOutput {
			if rule.pattern.MatchString(blob) && (rule.unless == nil || !rule.unless.MatchString(blob)) {
				return rule.message, append(applied, filter.id+":match-output")
			}
		}
	}

	if len(filter.stripPatterns) > 0 {
		before := len(lines)
		lines = filterLines(lines, func(line string) bool {
			return !anyRegexpMatches(filter.stripPatterns, line)
		})
		if len(lines) != before {
			applied = append(applied, filter.id+":strip")
		}
	}
	if len(filter.keepPatterns) > 0 {
		kept := filterLines(lines, func(line string) bool {
			return anyRegexpMatches(filter.keepPatterns, line)
		})
		if len(kept) > 0 {
			lines = kept
			applied = append(applied, filter.id+":keep")
		}
	}
	if len(filter.collapsePatterns) > 0 {
		seen := make(map[string]struct{})
		before := len(lines)
		lines = filterLines(lines, func(line string) bool {
			if !anyRegexpMatches(filter.collapsePatterns, line) {
				return true
			}
			key := strings.TrimSpace(line)
			if _, ok := seen[key]; ok {
				return false
			}
			seen[key] = struct{}{}
			return true
		})
		if len(lines) != before {
			applied = append(applied, filter.id+":collapse")
		}
	}
	if len(lines) == 0 && filter.onEmpty != "" {
		return filter.onEmpty, append(applied, filter.id+":empty")
	}
	return strings.Join(lines, "\n"), applied
}

func smartTruncate(text string, config Config, preserveHead int, preserveTail int) (string, bool) {
	maxLines := config.MaxLines
	if maxLines <= 0 {
		maxLines = 120
	}
	maxChars := config.MaxChars
	if maxChars <= 0 {
		maxChars = 12000
	}
	lines := splitLines(text)
	overLines := len(lines) > maxLines
	overChars := len([]rune(text)) > maxChars
	if !overLines && !overChars {
		return text, false
	}
	if preserveHead <= 0 {
		preserveHead = 24
	}
	if preserveTail <= 0 {
		preserveTail = 24
	}
	selected := make([]string, 0, preserveHead+preserveTail+8)
	selected = append(selected, lines[:minInt(preserveHead, len(lines))]...)
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(strings.ToLower(line), "failed") ||
			strings.Contains(strings.ToLower(line), "exception") ||
			strings.Contains(line, "TS") ||
			strings.Contains(line, "FAIL") {
			if !containsString(selected, line) {
				selected = append(selected, line)
			}
		}
	}
	tailStart := maxInt(0, len(lines)-preserveTail)
	for _, line := range lines[tailStart:] {
		if !containsString(selected, line) {
			selected = append(selected, line)
		}
	}
	dropped := maxInt(0, len(lines)-len(selected))
	result := strings.Join(append(selected[:minInt(preserveHead, len(selected))], append([]string{fmt.Sprintf("[rtk:truncated %d lines]", dropped)}, selected[minInt(preserveHead, len(selected)):]...)...), "\n")
	if len([]rune(result)) > maxChars {
		runes := []rune(result)
		marker := []rune("\n[rtk:truncated by chars]\n")
		budget := maxInt(0, maxChars-len(marker))
		head := int(float64(budget) * 0.55)
		tail := budget - head
		if budget == 0 {
			return string(marker[:minInt(len(marker), maxChars)]), true
		}
		result = string(runes[:minInt(head, len(runes))]) + string(marker)
		if tail > 0 && len(runes) > tail {
			result += string(runes[len(runes)-tail:])
		}
	}
	return result, true
}

func deduplicateRepeatedLines(text string, threshold int) (string, int) {
	lines := splitLines(text)
	output := make([]string, 0, len(lines))
	collapsed := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		runLength := 1
		for i+runLength < len(lines) && lines[i+runLength] == line {
			runLength++
		}
		if strings.TrimSpace(line) != "" && runLength >= threshold {
			output = append(output, line, fmt.Sprintf("[line repeated %dx]", runLength-1), fmt.Sprintf("[rtk:dropped %d repeated lines]", runLength-1))
			collapsed += runLength - 1
			i += runLength - 1
			continue
		}
		output = append(output, line)
	}
	return strings.Join(output, "\n"), collapsed
}

func regexps(patterns ...string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		result = append(result, regexp.MustCompile(pattern))
	}
	return result
}

func anyRegexpMatches(patterns []*regexp.Regexp, text string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func filterLines(lines []string, keep func(string) bool) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if keep(line) {
			result = append(result, line)
		}
	}
	return result
}

func splitLines(text string) []string {
	return regexp.MustCompile(`\r?\n`).Split(text, -1)
}

func normalizeStderrPrefix(line string) string {
	return regexp.MustCompile(`(?i)^\s*(?:stderr|err)\s*(?:\||:)\s*`).ReplaceAllString(line, "")
}

func normalizedDedupThreshold(config Config) int {
	if config.DeduplicateThreshold < 2 {
		return 3
	}
	return config.DeduplicateThreshold
}
