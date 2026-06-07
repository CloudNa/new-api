package promptcompress

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/QuantumNous/new-api/common"
	"github.com/dlclark/regexp2"
)

type rtkFilter struct {
	ID               string
	Name             string
	Description      string
	Category         string
	CommandTypes     []string
	CommandPatterns  []rtkPattern
	MatchPatterns    []rtkPattern
	StripPatterns    []rtkPattern
	KeepPatterns     []rtkPattern
	PriorityPatterns []rtkPattern
	CollapsePatterns []rtkPattern
	StripAnsi        bool
	Replace          []rtkReplaceRule
	MatchOutput      []rtkMatchOutput
	TruncateLineAt   int
	OnEmpty          string
	FilterStderr     bool
	Deduplicate      bool
	MaxLines         int
	HeadLines        int
	TailLines        int
	Priority         int
	Tests            []rtkInlineTest
}

type rtkPattern struct {
	raw string
	re  *regexp2.Regexp
}

type rtkReplaceRule struct {
	Pattern     rtkPattern
	Replacement string
}

type rtkMatchOutput struct {
	Pattern rtkPattern
	Unless  *rtkPattern
	Message string
}

type rtkInlineTest struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Command  string `json:"command"`
}

type RtkFilterCatalogItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	CommandTypes []string `json:"commandTypes"`
	Priority     int      `json:"priority"`
}

type rtkFilterFile struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Priority    int             `json:"priority"`
	Match       rtkMatchFile    `json:"match"`
	Rules       rtkRulesFile    `json:"rules"`
	Preserve    rtkPreserveSet  `json:"preserve"`
	Tests       []rtkInlineTest `json:"tests"`
}

type rtkMatchFile struct {
	Commands    []string `json:"commands"`
	Patterns    []string `json:"patterns"`
	OutputTypes []string `json:"outputTypes"`
}

type rtkRulesFile struct {
	StripAnsi        bool                 `json:"stripAnsi"`
	Replace          []rtkReplaceRuleFile `json:"replace"`
	MatchOutput      []rtkMatchOutputFile `json:"matchOutput"`
	IncludePatterns  []string             `json:"includePatterns"`
	DropPatterns     []string             `json:"dropPatterns"`
	CollapsePatterns []string             `json:"collapsePatterns"`
	Deduplicate      bool                 `json:"deduplicate"`
	TruncateLineAt   int                  `json:"truncateLineAt"`
	MaxLines         int                  `json:"maxLines"`
	HeadLines        int                  `json:"headLines"`
	TailLines        int                  `json:"tailLines"`
	OnEmpty          string               `json:"onEmpty"`
	FilterStderr     bool                 `json:"filterStderr"`
}

type rtkReplaceRuleFile struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

type rtkMatchOutputFile struct {
	Pattern string `json:"pattern"`
	Message string `json:"message"`
	Unless  string `json:"unless"`
}

type rtkPreserveSet struct {
	ErrorPatterns   []string `json:"errorPatterns"`
	SummaryPatterns []string `json:"summaryPatterns"`
}

type rtkDetector struct {
	Type            string
	CommandPatterns []rtkPattern
	ContentPatterns []rtkPattern
}

type rtkDetection struct {
	Type    string
	Command string
}

type rtkFilterLoadOptions struct {
	customFiltersEnabled bool
	trustProjectFilters  bool
}

type rtkFilterCacheEntry struct {
	filters []rtkFilter
	err     error
}

type rtkFilterSource struct {
	kind string
	path string
	data []byte
}

type rtkFilterTrustState string

const (
	rtkFilterTrustAccepted rtkFilterTrustState = "accepted"
	rtkFilterTrustRejected rtkFilterTrustState = "rejected"
	rtkFilterTrustChanged  rtkFilterTrustState = "changed"
)

var (
	rtkFilterCacheMu sync.Mutex
	rtkFilterCache   = map[string]rtkFilterCacheEntry{}
	ansiPattern      = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
)

