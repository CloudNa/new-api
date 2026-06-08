package service

import (
	"github.com/QuantumNous/new-api/pkg/profit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func profitModelAliasDecision(info *relaycommon.RelayInfo) *profit.ModelAliasDecision {
	if info == nil || !info.ModelAliasApplied {
		return nil
	}
	return &profit.ModelAliasDecision{
		Applied:           true,
		Mode:              info.ModelAliasMode,
		AliasID:           info.ModelAliasID,
		AliasName:         info.ModelAliasName,
		SKU:               info.ModelAliasSKU,
		UpstreamModelName: info.ModelAliasUpstreamModelName,
		TargetChannelID:   info.ModelAliasTargetChannelID,
		TargetChannelName: info.ModelAliasTargetChannelName,
		CandidateCount:    info.ModelAliasCandidateCount,
		ObserveOnly:       info.ModelAliasObserveOnly,
		BypassReason:      info.ModelAliasBypassReason,
	}
}

func profitRoutingModelName(info *relaycommon.RelayInfo, fallback string) string {
	if info != nil && info.ModelAliasApplied && info.ModelAliasUpstreamModelName != "" {
		return info.ModelAliasUpstreamModelName
	}
	return fallback
}
