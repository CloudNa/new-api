package profit

import (
	"errors"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	SettingsOptionKey     = "glart.profit.settings"
	CostProfilesOptionKey = "glart.profit.cost_profiles"

	ModeOff             = "off"
	ModeObserve         = "observe"
	ModePreferMargin    = "prefer_margin"
	ModeCap             = "cap"
	ModePremiumRequired = "premium_required"
	ModeEnforce         = "enforce"

	RiskModeAlert = "alert"

	DefaultCostRoutingMinSamples        = 20
	DefaultCostRoutingMinSuccessRatePct = 95
	DefaultCostRoutingHealthWindowHours = 24
)

type Settings struct {
	Version                      int                 `json:"version"`
	Enabled                      bool                `json:"enabled"`
	ObserveOnly                  bool                `json:"observe_only"`
	ObserveGroups                []string            `json:"observe_groups"`
	GlobalKillSwitch             bool                `json:"global_kill_switch"`
	CostRoutingMode              string              `json:"cost_routing_mode"`
	CostRoutingMinSamples        int                 `json:"cost_routing_min_samples,omitempty"`
	CostRoutingMinSuccessRatePct float64             `json:"cost_routing_min_success_rate_pct,omitempty"`
	CostRoutingHealthWindowHours int                 `json:"cost_routing_health_window_hours,omitempty"`
	CacheMode                    string              `json:"cache_mode"`
	LongContextMode              string              `json:"long_context_mode"`
	OutputCapMode                string              `json:"output_cap_mode"`
	ModelAliasMode               string              `json:"model_alias_mode"`
	RetryBudgetMode              string              `json:"retry_budget_mode"`
	MaxRetryCostUSD              float64             `json:"max_retry_cost_usd,omitempty"`
	RetryLowMarginSkip           bool                `json:"retry_low_margin_skip,omitempty"`
	RiskEnforcement              string              `json:"risk_enforcement"`
	RiskMinGrossUSD              float64             `json:"risk_min_gross_margin_usd,omitempty"`
	RiskMinGrossPct              float64             `json:"risk_min_gross_margin_pct,omitempty"`
	RiskMinExpectUSD             float64             `json:"risk_min_expected_margin_usd,omitempty"`
	RiskMinExpectPct             float64             `json:"risk_min_expected_margin_pct,omitempty"`
	SettingsWritable             bool                `json:"settings_writable"`
	CostProfilesUsed             bool                `json:"cost_profiles_used"`
	LongContextPolicies          []LongContextPolicy `json:"long_context_policies,omitempty"`
	OutputPolicies               []OutputPolicy      `json:"output_policies,omitempty"`
	ModelAliases                 []ModelAlias        `json:"model_aliases,omitempty"`
	ResponseCacheRules           []ResponseCacheRule `json:"response_cache_rules,omitempty"`
}

type LongContextPolicy struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Enabled      bool              `json:"enabled"`
	Priority     int               `json:"priority,omitempty"`
	Group        string            `json:"group,omitempty"`
	ModelName    string            `json:"model_name,omitempty"`
	ChannelID    int               `json:"channel_id,omitempty"`
	ChannelName  string            `json:"channel_name,omitempty"`
	Mode         string            `json:"mode,omitempty"`
	PremiumGroup string            `json:"premium_group,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Tiers        []LongContextTier `json:"tiers,omitempty"`
}

type LongContextTier struct {
	ID               string  `json:"id"`
	Name             string  `json:"name,omitempty"`
	MinContextTokens int     `json:"min_context_tokens,omitempty"`
	MaxContextTokens int     `json:"max_context_tokens,omitempty"`
	InputMultiplier  float64 `json:"input_multiplier"`
	PremiumRequired  bool    `json:"premium_required,omitempty"`
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

type ModelAlias struct {
	ID       string             `json:"id"`
	Name     string             `json:"name,omitempty"`
	Enabled  bool               `json:"enabled"`
	Priority int                `json:"priority,omitempty"`
	Group    string             `json:"group,omitempty"`
	SKU      string             `json:"sku"`
	Mode     string             `json:"mode,omitempty"`
	Targets  []ModelAliasTarget `json:"targets,omitempty"`
	Notes    string             `json:"notes,omitempty"`
}

type ModelAliasTarget struct {
	ModelName   string `json:"model_name"`
	ChannelID   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	Priority    int    `json:"priority,omitempty"`
	Weight      int    `json:"weight,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

