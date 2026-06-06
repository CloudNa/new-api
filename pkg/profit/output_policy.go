package profit

import (
	"sort"
	"strings"
)

type OutputPolicyInput struct {
	Group            string `json:"group"`
	ModelName        string `json:"model_name"`
	ChannelID        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	CompletionTokens int    `json:"completion_tokens"`
}

type OutputPolicyDecision struct {
	Mode             string `json:"mode"`
	PolicyID         string `json:"policy_id,omitempty"`
	PolicyName       string `json:"policy_name,omitempty"`
	CompletionTokens int    `json:"completion_tokens"`
	DefaultMaxTokens int    `json:"default_max_tokens,omitempty"`
	HardMaxTokens    int    `json:"hard_max_tokens,omitempty"`
	ExceededDefault  bool   `json:"exceeded_default"`
	ExceededHard     bool   `json:"exceeded_hard"`
	RewriteOverLimit bool   `json:"rewrite_over_limit"`
	WouldCap         bool   `json:"would_cap"`
	PremiumRequired  bool   `json:"premium_required"`
	PremiumGroup     string `json:"premium_group,omitempty"`
	ObserveOnly      bool   `json:"observe_only"`
	LiveEnforced     bool   `json:"live_enforced"`
}

func BuildOutputPolicyDecision(input OutputPolicyInput) *OutputPolicyDecision {
	settings := CurrentSettings().Normalize()
	return BuildOutputPolicyDecisionWithSettings(input, settings)
}

func BuildOutputPolicyDecisionWithSettings(input OutputPolicyInput, settings Settings) *OutputPolicyDecision {
	settings = settings.Normalize()
	input = normalizeOutputPolicyInput(input)
	if !settingsEnabledForGroup(settings, input.Group) || settings.OutputCapMode == ModeOff {
		return nil
	}

	policy, ok := MatchOutputPolicy(settings, input)
	mode := settings.OutputCapMode
	decision := &OutputPolicyDecision{
		Mode:             mode,
		CompletionTokens: input.CompletionTokens,
		ObserveOnly:      true,
		LiveEnforced:     false,
	}
	if ok {
		mode = strings.TrimSpace(policy.Mode)
		if mode == "" {
			mode = settings.OutputCapMode
		}
		if mode == ModeOff {
			return nil
		}
		decision.Mode = mode
		decision.PolicyID = policy.ID
		decision.PolicyName = policy.Name
		decision.DefaultMaxTokens = policy.DefaultMaxTokens
		decision.HardMaxTokens = policy.HardMaxTokens
		decision.RewriteOverLimit = policy.RewriteOverLimit
		decision.PremiumRequired = policy.PremiumRequired
		decision.PremiumGroup = policy.PremiumGroup
	}

	decision.ExceededDefault = decision.DefaultMaxTokens > 0 && input.CompletionTokens > decision.DefaultMaxTokens
	decision.ExceededHard = decision.HardMaxTokens > 0 && input.CompletionTokens > decision.HardMaxTokens
	if decision.Mode == ModeCap {
		decision.WouldCap = decision.ExceededHard || (decision.RewriteOverLimit && decision.ExceededDefault)
	}
	if decision.Mode == ModePremiumRequired && (decision.ExceededDefault || decision.ExceededHard) {
		decision.PremiumRequired = true
	}
	return decision
}

func MatchOutputPolicy(settings Settings, input OutputPolicyInput) (OutputPolicy, bool) {
	settings = settings.Normalize()
	input = normalizeOutputPolicyInput(input)
	type scoredPolicy struct {
		policy OutputPolicy
		score  int
	}
	matches := make([]scoredPolicy, 0)
	for _, policy := range settings.OutputPolicies {
		if !policy.Enabled {
			continue
		}
		score, ok := outputPolicyMatchScore(policy, input)
		if !ok {
			continue
		}
		matches = append(matches, scoredPolicy{policy: policy, score: score})
	}
	if len(matches) == 0 {
		return OutputPolicy{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].policy.Priority > matches[j].policy.Priority
	})
	return matches[0].policy, true
}

func outputPolicyMatchScore(policy OutputPolicy, input OutputPolicyInput) (int, bool) {
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

func normalizeOutputPolicyInput(input OutputPolicyInput) OutputPolicyInput {
	input.Group = strings.TrimSpace(input.Group)
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ChannelName = strings.TrimSpace(input.ChannelName)
	input.CompletionTokens = maxInt(0, input.CompletionTokens)
	return input
}