var rtkDetectors = []rtkDetector{
	newRtkDetector("git-status", []string{`^git\s+status\b`}, []string{`^On branch `, `^Changes (?:not staged|to be committed)`, `^Untracked files:`}),
	newRtkDetector("git-branch", []string{`^git\s+branch\b`, `^git\s+checkout\b`, `^git\s+switch\b`}, []string{`^\*\s+\S+`, `Switched to (?:a new )?branch`, `Already on ['"][^'"]+['"]`}),
	newRtkDetector("git-diff", []string{`^git\s+diff\b`, `^git\s+show\b`}, []string{`^diff --git `, `^@@\s+-\d+,\d+\s+\+\d+,\d+\s+@@`}),
	newRtkDetector("git-log", []string{`^git\s+log\b`}, []string{`^commit [0-9a-f]{7,40}`, `^Author: `}),
	newRtkDetector("make", []string{`^make\b`}, []string{`^make\[\d+\]: (?:Entering|Leaving) directory`, `make: \*\*\* `}),
	newRtkDetector("terraform-plan", []string{`^terraform\s+plan\b`}, []string{`Terraform will perform the following actions:`, `Plan: \d+ to add`}),
	newRtkDetector("tofu-plan", []string{`^(?:tofu|opentofu)\s+plan\b`}, []string{`OpenTofu will perform the following actions:`, `Plan: \d+ to add`}),
	newRtkDetector("systemctl-status", []string{`^systemctl\s+status\b`}, []string{`^\s*Loaded:\s+`, `^\s*Active:\s+`, `^●\s+\S+\.service`}),
	newRtkDetector("test-vitest", []string{`^vitest\b`, `^npm\s+(?:run\s+)?test:vitest\b`}, []string{`\bvitest\b`, `^ ✓ `, `^ ❯ `, `Test Files\s+\d+\s+(?:passed|failed)`}),
	newRtkDetector("test-jest", []string{`^jest\b`, `^npm\s+(?:run\s+)?test\b`}, []string{`Test Suites:\s+\d+`, `Tests:\s+\d+`, `^PASS\s+`, `^FAIL\s+`}),
	newRtkDetector("test-pytest", []string{`^pytest\b`, `^python\s+-m\s+pytest\b`}, []string{`=+\s+(?:\d+\s+)?(?:passed|failed|errors?)`, `^E\s+`, `^FAILED `}),
	newRtkDetector("test-cargo", []string{`^cargo\s+test\b`, `^cargo\s+nextest\b`}, []string{`^running \d+ tests?`, `^test\s+[\w:.-]+\s+\.\.\.\s+(?:ok|FAILED|ignored)`, `test result:\s+(?:ok|FAILED)`}),
	newRtkDetector("test-go", []string{`^go\s+test\b`}, []string{`^(?:ok|FAIL)\s+[\w./-]+\s+[\d.]+s`, `^--- FAIL: `, `^panic: `}),
	newRtkDetector("build-typescript", []string{`^tsc\b`, `^npm\s+run\s+typecheck\b`}, []string{`TS\d{4}:`, `error TS\d{4}`}),
	newRtkDetector("build-eslint", []string{`^eslint\b`, `^npm\s+run\s+lint\b`}, []string{`\s+\d+:\d+\s+(?:error|warning)\s+`, `✖\s+\d+\s+problems?`}),
	newRtkDetector("build-webpack", []string{`^webpack\b`, `^npx\s+webpack\b`, `^npm\s+run\s+build:webpack\b`}, []string{`webpack\s+\d`, `compiled (?:successfully|with \d+ errors?)`, `asset .+\.js`}),
	newRtkDetector("build-vite", []string{`^vite\s+build\b`, `^npm\s+run\s+build\b`, `^pnpm\s+build\b`}, []string{`vite v[\d.]+`, `✓ built in`, `transforming \(\d+\)`}),
	newRtkDetector("biome", []string{`^biome\b`, `^npx\s+biome\b`}, []string{`lint\/[A-Za-z0-9/.-]+`, `Checked \d+ files? in`}),
	newRtkDetector("prettier", []string{`^prettier\b`, `^npx\s+prettier\b`}, []string{`^Checking formatting\.\.\.`, `Code style issues found`}),
	newRtkDetector("turbo", []string{`^turbo\b`, `^npx\s+turbo\b`}, []string{`^• Packages in scope:`, `^Tasks:\s+\d+\s+successful`}),
	newRtkDetector("nx", []string{`^nx\b`, `^npx\s+nx\b`}, []string{`^NX\s+`, `^> nx run `}),
	newRtkDetector("playwright", []string{`^playwright\s+test\b`, `^npx\s+playwright\s+test\b`}, []string{`Running \d+ tests? using \d+ workers?`, `^\s+\d+ failed`}),
	newRtkDetector("npm-install", []string{`^(?:npm|pnpm|yarn)\s+(?:install|add|update)\b`}, []string{`added \d+ packages`, `packages are looking for funding`, `audited \d+ packages`}),
	newRtkDetector("npm-audit", []string{`^(?:npm|pnpm|yarn)\s+audit\b`}, []string{`found \d+ vulnerabilities`, `\b(?:low|moderate|high|critical)\b`}),
	newRtkDetector("ruff", []string{`^ruff\b`, `^uv\s+run\s+ruff\b`}, []string{`^[\w./-]+\.py:\d+:\d+:\s+[A-Z]\d+`, `Found \d+ errors?\.`}),
	newRtkDetector("mypy", []string{`^mypy\b`, `^python\s+-m\s+mypy\b`}, []string{`^[\w./-]+\.py:\d+:\s+error:`, `Found \d+ errors? in \d+ files?`}),
	newRtkDetector("pip", []string{`^pip\s+(?:install|download|uninstall)\b`, `^python\s+-m\s+pip\b`}, []string{`^Collecting `, `^Successfully installed `}),
	newRtkDetector("uv-sync", []string{`^uv\s+sync\b`, `^uv\s+pip\s+install\b`}, []string{`^Resolved \d+ packages?`, `^Installed \d+ packages?`}),
	newRtkDetector("poetry-install", []string{`^poetry\s+install\b`}, []string{`^Installing dependencies from lock file`, `^Package operations:`}),
	newRtkDetector("golangci-lint", []string{`^golangci-lint\b`}, []string{`^[\w./-]+\.go:\d+:\d+:`, `^\d+ issues?:`}),
	newRtkDetector("bundle-install", []string{`^bundle\s+install\b`}, []string{`^Fetching gem metadata from `, `^Bundle complete!`}),
	newRtkDetector("rubocop", []string{`^rubocop\b`, `^bundle\s+exec\s+rubocop\b`}, []string{`^Inspecting \d+ files`, `^[\w./-]+\.rb:\d+:\d+:\s+[A-Z]:`}),
	newRtkDetector("docker-ps", []string{`^docker\s+ps\b`}, []string{`^CONTAINER ID\s+IMAGE\s+COMMAND`}),
	newRtkDetector("docker-logs", []string{`^docker\s+logs\b`, `^docker\s+compose\s+logs\b`}, []string{`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, `\b(?:ERROR|WARN|INFO)\b`, `^Attaching to `}),
	newRtkDetector("docker-build", []string{`^docker\s+(?:build|buildx\s+build)\b`, `^(?:docker\s+compose|docker-compose)\s+build\b`}, []string{`Successfully built \w+`, `Successfully tagged \S+`, `writing image sha256:`, `ERROR \[`, `exit code: \d+`}),
	newRtkDetector("aws", []string{`^aws\b`}, []string{`An error occurred \([A-Za-z0-9]+\) when calling`, `^(?:upload|download): `}),
	newRtkDetector("gcloud", []string{`^gcloud\b`}, []string{`^ERROR: \(gcloud\.`, `^Updated property \[`}),
	newRtkDetector("ssh", []string{`^ssh\b`}, []string{`Permission denied \(`, `Host key verification failed`, `Connection timed out`}),
	newRtkDetector("rsync", []string{`^rsync\b`}, []string{`^sending incremental file list`, `^rsync error:`}),
	newRtkDetector("curl", []string{`^curl\b`}, []string{`curl: \(\d+\)`, `^HTTP\/\d(?:\.\d)? \d{3}`}),
	newRtkDetector("wget", []string{`^wget\b`}, []string{`^--\d{4}-\d{2}-\d{2}`, `^ERROR \d{3}:`}),
	newRtkDetector("json-output", []string{`^jq\b`, `^cat\s+.*\.json\b`}, []string{`^\s*[\[{][\s\S]*[\]}]\s*$`}),
	newRtkDetector("shell-ls", []string{`^ls(?:\s+-[A-Za-z]+)?\b`}, []string{`^total \d+`, `^\S+\s+\S+\s+\d+\s+\w+\s+\d{1,2}\s+`}),
	newRtkDetector("shell-find", []string{`^find\b`}, []string{`^(?:\.{1,2}|\/|[\w.-]+\/).+`}),
	newRtkDetector("shell-grep", []string{`^(?:grep|rg|ag)\b`}, []string{`^[\w./-]+\.(?:ts|tsx|js|jsx|py|go|rs|java|rb|md|json|ya?ml|txt):\d*:`, `^[\w./-]+\/[\w./-]+:\d*:`}),
	newRtkDetector("shell-ps", []string{`^ps\b`}, []string{`^(?:USER\s+PID|\s*PID\s+)`}),
	newRtkDetector("shell-df", []string{`^df\b`}, []string{`^Filesystem\s+.*Use%`}),
	newRtkDetector("shell-du", []string{`^du\b`}, []string{`^\d+(?:\.\d+)?[KMGTP]?\s+\S+`}),
	newRtkDetector("error-stacktrace", nil, []string{`Traceback \(most recent call last\):`, `^\s+at\s+\S+\s+\(.+:\d+:\d+\)`, `^panic: `, `^thread '[^']+' panicked at`}),
	newRtkDetector("generic-error", nil, []string{`Error:`, `Exception:`, `Traceback \(most recent call last\):`}),
}

type rtkApplyOptions struct {
	command     string
	skipFilters bool
}

var (
	rtkFencedCodePattern    = regexp.MustCompile("```([A-Za-z0-9_+.-]*)\\r?\\n([\\s\\S]*?)```")
	rtkCodeOnlyFencePattern = regexp.MustCompile("```([\\s\\S]*?)```")
	rtkShellPromptPattern   = regexp.MustCompile(`(?m)^\s*(?:[$>#]|PS\s+[^>]+>)\s*\S+`)
)

func applyRTK(text string, config Config, command string) (string, []string, []string) {
	return applyRTKWithOptions(text, config, rtkApplyOptions{command: command})
}

func applyRTKWithOptions(text string, config Config, options rtkApplyOptions) (string, []string, []string) {
	var filter *rtkFilter
	if !options.skipFilters {
		filter = matchRTKFilterWithConfig(text, options.command, config)
	}
	result := text
	techniques := []string{}
	rules := []string{}
	filterApplied := false
	if filter != nil {
		if rtkFilterAllowed(filter.ID, config) {
			filtered, applied := applyRTKFilterWithMaxLines(result, *filter, config.MaxLines)
			result = filtered
			if len(applied) > 0 {
				filterApplied = true
				techniques = append(techniques, "rtk-filter")
				rules = append(rules, applied...)
			}
		}
	}
	if config.ApplyToCodeBlocks {
		stripped, stripTechniques, stripRules := applyRTKCodeStrip(result, config)
		result = stripped
		techniques = append(techniques, stripTechniques...)
		rules = append(rules, stripRules...)
	}
	deduped, collapsed := deduplicateRepeatedLines(result, normalizedDedupThreshold(config))
	if collapsed > 0 {
		result = deduped
		techniques = append(techniques, "rtk-dedup")
		rules = append(rules, "rtk:dedup")
	}
	head, tail := rtkTruncateWindow(config)
	priorityPatterns := rtkDefaultPriorityPatterns()
	if filter != nil && filterApplied {
		priorityPatterns = append(priorityPatterns, filter.PriorityPatterns...)
	}
	truncated, ok := smartTruncateWithPriority(result, normalizedMaxLines(config.MaxLines), normalizedMaxChars(config.MaxChars), head, tail, priorityPatterns)
	if ok {
		result = truncated
		techniques = append(techniques, "rtk-truncate")
		rules = append(rules, "rtk:truncate")
	}
	return result, uniqueStrings(techniques), uniqueStrings(rules)
}

func applyRTKCodeBlocksOnly(text string, config Config) (string, []string, []string) {
	techniques := []string{}
	rules := []string{}
	changed := false
	result := rtkCodeOnlyFencePattern.ReplaceAllStringFunc(text, func(match string) string {
		processed, processedTechniques, processedRules := applyRTKWithOptions(match, config, rtkApplyOptions{})
		techniques = append(techniques, processedTechniques...)
		rules = append(rules, processedRules...)
		if EstimateTokens(processed) >= EstimateTokens(match) && len([]rune(processed)) >= len([]rune(match)) {
			return match
		}
		changed = true
		return processed
	})
	if !changed {
		return text, uniqueStrings(techniques), uniqueStrings(rules)
	}
	return result, uniqueStrings(techniques), uniqueStrings(rules)
}

// ContainsRTKOutput reports whether text contains an explicit terminal/tool output
// segment that can be handled by the OmniRoute RTK filter catalog.
func ContainsRTKOutput(text string, config Config) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if rtkFencedCodePattern.MatchString(text) {
		for _, match := range rtkFencedCodePattern.FindAllStringSubmatch(text, -1) {
			if len(match) != 3 {
				continue
			}
			if rtkFenceLooksLikeOutput(match[1], match[2], config) {
				return true
			}
		}
		return false
	}
	return rtkPlainTextLooksLikeOutput(text, config)
}

func applyRTKEmbeddedOutputsOnly(text string, config Config) (string, []string, []string) {
	techniques := []string{}
	rules := []string{}
	changed := false
	result := rtkFencedCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := rtkFencedCodePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		languageHint := strings.TrimSpace(parts[1])
		output := parts[2]
		if !rtkFenceLooksLikeOutput(languageHint, output, config) {
			return match
		}
		processed, processedTechniques, processedRules := applyRTKWithOptions(output, config, rtkApplyOptions{})
		techniques = append(techniques, processedTechniques...)
		rules = append(rules, processedRules...)
		if EstimateTokens(processed) >= EstimateTokens(output) && len([]rune(processed)) >= len([]rune(output)) {
			return match
		}
		changed = true
		return "```" + languageHint + "\n" + strings.TrimRight(processed, "\r\n") + "\n```"
	})
	if changed {
		return result, uniqueStrings(techniques), uniqueStrings(rules)
	}
	if !strings.Contains(text, "```") && rtkPlainTextLooksLikeOutput(text, config) {
		return applyRTKWithOptions(text, config, rtkApplyOptions{})
	}
	return text, uniqueStrings(techniques), uniqueStrings(rules)
}

func rtkFenceLooksLikeOutput(language string, text string, config Config) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if rtkTextLooksLikeOutput(text, config) {
		return true
	}
	return rtkTerminalFenceLanguage(language) && rtkTerminalTextHasCommandSignal(text)
}

func rtkTextLooksLikeOutput(text string, config Config) bool {
	filter := matchRTKSignalFilterWithConfig(text, "", config)
	return filter != nil && rtkFilterAllowed(filter.ID, config)
}

func rtkPlainTextLooksLikeOutput(text string, config Config) bool {
	filter := matchRTKSignalFilterWithConfig(text, "", config)
	if filter == nil || !rtkFilterAllowed(filter.ID, config) {
		return false
	}
	if rtkTerminalTextHasCommandSignal(text) {
		return true
	}
	if filter.ID == "generic-output" {
		return false
	}
	return len(splitLines(text)) >= 2
}

func rtkTerminalFenceLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "text", "txt", "log", "logs", "console", "terminal", "term", "shell", "sh", "bash", "zsh", "fish", "powershell", "pwsh", "ps1", "cmd", "diff", "patch":
		return true
	default:
		return false
	}
}

