package profit

import (
	"sort"
	"strings"
)

const ContextKeyModelAliasDecision = "profit_model_alias_decision"

type ModelAliasInput struct {
	Group string
	SKU   string
}

type ModelAliasDecision struct {
	Applied           bool   `json:"applied"`
	Mode              string `json:"mode,omitempty"`
	AliasID           string `json:"alias_id,omitempty"`
	AliasName         string `json:"alias_name,omitempty"`
	SKU               string `json:"sku,omitempty"`
	UpstreamModelName string `json:"upstream_model_name,omitempty"`
	TargetChannelID   int    `json:"target_channel_id,omitempty"`
	TargetChannelName string `json:"target_channel_name,omitempty"`
	CandidateCount    int    `json:"candidate_count,omitempty"`
	ObserveOnly       bool   `json:"observe_only"`
	BypassReason      string `json:"bypass_reason,omitempty"`
}

func ResolveModelAlias(settings Settings, input ModelAliasInput) ModelAliasDecision {
	settings = settings.Normalize()
	group := strings.TrimSpace(input.Group)
	sku := strings.TrimSpace(input.SKU)
	switch {
	case !settings.Enabled:
		return ModelAliasDecision{SKU: sku, BypassReason: "disabled"}
	case settings.GlobalKillSwitch:
		return ModelAliasDecision{SKU: sku, BypassReason: "global_kill_switch"}
	case settings.ModelAliasMode == ModeOff:
		return ModelAliasDecision{SKU: sku, BypassReason: "mode_off"}
	case !settingsEnabledForGroup(settings, group):
		return ModelAliasDecision{SKU: sku, BypassReason: "group_not_enabled"}
	case sku == "":
		return ModelAliasDecision{BypassReason: "empty_sku"}
	}

	aliases := matchingModelAliases(settings.ModelAliases, group, sku)
	if len(aliases) == 0 {
		return ModelAliasDecision{SKU: sku, BypassReason: "no_alias"}
	}
	alias := aliases[0]
	mode := strings.TrimSpace(alias.Mode)
	if mode == "" {
		mode = settings.ModelAliasMode
	}
	if mode == ModeOff {
		return ModelAliasDecision{
			Mode:         mode,
			AliasID:      alias.ID,
			AliasName:    alias.Name,
			SKU:          sku,
			BypassReason: "alias_off",
		}
	}
	targets := sortedModelAliasTargets(alias.Targets)
	if len(targets) == 0 {
		return ModelAliasDecision{
			Mode:         mode,
			AliasID:      alias.ID,
			AliasName:    alias.Name,
			SKU:          sku,
			BypassReason: "no_target",
		}
	}
	target := targets[0]
	return ModelAliasDecision{
		Applied:           true,
		Mode:              mode,
		AliasID:           alias.ID,
		AliasName:         alias.Name,
		SKU:               sku,
		UpstreamModelName: target.ModelName,
		TargetChannelID:   target.ChannelID,
		TargetChannelName: target.ChannelName,
		CandidateCount:    len(targets),
		ObserveOnly:       settings.ObserveOnly || mode == ModeObserve,
	}
}

func matchingModelAliases(aliases []ModelAlias, group string, sku string) []ModelAlias {
	out := make([]ModelAlias, 0, len(aliases))
	for _, alias := range normalizeModelAliases(aliases) {
		if !alias.Enabled {
			continue
		}
		if alias.Group != group || alias.SKU != sku {
			continue
		}
		out = append(out, alias)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out
}

func sortedModelAliasTargets(targets []ModelAliasTarget) []ModelAliasTarget {
	out := normalizeModelAliasTargets(targets)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].Weight > out[j].Weight
		}
		return out[i].Priority > out[j].Priority
	})
	return out
}
