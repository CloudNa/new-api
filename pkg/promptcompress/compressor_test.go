package promptcompress

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCavemanPreservesStructuredContent(t *testing.T) {
	input := "Hello, please provide a detailed explanation of this.\n```go\nfmt.Println(\"the code\")\n```\nSee https://example.com/docs.\n{\"path\":\"/tmp/app.go\",\"ok\":true}"

	result := CompressText(input, Config{Mode: ModeStandard})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.NotContains(t, strings.ToLower(result.Text), "please")
	require.Contains(t, result.Text, "```go\nfmt.Println(\"the code\")\n```")
	require.Contains(t, result.Text, "https://example.com/docs")
	require.Contains(t, result.Text, "{\"path\":\"/tmp/app.go\",\"ok\":true}")
	require.GreaterOrEqual(t, result.Stats.PreservedBlockCount, 3)
	require.Contains(t, result.Stats.TechniquesUsed, "caveman")
}

func TestOmniRouteCavemanPreservationValidationParity(t *testing.T) {
	input := strings.Join([]string{
		"---",
		"title: Keep Me",
		"---",
		"# Install Guide",
		"Please provide a detailed explanation of this.",
		"~~~bash",
		"npm test",
		"~~~",
		"| name | value |",
		"| --- | --- |",
		"| API_VERSION | 1.2.3-beta |",
		"Use `process.env.API_KEY` and [docs](https://example.com/docs).",
		"Equation $$E=mc^2$$ and inline $a+b$ plus \\[x=y\\].",
		"\\begin{align}a&=b\\end{align}",
		"#set page(width: auto)",
		"Call client.api.run() and render(value).",
	}, "\n")

	result := CompressText(input, Config{Mode: ModeStandard})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.False(t, result.Stats.Bypassed, "%+v text=%q", result.Stats, result.Text)
	require.NotContains(t, strings.ToLower(result.Text), "please provide a detailed explanation")
	for _, preserved := range []string{
		"---\ntitle: Keep Me\n---\n",
		"# Install Guide",
		"~~~bash\nnpm test\n~~~",
		"| name | value |",
		"| --- | --- |",
		"| API_VERSION | 1.2.3-beta |",
		"`process.env.API_KEY`",
		"[docs](https://example.com/docs)",
		"$$E=mc^2$$",
		"$a+b$",
		"\\[x=y\\]",
		"\\begin{align}a&=b\\end{align}",
		"#set page(width: auto)",
		"client.api.run()",
		"render(value)",
	} {
		require.Contains(t, result.Text, preserved)
	}
	require.GreaterOrEqual(t, result.Stats.PreservedBlockCount, 12)
}

func TestOmniRouteCavemanValidationRejectsDroppedStructures(t *testing.T) {
	original := strings.Join([]string{
		"# Heading",
		"Use `exact_code`, API_VERSION 1.2.3, and https://example.com/docs.",
		"```go",
		"fmt.Println(\"keep\")",
		"```",
		"| a | b |",
		"| --- | --- |",
		"$$E=mc^2$$",
	}, "\n")

	invalid := strings.Join([]string{
		"Heading",
		"Use exact_code and docs.",
		"```go",
		"```",
	}, "\n")

	validation := validateCompression(original, invalid)

	require.False(t, validation.Valid)
	require.True(t, validation.FallbackApplied)
	require.NotEmpty(t, validation.Errors)
}

func TestOmniRouteRTKCatalogParity(t *testing.T) {
	filters, err := loadRTKFilters()
	require.NoError(t, err)
	require.Len(t, filters, 53)

	testCount := 0
	for _, filter := range filters {
		require.NotEmpty(t, filter.Tests, "filter %s should keep OmniRoute inline tests vendored", filter.ID)
		for _, test := range filter.Tests {
			testCount++
			t.Run(filter.ID+"/"+test.Name, func(t *testing.T) {
				actual, _ := applyRTKFilter(test.Input, filter)
				require.Equal(t, trimComparable(test.Expected), trimComparable(actual))
			})
		}
	}
	require.Greater(t, testCount, 0)
}

