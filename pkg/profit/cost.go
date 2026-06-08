package profit

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	CostStatusConfigured = "configured"
)

type CostProfilesDocument struct {
	Version int           `json:"version"`
	Items   []CostProfile `json:"items"`
}

type CostProfile struct {
	ID                         string  `json:"id"`
	Name                       string  `json:"name"`
	Enabled                    bool    `json:"enabled"`
	Priority                   int     `json:"priority"`
	Provider                   string  `json:"provider"`
	ChannelID                  int     `json:"channel_id"`
	ChannelName                string  `json:"channel_name"`
	ModelName                  string  `json:"model_name"`
	InputUSDPerMillion         float64 `json:"input_usd_per_million"`
	OutputUSDPerMillion        float64 `json:"output_usd_per_million"`
	CacheReadUSDPerMillion     float64 `json:"cache_read_usd_per_million"`
	CacheWriteUSDPerMillion    float64 `json:"cache_write_usd_per_million"`
	FixedRequestUSD            float64 `json:"fixed_request_usd"`
	FailurePenaltyUSD          float64 `json:"failure_penalty_usd"`
	LatencyPenaltyUSDPerSecond float64 `json:"latency_penalty_usd_per_second"`
	RiskPenaltyUSD             float64 `json:"risk_penalty_usd"`
	Notes                      string  `json:"notes"`
}

type CostInput struct {
	Group                    string  `json:"group"`
	Provider                 string  `json:"provider"`
	ChannelID                int     `json:"channel_id"`
	ChannelName              string  `json:"channel_name"`
	ModelName                string  `json:"model_name"`
	BillablePromptTokens     int     `json:"billable_prompt_tokens"`
	BillableCompletionTokens int     `json:"billable_completion_tokens"`
	UpstreamPromptTokens     int     `json:"upstream_actual_prompt_tokens"`
	UpstreamCompletionTokens int     `json:"upstream_actual_completion_tokens"`
	CacheReadTokens          int     `json:"cache_read_tokens"`
	CacheWriteTokens         int     `json:"cache_write_tokens"`
	RevenueUSD               float64 `json:"estimated_revenue_usd"`
	LatencyMs                int     `json:"latency_ms"`
	FailureRate              float64 `json:"failure_rate"`
}

type CostEstimate struct {
	CostKnown                bool     `json:"cost_known"`
	CostStatus               string   `json:"profit_cost_status"`
	CostProfileID            string   `json:"cost_profile_id,omitempty"`
	CostProfileName          string   `json:"cost_profile_name,omitempty"`
	EstimatedRevenueUSD      float64  `json:"estimated_revenue_usd"`
	EstimatedUpstreamCostUSD *float64 `json:"estimated_upstream_cost_usd"`
	TokenCostUSD             *float64 `json:"token_cost_usd"`
	FixedRequestUSD          float64  `json:"fixed_request_usd"`
	FailurePenaltyUSD        float64  `json:"failure_penalty_usd"`
	LatencyPenaltyUSD        float64  `json:"latency_penalty_usd"`
	RiskPenaltyUSD           float64  `json:"risk_penalty_usd"`
	ExpectedCostUSD          *float64 `json:"expected_cost_usd"`
	GrossMarginUSD           *float64 `json:"gross_margin_usd"`
	GrossMarginPct           *float64 `json:"gross_margin_pct"`
	ExpectedMarginUSD        *float64 `json:"expected_margin_usd"`
	ExpectedMarginPct        *float64 `json:"expected_margin_pct"`
}

type RoutePreviewRequest struct {
	Group        string      `json:"group"`
	ModelName    string      `json:"model_name"`
	Provider     string      `json:"provider"`
	ChannelID    int         `json:"channel_id"`
	ChannelName  string      `json:"channel_name"`
	PromptTokens int         `json:"prompt_tokens"`
	OutputTokens int         `json:"output_tokens"`
	RevenueUSD   float64     `json:"estimated_revenue_usd"`
	Candidates   []CostInput `json:"candidates"`
}

