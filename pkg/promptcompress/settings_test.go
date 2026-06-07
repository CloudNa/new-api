package promptcompress

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRtkTargetDefaultsPreserveExplicitFalse(t *testing.T) {
	defaults := Settings{}.Normalize()
	require.True(t, defaults.Rtk.ApplyToToolResults)
	require.False(t, defaults.Rtk.ApplyToAssistantMessages)
	require.False(t, defaults.Rtk.ApplyToCodeBlocks)
	require.True(t, defaults.Rtk.CustomFiltersEnabled)
	require.False(t, defaults.Rtk.TrustProjectFilters)
	require.Equal(t, RtkRawOutputRetentionNever, defaults.Rtk.RawOutputRetention)
	require.Equal(t, 1048576, defaults.Rtk.RawOutputMaxBytes)

	var omitted Settings
	require.NoError(t, common.Unmarshal([]byte(`{"rtk":{"max_lines":80}}`), &omitted))
	omitted = omitted.Normalize()
	require.True(t, omitted.Rtk.ApplyToToolResults)
	require.False(t, omitted.Rtk.ApplyToAssistantMessages)
	require.False(t, omitted.Rtk.ApplyToCodeBlocks)
	require.True(t, omitted.Rtk.CustomFiltersEnabled)

	var explicit Settings
	require.NoError(t, common.Unmarshal([]byte(`{"rtk":{"apply_to_tool_results":false,"apply_to_assistant_messages":false,"apply_to_code_blocks":false,"custom_filters_enabled":false}}`), &explicit))
	explicit = explicit.Normalize()
	require.False(t, explicit.Rtk.ApplyToToolResults)
	require.False(t, explicit.Rtk.ApplyToAssistantMessages)
	require.False(t, explicit.Rtk.ApplyToCodeBlocks)
	require.False(t, explicit.Rtk.CustomFiltersEnabled)
}

func TestOmniRouteStackedPipelineSettingsParity(t *testing.T) {
	defaults := Settings{}.Normalize()
	require.Equal(t, RtkIntensityMinimal, defaults.Rtk.Intensity)
	require.Equal(t, DefaultStackedPipeline(), defaults.StackedPipeline)

	var settings Settings
	require.NoError(t, common.Unmarshal([]byte(`{
		"rtk": {"intensity": "aggressive"},
		"stacked_pipeline": [
			"rtk",
			{"engine": "standard"},
			{"engine": "caveman", "intensity": "full"}
		]
	}`), &settings))

	settings = settings.Normalize()
	require.Equal(t, RtkIntensityAggressive, settings.Rtk.Intensity)
	require.Equal(t, []PipelineStep{
		{Engine: "rtk"},
		{Engine: "caveman"},
		{Engine: "caveman", Intensity: "full"},
	}, settings.StackedPipeline)

	config := settings.ToConfig(ModeStacked)
	require.Equal(t, RtkIntensityAggressive, config.RtkIntensity)
	require.Equal(t, settings.StackedPipeline, config.StackedPipeline)
}

func TestCompressionSettingsValidatePipeline(t *testing.T) {
	settings := DefaultSettings()
	settings.StackedPipeline = []PipelineStep{{Engine: "unknown"}}
	require.ErrorContains(t, settings.Validate(), "invalid stacked_pipeline engine")

	settings = DefaultSettings()
	settings.Rtk.Intensity = RtkIntensity("maximum")
	require.ErrorContains(t, settings.Validate(), "invalid rtk intensity")

	settings = DefaultSettings()
	settings.StackedPipeline = []PipelineStep{{Engine: "rtk", Intensity: "maximum"}}
	require.ErrorContains(t, settings.Validate(), "invalid stacked_pipeline rtk intensity")
}

func TestCompressionSettingsValidateRtkRawOutput(t *testing.T) {
	settings := DefaultSettings()
	settings.Rtk.RawOutputRetention = RtkRawOutputRetention("sometimes")
	require.ErrorContains(t, settings.Validate(), "invalid rtk raw_output_retention")

	settings = DefaultSettings()
	settings.Rtk.RawOutputMaxBytes = -1
	require.ErrorContains(t, settings.Validate(), "invalid rtk raw_output_max_bytes")
}

