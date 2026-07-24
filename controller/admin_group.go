package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type adminGroupRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio"`
	Reason      string  `json:"reason"`
}

func AdminListGroups(c *gin.Context) {
	groups, err := model.ListManagedGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": groups})
}

func AdminUpsertGroup(c *gin.Context) {
	request := adminGroupRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpsertManagedGroup(request.Name, request.Description, request.Ratio); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "group.upsert", map[string]interface{}{"name": strings.TrimSpace(request.Name), "ratio": request.Ratio})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AdminDisableGroup(c *gin.Context) {
	request := adminGroupRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if err := model.SetManagedGroupDisabled(name, true, request.Reason, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "group.disable", map[string]interface{}{"name": name, "reason": strings.TrimSpace(request.Reason)})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AdminRestoreGroup(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if err := model.SetManagedGroupDisabled(name, false, "", c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "group.restore", map[string]interface{}{"name": name})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func AdminDeleteGroup(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if err := model.DeleteManagedGroup(name); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "group.delete", map[string]interface{}{"name": name})
	c.JSON(http.StatusOK, gin.H{"success": true})
}
