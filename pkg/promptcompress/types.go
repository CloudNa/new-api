package promptcompress

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type Mode string

const (
	ModeOff        Mode = "off"
	ModeLite       Mode = "lite"
	ModeStandard   Mode = "standard"
	ModeAggressive Mode = "aggressive"
	ModeUltra      Mode = "ultra"
	ModeRTK        Mode = "rtk"
	ModeStacked    Mode = "stacked"
)

type Intensity string

const (
	IntensityLite       Intensity = "lite"
	IntensityFull       Intensity = "full"
	IntensityStandard   Intensity = "standard"
	IntensityAggressive Intensity = "aggressive"
	IntensityUltra      Intensity = "ultra"
)

type RtkIntensity string

const (
	RtkIntensityMinimal    RtkIntensity = "minimal"
	RtkIntensityStandard   RtkIntensity = "standard"
	RtkIntensityAggressive RtkIntensity = "aggressive"
)

type RtkRawOutputRetention string

const (
	RtkRawOutputRetentionNever    RtkRawOutputRetention = "never"
	RtkRawOutputRetentionFailures RtkRawOutputRetention = "failures"
	RtkRawOutputRetentionAlways   RtkRawOutputRetention = "always"
)

type RtkRawOutputPointer struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Bytes    int    `json:"bytes"`
	SHA256   string `json:"sha256"`
	Redacted bool   `json:"redacted"`
}

type PipelineStep struct {
	Engine    string `json:"engine"`
	Intensity string `json:"intensity,omitempty"`
}

func (p *PipelineStep) UnmarshalJSON(data []byte) error {
	var engine string
	if err := common.Unmarshal(data, &engine); err == nil {
		*p = PipelineStep{Engine: engine}
		return nil
	}
	type alias PipelineStep
	var decoded alias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = PipelineStep(decoded)
	return nil
}

type Config struct {
	Mode                        Mode
	MinTokens                   int
	PreserveSystemPrompt        bool
	RtkIntensity                RtkIntensity
	RawOutputRetention          RtkRawOutputRetention
	RawOutputMaxBytes           int
	CustomFiltersEnabled        bool
	TrustProjectFilters         bool
	MaxLines                    int
	MaxChars                    int
	DeduplicateThreshold        int
	EnabledFilters              []string
	DisabledFilters             []string
	ApplyToToolResults          bool
	ApplyToAssistantMessages    bool
	ApplyToCodeBlocks           bool
	AggressiveThresholds        AggressiveThresholds
	AggressiveToolStrategies    ToolStrategiesConfig
	AggressiveToolConfigured    bool
	AggressiveSummarizerEnabled bool
	AggressiveSummarizerSet     bool
	AggressiveMaxTokensPerMsg   int
	AggressiveMinSavingsRate    float64
	AggressiveMinSavingsRateSet bool
	UltraCompressionRate        float64
	UltraCompressionRateSet     bool
	UltraMinScoreThreshold      float64
	UltraMinScoreThresholdSet   bool
	UltraSlmFallbackAggressive  bool
	UltraSlmFallbackSet         bool
	UltraModelPath              string
	UltraMaxTokensPerMessage    int
	CavemanSkipRules            []string
	CavemanPreservePatterns     []string
	CavemanIntensity            Intensity
	CavemanLanguage             string
	CavemanAutoDetectLanguage   bool
	CavemanEnabledLanguagePacks []string
	StackedPipeline             []PipelineStep
}

type RtkTextOptions struct {
	Command        string
	SkipFilters    bool
	CodeBlocksOnly bool
}

type Message struct {
	Role    string
	Content string
}

type AggressiveStats struct {
	SummarizerSavings int `json:"summarizer_savings"`
	ToolResultSavings int `json:"tool_result_savings"`
	AgingSavings      int `json:"aging_savings"`
}

type AggressiveThresholds struct {
	FullSummary int `json:"full_summary"`
	Moderate    int `json:"moderate"`
	Light       int `json:"light"`
	Verbatim    int `json:"verbatim"`
}

