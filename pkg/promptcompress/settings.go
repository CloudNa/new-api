package promptcompress

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const SettingsOptionKey = "glart.prompt_compression.settings"

type Settings struct {
	Enabled              bool               `json:"enabled"`
	DefaultMode          Mode               `json:"default_mode"`
	AutoTriggerMode      Mode               `json:"auto_trigger_mode"`
	AutoTriggerTokens    int                `json:"auto_trigger_tokens"`
	MinTokens            int                `json:"min_tokens"`
	PreserveSystemPrompt bool               `json:"preserve_system_prompt"`
	AllowedGroups        []string           `json:"allowed_groups"`
	GroupModes           map[string]Mode    `json:"group_modes"`
	ModelModes           map[string]Mode    `json:"model_modes"`
	ChannelModes         map[string]Mode    `json:"channel_modes"`
	GlobalKillSwitch     bool               `json:"global_kill_switch"`
	Rtk                  RtkSettings        `json:"rtk"`
	Caveman              CavemanSettings    `json:"caveman"`
	Aggressive           AggressiveSettings `json:"aggressive"`
	Ultra                UltraSettings      `json:"ultra"`
	StackedPipeline      []PipelineStep     `json:"stacked_pipeline"`
	Attribution          string             `json:"attribution"`
}

type RtkSettings struct {
	Intensity                RtkIntensity          `json:"intensity"`
	RawOutputRetention       RtkRawOutputRetention `json:"raw_output_retention"`
	RawOutputMaxBytes        int                   `json:"raw_output_max_bytes"`
	CustomFiltersEnabled     bool                  `json:"custom_filters_enabled"`
	TrustProjectFilters      bool                  `json:"trust_project_filters"`
	MaxLines                 int                   `json:"max_lines"`
	MaxChars                 int                   `json:"max_chars"`
	DeduplicateThreshold     int                   `json:"deduplicate_threshold"`
	EnabledFilters           []string              `json:"enabled_filters"`
	DisabledFilters          []string              `json:"disabled_filters"`
	ApplyToToolResults       bool                  `json:"apply_to_tool_results"`
	ApplyToAssistantMessages bool                  `json:"apply_to_assistant_messages"`
	ApplyToCodeBlocks        bool                  `json:"apply_to_code_blocks"`
	targetFieldsSet          bool
	customFiltersEnabledSet  bool
}

func (r *RtkSettings) UnmarshalJSON(data []byte) error {
	type alias RtkSettings
	var decoded alias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var targetFields struct {
		ApplyToToolResults       *bool `json:"apply_to_tool_results"`
		ApplyToAssistantMessages *bool `json:"apply_to_assistant_messages"`
		ApplyToCodeBlocks        *bool `json:"apply_to_code_blocks"`
		CustomFiltersEnabled     *bool `json:"custom_filters_enabled"`
	}
	if err := common.Unmarshal(data, &targetFields); err != nil {
		return err
	}
	*r = RtkSettings(decoded)
	if targetFields.ApplyToToolResults != nil {
		r.ApplyToToolResults = *targetFields.ApplyToToolResults
	}
	if targetFields.ApplyToAssistantMessages != nil {
		r.ApplyToAssistantMessages = *targetFields.ApplyToAssistantMessages
	}
	if targetFields.ApplyToCodeBlocks != nil {
		r.ApplyToCodeBlocks = *targetFields.ApplyToCodeBlocks
	}
	r.targetFieldsSet = targetFields.ApplyToToolResults != nil || targetFields.ApplyToAssistantMessages != nil || targetFields.ApplyToCodeBlocks != nil
	if targetFields.CustomFiltersEnabled != nil {
		r.CustomFiltersEnabled = *targetFields.CustomFiltersEnabled
		r.customFiltersEnabledSet = true
	}
	return nil
}

type CavemanSettings struct {
	Intensity            Intensity `json:"intensity"`
	CompressRoles        []string  `json:"compress_roles"`
	SkipRules            []string  `json:"skip_rules"`
	MinMessageLength     int       `json:"min_message_length"`
	PreservePatterns     []string  `json:"preserve_patterns"`
	Language             string    `json:"language"`
	AutoDetectLanguage   bool      `json:"auto_detect_language"`
	EnabledLanguagePacks []string  `json:"enabled_language_packs"`
}