func TestOmniRouteRTKCommandMatchParity(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		input    string
		filterID string
	}{
		{name: "docker build", command: "docker build", input: "Successfully built abc123\n", filterID: "docker-build"},
		{name: "go test", command: "go test ./...", input: "ok  github.com/acme/app  0.1s\n", filterID: "test-go"},
		{name: "vite build", command: "vite build", input: "vite v5.0.0 building\n✓ built in 10ms\n", filterID: "build-vite"},
		{name: "gh pr", command: "gh pr create", input: "https://github.com/owner/repo/pull/1\n", filterID: "gh"},
		{name: "kubectl get", command: "kubectl get pods", input: "NAME READY STATUS\napi 1/1 Running\n", filterID: "kubectl"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filter := matchRTKFilter(tc.input, tc.command)
			require.NotNil(t, filter)
			require.Equal(t, tc.filterID, filter.ID)
		})
	}
}

func TestRTKGlobalCustomFilterParity(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	filterPath := dataDir + "/rtk/filters.json"
	writeRTKFilterFile(t, filterPath, []rtkFilterFile{
		testRTKFilterFile("glart-global-test", "GLART_CUSTOM_GLOBAL_MATCH", "GLOBAL_FILTERED"),
	})

	input := strings.Repeat("noise ", 40) + "GLART_CUSTOM_GLOBAL_MATCH " + strings.Repeat("noise ", 40)
	result := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: true,
	}, RtkTextOptions{})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Equal(t, "GLOBAL_FILTERED", result.Text)
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, result.Stats.RulesApplied, "glart-global-test:match-output")

	disabled := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: false,
	}, RtkTextOptions{})
	require.False(t, disabled.Compressed, "%+v text=%q", disabled.Stats, disabled.Text)
	require.Equal(t, input, disabled.Text)
}

func TestRTKProjectFilterRequiresTrust(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	originalCWD, err := os.Getwd()
	require.NoError(t, err)
	projectDir := t.TempDir()
	require.NoError(t, os.Chdir(projectDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalCWD))
	})

	filterPath := projectDir + "/.rtk/filters.json"
	filters := []rtkFilterFile{
		testRTKFilterFile("glart-project-test", "GLART_CUSTOM_PROJECT_MATCH", "PROJECT_FILTERED"),
	}
	filterBytes := writeRTKFilterFile(t, filterPath, filters)
	input := strings.Repeat("project noise ", 30) + "GLART_CUSTOM_PROJECT_MATCH " + strings.Repeat("project noise ", 30)

	untrusted := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: true,
		TrustProjectFilters:  false,
	}, RtkTextOptions{})
	require.False(t, untrusted.Compressed, "%+v text=%q", untrusted.Stats, untrusted.Text)
	require.Equal(t, input, untrusted.Text)

	trustedByConfig := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: true,
		TrustProjectFilters:  true,
	}, RtkTextOptions{})
	require.True(t, trustedByConfig.Compressed, "%+v text=%q", trustedByConfig.Stats, trustedByConfig.Text)
	require.Equal(t, "PROJECT_FILTERED", trustedByConfig.Text)
	require.Contains(t, trustedByConfig.Stats.RulesApplied, "glart-project-test:match-output")

	trust := map[string]string{"filtersSha256": fmt.Sprintf("%x", common.Sha256Raw(filterBytes))}
	trustBytes, err := common.Marshal(trust)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(projectDir+"/.rtk/trust.json", trustBytes, 0o600))

	trustedByHash := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: true,
		TrustProjectFilters:  false,
	}, RtkTextOptions{})
	require.True(t, trustedByHash.Compressed, "%+v text=%q", trustedByHash.Stats, trustedByHash.Text)
	require.Equal(t, "PROJECT_FILTERED", trustedByHash.Text)

	require.NoError(t, os.WriteFile(projectDir+"/.rtk/trust.json", []byte(`{"filtersSha256":"changed"}`), 0o600))
	changedTrust := CompressRTKText(input, Config{
		Mode:                 ModeRTK,
		CustomFiltersEnabled: true,
		TrustProjectFilters:  false,
	}, RtkTextOptions{})
	require.False(t, changedTrust.Compressed, "%+v text=%q", changedTrust.Stats, changedTrust.Text)
	require.Equal(t, input, changedTrust.Text)
}

