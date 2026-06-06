package profit

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	SettingsOptionKey     = "glart.profit.settings"
	CostProfilesOptionKey = "glart.profit.cost_profiles"

	ModeOff             = "off"
	ModeObserve         = "observe"
	ModeCap             = "cap"
	ModePremiumRequired = "premium_required"

	RiskModeAlert = "alert"
)

type Settings struct {
	Version          int            `json:"version"`
	Enabled          bool           `json:"enabled"`
	ObserveOnly      bool           `json:"observe_only"`
	ObserveGroups    []string       `json:"observe_groups"`
	GlobalKillSwitch bool           `json:"global_kill_switch"`
	CostRoutingMode  string         `json:"cost_routing_mode"`
	CacheMode        string         `json:"cache_mode"`
	OutputCapMode    string         `json:"output_cap_mode"`
	RiskEnforcement  string         `json:"risk_enforcement"`
	SettingsWritable bool           `json:"settings_writable"`
	CostProfilesUsed bool           `json:"cost_profiles_used"`
	OutputPolicies   []OutputPolicy `json:"output_policies,omitempty"`
}

type OutputPolicy struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority,omitempty"`
	Group            string `json:"group,omitempty"`
	ModelName        string `json:"model_name,omitempty"`
	ChannelID        int    `json:"channel_id,omitempty"`
	ChannelName      string `json:"channel_name,omitempty"`
	Mode             string `json:"mode,omitempty"`
	DefaultMaxTokens int    `json:"default_max_tokens,omitempty"`
	HardMaxTokens    int    `json:"hard_max_tokens,omitempty"`
	RewriteOverLimit bool   `json:"rewrite_over_limit,omitempty"`
	PremiumRequired  bool   `json:"premium_required,omitempty"`
	PremiumGroup     string `json:"premium_group,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{
		Version:          ObservationVersion,
		Enabled:          true,
		ObserveOnly:      true,
		ObserveGroups:    []string{DefaultObserveGroup},
		GlobalKillSwitch: false,
		CostRoutingMode:  ModeObserve,
		CacheMode:        ModeOff,
		OutputCapMode:    ModeOff,
		RiskEnforcement:  ModeOff,
		SettingsWritable: true,
		CostProfilesUsed: true,
	}
}

func LoadSettings() (Settings, error) {
	settings := DefaultSettings()
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[SettingsOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(raw) == "" {
		return settings, nil
	}
	if err := common.UnmarshalJsonStr(raw, &settings); err != nil {
		return Settings{}, errors.New("invalid stored profit settings")
	}
	if err := settings.Validate(); err != nil {
		return Settings{}, err
	}
	return settings.Normalize(), nil
}

func CurrentSettings() Settings {
	settings, err := LoadSettings()
	if err != nil {
		return DefaultSettings()
	}
	return settings
}

func (s Settings) Normalize() Settings {
	defaults := DefaultSettings()
	s.Version = ObservationVersion
	s.ObserveOnly = true
	s.SettingsWritable = true
	s.CostProfilesUsed = true
	if len(s.ObserveGroups) == 0 {
		s.ObserveGroups = defaults.ObserveGroups
	}
	s.ObserveGroups = cleanStringSlice(s.ObserveGroups)
	if !validOffObserveMode(s.CostRoutingMode) {
		s.CostRoutingMode = defaults.CostRoutingMode
	}
	if !validOffObserveMode(s.CacheMode) {
		s.CacheMode = defaults.CacheMode
	}
	if !validOutputCapMode(s.OutputCapMode) {
		s.OutputCapMode = defaults.OutputCapMode
	}
	s.OutputPolicies = normalizeOutputPolicies(s.OutputPolicies)
	if s.RiskEnforcement != ModeOff && s.RiskEnforcement != RiskModeAlert {
		s.RiskEnforcement = defaults.RiskEnforcement
	}
	return s
}

func (s Settings) Validate() error {
	if s.CostRoutingMode != "" && !validOffObserveMode(s.CostRoutingMode) {
		return errors.New("invalid cost_routing_mode")
	}
	if s.CacheMode != "" && !validOffObserveMode(s.CacheMode) {
		return errors.New("invalid cache_mode")
	}
	if s.OutputCapMode != "" && !validOutputCapMode(s.OutputCapMode) {
		return errors.New("invalid output_cap_mode")
	}
	for _, policy := range s.OutputPolicies {
		mode := strings.TrimSpace(policy.Mode)
		if mode != "" && !validOutputCapMode(mode) {
			return errors.New("invalid output policy mode")
		}
		if policy.DefaultMaxTokens < 0 || policy.HardMaxTokens < 0 {
			return errors.New("output policy token limits must be non-negative")
		}
	}
	if s.RiskEnforcement != "" && s.RiskEnforcement != ModeOff && s.RiskEnforcement != RiskModeAlert {
		return errors.New("invalid risk_enforcement")
	}
	return nil
}

func validOffObserveMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve
}

func validOutputCapMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve || mode == ModeCap || mode == ModePremiumRequired
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

func normalizeOutputPolicies(policies []OutputPolicy) []OutputPolicy {
	out := make([]OutputPolicy, 0, len(policies))
	for _, policy := range policies {
		policy.ID = strings.TrimSpace(policy.ID)
		policy.Name = strings.TrimSpace(policy.Name)
		policy.Group = strings.TrimSpace(policy.Group)
		policy.ModelName = strings.TrimSpace(policy.ModelName)
		policy.ChannelName = strings.TrimSpace(policy.ChannelName)
		policy.Mode = strings.TrimSpace(policy.Mode)
		policy.PremiumGroup = strings.TrimSpace(policy.PremiumGroup)
		policy.Notes = strings.TrimSpace(policy.Notes)
		if policy.Mode == "" {
			policy.Mode = ModeObserve
		}
		if !validOutputCapMode(policy.Mode) {
			policy.Mode = ModeObserve
		}
		if policy.DefaultMaxTokens < 0 {
			policy.DefaultMaxTokens = 0
		}
		if policy.HardMaxTokens < 0 {
			policy.HardMaxTokens = 0
		}
		if policy.ID == "" && policy.Group == "" && policy.ModelName == "" && policy.ChannelID == 0 && policy.ChannelName == "" {
			continue
		}
		out = append(out, policy)
	}
	return out
}
