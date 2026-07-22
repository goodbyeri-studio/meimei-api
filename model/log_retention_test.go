package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrimUserLogsBeyondLimitKeepsNewestRows(t *testing.T) {
	const userId = 910001
	const otherUserId = 910002
	require.NoError(t, LOG_DB.Where("user_id IN ?", []int{userId, otherUserId}).Delete(&Log{}).Error)
	t.Cleanup(func() {
		require.NoError(t, LOG_DB.Where("user_id IN ?", []int{userId, otherUserId}).Delete(&Log{}).Error)
	})

	logs := make([]Log, 0, UserLogRetentionLimit+8)
	for i := 1; i <= UserLogRetentionLimit+5; i++ {
		logs = append(logs, Log{
			UserId:    userId,
			CreatedAt: int64(i),
			RequestId: fmt.Sprintf("retention-%03d", i),
		})
	}
	for i := 1; i <= 3; i++ {
		logs = append(logs, Log{
			UserId:    otherUserId,
			CreatedAt: int64(i),
			RequestId: fmt.Sprintf("other-%03d", i),
		})
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)

	deleted, err := TrimUserLogsBeyondLimit(context.Background(), userId, UserLogRetentionLimit)
	require.NoError(t, err)
	assert.Equal(t, int64(5), deleted)

	var retained []Log
	require.NoError(t, LOG_DB.Where("user_id = ?", userId).Order("id asc").Find(&retained).Error)
	require.Len(t, retained, UserLogRetentionLimit)
	assert.Equal(t, int64(6), retained[0].CreatedAt)
	assert.Equal(t, int64(UserLogRetentionLimit+5), retained[len(retained)-1].CreatedAt)

	var otherCount int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ?", otherUserId).Count(&otherCount).Error)
	assert.Equal(t, int64(3), otherCount)
}

func TestGetUserLogsCapsVisibleHistoryAtRetentionLimit(t *testing.T) {
	const userId = 910003
	require.NoError(t, LOG_DB.Where("user_id = ?", userId).Delete(&Log{}).Error)
	t.Cleanup(func() {
		require.NoError(t, LOG_DB.Where("user_id = ?", userId).Delete(&Log{}).Error)
	})

	logs := make([]Log, 0, UserLogRetentionLimit+1)
	for i := 1; i <= UserLogRetentionLimit+1; i++ {
		logs = append(logs, Log{
			UserId:    userId,
			CreatedAt: int64(i),
			RequestId: fmt.Sprintf("visible-%03d", i),
		})
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)

	logsPage, total, err := GetUserLogs(userId, LogTypeUnknown, 0, 0, "", "", 490, 20, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(UserLogRetentionLimit), total)
	require.Len(t, logsPage, 10)

	beyondLimit, total, err := GetUserLogs(userId, LogTypeUnknown, 0, 0, "", "", UserLogRetentionLimit, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(UserLogRetentionLimit), total)
	assert.Empty(t, beyondLimit)
}