func rtkTerminalTextHasCommandSignal(text string) bool {
	if detectCommandFromText(text) != "" {
		return true
	}
	return rtkShellPromptPattern.MatchString(text)
}

func applyRTKCodeStrip(text string, config Config) (string, []string, []string) {
	strippedBlocks := 0
	result := rtkFencedCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := rtkFencedCodePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		languageHint := strings.TrimSpace(parts[1])
		code := parts[2]
		stripped := stripRTKCode(code, languageHint)
		if stripped.text == strings.TrimSpace(code) && stripped.strippedLines <= 0 {
			return match
		}
		strippedBlocks++
		fenceLanguage := languageHint
		if fenceLanguage == "" {
			fenceLanguage = stripped.language
		}
		if fenceLanguage == "unknown" {
			fenceLanguage = ""
		}
		return "```" + fenceLanguage + "\n" + stripped.text + "\n```"
	})
	if strippedBlocks == 0 {
		return text, nil, nil
	}
	return result, []string{"rtk-code-strip"}, []string{"rtk:code-strip"}
}

type rtkCodeStripResult struct {
	text          string
	strippedLines int
	language      string
}

func stripRTKCode(text string, languageHint string) rtkCodeStripResult {
	language := normalizeRTKCodeLanguage(languageHint)
	if language == "unknown" {
		language = detectRTKCodeLanguage(text)
	}
	originalLines := splitLines(text)
	lines := make([]string, 0, len(originalLines))
	for _, line := range originalLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, trimEndHorizontalWhitespace(line))
	}
	result := strings.Join(lines, "\n")
	result = strings.TrimLeft(result, "\r\n")
	result = strings.TrimRight(result, "\r\n\t ")
	lineCount := 0
	if result != "" {
		lineCount = len(splitLines(result))
	}
	return rtkCodeStripResult{
		text:          result,
		strippedLines: maxInt(0, len(originalLines)-lineCount),
		language:      language,
	}
}

func normalizeRTKCodeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "js", "jsx", "javascript":
		return "javascript"
	case "ts", "tsx", "typescript":
		return "typescript"
	case "py", "python":
		return "python"
	case "rs", "rust":
		return "rust"
	case "go":
		return "go"
	case "rb", "ruby":
		return "ruby"
	case "java":
		return "java"
	default:
		return "unknown"
	}
}

func detectRTKCodeLanguage(text string) string {
	for _, item := range []struct {
		language string
		pattern  *regexp.Regexp
	}{
		{"typescript", regexp.MustCompile(`\b(?:interface|type)\s+\w+\s*=|:\s*(?:string|number|boolean)\b`)},
		{"javascript", regexp.MustCompile(`\b(?:const|let|function|import|export)\b|=>`)},
		{"python", regexp.MustCompile(`\bdef\s+\w+\(|\bimport\s+\w+|print\(`)},
		{"rust", regexp.MustCompile(`\bfn\s+\w+\(|\blet\s+mut\b|println!\(`)},
		{"go", regexp.MustCompile(`\bfunc\s+\w+\(|package\s+\w+`)},
		{"java", regexp.MustCompile(`\bclass\s+\w+|System\.out\.println`)},
		{"ruby", regexp.MustCompile(`\bdef\s+\w+|puts\s+|end\s*$`)},
	} {
		if item.pattern.MatchString(text) {
			return item.language
		}
	}
	return "unknown"
}

