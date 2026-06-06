package profit

import "sort"

const DefaultRouteCandidateLimit = 8

type RouteDecisionInput struct {
	Group                          string
	Provider                       string
	SelectedChannelID              int
	SelectedChannelName            string
	ModelName                      string
	BillablePromptTokens           int
	BillableCompletionTokens       int
	UpstreamActualPromptTokens     int
	UpstreamActualCompletionTokens int
	CacheReadTokens                int
	CacheWriteTokens               int
	UserQuota                      int
	LatencyMs                      int
	Candidates                     []RouteCandidateInput
}

type RouteCandidateInput struct {
	Provider                       string  `json:"provider,omitempty"`
	ChannelID                      int     `json:"channel_id"`
	ChannelName                    string  `json:"channel_name,omitempty"`
	ModelName                      string  `json:"model_name,omitempty"`
	BillablePromptTokens           int     `json:"billable_prompt_tokens,omitempty"`
	BillableCompletionTokens       int     `json:"billable_completion_tokens,omitempty"`
	UpstreamActualPromptTokens     int     `json:"upstream_actual_prompt_tokens,omitempty"`
	UpstreamActualCompletionTokens int     `json:"upstream_actual_completion_tokens,omitempty"`
	CacheReadTokens                int     `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens               int     `json:"cache_write_tokens,omitempty"`
	UserQuota                      int     `json:"user_quota,omitempty"`
	LatencyMs                      int     `json:"latency_ms,omitempty"`
	FailureRate                    float64 `json:"failure_rate,omitempty"`
	Priority                       int64   `json:"priority,omitempty"`
	Weight                         int     `json:"weight,omitempty"`
}

type RouteDecision struct {
	Mode                  string                   `json:"mode"`
	CandidateCount        int                      `json:"candidate_count"`
	SelectedChannelID     int                      `json:"selected_channel_id"`
	SelectedMarginRank    int                      `json:"selected_margin_rank"`
	BestChannelID         int                      `json:"best_channel_id"`
	BestChannelName       string                   `json:"best_channel_name,omitempty"`
	BestCostProfileID     string                   `json:"best_cost_profile_id,omitempty"`
	BestExpectedMarginUSD *float64                 `json:"best_expected_margin_usd,omitempty"`
	WouldPreferDifferent  bool                     `json:"would_prefer_different"`
	Candidates            []RouteDecisionCandidate `json:"candidates,omitempty"`
}

type RouteDecisionCandidate struct {
	ChannelID         int      `json:"channel_id"`
	ChannelName       string   `json:"channel_name,omitempty"`
	CostKnown         bool     `json:"cost_known"`
	CostStatus        string   `json:"profit_cost_status"`
	CostProfileID     string   `json:"cost_profile_id,omitempty"`
	CostProfileName   string   `json:"cost_profile_name,omitempty"`
	ExpectedCostUSD   *float64 `json:"expected_cost_usd,omitempty"`
	ExpectedMarginUSD *float64 `json:"expected_margin_usd,omitempty"`
	ExpectedMarginPct *float64 `json:"expected_margin_pct,omitempty"`
	Selected          bool     `json:"selected"`
	WouldPrefer       bool     `json:"would_prefer"`
	MarginRank        int      `json:"margin_rank"`
	Priority          int64    `json:"priority,omitempty"`
	Weight            int      `json:"weight,omitempty"`
}

func BuildRouteDecision(input RouteDecisionInput) *RouteDecision {
	settings := CurrentSettings().Normalize()
	if !settings.Enabled || settings.GlobalKillSwitch || settings.CostRoutingMode != ModeObserve || !EnabledForGroup(input.Group) {
		return nil
	}

	candidates := input.Candidates
	if len(candidates) == 0 && input.SelectedChannelID > 0 {
		candidates = []RouteCandidateInput{{
			Provider:    input.Provider,
			ChannelID:   input.SelectedChannelID,
			ChannelName: input.SelectedChannelName,
		}}
	}
	if len(candidates) == 0 {
		return nil
	}

	revenueUSD := quotaToUSD(input.UserQuota)
	doc := CurrentCostProfiles()
	decisionCandidates := make([]RouteDecisionCandidate, 0, len(candidates))
	selectedFound := false
	for _, candidate := range candidates {
		candidate = inheritRouteDecisionDefaults(input, candidate)
		if candidate.ChannelID <= 0 && candidate.ChannelName == "" {
			continue
		}
		if candidate.ChannelID == input.SelectedChannelID {
			selectedFound = true
		}
		estimate := EstimateCost(CostInput{
			Group:                    input.Group,
			Provider:                 candidate.Provider,
			ChannelID:                candidate.ChannelID,
			ChannelName:              candidate.ChannelName,
			ModelName:                candidate.ModelName,
			BillablePromptTokens:     candidate.BillablePromptTokens,
			BillableCompletionTokens: candidate.BillableCompletionTokens,
			UpstreamPromptTokens:     candidate.UpstreamActualPromptTokens,
			UpstreamCompletionTokens: candidate.UpstreamActualCompletionTokens,
			CacheReadTokens:          candidate.CacheReadTokens,
			CacheWriteTokens:         candidate.CacheWriteTokens,
			RevenueUSD:               revenueUSD,
			LatencyMs:                candidate.LatencyMs,
			FailureRate:              candidate.FailureRate,
		}, doc)
		decisionCandidates = append(decisionCandidates, RouteDecisionCandidate{
			ChannelID:         candidate.ChannelID,
			ChannelName:       candidate.ChannelName,
			CostKnown:         estimate.CostKnown,
			CostStatus:        estimate.CostStatus,
			CostProfileID:     estimate.CostProfileID,
			CostProfileName:   estimate.CostProfileName,
			ExpectedCostUSD:   estimate.ExpectedCostUSD,
			ExpectedMarginUSD: estimate.ExpectedMarginUSD,
			ExpectedMarginPct: estimate.ExpectedMarginPct,
			Selected:          candidate.ChannelID == input.SelectedChannelID,
			Priority:          candidate.Priority,
			Weight:            candidate.Weight,
		})
	}
	if len(decisionCandidates) == 0 {
		return nil
	}
	if !selectedFound && input.SelectedChannelID > 0 {
		selected := inheritRouteDecisionDefaults(input, RouteCandidateInput{
			Provider:    input.Provider,
			ChannelID:   input.SelectedChannelID,
			ChannelName: input.SelectedChannelName,
		})
		estimate := EstimateCost(CostInput{
			Group:                    input.Group,
			Provider:                 selected.Provider,
			ChannelID:                selected.ChannelID,
			ChannelName:              selected.ChannelName,
			ModelName:                selected.ModelName,
			BillablePromptTokens:     selected.BillablePromptTokens,
			BillableCompletionTokens: selected.BillableCompletionTokens,
			UpstreamPromptTokens:     selected.UpstreamActualPromptTokens,
			UpstreamCompletionTokens: selected.UpstreamActualCompletionTokens,
			CacheReadTokens:          selected.CacheReadTokens,
			CacheWriteTokens:         selected.CacheWriteTokens,
			RevenueUSD:               revenueUSD,
			LatencyMs:                selected.LatencyMs,
			FailureRate:              selected.FailureRate,
		}, doc)
		decisionCandidates = append(decisionCandidates, RouteDecisionCandidate{
			ChannelID:         selected.ChannelID,
			ChannelName:       selected.ChannelName,
			CostKnown:         estimate.CostKnown,
			CostStatus:        estimate.CostStatus,
			CostProfileID:     estimate.CostProfileID,
			CostProfileName:   estimate.CostProfileName,
			ExpectedCostUSD:   estimate.ExpectedCostUSD,
			ExpectedMarginUSD: estimate.ExpectedMarginUSD,
			ExpectedMarginPct: estimate.ExpectedMarginPct,
			Selected:          true,
		})
	}

	ranked := rankedRouteDecisionIndexes(decisionCandidates)
	best := decisionCandidates[ranked[0]]
	selectedRank := 0
	for rank, idx := range ranked {
		decisionCandidates[idx].MarginRank = rank + 1
		if decisionCandidates[idx].ChannelID == best.ChannelID {
			decisionCandidates[idx].WouldPrefer = best.ExpectedMarginUSD != nil
		}
		if decisionCandidates[idx].Selected {
			selectedRank = rank + 1
		}
	}

	return &RouteDecision{
		Mode:                  settings.CostRoutingMode,
		CandidateCount:        len(decisionCandidates),
		SelectedChannelID:     input.SelectedChannelID,
		SelectedMarginRank:    selectedRank,
		BestChannelID:         best.ChannelID,
		BestChannelName:       best.ChannelName,
		BestCostProfileID:     best.CostProfileID,
		BestExpectedMarginUSD: best.ExpectedMarginUSD,
		WouldPreferDifferent:  selectedRank > 1 && best.ExpectedMarginUSD != nil && best.ChannelID != input.SelectedChannelID,
		Candidates:            decisionCandidates,
	}
}

func inheritRouteDecisionDefaults(input RouteDecisionInput, candidate RouteCandidateInput) RouteCandidateInput {
	if candidate.Provider == "" {
		candidate.Provider = input.Provider
	}
	if candidate.ChannelName == "" && candidate.ChannelID == input.SelectedChannelID {
		candidate.ChannelName = input.SelectedChannelName
	}
	if candidate.ModelName == "" {
		candidate.ModelName = input.ModelName
	}
	if candidate.BillablePromptTokens == 0 {
		candidate.BillablePromptTokens = input.BillablePromptTokens
	}
	if candidate.BillableCompletionTokens == 0 {
		candidate.BillableCompletionTokens = input.BillableCompletionTokens
	}
	if candidate.UpstreamActualPromptTokens == 0 {
		candidate.UpstreamActualPromptTokens = input.UpstreamActualPromptTokens
	}
	if candidate.UpstreamActualCompletionTokens == 0 {
		candidate.UpstreamActualCompletionTokens = input.UpstreamActualCompletionTokens
	}
	if candidate.CacheReadTokens == 0 {
		candidate.CacheReadTokens = input.CacheReadTokens
	}
	if candidate.CacheWriteTokens == 0 {
		candidate.CacheWriteTokens = input.CacheWriteTokens
	}
	if candidate.UserQuota == 0 {
		candidate.UserQuota = input.UserQuota
	}
	if candidate.LatencyMs == 0 {
		candidate.LatencyMs = input.LatencyMs
	}
	return candidate
}

func rankedRouteDecisionIndexes(candidates []RouteDecisionCandidate) []int {
	ranked := make([]int, 0, len(candidates))
	for idx := range candidates {
		ranked = append(ranked, idx)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		left := candidates[ranked[i]]
		right := candidates[ranked[j]]
		leftKnown := left.ExpectedMarginUSD != nil
		rightKnown := right.ExpectedMarginUSD != nil
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown && *left.ExpectedMarginUSD != *right.ExpectedMarginUSD {
			return *left.ExpectedMarginUSD > *right.ExpectedMarginUSD
		}
		if left.Priority != right.Priority {
			return left.Priority > right.Priority
		}
		if left.Weight != right.Weight {
			return left.Weight > right.Weight
		}
		return left.ChannelID < right.ChannelID
	})
	return ranked
}