type RoutePreviewCandidate struct {
	Input       CostInput    `json:"input"`
	Estimate    CostEstimate `json:"estimate"`
	WouldPrefer bool         `json:"would_prefer"`
}

type RoutePreviewResponse struct {
	ObserveOnly     bool                    `json:"observe_only"`
	LiveRoutingUsed bool                    `json:"live_routing_used"`
	RoutingMode     string                  `json:"routing_mode"`
	Candidates      []RoutePreviewCandidate `json:"candidates"`
	SelectedIndex   int                     `json:"selected_index"`
	Message         string                  `json:"message"`
}

func DefaultCostProfilesDocument() CostProfilesDocument {
	return CostProfilesDocument{
		Version: ObservationVersion,
		Items:   DefaultCostProfiles(),
	}
}

func DefaultCostProfiles() []CostProfile {
	return []CostProfile{
		{
			ID:                         "generic-gpt-5-premium",
			Name:                       "Generic GPT-5 family fallback estimate",
			Enabled:                    true,
			Priority:                   10,
			ModelName:                  "gpt-5*",
			InputUSDPerMillion:         1.25,
			OutputUSDPerMillion:        10,
			FailurePenaltyUSD:          0.01,
			LatencyPenaltyUSDPerSecond: 0.00005,
			Notes:                      "Generic fallback for GPT-5 family across any channel/provider; use channel-specific profiles only when real cost differs.",
		},
		{
			ID:                         "generic-gemini-family",
			Name:                       "Generic Gemini family fallback estimate",
			Enabled:                    true,
			Priority:                   9,
			ModelName:                  "gemini*",
			InputUSDPerMillion:         0.2,
			OutputUSDPerMillion:        1,
			FailurePenaltyUSD:          0.004,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0002,
			Notes:                      "Generic fallback for Gemini family across any channel/provider; override per channel only when needed.",
		},
		{
			ID:                         "generic-claude-family",
			Name:                       "Generic Claude family fallback estimate",
			Enabled:                    true,
			Priority:                   8,
			ModelName:                  "claude*",
			InputUSDPerMillion:         3,
			OutputUSDPerMillion:        15,
			FailurePenaltyUSD:          0.02,
			LatencyPenaltyUSDPerSecond: 0.00008,
			RiskPenaltyUSD:             0.0005,
			Notes:                      "Generic fallback for Claude family across any channel/provider; override per channel only when needed.",
		},
		{
			ID:                         "generic-gpt-oss-family",
			Name:                       "Generic gpt-oss family fallback estimate",
			Enabled:                    true,
			Priority:                   8,
			ModelName:                  "gpt-oss*",
			InputUSDPerMillion:         0.1,
			OutputUSDPerMillion:        0.5,
			FailurePenaltyUSD:          0.003,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0001,
			Notes:                      "Generic fallback for gpt-oss family across any channel/provider; override per channel only when needed.",
		},
		{
			ID:                         "generic-deepseek-family",
			Name:                       "Generic DeepSeek family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "deepseek*",
			InputUSDPerMillion:         0.2,
			OutputUSDPerMillion:        0.8,
			FailurePenaltyUSD:          0.004,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0002,
			Notes:                      "Generic fallback for DeepSeek family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-qwen-family",
			Name:                       "Generic Qwen family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "qwen*",
			InputUSDPerMillion:         0.3,
			OutputUSDPerMillion:        1.2,
			FailurePenaltyUSD:          0.004,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0002,
			Notes:                      "Generic fallback for Qwen family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-kimi-family",
			Name:                       "Generic Kimi/Moonshot family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "kimi*",
			InputUSDPerMillion:         0.6,
			OutputUSDPerMillion:        2,
			FailurePenaltyUSD:          0.006,
			LatencyPenaltyUSDPerSecond: 0.00004,
			RiskPenaltyUSD:             0.0003,
			Notes:                      "Generic fallback for Kimi family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-moonshot-family",
			Name:                       "Generic Moonshot family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "moonshot*",
			InputUSDPerMillion:         0.6,
			OutputUSDPerMillion:        2,
			FailurePenaltyUSD:          0.006,
			LatencyPenaltyUSDPerSecond: 0.00004,
			RiskPenaltyUSD:             0.0003,
			Notes:                      "Generic fallback for Moonshot family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-kiro-family",
			Name:                       "Generic Kiro family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "kiro*",
			InputUSDPerMillion:         3,
			OutputUSDPerMillion:        15,
			FailurePenaltyUSD:          0.02,
			LatencyPenaltyUSDPerSecond: 0.00008,
			RiskPenaltyUSD:             0.0005,
			Notes:                      "Generic fallback for Kiro-style CLI/OAuth models across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-grok-family",
			Name:                       "Generic Grok/xAI family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "grok*",
			InputUSDPerMillion:         3,
			OutputUSDPerMillion:        15,
			FailurePenaltyUSD:          0.015,
			LatencyPenaltyUSDPerSecond: 0.00006,
			RiskPenaltyUSD:             0.0005,
			Notes:                      "Generic fallback for Grok family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-mistral-family",
			Name:                       "Generic Mistral family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "mistral*",
			InputUSDPerMillion:         0.5,
			OutputUSDPerMillion:        1.5,
			FailurePenaltyUSD:          0.004,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0002,
			Notes:                      "Generic fallback for Mistral family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-llama-family",
			Name:                       "Generic Llama family fallback estimate",
			Enabled:                    true,
			Priority:                   7,
			ModelName:                  "llama*",
			InputUSDPerMillion:         0.2,
			OutputUSDPerMillion:        0.8,
			FailurePenaltyUSD:          0.003,
			LatencyPenaltyUSDPerSecond: 0.00003,
			RiskPenaltyUSD:             0.0001,
			Notes:                      "Generic fallback for Llama family across any channel/provider; replace with exact upstream prices when available.",
		},
		{
			ID:                         "generic-any-model",
			Name:                       "Generic any-model fallback estimate",
			Enabled:                    true,
			Priority:                   1,
			ModelName:                  "*",
			InputUSDPerMillion:         2,
			OutputUSDPerMillion:        8,
			FailurePenaltyUSD:          0.01,
			LatencyPenaltyUSDPerSecond: 0.00005,
			RiskPenaltyUSD:             0.001,
			Notes:                      "Last-resort fallback for any model without a more specific cost profile; use only as an estimate and override exact high-volume channels.",
		},
	}
}

