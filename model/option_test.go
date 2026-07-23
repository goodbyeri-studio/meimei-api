package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitOptionMapNotice(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))

	testCases := []struct {
		name     string
		stored   *Option
		expected string
	}{
		{
			name:     "uses default when option is absent",
			expected: defaultNotice,
		},
		{
			name:     "uses default when stored option is empty",
			stored:   &Option{Key: "Notice", Value: ""},
			expected: defaultNotice,
		},
		{
			name:     "preserves administrator notice",
			stored:   &Option{Key: "Notice", Value: "管理员自定义通知"},
			expected: "管理员自定义通知",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, DB.Where("1 = 1").Delete(&Option{}).Error)
			if testCase.stored != nil {
				require.NoError(t, DB.Create(testCase.stored).Error)
			}

			InitOptionMap()

			common.OptionMapRWMutex.RLock()
			actual := common.OptionMap["Notice"]
			common.OptionMapRWMutex.RUnlock()
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func TestUpdateGroupRatioAtomicallyPreservesConfiguration(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))

	var previous Option
	previousResult := DB.Where("key = ?", "GroupRatio").First(&previous)
	hadPrevious := previousResult.Error == nil
	if previousResult.Error != nil {
		require.ErrorIs(t, previousResult.Error, gorm.ErrRecordNotFound)
	}
	previousRuntimeValue := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		if hadPrevious {
			require.NoError(t, DB.Save(&previous).Error)
		} else {
			require.NoError(t, DB.Where("key = ?", "GroupRatio").Delete(&Option{}).Error)
		}
		require.NoError(t, updateOptionMap("GroupRatio", previousRuntimeValue))
	})

	require.NoError(t, DB.Where("key = ?", "GroupRatio").Delete(&Option{}).Error)
	require.NoError(t, DB.Create(&Option{
		Key:   "GroupRatio",
		Value: `{"default":1,"vip":1.5}`,
	}).Error)
	InitOptionMap()

	ratios, err := UpdateGroupRatioAtomically("default", 1.2)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"default": 1.2, "vip": 1.5}, ratios)

	var stored Option
	require.NoError(t, DB.Where("key = ?", "GroupRatio").First(&stored).Error)
	var storedRatios map[string]float64
	require.NoError(t, common.Unmarshal([]byte(stored.Value), &storedRatios))
	assert.Equal(t, ratios, storedRatios)

	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "GroupRatio").
		Update("value", `{"default":"invalid"}`).Error)
	_, err = UpdateGroupRatioAtomically("vip", 2)
	require.Error(t, err)
	require.NoError(t, DB.Where("key = ?", "GroupRatio").First(&stored).Error)
	assert.JSONEq(t, `{"default":"invalid"}`, stored.Value)
}