func TestRTKInvalidCustomFilterSkipped(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	require.NoError(t, os.MkdirAll(dataDir+"/rtk", 0o700))
	require.NoError(t, os.WriteFile(dataDir+"/rtk/filters.json", []byte(`{not-valid-json`), 0o600))

	result := CompressRTKText(repeatedBuildOutput("ok"), Config{
		Mode:                 ModeRTK,
		MaxLines:             4,
		MaxChars:             12000,
		CustomFiltersEnabled: true,
	}, RtkTextOptions{Command: "go test ./..."})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, strings.Join(result.Stats.RulesApplied, ","), "test-go:")
}

func TestOmniRouteCavemanRulePackParity(t *testing.T) {
	rules, err := loadCavemanRules()
	require.NoError(t, err)
	require.Len(t, rules, 49)

	byName := make(map[string]cavemanRule, len(rules))
	for _, rule := range rules {
		byName[rule.Name] = rule
	}
	for _, name := range []string{
		"polite_framing",
		"verbose_instructions",
		"intent_clarification",
		"passive_voice",
		"ultra_dependency_abbreviation",
		"articles",
	} {
		require.Contains(t, byName, name)
	}
	require.Equal(t, "lite", byName["polite_framing"].MinIntensity)
	require.Equal(t, "full", byName["passive_voice"].MinIntensity)
	require.Equal(t, "ultra", byName["ultra_dependency_abbreviation"].MinIntensity)
}

func TestOmniRouteCavemanRuleBehaviorParity(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		intensity Intensity
		want      string
		rules     []string
	}{
		{
			name:      "polite verbose prompt",
			input:     "Please provide a detailed explanation of this.",
			intensity: IntensityLite,
			want:      "Explain this.",
			rules:     []string{"caveman:polite_framing", "caveman:verbose_instructions"},
		},
		{
			name:      "intent clarification",
			input:     "What I'm trying to do is deploy this.",
			intensity: IntensityLite,
			want:      "Goal:deploy this.",
			rules:     []string{"caveman:intent_clarification"},
		},
		{
			name:      "full passive voice and articles",
			input:     "The package was implemented yesterday.",
			intensity: IntensityStandard,
			want:      "Package implemented yesterday.",
			rules:     []string{"caveman:passive_voice", "caveman:articles"},
		},
		{
			name:      "ultra dependency abbreviation",
			input:     "dependency dependencies",
			intensity: IntensityUltra,
			want:      "Dep deps",
			rules:     []string{"caveman:ultra_dependency_abbreviation"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, rules := applyCavemanForRole(tc.input, tc.intensity, "user")
			require.Equal(t, tc.want, got)
			for _, rule := range tc.rules {
				require.Contains(t, rules, rule)
			}
		})
	}
}

func TestOmniRouteCavemanSkipRulesParity(t *testing.T) {
	input := "Please do this."

	withRule, withRules := applyCavemanWithConfig(input, IntensityLite, "user", Config{
		CavemanLanguage:             "en",
		CavemanEnabledLanguagePacks: []string{"en"},
	})
	skipped, skippedRules := applyCavemanWithConfig(input, IntensityLite, "user", Config{
		CavemanLanguage:             "en",
		CavemanEnabledLanguagePacks: []string{"en"},
		CavemanSkipRules:            []string{"polite_framing"},
	})

	require.Equal(t, "Do this.", withRule)
	require.Contains(t, withRules, "caveman:polite_framing")
	require.Equal(t, input, skipped)
	require.NotContains(t, skippedRules, "caveman:polite_framing")
}

func TestOmniRouteCavemanCustomPreservePatternParity(t *testing.T) {
	input := "Please provide a detailed explanation of this."

	result := CompressCavemanText(input, Config{
		CavemanPreservePatterns:     []string{`Please`},
		CavemanLanguage:             "en",
		CavemanEnabledLanguagePacks: []string{"en"},
	}, IntensityLite, "user")

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "Please")
	require.Contains(t, result.Text, "explain this")
	require.NotContains(t, strings.ToLower(result.Text), "detailed explanation")
	require.Contains(t, result.Stats.RulesApplied, "caveman:verbose_instructions")
}