func LoadCostProfiles() (CostProfilesDocument, error) {
	doc := DefaultCostProfilesDocument()
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[CostProfilesOptionKey]
	common.OptionMapRWMutex.RUnlock()
	if strings.TrimSpace(raw) == "" {
		return doc.Normalize(), nil
	}
	var stored CostProfilesDocument
	if err := common.UnmarshalJsonStr(raw, &stored); err != nil {
		return CostProfilesDocument{}, errors.New("invalid stored profit cost profiles")
	}
	if err := stored.Validate(); err != nil {
		return CostProfilesDocument{}, err
	}
	return mergeCostProfileDefaults(doc, stored).Normalize(), nil
}

func CurrentCostProfiles() CostProfilesDocument {
	doc, err := LoadCostProfiles()
	if err != nil {
		return DefaultCostProfilesDocument()
	}
	return doc
}

func (doc CostProfilesDocument) Normalize() CostProfilesDocument {
	doc.Version = ObservationVersion
	if doc.Items == nil {
		doc.Items = []CostProfile{}
	}
	out := make([]CostProfile, 0, len(doc.Items))
	for i, profile := range doc.Items {
		profile = profile.Normalize(i)
		if profile.ID == "" && profile.Name == "" && profile.ModelName == "" && profile.ChannelID == 0 && profile.ChannelName == "" {
			continue
		}
		out = append(out, profile)
	}
	doc.Items = out
	return doc
}

