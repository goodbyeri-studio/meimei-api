package model

import (
	"time"
)

type ChannelHealthStat struct {
	ChannelID        int     `json:"channel_id"`
	ChannelName      string  `json:"channel_name"`
	ChannelGroup     string  `json:"channel_group"`
	ChannelStatus    int     `json:"channel_status"`
	RequestCount     int64   `json:"request_count"`
	SuccessCount     int64   `json:"success_count"`
	ErrorCount       int64   `json:"error_count"`
	SuccessRate      float64 `json:"success_rate"`
	AverageLatencyMs int64   `json:"average_latency_ms"`
	LastRequestAt    int64   `json:"last_request_at"`
}

func GetChannelHealthStats(since time.Time) ([]ChannelHealthStat, error) {
	type aggregate struct {
		ChannelID    int     `gorm:"column:channel_id"`
		RequestCount int64   `gorm:"column:request_count"`
		SuccessCount int64   `gorm:"column:success_count"`
		ErrorCount   int64   `gorm:"column:error_count"`
		AverageUse   float64 `gorm:"column:average_use"`
		LastRequest  int64   `gorm:"column:last_request"`
	}
	aggregates := make([]aggregate, 0)
	err := LOG_DB.Table("logs").
		Select("channel_id, COUNT(*) AS request_count, SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_count, SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS error_count, COALESCE(AVG(CASE WHEN type = ? THEN use_time ELSE NULL END), 0) AS average_use, MAX(created_at) AS last_request", LogTypeConsume, LogTypeError, LogTypeConsume).
		Where("channel_id > 0 AND created_at >= ? AND type IN ?", since.Unix(), []int{LogTypeConsume, LogTypeError}).
		Group("channel_id").Scan(&aggregates).Error
	if err != nil {
		return nil, err
	}
	channels := make([]Channel, 0)
	if err := DB.Select("id", "name", commonGroupCol, "status").Find(&channels).Error; err != nil {
		return nil, err
	}
	byChannel := make(map[int]aggregate, len(aggregates))
	for _, item := range aggregates {
		byChannel[item.ChannelID] = item
	}
	result := make([]ChannelHealthStat, 0, len(channels))
	for _, channel := range channels {
		item := byChannel[channel.Id]
		stat := ChannelHealthStat{
			ChannelID: channel.Id, ChannelName: channel.Name, ChannelGroup: channel.Group, ChannelStatus: channel.Status,
			RequestCount: item.RequestCount, SuccessCount: item.SuccessCount, ErrorCount: item.ErrorCount,
			AverageLatencyMs: int64(item.AverageUse * 1000), LastRequestAt: item.LastRequest,
		}
		if item.RequestCount > 0 {
			stat.SuccessRate = float64(item.SuccessCount) / float64(item.RequestCount) * 100
		}
		result = append(result, stat)
	}
	return result, nil
}