type ResponseCacheRule struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	Enabled      bool   `json:"enabled"`
	Priority     int    `json:"priority,omitempty"`
	Group        string `json:"group,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ChannelID    int    `json:"channel_id,omitempty"`
	ChannelName  string `json:"channel_name,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Scope        string `json:"scope,omitempty"`
	TTLSeconds   int    `json:"ttl_seconds,omitempty"`
	MaxBodyBytes int    `json:"max_body_bytes,omitempty"`
	PublicStatic bool   `json:"public_static,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{
		Version:                      ObservationVersion,
		Enabled:                      true,
		ObserveOnly:                  true,
		ObserveGroups:                []string{DefaultObserveGroup},
		GlobalKillSwitch:             false,
		CostRoutingMode:              ModeObserve,
		CostRoutingMinSamples:        DefaultCostRoutingMinSamples,
		CostRoutingMinSuccessRatePct: DefaultCostRoutingMinSuccessRatePct,
		CostRoutingHealthWindowHours: DefaultCostRoutingHealthWindowHours,
		CacheMode:                    ModeOff,
		LongContextMode:              ModeOff,
		OutputCapMode:                ModeOff,
		ModelAliasMode:               ModeOff,
		RetryBudgetMode:              ModeOff,
		MaxRetryCostUSD:              0,
		RetryLowMarginSkip:           false,
		RiskEnforcement:              ModeOff,
		SettingsWritable:             true,
		CostProfilesUsed:             true,
		ModelAliases:                 DefaultModelAliases(),
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
	s.SettingsWritable = true
	s.CostProfilesUsed = true
	if len(s.ObserveGroups) == 0 {
		s.ObserveGroups = defaults.ObserveGroups
	}
	s.ObserveGroups = cleanStringSlice(s.ObserveGroups)
	if !validCostRoutingMode(s.CostRoutingMode) {
		s.CostRoutingMode = defaults.CostRoutingMode
	}
	if s.CostRoutingMinSamples <= 0 {
		s.CostRoutingMinSamples = defaults.CostRoutingMinSamples
	}
	if s.CostRoutingMinSuccessRatePct <= 0 {
		s.CostRoutingMinSuccessRatePct = defaults.CostRoutingMinSuccessRatePct
	}
	if s.CostRoutingMinSuccessRatePct > 100 {
		s.CostRoutingMinSuccessRatePct = 100
	}
	if s.CostRoutingHealthWindowHours <= 0 {
		s.CostRoutingHealthWindowHours = defaults.CostRoutingHealthWindowHours
	}
	if !validResponseCacheMode(s.CacheMode) {
		s.CacheMode = defaults.CacheMode
	}
	if !validOffObserveMode(s.LongContextMode) {
		s.LongContextMode = defaults.LongContextMode
	}
	s.LongContextPolicies = normalizeLongContextPolicies(s.LongContextPolicies)
	if !validOutputCapMode(s.OutputCapMode) {
		s.OutputCapMode = defaults.OutputCapMode
	}
	s.OutputPolicies = normalizeOutputPolicies(s.OutputPolicies)
	if !validOffObserveMode(s.ModelAliasMode) {
		s.ModelAliasMode = defaults.ModelAliasMode
	}
	if len(s.ModelAliases) == 0 {
		s.ModelAliases = defaults.ModelAliases
	}
	s.ModelAliases = normalizeModelAliases(s.ModelAliases)
	s.ResponseCacheRules = normalizeResponseCacheRules(s.ResponseCacheRules)
	if !validRetryBudgetMode(s.RetryBudgetMode) {
		s.RetryBudgetMode = defaults.RetryBudgetMode
	}
	s.MaxRetryCostUSD = normalizeNonNegativeFloat(s.MaxRetryCostUSD)
	if !validRiskMode(s.RiskEnforcement) {
		s.RiskEnforcement = defaults.RiskEnforcement
	}
	s.RiskMinGrossUSD = normalizeNonNegativeFloat(s.RiskMinGrossUSD)
	s.RiskMinGrossPct = normalizeNonNegativeFloat(s.RiskMinGrossPct)
	s.RiskMinExpectUSD = normalizeNonNegativeFloat(s.RiskMinExpectUSD)
	s.RiskMinExpectPct = normalizeNonNegativeFloat(s.RiskMinExpectPct)
	return s
}

func (s Settings) Validate() error {
	if s.CostRoutingMode != "" && !validCostRoutingMode(s.CostRoutingMode) {
		return errors.New("invalid cost_routing_mode")
	}
	if s.CostRoutingMinSamples < 0 {
		return errors.New("cost_routing_min_samples must be non-negative")
	}
	if s.CostRoutingMinSuccessRatePct < 0 || s.CostRoutingMinSuccessRatePct > 100 || math.IsNaN(s.CostRoutingMinSuccessRatePct) || math.IsInf(s.CostRoutingMinSuccessRatePct, 0) {
		return errors.New("cost_routing_min_success_rate_pct must be between 0 and 100")
	}
	if s.CostRoutingHealthWindowHours < 0 {
		return errors.New("cost_routing_health_window_hours must be non-negative")
	}
	if s.CacheMode != "" && !validResponseCacheMode(s.CacheMode) {
		return errors.New("invalid cache_mode")
	}
	if s.LongContextMode != "" && !validOffObserveMode(s.LongContextMode) {
		return errors.New("invalid long_context_mode")
	}
	for _, policy := range s.LongContextPolicies {
		mode := strings.TrimSpace(policy.Mode)
		if mode != "" && !validOffObserveMode(mode) {
			return errors.New("invalid long context policy mode")
		}
		if policy.ChannelID < 0 {
			return errors.New("long context policy channel_id cannot be negative")
		}
		for _, tier := range policy.Tiers {
			if tier.MinContextTokens < 0 || tier.MaxContextTokens < 0 {
				return errors.New("long context tier token limits must be non-negative")
			}
			if tier.MaxContextTokens > 0 && tier.MaxContextTokens < tier.MinContextTokens {
				return errors.New("long context tier max_context_tokens cannot be below min_context_tokens")
			}
			if tier.InputMultiplier < 0 || math.IsNaN(tier.InputMultiplier) || math.IsInf(tier.InputMultiplier, 0) {
				return errors.New("long context tier input_multiplier must be non-negative")
			}
		}
	}
	if s.OutputCapMode != "" && !validOutputCapMode(s.OutputCapMode) {
		return errors.New("invalid output_cap_mode")
	}
	if s.ModelAliasMode != "" && !validOffObserveMode(s.ModelAliasMode) {
		return errors.New("invalid model_alias_mode")
	}
	for _, alias := range s.ModelAliases {
		mode := strings.TrimSpace(alias.Mode)
		if mode != "" && !validOffObserveMode(mode) {
			return errors.New("invalid model alias mode")
		}
		if alias.ChannelIDLessThanZero() {
			return errors.New("model alias target channel_id cannot be negative")
		}
	}
	for _, rule := range s.ResponseCacheRules {
		mode := strings.TrimSpace(rule.Mode)
		if mode != "" && !validResponseCacheMode(mode) {
			return errors.New("invalid response cache rule mode")
		}
		if rule.ChannelID < 0 {
			return errors.New("response cache rule channel_id cannot be negative")
		}
		if rule.TTLSeconds < 0 {
			return errors.New("response cache rule ttl_seconds cannot be negative")
		}
		if rule.MaxBodyBytes < 0 {
			return errors.New("response cache rule max_body_bytes cannot be negative")
		}
		scope := strings.TrimSpace(rule.Scope)
		if scope != "" && !validResponseCacheScope(scope) {
			return errors.New("invalid response cache rule scope")
		}
	}
	if s.RetryBudgetMode != "" && !validRetryBudgetMode(s.RetryBudgetMode) {
		return errors.New("invalid retry_budget_mode")
	}
	if s.MaxRetryCostUSD < 0 || math.IsNaN(s.MaxRetryCostUSD) || math.IsInf(s.MaxRetryCostUSD, 0) {
		return errors.New("max_retry_cost_usd must be non-negative")
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
	if s.RiskEnforcement != "" && !validRiskMode(s.RiskEnforcement) {
		return errors.New("invalid risk_enforcement")
	}
	if err := s.validateObserveOnlyScope(); err != nil {
		return err
	}
	for name, value := range map[string]float64{
		"risk_min_gross_margin_usd":    s.RiskMinGrossUSD,
		"risk_min_gross_margin_pct":    s.RiskMinGrossPct,
		"risk_min_expected_margin_usd": s.RiskMinExpectUSD,
		"risk_min_expected_margin_pct": s.RiskMinExpectPct,
	} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New(name + " must be non-negative")
		}
	}
	for _, alias := range s.ModelAliases {
		if !alias.Enabled {
			continue
		}
		mode := strings.TrimSpace(alias.Mode)
		if mode == "" {
			mode = ModeObserve
		}
		if mode == ModeOff {
			continue
		}
		if strings.TrimSpace(alias.Group) != DefaultObserveGroup {
			return errors.New("model aliases must target proxy-test group")
		}
		if !strings.HasPrefix(strings.TrimSpace(alias.SKU), "glart-") {
			return errors.New("model alias sku must start with glart-")
		}
		if len(alias.Targets) == 0 {
			return errors.New("model alias must have at least one target")
		}
	}
	for _, rule := range s.ResponseCacheRules {
		if !rule.Enabled {
			continue
		}
		mode := strings.TrimSpace(rule.Mode)
		if mode == "" {
			mode = ModeObserve
		}
		if mode == ModeOff {
			continue
		}
		if strings.TrimSpace(rule.Group) != DefaultObserveGroup {
			return errors.New("response cache rules must target proxy-test group")
		}
		if !rule.PublicStatic {
			return errors.New("response cache rules must be marked public_static")
		}
	}
	return nil
}

func (s Settings) validateObserveOnlyScope() error {
	for _, group := range cleanStringSlice(s.ObserveGroups) {
		if group != DefaultObserveGroup {
			return errors.New("profit observe_groups are limited to proxy-test")
		}
	}
	for _, policy := range s.LongContextPolicies {
		if !policy.Enabled {
			continue
		}
		mode := strings.TrimSpace(policy.Mode)
		if mode == "" {
			mode = ModeObserve
		}
		if mode == ModeOff {
			continue
		}
		if strings.TrimSpace(policy.Group) != DefaultObserveGroup {
			return errors.New("long context policies must target proxy-test group")
		}
	}
	for _, policy := range s.OutputPolicies {
		if !policy.Enabled {
			continue
		}
		mode := strings.TrimSpace(policy.Mode)
		if mode == "" {
			mode = ModeObserve
		}
		if mode == ModeOff {
			continue
		}
		if strings.TrimSpace(policy.Group) != DefaultObserveGroup {
			return errors.New("output policies must target proxy-test group")
		}
	}
	for _, rule := range s.ResponseCacheRules {
		if !rule.Enabled {
			continue
		}
		mode := strings.TrimSpace(rule.Mode)
		if mode == "" {
			mode = ModeObserve
		}
		if mode == ModeOff {
			continue
		}
		if strings.TrimSpace(rule.Group) != DefaultObserveGroup {
			return errors.New("response cache rules must target proxy-test")
		}
	}
	return nil
}

func validOffObserveMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve
}

func validResponseCacheMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve || mode == ModeEnforce
}

func validResponseCacheScope(scope string) bool {
	return scope == ResponseCacheScopeGlobal || scope == ResponseCacheScopeUser || scope == ResponseCacheScopeSession
}

func validCostRoutingMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve || mode == ModePreferMargin
}

func validOutputCapMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve || mode == ModeCap || mode == ModePremiumRequired
}

func validRetryBudgetMode(mode string) bool {
	return mode == ModeOff || mode == ModeObserve || mode == ModeEnforce
}

func validRiskMode(mode string) bool {
	return mode == ModeOff || mode == RiskModeAlert || mode == ModeEnforce
}

func normalizeNonNegativeFloat(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
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

func normalizeLongContextPolicies(policies []LongContextPolicy) []LongContextPolicy {
	out := make([]LongContextPolicy, 0, len(policies))
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
		if !validOffObserveMode(policy.Mode) {
			policy.Mode = ModeObserve
		}
		if policy.ChannelID < 0 {
			policy.ChannelID = 0
		}
		policy.Tiers = normalizeLongContextTiers(policy.Tiers)
		if policy.ID == "" && policy.Group == "" && policy.ModelName == "" && policy.ChannelID == 0 && policy.ChannelName == "" {
			continue
		}
		out = append(out, policy)
	}
	return out
}

func normalizeLongContextTiers(tiers []LongContextTier) []LongContextTier {
	out := make([]LongContextTier, 0, len(tiers))
	for _, tier := range tiers {
		tier.ID = strings.TrimSpace(tier.ID)
		tier.Name = strings.TrimSpace(tier.Name)
		if tier.MinContextTokens < 0 {
			tier.MinContextTokens = 0
		}
		if tier.MaxContextTokens < 0 {
			tier.MaxContextTokens = 0
		}
		if tier.MaxContextTokens > 0 && tier.MaxContextTokens < tier.MinContextTokens {
			tier.MaxContextTokens = 0
		}
		if tier.InputMultiplier <= 0 || math.IsNaN(tier.InputMultiplier) || math.IsInf(tier.InputMultiplier, 0) {
			tier.InputMultiplier = 1
		}
		if tier.ID == "" && tier.Name == "" && tier.MinContextTokens == 0 && tier.MaxContextTokens == 0 && tier.InputMultiplier == 1 && !tier.PremiumRequired {
			continue
		}
		out = append(out, tier)
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

func DefaultModelAliases() []ModelAlias {
	return []ModelAlias{
		defaultModelAlias("glart-fast", "glart-fast", "gpt-5.4-mini", 100),
		defaultModelAlias("glart-balanced", "glart-balanced", "gpt-5.5", 90),
		defaultModelAlias("glart-coder", "glart-coder", "gpt-5.5", 80),
		defaultModelAlias("glart-long", "glart-long", "gpt-5.5", 70),
		defaultModelAlias("glart-premium", "glart-premium", "gpt-5.5", 60),
	}
}

func defaultModelAlias(id string, sku string, modelName string, priority int) ModelAlias {
	return ModelAlias{
		ID:       id,
		Name:     sku,
		Enabled:  true,
		Priority: priority,
		Group:    DefaultObserveGroup,
		SKU:      sku,
		Mode:     ModeObserve,
		Targets: []ModelAliasTarget{{
			ModelName: modelName,
			Priority:  100,
		}},
		Notes: "Proxy-test SKU alias seed. Users buy the stable glart SKU; upstream model can be changed later.",
	}
}

func normalizeModelAliases(aliases []ModelAlias) []ModelAlias {
	out := make([]ModelAlias, 0, len(aliases))
	for _, alias := range aliases {
		alias.ID = strings.TrimSpace(alias.ID)
		alias.Name = strings.TrimSpace(alias.Name)
		alias.Group = strings.TrimSpace(alias.Group)
		alias.SKU = strings.TrimSpace(alias.SKU)
		alias.Mode = strings.TrimSpace(alias.Mode)
		alias.Notes = strings.TrimSpace(alias.Notes)
		if alias.Mode == "" {
			alias.Mode = ModeObserve
		}
		if !validOffObserveMode(alias.Mode) {
			alias.Mode = ModeObserve
		}
		alias.Targets = normalizeModelAliasTargets(alias.Targets)
		if alias.ID == "" && alias.Group == "" && alias.SKU == "" && len(alias.Targets) == 0 {
			continue
		}
		out = append(out, alias)
	}
	return out
}

func normalizeModelAliasTargets(targets []ModelAliasTarget) []ModelAliasTarget {
	out := make([]ModelAliasTarget, 0, len(targets))
	for _, target := range targets {
		target.ModelName = strings.TrimSpace(target.ModelName)
		target.ChannelName = strings.TrimSpace(target.ChannelName)
		target.Notes = strings.TrimSpace(target.Notes)
		if target.ChannelID < 0 {
			target.ChannelID = 0
		}
		if target.Weight < 0 {
			target.Weight = 0
		}
		if target.ModelName == "" {
			continue
		}
		out = append(out, target)
	}
	return out
}

func (alias ModelAlias) ChannelIDLessThanZero() bool {
	for _, target := range alias.Targets {
		if target.ChannelID < 0 {
			return true
		}
	}
	return false
}

func normalizeResponseCacheRules(rules []ResponseCacheRule) []ResponseCacheRule {
	out := make([]ResponseCacheRule, 0, len(rules))
	for _, rule := range rules {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Name = strings.TrimSpace(rule.Name)
		rule.Group = strings.TrimSpace(rule.Group)
		rule.ModelName = strings.TrimSpace(rule.ModelName)
		rule.ChannelName = strings.TrimSpace(rule.ChannelName)
		rule.Mode = strings.TrimSpace(rule.Mode)
		rule.Scope = strings.TrimSpace(rule.Scope)
		rule.Notes = strings.TrimSpace(rule.Notes)
		if rule.Mode == "" {
			rule.Mode = ModeObserve
		}
		if !validResponseCacheMode(rule.Mode) {
			rule.Mode = ModeObserve
		}
		if rule.Scope == "" {
			rule.Scope = ResponseCacheScopeSession
		}
		if !validResponseCacheScope(rule.Scope) {
			rule.Scope = ResponseCacheScopeSession
		}
		if rule.ChannelID < 0 {
			rule.ChannelID = 0
		}
		if rule.TTLSeconds < 0 {
			rule.TTLSeconds = 0
		}
		if rule.MaxBodyBytes < 0 {
			rule.MaxBodyBytes = 0
		}
		if rule.ID == "" && rule.Group == "" && rule.ModelName == "" && rule.ChannelID == 0 && rule.ChannelName == "" {
			continue
		}
		out = append(out, rule)
	}
	return out
}