type AggressiveSettings struct {
	Thresholds             AggressiveThresholds `json:"thresholds"`
	ToolStrategies         ToolStrategiesConfig `json:"tool_strategies"`
	SummarizerEnabled      bool                 `json:"summarizer_enabled"`
	MaxTokensPerMessage    int                  `json:"max_tokens_per_message"`
	MinSavingsThreshold    float64              `json:"min_savings_threshold"`
	summarizerEnabledSet   bool
	minSavingsThresholdSet bool
}

func (a *AggressiveSettings) UnmarshalJSON(data []byte) error {
	type alias AggressiveSettings
	var decoded alias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields struct {
		SummarizerEnabled   *bool    `json:"summarizer_enabled"`
		MinSavingsThreshold *float64 `json:"min_savings_threshold"`
	}
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = AggressiveSettings(decoded)
	if fields.SummarizerEnabled != nil {
		a.SummarizerEnabled = *fields.SummarizerEnabled
		a.summarizerEnabledSet = true
	}
	if fields.MinSavingsThreshold != nil {
		a.MinSavingsThreshold = *fields.MinSavingsThreshold
		a.minSavingsThresholdSet = true
	}
	return nil
}

type UltraSettings struct {
	CompressionRate         float64 `json:"compression_rate"`
	MinScoreThreshold       float64 `json:"min_score_threshold"`
	SlmFallbackToAggressive bool    `json:"slm_fallback_to_aggressive"`
	ModelPath               string  `json:"model_path,omitempty"`
	MaxTokensPerMessage     int     `json:"max_tokens_per_message"`
	compressionRateSet      bool
	minScoreThresholdSet    bool
	slmFallbackSet          bool
}

func (u *UltraSettings) UnmarshalJSON(data []byte) error {
	type alias UltraSettings
	var decoded alias
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields struct {
		CompressionRate         *float64 `json:"compression_rate"`
		MinScoreThreshold       *float64 `json:"min_score_threshold"`
		SlmFallbackToAggressive *bool    `json:"slm_fallback_to_aggressive"`
	}
	if err := common.Unmarshal(data, &fields); err != nil {
		return err
	}
	*u = UltraSettings(decoded)
	if fields.CompressionRate != nil {
		u.CompressionRate = *fields.CompressionRate
		u.compressionRateSet = true
	}
	if fields.MinScoreThreshold != nil {
		u.MinScoreThreshold = *fields.MinScoreThreshold
		u.minScoreThresholdSet = true
	}
	if fields.SlmFallbackToAggressive != nil {
		u.SlmFallbackToAggressive = *fields.SlmFallbackToAggressive
		u.slmFallbackSet = true
	}
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:              false,
		DefaultMode:          ModeOff,
		AutoTriggerMode:      ModeLite,
		AutoTriggerTokens:    0,
		MinTokens:            0,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{"proxy-test"},
		GroupModes:           map[string]Mode{},
		ModelModes:           map[string]Mode{},
		ChannelModes:         map[string]Mode{},
		GlobalKillSwitch:     false,
		Rtk: RtkSettings{
			Intensity:                RtkIntensityMinimal,
			RawOutputRetention:       RtkRawOutputRetentionNever,
			RawOutputMaxBytes:        1048576,
			CustomFiltersEnabled:     true,
			TrustProjectFilters:      false,
			MaxLines:                 120,
			MaxChars:                 12000,
			DeduplicateThreshold:     3,
			EnabledFilters:           []string{},
			DisabledFilters:          []string{},
			ApplyToToolResults:       true,
			ApplyToAssistantMessages: false,
			ApplyToCodeBlocks:        false,
			targetFieldsSet:          true,
		},
		Caveman: CavemanSettings{
			Intensity:            IntensityLite,
			CompressRoles:        []string{"user"},
			SkipRules:            []string{},
			MinMessageLength:     50,
			PreservePatterns:     []string{},
			Language:             "en",
			AutoDetectLanguage:   false,
			EnabledLanguagePacks: []string{"en"},
		},
		Aggressive: AggressiveSettings{
			Thresholds:             DefaultAggressiveThresholds(),
			ToolStrategies:         DefaultToolStrategiesConfig(),
			SummarizerEnabled:      true,
			MaxTokensPerMessage:    2048,
			MinSavingsThreshold:    0.05,
			summarizerEnabledSet:   true,
			minSavingsThresholdSet: true,
		},
		Ultra: UltraSettings{
			CompressionRate:         0.5,
			MinScoreThreshold:       0.3,
			SlmFallbackToAggressive: true,
			MaxTokensPerMessage:     0,
			compressionRateSet:      true,
			minScoreThresholdSet:    true,
			slmFallbackSet:          true,
		},
		StackedPipeline: DefaultStackedPipeline(),
		Attribution:     Attribution,
	}
}