func TestOmniRouteCavemanLanguagePackParity(t *testing.T) {
	input := "por favor necesito que puedes explicar por que este error."

	output, rules := applyCavemanWithConfig(input, IntensityLite, "user", Config{
		CavemanLanguage:             "en",
		CavemanAutoDetectLanguage:   true,
		CavemanEnabledLanguagePacks: []string{"en", "es"},
	})

	require.NotContains(t, strings.ToLower(output), "por favor")
	require.Contains(t, rules, "caveman:es_polite_framing")
	require.Contains(t, rules, "caveman:es_question_directive")
}

func TestSecretLikeTextIsRedacted(t *testing.T) {
	input := "please use api_key=test-value-redacted-123456 for this request"

	result := CompressText(input, Config{Mode: ModeLite})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.NotContains(t, result.Text, "test-value-redacted-123456")
	require.Contains(t, result.Text, "[redacted_secret]")
	require.Equal(t, 1, result.Stats.RedactedSecretCount)
}

func TestRTKCompressesDockerBuildOutput(t *testing.T) {
	input := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Step 2/3 : COPY . /app",
		"Step 3/3 : RUN npm install",
		" ---> Running in abc123",
		" ---> def456",
		"Removing intermediate container abc123",
		"Successfully built def456",
		"Successfully tagged myapp:latest",
	}, "\n")

	result := CompressText(input, Config{Mode: ModeRTK})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "Step 1/3 : FROM node:20")
	require.Contains(t, result.Text, "Successfully built def456")
	require.NotContains(t, result.Text, "Removing intermediate container")
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, result.Stats.RulesApplied, "docker-build:strip")
}

func TestOmniRouteRTKFilterAllowDenyParity(t *testing.T) {
	input := strings.Join([]string{
		"Step 1/3 : FROM node:20",
		"Step 2/3 : COPY . /app",
		"Step 3/3 : RUN npm install",
		" ---> Running in abc123",
		"Removing intermediate container abc123",
		"Successfully built def456",
	}, "\n")

	for _, config := range []Config{
		{Mode: ModeRTK, DisabledFilters: []string{"docker-build"}},
		{Mode: ModeRTK, EnabledFilters: []string{"test-go"}},
		{Mode: ModeRTK, EnabledFilters: []string{"docker-build"}, DisabledFilters: []string{"docker-build"}},
	} {
		result := CompressText(input, config)

		require.False(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
		require.Contains(t, result.Text, "Removing intermediate container")
		require.NotContains(t, result.Stats.TechniquesUsed, "rtk-filter")
		require.NotContains(t, result.Stats.RulesApplied, "docker-build:strip")
	}
}

func TestOmniRouteRTKCodeBlocksOnlyParity(t *testing.T) {
	input := strings.Join([]string{
		"before",
		"```go",
		"",
		"func main() {",
		"    println(\"x\")   ",
		"",
		"}",
		"",
		"```",
		"after",
	}, "\n")

	result := CompressRTKText(input, Config{
		Mode:              ModeRTK,
		ApplyToCodeBlocks: true,
	}, RtkTextOptions{CodeBlocksOnly: true})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "before\n```go\nfunc main() {\n    println(\"x\")\n}\n```\nafter")
	require.NotContains(t, result.Text, "println(\"x\")   ")
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-code-strip")
	require.Contains(t, result.Stats.RulesApplied, "rtk:code-strip")
}

func TestOmniRouteRTKCodeBlocksOnlyWideFenceParity(t *testing.T) {
	lines := []string{"before", "```go title=main.go"}
	for i := 0; i < 80; i++ {
		lines = append(lines, "line "+strings.Repeat("x", 80))
	}
	lines = append(lines, "```", "after")
	input := strings.Join(lines, "\n")

	result := CompressRTKText(input, Config{
		Mode:              ModeRTK,
		ApplyToCodeBlocks: true,
		MaxLines:          6,
		MaxChars:          12000,
	}, RtkTextOptions{CodeBlocksOnly: true})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "before")
	require.Contains(t, result.Text, "```go title=main.go")
	require.Contains(t, result.Text, "after")
	require.NotEqual(t, input, result.Text)
	require.NotEmpty(t, result.Stats.TechniquesUsed)
	require.NotEmpty(t, result.Stats.RulesApplied)
}