type ToolStrategiesConfig struct {
	FileContent     bool `json:"file_content"`
	GrepSearch      bool `json:"grep_search"`
	ShellOutput     bool `json:"shell_output"`
	JSON            bool `json:"json"`
	ErrorMessage    bool `json:"error_message"`
	fileContentSet  bool
	grepSearchSet   bool
	shellOutputSet  bool
	jsonSet         bool
	errorMessageSet bool
}

func (t *ToolStrategiesConfig) UnmarshalJSON(data []byte) error {
	type alias ToolStrategiesConfig
	var decoded alias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields struct {
		FileContent  *bool `json:"file_content"`
		GrepSearch   *bool `json:"grep_search"`
		ShellOutput  *bool `json:"shell_output"`
		JSON         *bool `json:"json"`
		ErrorMessage *bool `json:"error_message"`
	}
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	*t = ToolStrategiesConfig(decoded)
	if fields.FileContent != nil {
		t.FileContent = *fields.FileContent
		t.fileContentSet = true
	}
	if fields.GrepSearch != nil {
		t.GrepSearch = *fields.GrepSearch
		t.grepSearchSet = true
	}
	if fields.ShellOutput != nil {
		t.ShellOutput = *fields.ShellOutput
		t.shellOutputSet = true
	}
	if fields.JSON != nil {
		t.JSON = *fields.JSON
		t.jsonSet = true
	}
	if fields.ErrorMessage != nil {
		t.ErrorMessage = *fields.ErrorMessage
		t.errorMessageSet = true
	}
	return nil
}

type EngineBreakdownItem struct {
	Engine           string   `json:"engine"`
	OriginalTokens   int      `json:"original_tokens"`
	CompressedTokens int      `json:"compressed_tokens"`
	SavingsPercent   float64  `json:"savings_percent"`
	TechniquesUsed   []string `json:"techniques_used"`
	RulesApplied     []string `json:"rules_applied,omitempty"`
	DurationMs       int64    `json:"duration_ms,omitempty"`
}

type Stats struct {
	OriginalTokens          int                   `json:"original_tokens"`
	CompressedTokens        int                   `json:"compressed_tokens"`
	SavingsPercent          float64               `json:"savings_percent"`
	Mode                    Mode                  `json:"mode"`
	Engine                  string                `json:"engine,omitempty"`
	TechniquesUsed          []string              `json:"techniques_used"`
	RulesApplied            []string              `json:"rules_applied"`
	PreservedBlockCount     int                   `json:"preserved_block_count"`
	RedactedSecretCount     int                   `json:"redacted_secret_count"`
	CompressionSavedTokens  int                   `json:"compression_saved_tokens"`
	DurationMs              int64                 `json:"duration_ms"`
	Timestamp               int64                 `json:"timestamp"`
	ValidationWarnings      []string              `json:"validation_warnings,omitempty"`
	ValidationErrors        []string              `json:"validation_errors,omitempty"`
	FallbackApplied         bool                  `json:"fallback_applied,omitempty"`
	RtkRawOutputPointers    []RtkRawOutputPointer `json:"rtk_raw_output_pointers,omitempty"`
	Aggressive              *AggressiveStats      `json:"aggressive,omitempty"`
	EngineBreakdown         []EngineBreakdownItem `json:"engine_breakdown,omitempty"`
	Bypassed                bool                  `json:"bypassed"`
	BypassReason            string                `json:"bypass_reason,omitempty"`
	OmniRouteCompatibleMode string                `json:"omniroute_compatible_mode,omitempty"`
}

type Result struct {
	Text       string
	Compressed bool
	Stats      Stats
}