func RTKFilterCatalog() ([]RtkFilterCatalogItem, error) {
	filters, err := loadRTKFilters()
	if err != nil {
		return nil, err
	}
	catalog := make([]RtkFilterCatalogItem, 0, len(filters))
	for _, filter := range filters {
		catalog = append(catalog, RtkFilterCatalogItem{
			ID:           filter.ID,
			Name:         filter.Name,
			Description:  filter.Description,
			Category:     filter.Category,
			CommandTypes: append([]string(nil), filter.CommandTypes...),
			Priority:     filter.Priority,
		})
	}
	return catalog, nil
}

func loadRTKFilters() ([]rtkFilter, error) {
	return loadRTKFiltersWithOptions(rtkFilterLoadOptions{})
}

func loadRTKFiltersWithOptions(options rtkFilterLoadOptions) ([]rtkFilter, error) {
	if options.customFiltersEnabled {
		return readRTKFilters(options)
	}

	key := rtkFilterCacheKey(options)
	rtkFilterCacheMu.Lock()
	if entry, ok := rtkFilterCache[key]; ok {
		rtkFilterCacheMu.Unlock()
		return entry.filters, entry.err
	}
	rtkFilterCacheMu.Unlock()

	filters, err := readRTKFilters(options)
	rtkFilterCacheMu.Lock()
	rtkFilterCache[key] = rtkFilterCacheEntry{filters: filters, err: err}
	rtkFilterCacheMu.Unlock()
	return filters, err
}

func rtkFilterCacheKey(options rtkFilterLoadOptions) string {
	cwd, _ := os.Getwd()
	return strings.Join([]string{
		cwd,
		rtkRawOutputDataDir(),
		fmt.Sprintf("custom=%t", options.customFiltersEnabled),
		fmt.Sprintf("trust=%t", options.trustProjectFilters),
		fmt.Sprintf("envTrust=%t", os.Getenv("OMNIROUTE_RTK_TRUST_PROJECT_FILTERS") == "1"),
	}, "|")
}

func readRTKFilters(options rtkFilterLoadOptions) ([]rtkFilter, error) {
	sources, err := collectRTKFilterSources(options)
	if err != nil {
		return nil, err
	}
	filters := make([]rtkFilter, 0, len(sources))
	for _, source := range sources {
		sourceFilters, err := parseRTKFilterSource(source)
		if err != nil {
			return nil, err
		}
		filters = append(filters, sourceFilters...)
	}
	sortRTKFilters(filters)
	return filters, nil
}

func collectRTKFilterSources(options rtkFilterLoadOptions) ([]rtkFilterSource, error) {
	sources := []rtkFilterSource{}
	if options.customFiltersEnabled {
		if projectSource := readRTKProjectFilterSource(options); projectSource != nil {
			sources = append(sources, *projectSource)
		}
		if globalSource := readRTKGlobalFilterSource(); globalSource != nil {
			sources = append(sources, *globalSource)
		}
	}
	builtin, err := readRTKBuiltinFilterSources()
	if err != nil {
		return nil, err
	}
	sources = append(sources, builtin...)
	return sources, nil
}

func readRTKProjectFilterSource(options rtkFilterLoadOptions) *rtkFilterSource {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	filtersPath := filepath.Join(cwd, ".rtk", "filters.json")
	data, err := os.ReadFile(filtersPath)
	if err != nil {
		return nil
	}
	if trust := projectRTKFiltersTrusted(filtersPath, data, options.trustProjectFilters); trust != rtkFilterTrustAccepted {
		return nil
	}
	return &rtkFilterSource{kind: "project", path: filtersPath, data: data}
}

func readRTKGlobalFilterSource() *rtkFilterSource {
	filtersPath := filepath.Join(rtkRawOutputDataDir(), "rtk", "filters.json")
	data, err := os.ReadFile(filtersPath)
	if err != nil {
		return nil
	}
	return &rtkFilterSource{kind: "global", path: filtersPath, data: data}
}

