package promptcompress

import (
	"errors"
	"strings"
)

const SettingsOptionKey = "glart.prompt_compression.settings"

type Settings struct {
	Enabled              bool            `json:"enabled"`
	DefaultMode          Mode            `json:"default_mode"`
	AutoTriggerMode      Mode            `json:"auto_trigger_mode"`
	AutoTriggerTokens    int             `json:"auto_trigger_tokens"`
	MinTokens            int             `json:"min_tokens"`
	PreserveSystemPrompt bool            `json:"preserve_system_prompt"`
	AllowedGroups        []string        `json:"allowed_groups"`
	GroupModes           map[string]Mode `json:"group_modes"`
	ModelModes           map[string]Mode `json:"model_modes"`
	ChannelModes         map[string]Mode `json:"channel_modes"`
	GlobalKillSwitch     bool            `json:"global_kill_switch"`
	Rtk                  RtkSettings     `json:"rtk"`
	Caveman              CavemanSettings `json:"caveman"`
	Attribution          string          `json:"attribution"`
}

type RtkSettings struct {
	MaxLines             int      `json:"max_lines"`
	MaxChars             int      `json:"max_chars"`
	DeduplicateThreshold int      `json:"deduplicate_threshold"`
	EnabledFilters       []string `json:"enabled_filters"`
	DisabledFilters      []string `json:"disabled_filters"`
}

type CavemanSettings struct {
	Intensity        Intensity `json:"intensity"`
	CompressRoles    []string  `json:"compress_roles"`
	MinMessageLength int       `json:"min_message_length"`
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:              false,
		DefaultMode:          ModeOff,
		AutoTriggerMode:      ModeStacked,
		AutoTriggerTokens:    32000,
		MinTokens:            0,
		PreserveSystemPrompt: true,
		AllowedGroups:        []string{"proxy-test"},
		GroupModes:           map[string]Mode{},
		ModelModes:           map[string]Mode{},
		ChannelModes:         map[string]Mode{},
		GlobalKillSwitch:     false,
		Rtk: RtkSettings{
			MaxLines:             120,
			MaxChars:             12000,
			DeduplicateThreshold: 3,
			EnabledFilters:       []string{},
			DisabledFilters:      []string{},
		},
		Caveman: CavemanSettings{
			Intensity:        IntensityStandard,
			CompressRoles:    []string{"user"},
			MinMessageLength: 50,
		},
		Attribution: Attribution,
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
	s.Rtk.EnabledFilters = cleanStringSlice(s.Rtk.EnabledFilters)
	s.Rtk.DisabledFilters = cleanStringSlice(s.Rtk.DisabledFilters)
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
	if s.Caveman.MinMessageLength <= 0 {
		s.Caveman.MinMessageLength = defaults.Caveman.MinMessageLength
	}
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
	return nil
}

func (s Settings) ToConfig(mode Mode) Config {
	s = s.Normalize()
	if mode == "" {
		mode = s.DefaultMode
	}
	return Config{
		Mode:                 normalizeMode(mode),
		MinTokens:            s.MinTokens,
		PreserveSystemPrompt: s.PreserveSystemPrompt,
		MaxLines:             s.Rtk.MaxLines,
		MaxChars:             s.Rtk.MaxChars,
		DeduplicateThreshold: s.Rtk.DeduplicateThreshold,
	}
}

func validIntensity(value Intensity) bool {
	switch value {
	case IntensityLite, IntensityStandard, IntensityAggressive, IntensityUltra:
		return true
	default:
		return false
	}
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
