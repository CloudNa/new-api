package promptcompress

import "time"

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
	IntensityStandard   Intensity = "standard"
	IntensityAggressive Intensity = "aggressive"
	IntensityUltra      Intensity = "ultra"
)

type Config struct {
	Mode                 Mode
	MinTokens            int
	PreserveSystemPrompt bool
	MaxLines             int
	MaxChars             int
	DeduplicateThreshold int
}

type Message struct {
	Role    string
	Content string
}

type Stats struct {
	OriginalTokens          int      `json:"original_tokens"`
	CompressedTokens        int      `json:"compressed_tokens"`
	SavingsPercent          float64  `json:"savings_percent"`
	Mode                    Mode     `json:"mode"`
	TechniquesUsed          []string `json:"techniques_used"`
	RulesApplied            []string `json:"rules_applied"`
	PreservedBlockCount     int      `json:"preserved_block_count"`
	RedactedSecretCount     int      `json:"redacted_secret_count"`
	CompressionSavedTokens  int      `json:"compression_saved_tokens"`
	DurationMs              int64    `json:"duration_ms"`
	Bypassed                bool     `json:"bypassed"`
	BypassReason            string   `json:"bypass_reason,omitempty"`
	OmniRouteCompatibleMode string   `json:"omniroute_compatible_mode,omitempty"`
}

type Result struct {
	Text       string
	Compressed bool
	Stats      Stats
}

func DefaultConfig() Config {
	return Config{
		Mode:                 ModeOff,
		MinTokens:            0,
		PreserveSystemPrompt: true,
		MaxLines:             120,
		MaxChars:             12000,
		DeduplicateThreshold: 3,
	}
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
		DurationMs:              time.Since(started).Milliseconds(),
		OmniRouteCompatibleMode: string(mode),
	}
}