func (s Settings) Normalize() Settings {
	defaults := DefaultSettings()
	if s.DefaultMode == "" {
		s.DefaultMode = defaults.DefaultMode
	}
	s.DefaultMode = normalizeMode(s.DefaultMode)
	if s.AutoTriggerMode == "" {
		s.AutoTriggerMode = defaults.AutoTriggerMode
	}
	s.AutoTriggerMode = normalizeMode(s.AutoTriggerMode)
	if s.AutoTriggerMode == ModeOff {
		s.AutoTriggerMode = defaults.AutoTriggerMode
	}
	if s.AutoTriggerTokens < 0 {
		s.AutoTriggerTokens = defaults.AutoTriggerTokens
	}
	if s.MinTokens < 0 {
		s.MinTokens = 0
	}
	if len(s.AllowedGroups) == 0 {
		s.AllowedGroups = defaults.AllowedGroups
	}
	s.AllowedGroups = cleanStringSlice(s.AllowedGroups)
	if s.GroupModes == nil {
		s.GroupModes = map[string]Mode{}
	}
	if s.ModelModes == nil {
		s.ModelModes = map[string]Mode{}
	}
	if s.ChannelModes == nil {
		s.ChannelModes = map[string]Mode{}
	}
	s.GroupModes = normalizeModeMap(s.GroupModes)
	s.ModelModes = normalizeModeMap(s.ModelModes)
	s.ChannelModes = normalizeModeMap(s.ChannelModes)
	if s.Rtk.MaxLines <= 0 {
		s.Rtk.MaxLines = defaults.Rtk.MaxLines
	}
	if s.Rtk.MaxChars <= 0 {
		s.Rtk.MaxChars = defaults.Rtk.MaxChars
	}
	if s.Rtk.DeduplicateThreshold < 2 {
		s.Rtk.DeduplicateThreshold = defaults.Rtk.DeduplicateThreshold
	}
	if s.Rtk.Intensity == "" {
		s.Rtk.Intensity = defaults.Rtk.Intensity
	}
	s.Rtk.Intensity = NormalizeRtkIntensity(s.Rtk.Intensity)
	if s.Rtk.RawOutputRetention == "" {
		s.Rtk.RawOutputRetention = defaults.Rtk.RawOutputRetention
	}
	s.Rtk.RawOutputRetention = NormalizeRtkRawOutputRetention(s.Rtk.RawOutputRetention)
	if s.Rtk.RawOutputMaxBytes == 0 {
		s.Rtk.RawOutputMaxBytes = defaults.Rtk.RawOutputMaxBytes
	} else if s.Rtk.RawOutputMaxBytes > 0 && s.Rtk.RawOutputMaxBytes < 1024 {
		s.Rtk.RawOutputMaxBytes = 1024
	}
	if !s.Rtk.customFiltersEnabledSet {
		s.Rtk.CustomFiltersEnabled = defaults.Rtk.CustomFiltersEnabled
	}
	s.Rtk.EnabledFilters = cleanStringSlice(s.Rtk.EnabledFilters)
	s.Rtk.DisabledFilters = cleanStringSlice(s.Rtk.DisabledFilters)
	if !s.Rtk.targetFieldsSet && !s.Rtk.ApplyToToolResults && !s.Rtk.ApplyToAssistantMessages && !s.Rtk.ApplyToCodeBlocks {
		s.Rtk.ApplyToToolResults = defaults.Rtk.ApplyToToolResults
	}
	if s.Caveman.Intensity == "" {
		s.Caveman.Intensity = defaults.Caveman.Intensity
	}
	if !validIntensity(s.Caveman.Intensity) {
		s.Caveman.Intensity = defaults.Caveman.Intensity
	}
	if len(s.Caveman.CompressRoles) == 0 {
		s.Caveman.CompressRoles = defaults.Caveman.CompressRoles
	}
	s.Caveman.CompressRoles = cleanStringSlice(s.Caveman.CompressRoles)
	s.Caveman.SkipRules = cleanStringSlice(s.Caveman.SkipRules)
	s.Caveman.PreservePatterns = cleanStringSlice(s.Caveman.PreservePatterns)
	if s.Caveman.MinMessageLength <= 0 {
		s.Caveman.MinMessageLength = defaults.Caveman.MinMessageLength
	}
	s.Caveman.Language = strings.TrimSpace(s.Caveman.Language)
	if s.Caveman.Language == "" {
		s.Caveman.Language = defaults.Caveman.Language
	}
	if len(s.Caveman.EnabledLanguagePacks) == 0 {
		s.Caveman.EnabledLanguagePacks = defaults.Caveman.EnabledLanguagePacks
	}
	s.Caveman.EnabledLanguagePacks = cleanStringSlice(s.Caveman.EnabledLanguagePacks)
	s.Aggressive.Thresholds = normalizeAggressiveThresholds(s.Aggressive.Thresholds, defaults.Aggressive.Thresholds)
	s.Aggressive.ToolStrategies = normalizeToolStrategies(s.Aggressive.ToolStrategies, defaults.Aggressive.ToolStrategies)
	if !s.Aggressive.summarizerEnabledSet {
		s.Aggressive.SummarizerEnabled = defaults.Aggressive.SummarizerEnabled
	}
	if s.Aggressive.MaxTokensPerMessage <= 0 {
		s.Aggressive.MaxTokensPerMessage = defaults.Aggressive.MaxTokensPerMessage
	}
	if !s.Aggressive.minSavingsThresholdSet && s.Aggressive.MinSavingsThreshold == 0 {
		s.Aggressive.MinSavingsThreshold = defaults.Aggressive.MinSavingsThreshold
	}
	if s.Aggressive.MinSavingsThreshold < 0 || s.Aggressive.MinSavingsThreshold > 1 {
		s.Aggressive.MinSavingsThreshold = defaults.Aggressive.MinSavingsThreshold
	}
	if !s.Ultra.compressionRateSet && s.Ultra.CompressionRate == 0 {
		s.Ultra.CompressionRate = defaults.Ultra.CompressionRate
	}
	if s.Ultra.CompressionRate < 0 || s.Ultra.CompressionRate > 1 {
		s.Ultra.CompressionRate = defaults.Ultra.CompressionRate
	}
	if !s.Ultra.minScoreThresholdSet && s.Ultra.MinScoreThreshold == 0 {
		s.Ultra.MinScoreThreshold = defaults.Ultra.MinScoreThreshold
	}
	if s.Ultra.MinScoreThreshold < 0 || s.Ultra.MinScoreThreshold > 1 {
		s.Ultra.MinScoreThreshold = defaults.Ultra.MinScoreThreshold
	}
	if !s.Ultra.slmFallbackSet {
		s.Ultra.SlmFallbackToAggressive = defaults.Ultra.SlmFallbackToAggressive
	}
	if s.Ultra.MaxTokensPerMessage < 0 {
		s.Ultra.MaxTokensPerMessage = defaults.Ultra.MaxTokensPerMessage
	}
	s.Ultra.ModelPath = strings.TrimSpace(s.Ultra.ModelPath)
	s.StackedPipeline = NormalizeStackedPipeline(s.StackedPipeline)
	s.Attribution = Attribution
	return s
}