func readRTKBuiltinFilterSources() ([]rtkFilterSource, error) {
	const root = "omniroute/rtk_filters"
	entries, err := fs.ReadDir(omniRouteFS, root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	sources := make([]rtkFilterSource, 0, len(names))
	for _, name := range names {
		data, err := omniRouteFS.ReadFile(path.Join(root, name))
		if err != nil {
			return nil, err
		}
		sources = append(sources, rtkFilterSource{kind: "builtin", path: path.Join(root, name), data: data})
	}
	return sources, nil
}

func parseRTKFilterSource(source rtkFilterSource) ([]rtkFilter, error) {
	var files []rtkFilterFile
	if err := common.Unmarshal(source.data, &files); err != nil {
		var file rtkFilterFile
		if singleErr := common.Unmarshal(source.data, &file); singleErr != nil {
			if source.kind == "builtin" {
				return nil, fmt.Errorf("invalid RTK filter %s: %w", source.path, err)
			}
			return nil, nil
		}
		files = []rtkFilterFile{file}
	}
	filters := make([]rtkFilter, 0, len(files))
	for _, file := range files {
		filter, err := compileRTKFilter(file)
		if err != nil {
			if source.kind == "builtin" {
				return nil, fmt.Errorf("invalid RTK filter %s: %w", source.path, err)
			}
			continue
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func projectRTKFiltersTrusted(filtersPath string, data []byte, trustProjectFilters bool) rtkFilterTrustState {
	if trustProjectFilters || os.Getenv("OMNIROUTE_RTK_TRUST_PROJECT_FILTERS") == "1" {
		return rtkFilterTrustAccepted
	}
	trustPath := filepath.Join(filepath.Dir(filtersPath), "trust.json")
	trustData, err := os.ReadFile(trustPath)
	if err != nil {
		return rtkFilterTrustRejected
	}
	var trust struct {
		FiltersSHA256        string `json:"filtersSha256"`
		TrustedFiltersSHA256 string `json:"trustedFiltersSha256"`
	}
	if err := common.Unmarshal(trustData, &trust); err != nil {
		return rtkFilterTrustRejected
	}
	trustedHash := strings.TrimSpace(trust.FiltersSHA256)
	if trustedHash == "" {
		trustedHash = strings.TrimSpace(trust.TrustedFiltersSHA256)
	}
	if trustedHash == "" {
		return rtkFilterTrustRejected
	}
	actualHash := fmt.Sprintf("%x", common.Sha256Raw(data))
	if trustedHash != actualHash {
		return rtkFilterTrustChanged
	}
	return rtkFilterTrustAccepted
}

func sortRTKFilters(filters []rtkFilter) {
	sort.SliceStable(filters, func(i, j int) bool {
		if filters[i].Priority == filters[j].Priority {
			return filters[i].ID < filters[j].ID
		}
		return filters[i].Priority > filters[j].Priority
	})
}

func compileRTKFilter(file rtkFilterFile) (rtkFilter, error) {
	commandPatterns, err := compileRTKPatterns(file.Match.Commands, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	matchPatterns, err := compileRTKPatterns(file.Match.Patterns, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	stripPatterns, err := compileRTKPatterns(file.Rules.DropPatterns, regexp2.IgnoreCase|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	keepPatterns, err := compileRTKPatterns(file.Rules.IncludePatterns, regexp2.IgnoreCase|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	collapsePatterns, err := compileRTKPatterns(file.Rules.CollapsePatterns, regexp2.IgnoreCase|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	priorityPatterns, err := compileRTKPatterns(append(file.Preserve.ErrorPatterns, file.Preserve.SummaryPatterns...), regexp2.IgnoreCase|regexp2.ECMAScript)
	if err != nil {
		return rtkFilter{}, err
	}
	replace := make([]rtkReplaceRule, 0, len(file.Rules.Replace))
	for _, item := range file.Rules.Replace {
		pattern, err := compileRTKPattern(item.Pattern, regexp2.ECMAScript)
		if err != nil {
			return rtkFilter{}, err
		}
		replace = append(replace, rtkReplaceRule{Pattern: pattern, Replacement: item.Replacement})
	}
	matchOutput := make([]rtkMatchOutput, 0, len(file.Rules.MatchOutput))
	for _, item := range file.Rules.MatchOutput {
		pattern, err := compileRTKPattern(item.Pattern, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
		if err != nil {
			return rtkFilter{}, err
		}
		var unless *rtkPattern
		if item.Unless != "" {
			compiled, err := compileRTKPattern(item.Unless, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
			if err != nil {
				return rtkFilter{}, err
			}
			unless = &compiled
		}
		matchOutput = append(matchOutput, rtkMatchOutput{Pattern: pattern, Unless: unless, Message: item.Message})
	}
	return rtkFilter{
		ID:               file.ID,
		Name:             file.Label,
		Description:      file.Description,
		Category:         file.Category,
		CommandTypes:     file.Match.OutputTypes,
		CommandPatterns:  commandPatterns,
		MatchPatterns:    matchPatterns,
		StripPatterns:    stripPatterns,
		KeepPatterns:     keepPatterns,
		PriorityPatterns: priorityPatterns,
		CollapsePatterns: collapsePatterns,
		StripAnsi:        file.Rules.StripAnsi,
		Replace:          replace,
		MatchOutput:      matchOutput,
		TruncateLineAt:   file.Rules.TruncateLineAt,
		OnEmpty:          file.Rules.OnEmpty,
		FilterStderr:     file.Rules.FilterStderr,
		Deduplicate:      file.Rules.Deduplicate,
		MaxLines:         file.Rules.MaxLines,
		HeadLines:        file.Rules.HeadLines,
		TailLines:        file.Rules.TailLines,
		Priority:         file.Priority,
		Tests:            file.Tests,
	}, nil
}

func matchRTKFilter(text string, command string) *rtkFilter {
	return matchRTKFilterWithOptions(text, command, rtkFilterLoadOptions{})
}

func matchRTKFilterWithConfig(text string, command string, config Config) *rtkFilter {
	return matchRTKFilterWithOptions(text, command, rtkFilterLoadOptions{
		customFiltersEnabled: config.CustomFiltersEnabled,
		trustProjectFilters:  config.TrustProjectFilters,
	})
}

func matchRTKSignalFilterWithConfig(text string, command string, config Config) *rtkFilter {
	return matchRTKFilterWithOptionsAndFallback(text, command, rtkFilterLoadOptions{
		customFiltersEnabled: config.CustomFiltersEnabled,
		trustProjectFilters:  config.TrustProjectFilters,
	}, false)
}

func matchRTKFilterWithOptions(text string, command string, options rtkFilterLoadOptions) *rtkFilter {
	return matchRTKFilterWithOptionsAndFallback(text, command, options, true)
}

func matchRTKFilterWithOptionsAndFallback(text string, command string, options rtkFilterLoadOptions, includeGenericFallback bool) *rtkFilter {
	filters, err := loadRTKFiltersWithOptions(options)
	if err != nil {
		return nil
	}
	detection := detectRTKCommandType(text, command)
	for i := range filters {
		if containsString(filters[i].CommandTypes, detection.Type) {
			return &filters[i]
		}
	}
	for i := range filters {
		if detection.Command != "" && anyRTKPatternMatches(filters[i].CommandPatterns, detection.Command) {
			return &filters[i]
		}
	}
	for i := range filters {
		if anyRTKPatternMatches(filters[i].MatchPatterns, text) {
			return &filters[i]
		}
	}
	if !includeGenericFallback {
		return nil
	}
	for i := range filters {
		if containsString(filters[i].CommandTypes, "generic-output") {
			return &filters[i]
		}
	}
	return nil
}

func rtkFilterAllowed(id string, config Config) bool {
	if containsString(config.DisabledFilters, id) {
		return false
	}
	if len(config.EnabledFilters) == 0 {
		return true
	}
	return containsString(config.EnabledFilters, id)
}

func applyRTKFilter(text string, filter rtkFilter) (string, []string) {
	return applyRTKFilterWithMaxLines(text, filter, 0)
}

func applyRTKFilterWithMaxLines(text string, filter rtkFilter, fallbackMaxLines int) (string, []string) {
	applied := make([]string, 0)
	lines := splitLines(text)
	originalLineCount := len(lines)
	if filter.StripAnsi {
		stripped := make([]string, len(lines))
		changed := false
		for i, line := range lines {
			stripped[i] = ansiPattern.ReplaceAllString(line, "")
			if stripped[i] != line {
				changed = true
			}
		}
		lines = stripped
		if changed {
			applied = append(applied, filter.ID+":strip-ansi")
		}
	}
	if filter.FilterStderr {
		normalized := make([]string, len(lines))
		changed := false
		for i, line := range lines {
			normalized[i] = normalizeStderrPrefix(line)
			if normalized[i] != line {
				changed = true
			}
		}
		lines = normalized
		if changed {
			applied = append(applied, filter.ID+":filter-stderr")
		}
	}
	for _, rule := range filter.Replace {
		changed := false
		for i, line := range lines {
			next, err := rule.Pattern.re.Replace(line, rule.Replacement, -1, -1)
			if err != nil {
				continue
			}
			if next != line {
				changed = true
				lines[i] = next
			}
		}
		if changed {
			applied = append(applied, filter.ID+":replace")
		}
	}
	if len(filter.MatchOutput) > 0 {
		blob := strings.Join(lines, "\n")
		for _, rule := range filter.MatchOutput {
			if !rtkPatternMatches(rule.Pattern, blob) {
				continue
			}
			if rule.Unless != nil && rtkPatternMatches(*rule.Unless, blob) {
				continue
			}
			return rule.Message, append(applied, filter.ID+":match-output")
		}
	}
	if len(filter.StripPatterns) > 0 {
		before := len(lines)
		lines = filterLines(lines, func(line string) bool {
			return !anyRTKPatternMatches(filter.StripPatterns, line)
		})
		if len(lines) != before {
			applied = append(applied, filter.ID+":strip")
		}
	}
	if len(filter.KeepPatterns) > 0 {
		kept := filterLines(lines, func(line string) bool {
			return anyRTKPatternMatches(filter.KeepPatterns, line)
		})
		if len(kept) > 0 {
			lines = kept
			applied = append(applied, filter.ID+":keep")
		}
	}
	if len(filter.CollapsePatterns) > 0 {
		seen := make(map[string]struct{})
		lines = filterLines(lines, func(line string) bool {
			if !anyRTKPatternMatches(filter.CollapsePatterns, line) {
				return true
			}
			key := strings.TrimSpace(line)
			if _, ok := seen[key]; ok {
				return false
			}
			seen[key] = struct{}{}
			return true
		})
		applied = append(applied, filter.ID+":collapse")
	}
	if filter.TruncateLineAt > 0 {
		changed := false
		for i, line := range lines {
			next := truncateUnicodeSafe(line, filter.TruncateLineAt)
			if next != line {
				changed = true
				lines[i] = next
			}
		}
		if changed {
			applied = append(applied, filter.ID+":truncate-line")
		}
	}
	maxLines := filter.MaxLines
	if maxLines <= 0 {
		maxLines = fallbackMaxLines
	}
	truncated, ok := smartTruncateWithPriority(strings.Join(lines, "\n"), maxLines, 0, filter.HeadLines, filter.TailLines, filter.PriorityPatterns)
	if ok {
		applied = append(applied, filter.ID+":truncate")
	}
	output := truncated
	if strings.TrimSpace(output) == "" && filter.OnEmpty != "" {
		output = filter.OnEmpty
	}
	_ = originalLineCount
	return output, applied
}

func detectRTKCommandType(text string, command string) rtkDetection {
	detectedCommand := strings.TrimSpace(command)
	if detectedCommand == "" {
		detectedCommand = detectCommandFromText(text)
	}
	best := rtkDetection{Type: "unknown", Command: detectedCommand}
	bestConfidence := 0.0
	for _, detector := range rtkDetectors {
		commandMatched := detectedCommand != "" && anyRTKPatternMatches(detector.CommandPatterns, detectedCommand)
		contentMatches := 0
		for _, pattern := range detector.ContentPatterns {
			if rtkPatternMatches(pattern, text) {
				contentMatches++
			}
		}
		if !commandMatched && contentMatches == 0 {
			continue
		}
		confidence := 0.0
		if commandMatched {
			confidence += 0.55
		}
		confidence += float64(contentMatches) * 0.25
		if confidence > 1 {
			confidence = 1
		}
		if confidence > bestConfidence {
			bestConfidence = confidence
			best.Type = detector.Type
		}
	}
	return best
}

func detectCommandFromText(text string) string {
	lines := splitLines(text)
	prefix := regexp.MustCompile(`^(?:git|make|terraform|tofu|opentofu|systemctl|npm|pnpm|yarn|vitest|jest|pytest|python|go|cargo|tsc|eslint|webpack|vite|biome|prettier|turbo|nx|playwright|ruff|mypy|pip|uv|poetry|golangci-lint|bundle|rubocop|kubectl|composer|gh|docker|aws|gcloud|ssh|rsync|curl|wget|ls|find|grep|rg|ag|ps|df|du)\b`)
	for i := 0; i < len(lines) && i < 4; i++ {
		trimmed := strings.TrimSpace(strings.TrimPrefix(lines[i], "$ "))
		if trimmed != "" && prefix.MatchString(trimmed) {
			return trimmed
		}
	}
	return ""
}

func newRtkDetector(kind string, commandPatterns []string, contentPatterns []string) rtkDetector {
	commands, _ := compileRTKPatterns(commandPatterns, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
	contents, _ := compileRTKPatterns(contentPatterns, regexp2.IgnoreCase|regexp2.Multiline|regexp2.ECMAScript)
	return rtkDetector{Type: kind, CommandPatterns: commands, ContentPatterns: contents}
}

func compileRTKPatterns(patterns []string, options regexp2.RegexOptions) ([]rtkPattern, error) {
	out := make([]rtkPattern, 0, len(patterns))
	for _, pattern := range patterns {
		compiled, err := compileRTKPattern(pattern, options)
		if err != nil {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func compileRTKPattern(pattern string, options regexp2.RegexOptions) (rtkPattern, error) {
	re, err := regexp2.Compile(pattern, options)
	if err != nil {
		return rtkPattern{}, err
	}
	re.MatchTimeout = 50 * time.Millisecond
	return rtkPattern{raw: pattern, re: re}, nil
}

func anyRTKPatternMatches(patterns []rtkPattern, text string) bool {
	for _, pattern := range patterns {
		if rtkPatternMatches(pattern, text) {
			return true
		}
	}
	return false
}

func rtkPatternMatches(pattern rtkPattern, text string) bool {
	matched, err := pattern.re.MatchString(text)
	return err == nil && matched
}

func smartTruncateWithPriority(text string, maxLines int, maxChars int, preserveHead int, preserveTail int, priorityPatterns []rtkPattern) (string, bool) {
	if maxLines < 0 {
		maxLines = 0
	}
	if maxChars < 0 {
		maxChars = 0
	}
	lines := splitLines(text)
	overLines := maxLines > 0 && len(lines) > maxLines
	overChars := maxChars > 0 && utf16CodeUnitLen(text) > maxChars
	if !overLines && !overChars {
		return text, false
	}
	if preserveHead <= 0 {
		preserveHead = 20
	}
	if preserveTail <= 0 {
		preserveTail = 20
	}
	selected := make([]string, 0, preserveHead+preserveTail+8)
	head := lines[:minInt(preserveHead, len(lines))]
	selected = append(selected, head...)
	for _, line := range lines {
		if anyRTKPatternMatches(priorityPatterns, line) && !containsString(selected, line) {
			selected = append(selected, line)
		}
	}
	tailStart := maxInt(0, len(lines)-preserveTail)
	for _, line := range lines[tailStart:] {
		if !containsString(selected, line) {
			selected = append(selected, line)
		}
	}
	dropped := maxInt(0, len(lines)-len(selected))
	result := strings.Join(append(selected[:minInt(len(head), len(selected))], append([]string{fmt.Sprintf("[rtk:truncated %d lines]", dropped)}, selected[minInt(len(head), len(selected)):]...)...), "\n")
	if maxChars > 0 && utf16CodeUnitLen(result) > maxChars {
		marker := "\n[rtk:truncated by chars]\n"
		budget := maxInt(0, maxChars-utf16CodeUnitLen(marker))
		if budget == 0 {
			return utf16CodeUnitSlice(marker, 0, maxChars), true
		}
		headChars := ceilPercent(budget, 55)
		tailChars := budget - headChars
		original := result
		result = utf16CodeUnitSlice(original, 0, headChars) + marker
		if tailChars > 0 {
			result += utf16CodeUnitTail(original, tailChars)
		}
		if utf16CodeUnitLen(result) > maxChars {
			result = utf16CodeUnitSlice(result, 0, maxChars)
		}
	}
	return result, true
}

func utf16CodeUnitLen(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func utf16CodeUnitSlice(text string, start int, end int) string {
	units := utf16.Encode([]rune(text))
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(units) {
		start = len(units)
	}
	if end > len(units) {
		end = len(units)
	}
	return string(utf16.Decode(units[start:end]))
}

func utf16CodeUnitTail(text string, count int) string {
	if count <= 0 {
		return ""
	}
	units := utf16.Encode([]rune(text))
	start := maxInt(0, len(units)-count)
	return string(utf16.Decode(units[start:]))
}

func ceilPercent(value int, percent int) int {
	if value <= 0 || percent <= 0 {
		return 0
	}
	return (value*percent + 99) / 100
}

func normalizedMaxLines(value int) int {
	if value <= 0 {
		return 120
	}
	return value
}

func normalizedMaxChars(value int) int {
	if value <= 0 {
		return 12000
	}
	return value
}

func rtkTruncateWindow(config Config) (int, int) {
	if NormalizeRtkIntensity(config.RtkIntensity) == RtkIntensityAggressive {
		return 16, 16
	}
	return 24, 24
}

func rtkDefaultPriorityPatterns() []rtkPattern {
	patterns, err := compileRTKPatterns([]string{`error|failed|exception|traceback|TS\d{4}|FAIL|✖`}, regexp2.IgnoreCase|regexp2.ECMAScript)
	if err != nil {
		return nil
	}
	return patterns
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

func truncateUnicodeSafe(line string, maxChars int) string {
	if maxChars <= 0 {
		return line
	}
	runes := []rune(line)
	if len(runes) <= maxChars {
		return line
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
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
