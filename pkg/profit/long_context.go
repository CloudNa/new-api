package profit

import (
	"sort"
	"strings"
)

const (
	DefaultLongContextPolicyID = "default-long-context-premium"
)

type LongContextInput struct {
	Group                    string  `json:"group"`
	ModelName                string  `json:"model_name"`
	ChannelID                int     `json:"channel_id"`
	ChannelName              string  `json:"channel_name"`
	BillablePromptTokens     int     `json:"billable_prompt_tokens"`
	BillableCompletionTokens int     `json:"billable_completion_tokens"`
	EstimatedRevenueUSD      float64 `json:"estimated_revenue_usd"`
}

type LongContextDecision struct {
	Mode                     string  `json:"mode"`
	PolicyID                 string  `json:"policy_id,omitempty"`
	PolicyName               string  `json:"policy_name,omitempty"`
	TierID                   string  `json:"tier_id,omitempty"`
	TierName                 string  `json:"tier_name,omitempty"`
	ContextTokens            int     `json:"context_tokens"`
	MinContextTokens         int     `json:"min_context_tokens,omitempty"`
	MaxContextTokens         int     `json:"max_context_tokens,omitempty"`
	InputMultiplier          float64 `json:"input_multiplier"`
	EstimatedInputRevenueUSD float64 `json:"estimated_input_revenue_usd"`
	SuggestedExtraRevenueUSD float64 `json:"suggested_extra_revenue_usd"`
	SuggestedRevenueUSD      float64 `json:"suggested_revenue_usd"`
	PremiumRequired          bool    `json:"premium_required"`
	PremiumGroup             string  `json:"premium_group,omitempty"`
	ObserveOnly              bool    `json:"observe_only"`
	LiveEnforced             bool    `json:"live_enforced"`
}

func DefaultLongContextPolicy() LongContextPolicy {
	return LongContextPolicy{
		ID:        DefaultLongContextPolicyID,
		Name:      "Default long context premium observe",
		Enabled:   true,
		Priority:  0,
		Group:     DefaultObserveGroup,
		ModelName: "*",
		Mode:      ModeObserve,
		Tiers: []LongContextTier{
			{
				ID:               "base",
				Name:             "<=32k",
				MinContextTokens: 0,
				MaxContextTokens: 32000,
				InputMultiplier:  1,
			},
			{
				ID:               "32k-128k",
				Name:             "32k-128k",
				MinContextTokens: 32001,
				MaxContextTokens: 128000,
				InputMultiplier:  1.25,
			},
			{
				ID:               "128k-512k",
				Name:             "128k-512k",
				MinContextTokens: 128001,
				MaxContextTokens: 512000,
				InputMultiplier:  1.75,
			},
			{
				ID:               "512k-plus",
				Name:             ">512k",
				MinContextTokens: 512001,
				InputMultiplier:  2.5,
				PremiumRequired:  true,
			},
		},
	}
}

func BuildLongContextDecision(input LongContextInput) *LongContextDecision {
	settings := CurrentSettings().Normalize()
	return BuildLongContextDecisionWithSettings(input, settings)
}

func BuildLongContextDecisionWithSettings(input LongContextInput, settings Settings) *LongContextDecision {
	settings = settings.Normalize()
	input = normalizeLongContextInput(input)
	if !settingsEnabledForGroup(settings, input.Group) || settings.LongContextMode == ModeOff {
		return nil
	}

	policy, ok := MatchLongContextPolicy(settings, input)
	if !ok {
		policy = DefaultLongContextPolicy()
	}
	policy = normalizeLongContextPolicy(policy)
	if !policy.Enabled {
		return nil
	}
	mode := strings.TrimSpace(policy.Mode)
	if mode == "" {
		mode = settings.LongContextMode
	}
	if mode == ModeOff {
		return nil
	}
	if len(policy.Tiers) == 0 {
		policy.Tiers = DefaultLongContextPolicy().Tiers
	}

	tier, ok := matchLongContextTier(policy.Tiers, input.BillablePromptTokens)
	if !ok {
		return nil
	}
	if tier.InputMultiplier <= 1 && !tier.PremiumRequired {
		return nil
	}
	inputRevenue := estimateInputRevenueUSD(input.EstimatedRevenueUSD, input.BillablePromptTokens, input.BillableCompletionTokens)
	extraRevenue := inputRevenue * (tier.InputMultiplier - 1)
	if extraRevenue < 0 {
		extraRevenue = 0
	}

	return &LongContextDecision{
		Mode:                     mode,
		PolicyID:                 policy.ID,
		PolicyName:               policy.Name,
		TierID:                   tier.ID,
		TierName:                 tier.Name,
		ContextTokens:            input.BillablePromptTokens,
		MinContextTokens:         tier.MinContextTokens,
		MaxContextTokens:         tier.MaxContextTokens,
		InputMultiplier:          tier.InputMultiplier,
		EstimatedInputRevenueUSD: inputRevenue,
		SuggestedExtraRevenueUSD: extraRevenue,
		SuggestedRevenueUSD:      input.EstimatedRevenueUSD + extraRevenue,
		PremiumRequired:          tier.PremiumRequired,
		PremiumGroup:             policy.PremiumGroup,
		ObserveOnly:              true,
		LiveEnforced:             false,
	}
}

