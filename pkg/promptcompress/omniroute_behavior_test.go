package promptcompress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOmniRouteOfficialRTKSmartTruncateBehavior(t *testing.T) {
	short, truncated := smartTruncateWithPriority("short\ntext", 10, 0, 20, 20, nil)
	require.False(t, truncated)
	require.Equal(t, "short\ntext", short)

	lines := make([]string, 20)
	for index := range lines {
		if index == 10 {
			lines[index] = "ERROR important failure"
			continue
		}
		lines[index] = "line " + strconvItoa(index)
	}
	priority, err := compileRTKPattern("ERROR", 0)
	require.NoError(t, err)
	result, truncated := smartTruncateWithPriority(strings.Join(lines, "\n"), 8, 0, 2, 2, []rtkPattern{priority})

	require.True(t, truncated)
	require.Contains(t, result, "line 0")
	require.Contains(t, result, "ERROR important failure")
	require.Contains(t, result, "line 19")
	require.Contains(t, result, "[rtk:truncated")

	charResult, truncated := smartTruncateWithPriority(strings.Repeat("x", 500), 0, 80, 20, 20, nil)
	require.True(t, truncated)
	require.LessOrEqual(t, len(charResult), 80)

	tinyResult, truncated := smartTruncateWithPriority(strings.Repeat("x", 500), 0, 10, 20, 20, nil)
	require.True(t, truncated)
	require.LessOrEqual(t, len(tinyResult), 10)
}

func TestOmniRouteOfficialRTKLineFilterFixtureBehavior(t *testing.T) {
	filters, err := loadRTKFilters()
	require.NoError(t, err)

	gitStatus := requireRTKFilter(t, filters, "git-status")
	gitStatusSample := strings.Join([]string{
		"On branch feature/rtk-compression-roadmap",
		"Changes not staged for commit:",
		"  modified:   open-sse/services/compression/engines/rtk/commandDetector.ts",
		"Untracked files:",
		"  tests/unit/compression/fixtures/rtk/git-status-sample.txt",
	}, "\n")
	gitOutput, gitRules := applyRTKFilter(gitStatusSample, gitStatus)

	require.Contains(t, gitRules, "git-status:keep")
	require.Contains(t, gitOutput, "On branch")
	require.NotContains(t, gitOutput, "nothing added")

	vitest := requireRTKFilter(t, filters, "test-vitest")
	vitestSample := strings.Join([]string{
		" RUN  v3.2.4 /repo",
		"",
		" ✓ tests/unit/compression/rtk-engine.test.ts (5 tests) 18ms",
		" ❯ tests/unit/compression/rtk-command-detector.test.ts (1 failed | 2 passed) 12ms",
		"",
		" Test Files  1 failed | 1 passed (2)",
		" Tests  1 failed | 7 passed (8)",
	}, "\n")
	vitestOutput, vitestRules := applyRTKFilter(vitestSample, vitest)

	require.Contains(t, vitestRules, "test-vitest:keep")
	require.Contains(t, vitestOutput, "Test Files  1 failed | 1 passed")
	require.Contains(t, vitestOutput, "Tests  1 failed | 7 passed")
	require.NotContains(t, vitestOutput, "RUN  v3.2.4")
}

func TestOmniRouteOfficialRTKCodeStripperBehavior(t *testing.T) {
	require.Equal(t, "typescript", detectRTKCodeLanguage("interface User { id: string }"))
	require.Equal(t, "python", detectRTKCodeLanguage("def run():\n    print('x')"))
	require.Equal(t, "rust", detectRTKCodeLanguage("fn main() { println!(\"x\"); }"))
	require.Equal(t, "go", detectRTKCodeLanguage("package main\nfunc main() {}"))
	require.Equal(t, "java", detectRTKCodeLanguage("class Main { }"))

	js := stripRTKCode("// comment\nconst url = 'https://example.com/a//b';\n/* block */\nconsole.log(url);", "javascript")
	require.Contains(t, js.text, "// comment")
	require.Contains(t, js.text, "https://example.com/a//b")
	require.Contains(t, js.text, "/* block */")

	python := stripRTKCode("\"\"\"doc\"\"\"\n# comment\nprint(\"ok\")", "python")
	require.Contains(t, python.text, "doc")
	require.Contains(t, python.text, "# comment")
}

func TestOmniRouteOfficialCavemanEngineBehavior(t *testing.T) {
	input := "Please could you help me analyze this code? I would like you to provide a detailed explanation of what the function does. Thank you so much for your help!"
	result := CompressCavemanText(input, DefaultSettings().ToConfig(ModeStandard), IntensityLite, "user")

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Greater(t, result.Stats.SavingsPercent, float64(0))
	require.NotEmpty(t, result.Stats.RulesApplied)

	codeContent := "const x = 42;\nconsole.log(x);"
	withCode := "Please analyze this code:\n```typescript\n" + codeContent + "\n```\nThank you so much!"
	codeResult := CompressCavemanText(withCode, DefaultSettings().ToConfig(ModeStandard), IntensityLite, "user")
	require.Contains(t, codeResult.Text, codeContent)

	url := "https://example.com/api/v1/users"
	urlResult := CompressCavemanText("Please check "+url+" for the API docs. Thank you so much!", DefaultSettings().ToConfig(ModeStandard), IntensityLite, "user")
	require.Contains(t, strings.Fields(urlResult.Text), url)

	skipped := CompressCavemanText("Please help me with this code. Thank you so much!", Config{
		CavemanSkipRules:            []string{"polite_framing"},
		CavemanLanguage:             "en",
		CavemanEnabledLanguagePacks: []string{"en"},
	}, IntensityLite, "user")
	require.NotContains(t, skipped.Stats.RulesApplied, "caveman:polite_framing")
}

func TestOmniRouteOfficialStackedPipelineBehavior(t *testing.T) {
	messages := []Message{
		{Role: "tool", Content: strings.Join(repeatString("same noisy line", 8), "\n")},
		{Role: "user", Content: "Please provide a detailed explanation of the authentication configuration"},
	}

	out, stats := CompressMessages(messages, Config{Mode: ModeStacked})

	require.Equal(t, "stacked", stats.Engine)
	require.Contains(t, stats.TechniquesUsed, "rtk-dedup")
	require.Contains(t, stats.TechniquesUsed, "caveman")
	require.Contains(t, out[0].Content, "[rtk:dropped")
	require.Contains(t, out[1].Content, "Explain")
	require.NotContains(t, strings.ToLower(out[1].Content), "please provide a detailed explanation")
}

func requireRTKFilter(t *testing.T, filters []rtkFilter, id string) rtkFilter {
	t.Helper()
	for _, filter := range filters {
		if filter.ID == id {
			return filter
		}
	}
	require.Failf(t, "missing RTK filter", "filter %s not found", id)
	return rtkFilter{}
}

func repeatString(value string, count int) []string {
	out := make([]string, count)
	for index := range out {
		out[index] = value
	}
	return out
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