func mergeCostProfileDefaults(defaults CostProfilesDocument, stored CostProfilesDocument) CostProfilesDocument {
	defaults = defaults.Normalize()
	stored = stored.Normalize()
	byID := make(map[string]int, len(stored.Items))
	for i, profile := range stored.Items {
		if profile.ID != "" {
			byID[profile.ID] = i
		}
	}
	out := make([]CostProfile, 0, len(defaults.Items)+len(stored.Items))
	for _, profile := range defaults.Items {
		if index, ok := byID[profile.ID]; ok {
			out = append(out, stored.Items[index])
			continue
		}
		out = append(out, profile)
	}
	defaultIDs := make(map[string]struct{}, len(defaults.Items))
	for _, profile := range defaults.Items {
		defaultIDs[profile.ID] = struct{}{}
	}
	for _, profile := range stored.Items {
		if _, ok := defaultIDs[profile.ID]; ok {
			continue
		}
		out = append(out, profile)
	}
	return CostProfilesDocument{
		Version: ObservationVersion,
		Items:   out,
	}
}

func (doc CostProfilesDocument) Validate() error {
	for _, profile := range doc.Items {
		if err := profile.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p CostProfile) Normalize(index int) CostProfile {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.Provider = strings.TrimSpace(p.Provider)
	p.ChannelName = strings.TrimSpace(p.ChannelName)
	p.ModelName = strings.TrimSpace(p.ModelName)
	p.Notes = strings.TrimSpace(p.Notes)
	if p.ID == "" {
		p.ID = fmt.Sprintf("profile-%d", index+1)
	}
	if p.Name == "" {
		p.Name = p.ID
	}
	return p
}

func (p CostProfile) Validate() error {
	if p.ChannelID < 0 {
		return errors.New("channel_id cannot be negative")
	}
	for name, value := range map[string]float64{
		"input_usd_per_million":          p.InputUSDPerMillion,
		"output_usd_per_million":         p.OutputUSDPerMillion,
		"cache_read_usd_per_million":     p.CacheReadUSDPerMillion,
		"cache_write_usd_per_million":    p.CacheWriteUSDPerMillion,
		"fixed_request_usd":              p.FixedRequestUSD,
		"failure_penalty_usd":            p.FailurePenaltyUSD,
		"latency_penalty_usd_per_second": p.LatencyPenaltyUSDPerSecond,
		"risk_penalty_usd":               p.RiskPenaltyUSD,
	} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s cannot be negative or non-finite", name)
		}
	}
	return nil
}

func EstimateCost(input CostInput, doc CostProfilesDocument) CostEstimate {
	input = normalizeCostInput(input)
	estimate := CostEstimate{
		CostKnown:           false,
		CostStatus:          CostStatusMissingCostProfile,
		EstimatedRevenueUSD: input.RevenueUSD,
	}
	profile, ok := MatchCostProfile(doc, input)
	if !ok {
		return estimate
	}

	tokenCost := costForMillionTokens(input.UpstreamPromptTokens, profile.InputUSDPerMillion) +
		costForMillionTokens(input.UpstreamCompletionTokens, profile.OutputUSDPerMillion) +
		costForMillionTokens(input.CacheReadTokens, profile.CacheReadUSDPerMillion) +
		costForMillionTokens(input.CacheWriteTokens, profile.CacheWriteUSDPerMillion)
	upstreamCost := tokenCost + profile.FixedRequestUSD
	failurePenalty := clamp01(input.FailureRate) * profile.FailurePenaltyUSD
	latencyPenalty := float64(maxInt(0, input.LatencyMs)) / 1000 * profile.LatencyPenaltyUSDPerSecond
	expectedCost := upstreamCost + failurePenalty + latencyPenalty + profile.RiskPenaltyUSD
	grossMargin := input.RevenueUSD - upstreamCost
	expectedMargin := input.RevenueUSD - expectedCost

	estimate.CostKnown = true
	estimate.CostStatus = CostStatusConfigured
	estimate.CostProfileID = profile.ID
	estimate.CostProfileName = profile.Name
	estimate.EstimatedUpstreamCostUSD = floatPtr(upstreamCost)
	estimate.TokenCostUSD = floatPtr(tokenCost)
	estimate.FixedRequestUSD = profile.FixedRequestUSD
	estimate.FailurePenaltyUSD = failurePenalty
	estimate.LatencyPenaltyUSD = latencyPenalty
	estimate.RiskPenaltyUSD = profile.RiskPenaltyUSD
	estimate.ExpectedCostUSD = floatPtr(expectedCost)
	estimate.GrossMarginUSD = floatPtr(grossMargin)
	estimate.ExpectedMarginUSD = floatPtr(expectedMargin)
	if input.RevenueUSD > 0 {
		estimate.GrossMarginPct = floatPtr(grossMargin / input.RevenueUSD * 100)
		estimate.ExpectedMarginPct = floatPtr(expectedMargin / input.RevenueUSD * 100)
	}
	return estimate
}