func TestOmniRouteRTKIntensityControlsTruncateWindow(t *testing.T) {
	head, tail := rtkTruncateWindow(Config{
		RtkIntensity:     RtkIntensityAggressive,
		CavemanIntensity: IntensityLite,
	})
	require.Equal(t, 16, head)
	require.Equal(t, 16, tail)

	head, tail = rtkTruncateWindow(Config{
		RtkIntensity:     RtkIntensityMinimal,
		CavemanIntensity: IntensityAggressive,
	})
	require.Equal(t, 24, head)
	require.Equal(t, 24, tail)
}

func TestRTKRawOutputRetentionDefaultNever(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	input := repeatedBuildOutput("")

	result := CompressRTKText(input, Config{
		Mode:     ModeRTK,
		MaxLines: 4,
		MaxChars: 12000,
	}, RtkTextOptions{})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Empty(t, result.Stats.RtkRawOutputPointers)
	_, err := os.Stat(dataDir + "/rtk/raw-output")
	require.True(t, os.IsNotExist(err), "raw-output dir should not exist by default: %v", err)
}

func TestRTKRawOutputRetentionAlwaysRedactsSecrets(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	fakeKey := "sk-test-redacted-1234567890abcdef"
	input := repeatedBuildOutput("api_key=" + fakeKey)

	result := CompressRTKText(input, Config{
		Mode:               ModeRTK,
		MaxLines:           4,
		MaxChars:           12000,
		RawOutputRetention: RtkRawOutputRetentionAlways,
		RawOutputMaxBytes:  4096,
	}, RtkTextOptions{Command: "go test ./..."})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Len(t, result.Stats.RtkRawOutputPointers, 1)
	pointer := result.Stats.RtkRawOutputPointers[0]
	require.NotEmpty(t, pointer.ID)
	require.NotEmpty(t, pointer.SHA256)
	require.True(t, pointer.Redacted)
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-raw-output-retention")
	require.Contains(t, result.Stats.RulesApplied, "rtk:raw-output-retention")

	data, err := os.ReadFile(pointer.Path)
	require.NoError(t, err)
	require.NotContains(t, string(data), fakeKey)
	require.Contains(t, string(data), "[redacted_secret]")
	require.Contains(t, pointer.Path, dataDir)
}

func TestRTKRawOutputRetentionFailuresOnly(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	success := CompressRTKText(repeatedBuildOutput("ok"), Config{
		Mode:               ModeRTK,
		MaxLines:           4,
		MaxChars:           12000,
		RawOutputRetention: RtkRawOutputRetentionFailures,
	}, RtkTextOptions{})
	require.True(t, success.Compressed, "%+v text=%q", success.Stats, success.Text)
	require.Empty(t, success.Stats.RtkRawOutputPointers)

	failure := CompressRTKText(repeatedBuildOutput("ERROR failed to compile"), Config{
		Mode:               ModeRTK,
		MaxLines:           4,
		MaxChars:           12000,
		RawOutputRetention: RtkRawOutputRetentionFailures,
	}, RtkTextOptions{})
	require.True(t, failure.Compressed, "%+v text=%q", failure.Stats, failure.Text)
	require.Len(t, failure.Stats.RtkRawOutputPointers, 1)
}

func TestStackedRunsRTKThenCaveman(t *testing.T) {
	input := strings.Join([]string{
		"Sure, I can help.",
		"INFO started",
		"INFO started",
		"INFO started",
		"ERROR failed to connect",
		"Please explain in detail why it failed.",
	}, "\n")

	result := CompressText(input, Config{Mode: ModeStacked})

	require.True(t, result.Compressed)
	require.Equal(t, 1, strings.Count(result.Text, "INFO started"))
	require.Contains(t, result.Text, "ERROR failed to connect")
	require.NotContains(t, strings.ToLower(result.Text), "please")
	require.Contains(t, result.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, result.Stats.RulesApplied, "docker-logs:collapse")
	require.Contains(t, result.Stats.TechniquesUsed, "caveman")
}