func TestCompressionSettingsNormalizeRtkRawOutput(t *testing.T) {
	var settings Settings
	require.NoError(t, common.Unmarshal([]byte(`{
		"rtk": {
			"raw_output_retention": "always",
			"raw_output_max_bytes": 128,
			"custom_filters_enabled": false,
			"trust_project_filters": true
		}
	}`), &settings))

	settings = settings.Normalize()
	require.Equal(t, RtkRawOutputRetentionAlways, settings.Rtk.RawOutputRetention)
	require.Equal(t, 1024, settings.Rtk.RawOutputMaxBytes)
	require.False(t, settings.Rtk.CustomFiltersEnabled)
	require.True(t, settings.Rtk.TrustProjectFilters)

	config := settings.ToConfig(ModeRTK)
	require.Equal(t, RtkRawOutputRetentionAlways, config.RawOutputRetention)
	require.Equal(t, 1024, config.RawOutputMaxBytes)
	require.False(t, config.CustomFiltersEnabled)
	require.True(t, config.TrustProjectFilters)
}

func TestCompressionSettingsAggressiveUltraDefaultsParity(t *testing.T) {
	settings := Settings{}.Normalize()

	require.Equal(t, DefaultAggressiveThresholds(), settings.Aggressive.Thresholds)
	require.True(t, settings.Aggressive.ToolStrategies.FileContent)
	require.True(t, settings.Aggressive.ToolStrategies.GrepSearch)
	require.True(t, settings.Aggressive.ToolStrategies.ShellOutput)
	require.True(t, settings.Aggressive.ToolStrategies.JSON)
	require.True(t, settings.Aggressive.ToolStrategies.ErrorMessage)
	require.True(t, settings.Aggressive.SummarizerEnabled)
	require.Equal(t, 2048, settings.Aggressive.MaxTokensPerMessage)
	require.Equal(t, 0.05, settings.Aggressive.MinSavingsThreshold)
	require.Equal(t, 0.5, settings.Ultra.CompressionRate)
	require.Equal(t, 0.3, settings.Ultra.MinScoreThreshold)
	require.True(t, settings.Ultra.SlmFallbackToAggressive)
	require.Equal(t, 0, settings.Ultra.MaxTokensPerMessage)

	config := settings.ToConfig(ModeUltra)
	require.Equal(t, 0.5, config.UltraCompressionRate)
	require.True(t, config.UltraCompressionRateSet)
	require.Equal(t, 0.3, config.UltraMinScoreThreshold)
	require.True(t, config.UltraMinScoreThresholdSet)
	require.True(t, config.UltraSlmFallbackAggressive)
	require.True(t, config.UltraSlmFallbackSet)
}

func TestCompressionSettingsPreserveAggressiveUltraExplicitFalseAndZero(t *testing.T) {
	var settings Settings
	require.NoError(t, common.Unmarshal([]byte(`{
		"aggressive": {
			"summarizer_enabled": false,
			"min_savings_threshold": 0,
			"tool_strategies": {"json": false}
		},
		"ultra": {
			"compression_rate": 0,
			"min_score_threshold": 0,
			"slm_fallback_to_aggressive": false,
			"model_path": "  /models/slm.onnx  "
		}
	}`), &settings))

	settings = settings.Normalize()
	require.False(t, settings.Aggressive.SummarizerEnabled)
	require.Equal(t, 0.0, settings.Aggressive.MinSavingsThreshold)
	require.False(t, settings.Aggressive.ToolStrategies.JSON)
	require.True(t, settings.Aggressive.ToolStrategies.FileContent)
	require.True(t, settings.Aggressive.ToolStrategies.GrepSearch)
	require.True(t, settings.Aggressive.ToolStrategies.ShellOutput)
	require.True(t, settings.Aggressive.ToolStrategies.ErrorMessage)
	require.Equal(t, 0.0, settings.Ultra.CompressionRate)
	require.Equal(t, 0.0, settings.Ultra.MinScoreThreshold)
	require.False(t, settings.Ultra.SlmFallbackToAggressive)
	require.Equal(t, "/models/slm.onnx", settings.Ultra.ModelPath)

	config := settings.ToConfig(ModeAggressive)
	require.False(t, config.AggressiveSummarizerEnabled)
	require.True(t, config.AggressiveSummarizerSet)
	require.Equal(t, 0.0, config.AggressiveMinSavingsRate)
	require.True(t, config.AggressiveMinSavingsRateSet)
	require.False(t, config.AggressiveToolStrategies.JSON)
	require.True(t, config.AggressiveToolConfigured)
	require.Equal(t, 0.0, config.UltraCompressionRate)
	require.True(t, config.UltraCompressionRateSet)
	require.Equal(t, 0.0, config.UltraMinScoreThreshold)
	require.True(t, config.UltraMinScoreThresholdSet)
	require.False(t, config.UltraSlmFallbackAggressive)
	require.True(t, config.UltraSlmFallbackSet)
	require.Equal(t, "/models/slm.onnx", config.UltraModelPath)
}