func MatchCostProfile(doc CostProfilesDocument, input CostInput) (CostProfile, bool) {
	input = normalizeCostInput(input)
	type scoredProfile struct {
		profile CostProfile
		score   int
	}
	var matches []scoredProfile
	for i, profile := range doc.Normalize().Items {
		if !profile.Enabled {
			continue
		}
		score, ok := profileMatchScore(profile.Normalize(i), input)
		if !ok {
			continue
		}
		matches = append(matches, scoredProfile{profile: profile.Normalize(i), score: score})
	}
	if len(matches) == 0 {
		return CostProfile{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].profile.Priority > matches[j].profile.Priority
	})
	return matches[0].profile, true
}

func PreviewRoute(settings Settings, doc CostProfilesDocument, req RoutePreviewRequest) RoutePreviewResponse {
	settings = settings.Normalize()
	candidates := req.Candidates
	if len(candidates) == 0 {
		candidates = []CostInput{{
			Group:                    req.Group,
			Provider:                 req.Provider,
			ChannelID:                req.ChannelID,
			ChannelName:              req.ChannelName,
			ModelName:                req.ModelName,
			BillablePromptTokens:     req.PromptTokens,
			BillableCompletionTokens: req.OutputTokens,
			UpstreamPromptTokens:     req.PromptTokens,
			UpstreamCompletionTokens: req.OutputTokens,
			RevenueUSD:               req.RevenueUSD,
		}}
	}

	response := RoutePreviewResponse{
		ObserveOnly:     true,
		LiveRoutingUsed: false,
		RoutingMode:     settings.CostRoutingMode,
		SelectedIndex:   -1,
		Message:         "observe-only preview; live routing is unchanged",
	}

	bestIdx := -1
	var bestMargin float64
	bestKnown := false
	for idx, candidate := range candidates {
		candidate = inheritRoutePreviewDefaults(req, candidate)
		estimate := EstimateCost(candidate, doc)
		preview := RoutePreviewCandidate{
			Input:    candidate,
			Estimate: estimate,
		}
		if estimate.ExpectedMarginUSD != nil {
			if !bestKnown || *estimate.ExpectedMarginUSD > bestMargin {
				bestKnown = true
				bestMargin = *estimate.ExpectedMarginUSD
				bestIdx = idx
			}
		}
		response.Candidates = append(response.Candidates, preview)
	}
	if bestIdx >= 0 {
		response.SelectedIndex = bestIdx
		response.Candidates[bestIdx].WouldPrefer = true
	}
	return response
}