func TestStackedPipelineCanRunCustomCavemanOnly(t *testing.T) {
	input := strings.Join([]string{
		"INFO started",
		"INFO started",
		"INFO started",
		"Please provide a detailed explanation of this.",
	}, "\n")

	result := CompressText(input, Config{
		Mode: ModeStacked,
		StackedPipeline: []PipelineStep{
			{Engine: "caveman", Intensity: string(IntensityLite)},
		},
	})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "INFO started\nINFO started\nINFO started")
	require.Contains(t, result.Text, "Explain this.")
	require.Contains(t, result.Stats.TechniquesUsed, "caveman")
	require.NotContains(t, result.Stats.TechniquesUsed, "rtk-filter")
}

func TestCompressMessagesPreservesSystemPrompt(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "Please keep the system prompt exactly as written."},
		{Role: "user", Content: "Please explain in detail what I need to do."},
	}

	out, stats := CompressMessages(messages, Config{Mode: ModeStandard, PreserveSystemPrompt: true})

	require.Equal(t, messages[0].Content, out[0].Content)
	require.NotEqual(t, messages[1].Content, out[1].Content)
	require.Contains(t, stats.TechniquesUsed, "caveman")
}

func TestAggressiveProgressiveAgingParity(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "Keep this system prompt exactly as written."},
		{Role: "user", Content: "Implement: update pkg/promptcompress/aggressive.go and preserve docs/testdata/example.yaml while fixing TypeError: missing handler. " + strings.Repeat("older context ", 80)},
		{Role: "assistant", Content: "I inspected pkg/promptcompress/aggressive.go, found TypeError: missing handler, and decided to compress only tool output.\n```go\nfunc one() {}\nfunc two() {}\nfunc three() {}\nfunc four() {}\nfunc five() {}\n```" + strings.Repeat("assistant notes ", 80)},
		{Role: "user", Content: "Please provide a detailed explanation of the current failure and please provide a detailed explanation of next steps." + strings.Repeat(" detail", 60)},
		{Role: "assistant", Content: "The next step is adding tests." + strings.Repeat(" summary", 60)},
		{Role: "user", Content: "Recent user message should stay close to verbatim."},
		{Role: "assistant", Content: "Recent assistant message should stay close to verbatim."},
		{Role: "user", Content: "Newest prompt should stay close to verbatim."},
	}

	out, stats := CompressAggressiveMessages(messages, Config{Mode: ModeAggressive, PreserveSystemPrompt: true})

	require.False(t, stats.Bypassed, "%+v", stats)
	require.Equal(t, messages[0].Content, out[0].Content)
	require.Contains(t, out[1].Content, "[COMPRESSED:aging:fullSummary]")
	require.Contains(t, out[1].Content, "Implement: update")
	require.Contains(t, out[2].Content, "[COMPRESSED:aging:fullSummary]")
	require.Contains(t, out[2].Content, "Files touched: pkg/promptcompress/aggressive.go")
	require.Contains(t, out[2].Content, "TypeError: missing handler")
	require.Equal(t, messages[5].Content, out[5].Content)
	require.Equal(t, messages[6].Content, out[6].Content)
	require.Equal(t, messages[7].Content, out[7].Content)
	require.NotNil(t, stats.Aggressive)
	require.Greater(t, stats.Aggressive.AgingSavings, 0)
	require.Contains(t, stats.TechniquesUsed, "aging")
}

func TestAggressiveToolResultCompressorNonTerminalFileContent(t *testing.T) {
	fileContent := aggressiveLongCodeFileContent()

	out, stats := CompressAggressiveMessages([]Message{
		{Role: "tool", Content: fileContent},
	}, Config{Mode: ModeAggressive, PreserveSystemPrompt: true})

	require.False(t, stats.Bypassed, "%+v", stats)
	require.Contains(t, out[0].Content, "... [10 lines elided] ...")
	require.Contains(t, out[0].Content, "const value00 = 0")
	require.Contains(t, out[0].Content, "const value34 = 34")
	require.NotContains(t, out[0].Content, "const value24 = 24")
	require.NotNil(t, stats.Aggressive)
	require.Greater(t, stats.Aggressive.ToolResultSavings, 0)
	require.Contains(t, stats.TechniquesUsed, "toolResult")
}