func (s Settings) Validate() error {
	normalized := s.Normalize()
	if normalized.DefaultMode != s.DefaultMode && s.DefaultMode != "" {
		return errors.New("invalid default_mode")
	}
	if normalized.AutoTriggerMode != s.AutoTriggerMode && s.AutoTriggerMode != "" {
		return errors.New("invalid auto_trigger_mode")
	}
	if !validIntensity(normalized.Caveman.Intensity) {
		return errors.New("invalid caveman intensity")
	}
	if s.Rtk.Intensity != "" && !validRtkIntensity(s.Rtk.Intensity) {
		return errors.New("invalid rtk intensity")
	}
	if s.Rtk.RawOutputRetention != "" && !validRtkRawOutputRetention(s.Rtk.RawOutputRetention) {
		return errors.New("invalid rtk raw_output_retention")
	}
	if s.Rtk.RawOutputMaxBytes < 0 {
		return errors.New("invalid rtk raw_output_max_bytes")
	}
	if hasInvalidAggressiveThreshold(s.Aggressive.Thresholds) {
		return errors.New("invalid aggressive thresholds")
	}
	if s.Aggressive.MaxTokensPerMessage < 0 {
		return errors.New("invalid aggressive max_tokens_per_message")
	}
	if s.Aggressive.MinSavingsThreshold < 0 || s.Aggressive.MinSavingsThreshold > 1 {
		return errors.New("invalid aggressive min_savings_threshold")
	}
	if s.Ultra.CompressionRate < 0 || s.Ultra.CompressionRate > 1 {
		return errors.New("invalid ultra compression_rate")
	}
	if s.Ultra.MinScoreThreshold < 0 || s.Ultra.MinScoreThreshold > 1 {
		return errors.New("invalid ultra min_score_threshold")
	}
	if s.Ultra.MaxTokensPerMessage < 0 {
		return errors.New("invalid ultra max_tokens_per_message")
	}
	if err := validateStackedPipeline(s.StackedPipeline); err != nil {
		return err
	}
	return nil
}

