package model

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type SatisfiedChannelCandidate struct {
	ChannelID    int    `json:"channel_id" gorm:"column:channel_id"`
	ChannelName  string `json:"channel_name" gorm:"column:channel_name"`
	ChannelType  int    `json:"channel_type" gorm:"column:channel_type"`
	ResponseTime int    `json:"response_time" gorm:"column:response_time"`
	Priority     int64  `json:"priority" gorm:"column:priority"`
	Weight       int    `json:"weight" gorm:"column:weight"`
}

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func GetSatisfiedChannelCandidatesForProfitObservation(group string, modelName string, limit int) ([]SatisfiedChannelCandidate, error) {
	if group == "" || modelName == "" || limit <= 0 {
		return nil, nil
	}
	if common.MemoryCacheEnabled {
		return getSatisfiedChannelCandidatesForProfitObservationCache(group, modelName, limit), nil
	}
	return getSatisfiedChannelCandidatesForProfitObservationDB(group, modelName, limit)
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func getSatisfiedChannelCandidatesForProfitObservationCache(group string, modelName string, limit int) []SatisfiedChannelCandidate {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil || channelsIDM == nil {
		return nil
	}
	channelIDs := group2model2channels[group][modelName]
	if len(channelIDs) == 0 {
		normalized := ratio_setting.FormatMatchingModelName(modelName)
		if normalized != "" && normalized != modelName {
			channelIDs = group2model2channels[group][normalized]
		}
	}
	if len(channelIDs) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(channelIDs))
	candidates := make([]SatisfiedChannelCandidate, 0, minInt(limit, len(channelIDs)))
	for _, channelID := range channelIDs {
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		channel := channelsIDM[channelID]
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		candidates = append(candidates, SatisfiedChannelCandidate{
			ChannelID:    channel.Id,
			ChannelName:  channel.Name,
			ChannelType:  channel.Type,
			ResponseTime: channel.ResponseTime,
			Priority:     channel.GetPriority(),
			Weight:       channel.GetWeight(),
		})
	}
	sortSatisfiedChannelCandidates(candidates)
	if len(candidates) > limit {
		return candidates[:limit]
	}
	return candidates
}

func getSatisfiedChannelCandidatesForProfitObservationDB(group string, modelName string, limit int) ([]SatisfiedChannelCandidate, error) {
	modelNames := []string{modelName}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		modelNames = append(modelNames, normalized)
	}
	for _, lookupModel := range modelNames {
		var candidates []SatisfiedChannelCandidate
		err := DB.Table("abilities").
			Select("channels.id as channel_id, channels.name as channel_name, channels.type as channel_type, channels.response_time as response_time, COALESCE(abilities.priority, 0) as priority, COALESCE(abilities.weight, 0) as weight").
			Joins("JOIN channels ON abilities.channel_id = channels.id").
			Where("abilities."+commonGroupCol+" = ? AND abilities.model = ? AND abilities.enabled = ? AND channels.status = ?", group, lookupModel, true, common.ChannelStatusEnabled).
			Order("COALESCE(abilities.priority, 0) DESC").
			Order("COALESCE(abilities.weight, 0) DESC").
			Order("abilities.channel_id ASC").
			Limit(limit).
			Scan(&candidates).Error
		if err != nil {
			return nil, err
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
	}
	return nil, nil
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}

func sortSatisfiedChannelCandidates(candidates []SatisfiedChannelCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if candidates[i].Weight != candidates[j].Weight {
			return candidates[i].Weight > candidates[j].Weight
		}
		return candidates[i].ChannelID < candidates[j].ChannelID
	})
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