func TestAggressiveToolResultCompressorJSON(t *testing.T) {
	items := make([]map[string]any, 0, 90)
	for i := 0; i < 90; i++ {
		items = append(items, map[string]any{
			"id":      i,
			"payload": strings.Repeat(fmt.Sprintf("item-%02d ", i), 8),
		})
	}
	data, err := common.Marshal(items)
	require.NoError(t, err)

	out, stats := CompressAggressiveMessages([]Message{
		{Role: "tool", Content: string(data)},
	}, Config{Mode: ModeAggressive, PreserveSystemPrompt: true})

	require.False(t, stats.Bypassed, "%+v", stats)
	require.Contains(t, out[0].Content, `"type":"array"`)
	require.Contains(t, out[0].Content, `"total":90`)
	require.Contains(t, out[0].Content, `"first5"`)
	require.Contains(t, out[0].Content, `"last2"`)
	require.NotNil(t, stats.Aggressive)
	require.Greater(t, stats.Aggressive.ToolResultSavings, 0)
	require.Contains(t, stats.TechniquesUsed, "toolResult")
}

func TestAggressiveToolStrategyJSONCanBeDisabled(t *testing.T) {
	items := make([]map[string]any, 0, 90)
	for i := 0; i < 90; i++ {
		items = append(items, map[string]any{
			"id":      i,
			"payload": strings.Repeat(fmt.Sprintf("item-%02d ", i), 8),
		})
	}
	data, err := common.Marshal(items)
	require.NoError(t, err)

	var settings Settings
	require.NoError(t, common.Unmarshal([]byte(`{"aggressive":{"tool_strategies":{"json":false}}}`), &settings))
	out, stats := CompressAggressiveMessages([]Message{
		{Role: "tool", Content: string(data)},
	}, settings.ToConfig(ModeAggressive))

	require.True(t, stats.Bypassed, "%+v", stats)
	require.Equal(t, string(data), out[0].Content)
	require.NotContains(t, stats.TechniquesUsed, "toolResult")
	require.NotNil(t, stats.Aggressive)
	require.Equal(t, 0, stats.Aggressive.ToolResultSavings)
}

func TestUltraHeuristicPruningParity(t *testing.T) {
	input := strings.Join([]string{
		"the and a an is are was were to of in on with from",
		"Critical Error: preserve https://example.com/docs and src/service/prompt_compression.go",
		"AlphaBeta importantContext should remain because it carries signal",
	}, " ")

	result := CompressText(input, Config{Mode: ModeUltra, PreserveSystemPrompt: true})

	require.True(t, result.Compressed, "%+v text=%q", result.Stats, result.Text)
	require.Contains(t, result.Text, "Error:")
	require.Contains(t, result.Text, "https://example.com/docs")
	require.Contains(t, result.Text, "src/service/prompt_compression.go")
	require.NotContains(t, result.Text, "the and a an")
	require.Contains(t, result.Stats.TechniquesUsed, "ultra-heuristic-pruning")
}

func TestUltraMaxTokensPerMessageSkipsSmallMessages(t *testing.T) {
	input := strings.Join([]string{
		"the and a an is are was were to of in on with from",
		"Critical Error: preserve https://example.com/docs and src/service/prompt_compression.go",
		"AlphaBeta importantContext should remain because it carries signal",
	}, " ")

	result := CompressText(input, Config{
		Mode:                     ModeUltra,
		PreserveSystemPrompt:     true,
		UltraMaxTokensPerMessage: 1000,
	})

	require.False(t, result.Compressed)
	require.True(t, result.Stats.Bypassed)
	require.Equal(t, "no_savings", result.Stats.BypassReason)
	require.Equal(t, input, result.Text)
}