func (s Settings) ToConfig(mode Mode) Config {
	s = s.Normalize()
	if mode == "" {
		mode = s.DefaultMode
	}
	return Config{
		Mode:                        normalizeMode(mode),
		MinTokens:                   s.MinTokens,
		PreserveSystemPrompt:        s.PreserveSystemPrompt,
		RtkIntensity:                s.Rtk.Intensity,
		RawOutputRetention:          s.Rtk.RawOutputRetention,
		RawOutputMaxBytes:           s.Rtk.RawOutputMaxBytes,
		CustomFiltersEnabled:        s.Rtk.CustomFiltersEnabled,
		TrustProjectFilters:         s.Rtk.TrustProjectFilters,
		MaxLines:                    s.Rtk.MaxLines,
		MaxChars:                    s.Rtk.MaxChars,
		DeduplicateThreshold:        s.Rtk.DeduplicateThreshold,
		EnabledFilters:              append([]string(nil), s.Rtk.EnabledFilters...),
		DisabledFilters:             append([]string(nil), s.Rtk.DisabledFilters...),
		ApplyToToolResults:          s.Rtk.ApplyToToolResults,
		ApplyToAssistantMessages:    s.Rtk.ApplyToAssistantMessages,
		ApplyToCodeBlocks:           s.Rtk.ApplyToCodeBlocks,
		AggressiveThresholds:        s.Aggressive.Thresholds,
		AggressiveToolStrategies:    s.Aggressive.ToolStrategies,
		AggressiveToolConfigured:    true,
		AggressiveSummarizerEnabled: s.Aggressive.SummarizerEnabled,
		AggressiveSummarizerSet:     true,
		AggressiveMaxTokensPerMsg:   s.Aggressive.MaxTokensPerMessage,
		AggressiveMinSavingsRate:    s.Aggressive.MinSavingsThreshold,
		AggressiveMinSavingsRateSet: true,
		UltraCompressionRate:        s.Ultra.CompressionRate,
		UltraCompressionRateSet:     true,
		UltraMinScoreThreshold:      s.Ultra.MinScoreThreshold,
		UltraMinScoreThresholdSet:   true,
		UltraSlmFallbackAggressive:  s.Ultra.SlmFallbackToAggressive,
		UltraSlmFallbackSet:         true,
		UltraModelPath:              s.Ultra.ModelPath,
		UltraMaxTokensPerMessage:    s.Ultra.MaxTokensPerMessage,
		CavemanSkipRules:            append([]string(nil), s.Caveman.SkipRules...),
		CavemanPreservePatterns:     append([]string(nil), s.Caveman.PreservePatterns...),
		CavemanIntensity:            s.Caveman.Intensity,
		CavemanLanguage:             s.Caveman.Language,
		CavemanAutoDetectLanguage:   s.Caveman.AutoDetectLanguage,
		CavemanEnabledLanguagePacks: append([]string(nil), s.Caveman.EnabledLanguagePacks...),
		StackedPipeline:             append([]PipelineStep(nil), s.StackedPipeline...),
	}
}