func MatchLongContextPolicy(settings Settings, input LongContextInput) (LongContextPolicy, bool) {
	settings = settings.Normalize()
	input = normalizeLongContextInput(input)
	type scoredPolicy struct {
		policy LongContextPolicy
		score  int
	}
	matches := make([]scoredPolicy, 0)
	for _, policy := range settings.LongContextPolicies {
		if !policy.Enabled {
			continue
		}
		score, ok := longContextPolicyMatchScore(policy, input)
		if !ok {
			continue
		}
		matches = append(matches, scoredPolicy{policy: policy, score: score})
	}
	if len(matches) == 0 {
		return LongContextPolicy{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].policy.Priority > matches[j].policy.Priority
	})
	return matches[0].policy, true
}

func longContextPolicyMatchScore(policy LongContextPolicy, input LongContextInput) (int, bool) {
	score := 0
	if policy.Group != "" {
		if !equalFoldTrim(policy.Group, input.Group) {
			return 0, false
		}
		score += 12
	}
	if policy.ChannelID != 0 {
		if policy.ChannelID != input.ChannelID {
			return 0, false
		}
		score += 10
	}
	if policy.ChannelName != "" {
		if !equalFoldTrim(policy.ChannelName, input.ChannelName) {
			return 0, false
		}
		score += 6
	}
	if policy.ModelName != "" && policy.ModelName != "*" {
		modelScore, ok := modelPatternScore(policy.ModelName, input.ModelName)
		if !ok {
			return 0, false
		}
		score += modelScore
	}
	return score, true
}

func matchLongContextTier(tiers []LongContextTier, contextTokens int) (LongContextTier, bool) {
	normalized := normalizeLongContextTiers(tiers)
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].MinContextTokens != normalized[j].MinContextTokens {
			return normalized[i].MinContextTokens > normalized[j].MinContextTokens
		}
		return normalized[i].MaxContextTokens > normalized[j].MaxContextTokens
	})
	for _, tier := range normalized {
		if contextTokens < tier.MinContextTokens {
			continue
		}
		if tier.MaxContextTokens > 0 && contextTokens > tier.MaxContextTokens {
			continue
		}
		return tier, true
	}
	return LongContextTier{}, false
}

func normalizeLongContextPolicy(policy LongContextPolicy) LongContextPolicy {
	policies := normalizeLongContextPolicies([]LongContextPolicy{policy})
	if len(policies) == 0 {
		return LongContextPolicy{}
	}
	return policies[0]
}

func normalizeLongContextInput(input LongContextInput) LongContextInput {
	input.Group = strings.TrimSpace(input.Group)
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ChannelName = strings.TrimSpace(input.ChannelName)
	input.BillablePromptTokens = maxInt(0, input.BillablePromptTokens)
	input.BillableCompletionTokens = maxInt(0, input.BillableCompletionTokens)
	if input.EstimatedRevenueUSD < 0 {
		input.EstimatedRevenueUSD = 0
	}
	return input
}

func estimateInputRevenueUSD(revenueUSD float64, promptTokens int, completionTokens int) float64 {
	if revenueUSD <= 0 {
		return 0
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens <= 0 {
		return revenueUSD
	}
	return revenueUSD * float64(promptTokens) / float64(totalTokens)
}