func DefaultConfig() Config {
	return Config{
		Mode:                        ModeOff,
		MinTokens:                   0,
		PreserveSystemPrompt:        true,
		RtkIntensity:                RtkIntensityMinimal,
		RawOutputRetention:          RtkRawOutputRetentionNever,
		RawOutputMaxBytes:           1048576,
		CustomFiltersEnabled:        true,
		TrustProjectFilters:         false,
		MaxLines:                    120,
		MaxChars:                    12000,
		DeduplicateThreshold:        3,
		ApplyToToolResults:          true,
		ApplyToAssistantMessages:    false,
		ApplyToCodeBlocks:           false,
		AggressiveThresholds:        DefaultAggressiveThresholds(),
		AggressiveToolStrategies:    DefaultToolStrategiesConfig(),
		AggressiveToolConfigured:    true,
		AggressiveSummarizerEnabled: true,
		AggressiveSummarizerSet:     true,
		AggressiveMaxTokensPerMsg:   2048,
		AggressiveMinSavingsRate:    0.05,
		AggressiveMinSavingsRateSet: true,
		UltraCompressionRate:        0.5,
		UltraCompressionRateSet:     true,
		UltraMinScoreThreshold:      0.3,
		UltraMinScoreThresholdSet:   true,
		UltraSlmFallbackAggressive:  true,
		UltraSlmFallbackSet:         true,
		UltraMaxTokensPerMessage:    0,
		CavemanIntensity:            IntensityLite,
		CavemanLanguage:             "en",
		CavemanAutoDetectLanguage:   false,
		CavemanEnabledLanguagePacks: []string{"en"},
		StackedPipeline:             DefaultStackedPipeline(),
	}
}

func DefaultAggressiveThresholds() AggressiveThresholds {
	return AggressiveThresholds{FullSummary: 5, Moderate: 3, Light: 2, Verbatim: 2}
}

func DefaultToolStrategiesConfig() ToolStrategiesConfig {
	return ToolStrategiesConfig{
		FileContent:     true,
		GrepSearch:      true,
		ShellOutput:     true,
		JSON:            true,
		ErrorMessage:    true,
		fileContentSet:  true,
		grepSearchSet:   true,
		shellOutputSet:  true,
		jsonSet:         true,
		errorMessageSet: true,
	}
}

func DefaultStackedPipeline() []PipelineStep {
	return []PipelineStep{
		{Engine: "rtk", Intensity: string(RtkIntensityStandard)},
		{Engine: "caveman", Intensity: string(IntensityFull)},
	}
}

func NormalizeRtkRawOutputRetention(value RtkRawOutputRetention) RtkRawOutputRetention {
	switch value {
	case RtkRawOutputRetentionNever, RtkRawOutputRetentionFailures, RtkRawOutputRetentionAlways:
		return value
	default:
		return RtkRawOutputRetentionNever
	}
}

func NormalizeRtkIntensity(value RtkIntensity) RtkIntensity {
	switch value {
	case RtkIntensityMinimal, RtkIntensityStandard, RtkIntensityAggressive:
		return value
	default:
		return RtkIntensityMinimal
	}
}

func NormalizeStackedPipeline(steps []PipelineStep) []PipelineStep {
	if len(steps) == 0 {
		return DefaultStackedPipeline()
	}
	out := make([]PipelineStep, 0, len(steps))
	for _, step := range steps {
		engine := strings.ToLower(strings.TrimSpace(step.Engine))
		intensity := strings.ToLower(strings.TrimSpace(step.Intensity))
		switch engine {
		case "rtk", "caveman", "lite", "aggressive", "ultra":
		case "standard":
			engine = "caveman"
		default:
			engine = "caveman"
		}
		out = append(out, PipelineStep{
			Engine:    engine,
			Intensity: intensity,
		})
	}
	if len(out) == 0 {
		return DefaultStackedPipeline()
	}
	return out
}

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	return (runes + 3) / 4
}

func newStats(mode Mode, original string, started time.Time) Stats {
	return Stats{
		OriginalTokens:          EstimateTokens(original),
		CompressedTokens:        EstimateTokens(original),
		Mode:                    mode,
		Engine:                  string(mode),
		DurationMs:              time.Since(started).Milliseconds(),
		Timestamp:               started.UnixMilli(),
		OmniRouteCompatibleMode: string(mode),
	}
}