func validRtkRawOutputRetention(value RtkRawOutputRetention) bool {
	switch value {
	case RtkRawOutputRetentionNever, RtkRawOutputRetentionFailures, RtkRawOutputRetentionAlways:
		return true
	default:
		return false
	}
}

func validRtkIntensity(value RtkIntensity) bool {
	switch value {
	case RtkIntensityMinimal, RtkIntensityStandard, RtkIntensityAggressive:
		return true
	default:
		return false
	}
}

func validIntensity(value Intensity) bool {
	switch value {
	case IntensityLite, IntensityFull, IntensityStandard, IntensityAggressive, IntensityUltra:
		return true
	default:
		return false
	}
}

func validateStackedPipeline(steps []PipelineStep) error {
	for _, step := range steps {
		engine := strings.ToLower(strings.TrimSpace(step.Engine))
		intensity := strings.ToLower(strings.TrimSpace(step.Intensity))
		switch engine {
		case "", "standard", "caveman":
			if intensity != "" && !validIntensity(Intensity(intensity)) {
				return errors.New("invalid stacked_pipeline caveman intensity")
			}
		case "rtk":
			if intensity != "" && !validRtkIntensity(RtkIntensity(intensity)) {
				return errors.New("invalid stacked_pipeline rtk intensity")
			}
		case "lite", "aggressive", "ultra":
			if intensity != "" && !validIntensity(Intensity(intensity)) && !validRtkIntensity(RtkIntensity(intensity)) {
				return errors.New("invalid stacked_pipeline intensity")
			}
		default:
			return errors.New("invalid stacked_pipeline engine")
		}
	}
	return nil
}

func hasInvalidAggressiveThreshold(thresholds AggressiveThresholds) bool {
	return thresholds.FullSummary < 0 || thresholds.Moderate < 0 || thresholds.Light < 0 || thresholds.Verbatim < 0
}

func normalizeAggressiveThresholds(thresholds AggressiveThresholds, defaults AggressiveThresholds) AggressiveThresholds {
	if thresholds.FullSummary <= 0 {
		thresholds.FullSummary = defaults.FullSummary
	}
	if thresholds.Moderate <= 0 {
		thresholds.Moderate = defaults.Moderate
	}
	if thresholds.Light <= 0 {
		thresholds.Light = defaults.Light
	}
	if thresholds.Verbatim <= 0 {
		thresholds.Verbatim = defaults.Verbatim
	}
	return thresholds
}

func normalizeToolStrategies(value ToolStrategiesConfig, defaults ToolStrategiesConfig) ToolStrategiesConfig {
	if !value.fileContentSet {
		value.FileContent = defaults.FileContent
	}
	if !value.grepSearchSet {
		value.GrepSearch = defaults.GrepSearch
	}
	if !value.shellOutputSet {
		value.ShellOutput = defaults.ShellOutput
	}
	if !value.jsonSet {
		value.JSON = defaults.JSON
	}
	if !value.errorMessageSet {
		value.ErrorMessage = defaults.ErrorMessage
	}
	return value
}

func normalizeModeMap(items map[string]Mode) map[string]Mode {
	out := make(map[string]Mode, len(items))
	for key, mode := range items {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = normalizeMode(mode)
	}
	return out
}

func cleanStringSlice(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
