package model

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

type ChannelKeyUsage struct {
	Id             int    `json:"id"`
	ChannelId      int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_key_usage"`
	KeyIndex       int    `json:"key_index" gorm:"uniqueIndex:idx_channel_key_usage"`
	KeyFingerprint string `json:"key_fingerprint" gorm:"type:varchar(24)"`
	LastUsedAt     int64  `json:"last_used_at" gorm:"index"`
	UpdatedAt      int64  `json:"updated_at"`
}

var channelKeyUsageThrottle sync.Map

func RecordChannelKeyUsageAsync(channelID, keyIndex int, key string) {
	if channelID <= 0 || key == "" {
		return
	}
	cacheKey := fmt.Sprintf("%d:%d", channelID, keyIndex)
	now := time.Now().Unix()
	if previous, ok := channelKeyUsageThrottle.Load(cacheKey); ok && now-previous.(int64) < 60 {
		return
	}
	channelKeyUsageThrottle.Store(cacheKey, now)
	digest := sha256.Sum256([]byte(key))
	go func() {
		usage := ChannelKeyUsage{
			ChannelId: channelID, KeyIndex: keyIndex,
			KeyFingerprint: fmt.Sprintf("%x", digest[:12]), LastUsedAt: now, UpdatedAt: common.GetTimestamp(),
		}
		if err := DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "channel_id"}, {Name: "key_index"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"key_fingerprint": usage.KeyFingerprint, "last_used_at": usage.LastUsedAt, "updated_at": usage.UpdatedAt,
			}),
		}).Create(&usage).Error; err != nil {
			common.SysError("failed to record channel key usage: " + err.Error())
		}
	}()
}

func GetChannelKeyUsage(channelID int) (map[int]ChannelKeyUsage, error) {
	rows := make([]ChannelKeyUsage, 0)
	if err := DB.Where("channel_id = ?", channelID).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int]ChannelKeyUsage, len(rows))
	for _, row := range rows {
		result[row.KeyIndex] = row
	}
	return result, nil
}

func DeleteChannelKeyUsage(channelID int, keyIndex *int) error {
	query := DB.Where("channel_id = ?", channelID)
	if keyIndex != nil {
		query = query.Where("key_index = ?", *keyIndex)
	}
	return query.Delete(&ChannelKeyUsage{}).Error
}
