package model

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

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
	normalized := ratio_setting.RoutingMatchModelName(modelName)
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

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	return slices.Contains(list, channelID)
}

// CountSatisfiedChannels returns the number of enabled candidate channels for a
// group+model pair after applying the request filters, mirroring the candidate
// discovery in GetRandomSatisfiedChannel (exact model name, then the normalized
// routing name). It is read-only and never advances selection state. Used to decide
// whether a per-model first-byte timeout may safely fail the current channel.
func CountSatisfiedChannels(group, model string, filters []dto.ChannelFilter) int {
	if !common.MemoryCacheEnabled {
		return countSatisfiedChannelsDB(group, model, filters)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	channels, _ := filterCandidateIDs(group2model2channels[group][model], model, filters)
	if len(channels) == 0 {
		normalized := ratio_setting.RoutingMatchModelName(model)
		if normalized != "" && normalized != model {
			channels, _ = filterCandidateIDs(group2model2channels[group][normalized], model, filters)
		}
	}
	return len(channels)
}

func countSatisfiedChannelsDB(group, model string, filters []dto.ChannelFilter) int {
	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).Find(&abilities).Error; err != nil {
		return 0
	}
	abilities = filterAbilitiesByConstraints(abilities, model, filters)
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		seen[ability.ChannelId] = struct{}{}
	}
	return len(seen)
}