func profileMatchScore(profile CostProfile, input CostInput) (int, bool) {
	score := 0
	if profile.ChannelID != 0 {
		if input.ChannelID != profile.ChannelID {
			return 0, false
		}
		score += 10
	}
	if profile.ChannelName != "" {
		if !equalFoldTrim(input.ChannelName, profile.ChannelName) {
			return 0, false
		}
		score += 6
	}
	if profile.Provider != "" {
		if !equalFoldTrim(input.Provider, profile.Provider) {
			return 0, false
		}
		score += 4
	}
	if profile.ModelName != "" {
		modelScore, ok := modelPatternScore(profile.ModelName, input.ModelName)
		if !ok {
			return 0, false
		}
		score += modelScore
	}
	return score, true
}

func modelPatternScore(pattern string, value string) (int, bool) {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" || pattern == "*" {
		return 0, pattern == "" || value != ""
	}
	if value == "" {
		return 0, false
	}
	if pattern == value {
		return 8, true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(value, strings.TrimSuffix(pattern, "*")) {
		return 5, true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		needle := strings.Trim(pattern, "*")
		return 3, needle != "" && strings.Contains(value, needle)
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(value, strings.TrimPrefix(pattern, "*")) {
		return 4, true
	}
	return 0, false
}

func normalizeCostInput(input CostInput) CostInput {
	input.Group = strings.TrimSpace(input.Group)
	input.Provider = strings.TrimSpace(input.Provider)
	input.ChannelName = strings.TrimSpace(input.ChannelName)
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.BillablePromptTokens = maxInt(0, input.BillablePromptTokens)
	input.BillableCompletionTokens = maxInt(0, input.BillableCompletionTokens)
	input.UpstreamPromptTokens = maxInt(0, input.UpstreamPromptTokens)
	input.UpstreamCompletionTokens = maxInt(0, input.UpstreamCompletionTokens)
	input.CacheReadTokens = maxInt(0, input.CacheReadTokens)
	input.CacheWriteTokens = maxInt(0, input.CacheWriteTokens)
	if input.UpstreamPromptTokens == 0 {
		input.UpstreamPromptTokens = input.BillablePromptTokens
	}
	if input.UpstreamCompletionTokens == 0 {
		input.UpstreamCompletionTokens = input.BillableCompletionTokens
	}
	if input.RevenueUSD < 0 || math.IsNaN(input.RevenueUSD) || math.IsInf(input.RevenueUSD, 0) {
		input.RevenueUSD = 0
	}
	return input
}

func inheritRoutePreviewDefaults(req RoutePreviewRequest, candidate CostInput) CostInput {
	if candidate.Group == "" {
		candidate.Group = req.Group
	}
	if candidate.Provider == "" {
		candidate.Provider = req.Provider
	}
	if candidate.ChannelID == 0 {
		candidate.ChannelID = req.ChannelID
	}
	if candidate.ChannelName == "" {
		candidate.ChannelName = req.ChannelName
	}
	if candidate.ModelName == "" {
		candidate.ModelName = req.ModelName
	}
	if candidate.BillablePromptTokens == 0 {
		candidate.BillablePromptTokens = req.PromptTokens
	}
	if candidate.BillableCompletionTokens == 0 {
		candidate.BillableCompletionTokens = req.OutputTokens
	}
	if candidate.UpstreamPromptTokens == 0 {
		candidate.UpstreamPromptTokens = candidate.BillablePromptTokens
	}
	if candidate.UpstreamCompletionTokens == 0 {
		candidate.UpstreamCompletionTokens = candidate.BillableCompletionTokens
	}
	if candidate.RevenueUSD == 0 {
		candidate.RevenueUSD = req.RevenueUSD
	}
	return normalizeCostInput(candidate)
}

func costForMillionTokens(tokens int, price float64) float64 {
	if tokens <= 0 || price <= 0 {
		return 0
	}
	return float64(tokens) / 1000000 * price
}

func equalFoldTrim(a string, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func clamp01(value float64) float64 {
	if value < 0 || math.IsNaN(value) {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func floatPtr(value float64) *float64 {
	return &value
}
