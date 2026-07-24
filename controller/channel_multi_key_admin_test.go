package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callManageMultiKeys(t *testing.T, request MultiKeyManageRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(request)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/multi_key/manage", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	ManageMultiKeys(ctx)
	return recorder
}

func TestManageMultiKeysUpgradesSingleKeyChannelAndRotatesKey(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelKeyUsage{}))
	channel := &model.Channel{
		Type: constant.ChannelTypeOpenAI, Name: "single-key-upgrade", Key: "sk-old",
		Status: common.ChannelStatusEnabled, Group: "default", Models: "gpt-4o",
	}
	require.NoError(t, db.Create(channel).Error)

	statusRecorder := callManageMultiKeys(t, MultiKeyManageRequest{
		ChannelId: channel.Id, Action: "get_key_status", Page: 1, PageSize: 10,
	})
	assert.Equal(t, http.StatusOK, statusRecorder.Code)
	var statusResponse struct {
		Success bool                   `json:"success"`
		Data    MultiKeyStatusResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(statusRecorder.Body.Bytes(), &statusResponse))
	assert.True(t, statusResponse.Success)
	require.Len(t, statusResponse.Data.Keys, 1)
	assert.NotContains(t, statusResponse.Data.Keys[0].KeyPreview, "sk-old")

	appendRecorder := callManageMultiKeys(t, MultiKeyManageRequest{
		ChannelId: channel.Id, Action: "append_keys", Keys: []string{"sk-new"},
	})
	assert.Equal(t, http.StatusOK, appendRecorder.Code)
	var appendResponse struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(appendRecorder.Body.Bytes(), &appendResponse))
	assert.True(t, appendResponse.Success)

	stored, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.True(t, stored.ChannelInfo.IsMultiKey)
	assert.Equal(t, constant.MultiKeyModePolling, stored.ChannelInfo.MultiKeyMode)
	assert.Equal(t, []string{"sk-old", "sk-new"}, stored.GetKeys())

	keyIndex := 0
	replaceRecorder := callManageMultiKeys(t, MultiKeyManageRequest{
		ChannelId: channel.Id, Action: "replace_key", KeyIndex: &keyIndex, Key: "sk-rotated",
	})
	assert.Equal(t, http.StatusOK, replaceRecorder.Code)
	stored, err = model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"sk-rotated", "sk-new"}, stored.GetKeys())
}