func TestUltraCompressionRateOneBypassesNoSavings(t *testing.T) {
	input := strings.Join([]string{
		"the and a an is are was were to of in on with from",
		"Critical Error: preserve https://example.com/docs and src/service/prompt_compression.go",
		"AlphaBeta importantContext should remain because it carries signal",
	}, " ")

	result := CompressText(input, Config{
		Mode:                       ModeUltra,
		PreserveSystemPrompt:       true,
		UltraCompressionRate:       1,
		UltraCompressionRateSet:    true,
		UltraMinScoreThreshold:     0.3,
		UltraMinScoreThresholdSet:  true,
		UltraMaxTokensPerMessage:   0,
		UltraSlmFallbackAggressive: true,
	})

	require.False(t, result.Compressed)
	require.True(t, result.Stats.Bypassed)
	require.Equal(t, "no_savings", result.Stats.BypassReason)
	require.Equal(t, input, result.Text)
}

func TestUltraValidationFallbackRestoresProtectedHeading(t *testing.T) {
	input := "# Keep This Heading\n" + strings.Repeat("the and a an is are was were to of in on with from ", 8)

	out, stats := CompressUltraMessages([]Message{
		{Role: "user", Content: input},
	}, Config{
		Mode:                      ModeUltra,
		PreserveSystemPrompt:      true,
		UltraCompressionRate:      0.2,
		UltraCompressionRateSet:   true,
		UltraMinScoreThreshold:    1,
		UltraMinScoreThresholdSet: true,
	})

	require.True(t, stats.Bypassed, "%+v", stats)
	require.True(t, stats.FallbackApplied, "%+v", stats)
	require.NotEmpty(t, stats.ValidationErrors)
	require.Equal(t, input, out[0].Content)
}

func TestCompressMessagesAggressiveUsesMessagePipeline(t *testing.T) {
	messages := []Message{
		{Role: "tool", Content: aggressiveLongCodeFileContent()},
		{Role: "user", Content: "Recent user message should stay readable."},
	}

	out, stats := CompressMessages(messages, Config{Mode: ModeAggressive, PreserveSystemPrompt: true})

	require.False(t, stats.Bypassed, "%+v", stats)
	require.Contains(t, out[0].Content, "... [10 lines elided] ...")
	require.Contains(t, stats.TechniquesUsed, "toolResult")
	require.NotNil(t, stats.Aggressive)
	require.Greater(t, stats.Aggressive.ToolResultSavings, 0)
}

func TestBypassesWhenPreservationWouldDropProtectedContent(t *testing.T) {
	input := "vite v5.0.0 building\nhttps://example.com/must-keep\nbuilt in 10ms"

	result := CompressText(input, Config{Mode: ModeRTK})

	require.False(t, result.Compressed)
	require.True(t, result.Stats.Bypassed)
	require.Equal(t, "no_savings", result.Stats.BypassReason)
	require.Equal(t, input, result.Text)
}

func trimComparable(value string) string {
	return strings.TrimRight(value, "\n")
}

func repeatedBuildOutput(suffix string) string {
	lines := []string{"go test ./...", "=== RUN TestOne"}
	for i := 0; i < 80; i++ {
		lines = append(lines, "line repeated output "+suffix)
	}
	lines = append(lines, "ok github.com/acme/app 0.1s")
	return strings.Join(lines, "\n")
}

func aggressiveLongCodeFileContent() string {
	lines := make([]string, 0, 35)
	for i := 0; i < 35; i++ {
		lines = append(lines, fmt.Sprintf("const value%02d = %d", i, i))
	}
	return strings.Join(lines, "\n")
}

func testRTKFilterFile(id string, pattern string, message string) rtkFilterFile {
	return rtkFilterFile{
		ID:          id,
		Label:       id,
		Description: "test filter",
		Category:    "test",
		Priority:    10000,
		Match: rtkMatchFile{
			Patterns: []string{pattern},
		},
		Rules: rtkRulesFile{
			MatchOutput: []rtkMatchOutputFile{
				{Pattern: pattern, Message: message},
			},
		},
	}
}

func writeRTKFilterFile(t *testing.T, filePath string, filters []rtkFilterFile) []byte {
	t.Helper()
	data, err := common.Marshal(filters)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o700))
	require.NoError(t, os.WriteFile(filePath, data, 0o600))
	return data
}
