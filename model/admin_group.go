package model

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const ManagedGroupStatusDisabled = "disabled"

type ManagedGroupState struct {
	Id        int    `json:"id"`
	GroupName string `json:"group_name" gorm:"type:varchar(64);uniqueIndex"`
	Status    string `json:"status" gorm:"type:varchar(24);index"`
	Reason    string `json:"reason" gorm:"type:varchar(255)"`
	UpdatedBy int    `json:"updated_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ManagedGroupReferences struct {
	Users                 int64 `json:"users"`
	Tokens                int64 `json:"tokens"`
	Channels              int64 `json:"channels"`
	SubscriptionPlans     int64 `json:"subscription_plans"`
	ActiveSubscriptions   int64 `json:"active_subscriptions"`
	AutoGroup             bool  `json:"auto_group"`
	UserUsableGroup       bool  `json:"user_usable_group"`
	GroupRatioOverrides   int   `json:"group_ratio_overrides"`
	SpecialUsableMappings int   `json:"special_usable_mappings"`
}

func (r ManagedGroupReferences) BlockingCount() int64 {
	return r.Users + r.Tokens + r.Channels + r.SubscriptionPlans + r.ActiveSubscriptions
}

type ManagedGroup struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Ratio       float64                `json:"ratio"`
	Disabled    bool                   `json:"disabled"`
	Reason      string                 `json:"reason,omitempty"`
	UpdatedAt   int64                  `json:"updated_at,omitempty"`
	References  ManagedGroupReferences `json:"references"`
}

var managedGroupStateCache struct {
	sync.RWMutex
	loadedAt time.Time
	disabled map[string]ManagedGroupState
}

func refreshManagedGroupStateCache() {
	managedGroupStateCache.Lock()
	defer managedGroupStateCache.Unlock()
	if time.Since(managedGroupStateCache.loadedAt) < 10*time.Second && managedGroupStateCache.disabled != nil {
		return
	}
	states := make([]ManagedGroupState, 0)
	if err := DB.Where("status = ?", ManagedGroupStatusDisabled).Find(&states).Error; err != nil {
		common.SysError("failed to load managed group states: " + err.Error())
		return
	}
	disabled := make(map[string]ManagedGroupState, len(states))
	for _, state := range states {
		disabled[state.GroupName] = state
	}
	managedGroupStateCache.disabled = disabled
	managedGroupStateCache.loadedAt = time.Now()
}

func IsManagedGroupDisabled(group string) bool {
	refreshManagedGroupStateCache()
	managedGroupStateCache.RLock()
	defer managedGroupStateCache.RUnlock()
	_, disabled := managedGroupStateCache.disabled[group]
	return disabled
}

func invalidateManagedGroupStateCache() {
	managedGroupStateCache.Lock()
	managedGroupStateCache.loadedAt = time.Time{}
	managedGroupStateCache.Unlock()
}

func GetManagedGroupReferences(group string) (ManagedGroupReferences, error) {
	refs := ManagedGroupReferences{}
	if err := DB.Model(&User{}).Where(commonGroupCol+" = ?", group).Count(&refs.Users).Error; err != nil {
		return refs, err
	}
	if err := DB.Model(&Token{}).Where(commonGroupCol+" = ?", group).Count(&refs.Tokens).Error; err != nil {
		return refs, err
	}
	if err := ApplyChannelGroupFilter(DB.Model(&Channel{}), group).Count(&refs.Channels).Error; err != nil {
		return refs, err
	}
	if err := DB.Model(&SubscriptionPlan{}).
		Where("upgrade_group = ? OR downgrade_group = ?", group, group).
		Count(&refs.SubscriptionPlans).Error; err != nil {
		return refs, err
	}
	if err := DB.Model(&UserSubscription{}).
		Where("status = ? AND (upgrade_group = ? OR downgrade_group = ? OR prev_user_group = ?)", "active", group, group, group).
		Count(&refs.ActiveSubscriptions).Error; err != nil {
		return refs, err
	}
	_, refs.UserUsableGroup = setting.GetUserUsableGroupsCopy()[group]
	for _, name := range setting.GetAutoGroups() {
		if name == group {
			refs.AutoGroup = true
			break
		}
	}
	for source, ratios := range ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll() {
		if source == group {
			refs.GroupRatioOverrides += len(ratios)
		}
		if _, ok := ratios[group]; ok {
			refs.GroupRatioOverrides++
		}
	}
	for source, mappings := range ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll() {
		if source == group {
			refs.SpecialUsableMappings += len(mappings)
		}
		for target := range mappings {
			target = strings.TrimPrefix(strings.TrimPrefix(target, "+:"), "-:")
			if target == group {
				refs.SpecialUsableMappings++
			}
		}
	}
	return refs, nil
}

func ListManagedGroups() ([]ManagedGroup, error) {
	ratios := ratio_setting.GetGroupRatioCopy()
	descriptions := setting.GetUserUsableGroupsCopy()
	groups := make([]ManagedGroup, 0, len(ratios))
	for name, ratio := range ratios {
		refs, err := GetManagedGroupReferences(name)
		if err != nil {
			return nil, err
		}
		group := ManagedGroup{Name: name, Description: descriptions[name], Ratio: ratio, References: refs}
		var state ManagedGroupState
		result := DB.Where("group_name = ?", name).Limit(1).Find(&state)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected > 0 && state.Status == ManagedGroupStatusDisabled {
			group.Disabled = true
			group.Reason = state.Reason
			group.UpdatedAt = state.UpdatedAt
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func UpsertManagedGroup(name, description string, ratio float64) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || len(name) > 64 || strings.ContainsAny(name, ",\r\n") {
		return errors.New("分组名称不能为空、不能包含逗号或换行，且最多 64 个字符")
	}
	if ratio <= 0 || ratio > 1000 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return errors.New("分组倍率必须在 (0, 1000] 范围内")
	}
	ratios := ratio_setting.GetGroupRatioCopy()
	descriptions := setting.GetUserUsableGroupsCopy()
	ratios[name] = ratio
	if description == "" {
		description = name
	}
	descriptions[name] = description
	ratioJSON, err := common.Marshal(ratios)
	if err != nil {
		return err
	}
	descriptionJSON, err := common.Marshal(descriptions)
	if err != nil {
		return err
	}
	return UpdateOptionsBulk(map[string]string{"GroupRatio": string(ratioJSON), "UserUsableGroups": string(descriptionJSON)})
}

func SetManagedGroupDisabled(group string, disabled bool, reason string, adminID int) error {
	group = strings.TrimSpace(group)
	if !ratio_setting.ContainsGroupRatio(group) {
		return gorm.ErrRecordNotFound
	}
	now := common.GetTimestamp()
	if disabled {
		state := ManagedGroupState{GroupName: group}
		if err := DB.Where("group_name = ?", group).FirstOrCreate(&state).Error; err != nil {
			return err
		}
		if err := DB.Model(&state).Updates(map[string]interface{}{
			"status": ManagedGroupStatusDisabled, "reason": strings.TrimSpace(reason),
			"updated_by": adminID, "updated_at": now,
		}).Error; err != nil {
			return err
		}
	} else if err := DB.Where("group_name = ?", group).Delete(&ManagedGroupState{}).Error; err != nil {
		return err
	}
	invalidateManagedGroupStateCache()
	return nil
}

func DeleteManagedGroup(group string) error {
	group = strings.TrimSpace(group)
	if group == "default" || group == "auto" {
		return errors.New("系统分组不能删除")
	}
	if !ratio_setting.ContainsGroupRatio(group) {
		return gorm.ErrRecordNotFound
	}
	refs, err := GetManagedGroupReferences(group)
	if err != nil {
		return err
	}
	if refs.BlockingCount() > 0 {
		return errors.New("分组仍被用户、密钥、渠道或套餐引用，不能删除")
	}
	ratios := ratio_setting.GetGroupRatioCopy()
	descriptions := setting.GetUserUsableGroupsCopy()
	autoGroups := setting.GetAutoGroups()
	delete(ratios, group)
	delete(descriptions, group)
	filteredAutoGroups := make([]string, 0, len(autoGroups))
	for _, name := range autoGroups {
		if name != group {
			filteredAutoGroups = append(filteredAutoGroups, name)
		}
	}
	ratioJSON, err := common.Marshal(ratios)
	if err != nil {
		return err
	}
	descriptionJSON, err := common.Marshal(descriptions)
	if err != nil {
		return err
	}
	autoJSON, err := common.Marshal(filteredAutoGroups)
	if err != nil {
		return err
	}
	if err := UpdateOptionsBulk(map[string]string{
		"GroupRatio": string(ratioJSON), "UserUsableGroups": string(descriptionJSON), "AutoGroups": string(autoJSON),
	}); err != nil {
		return err
	}
	if err := DB.Where("group_name = ?", group).Delete(&ManagedGroupState{}).Error; err != nil {
		return err
	}
	invalidateManagedGroupStateCache()
	return nil
}
