package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func AdminGetChannelHealth(c *gin.Context) {
	duration := 24 * time.Hour
	switch c.DefaultQuery("window", "24h") {
	case "1h":
		duration = time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	}
	stats, err := model.GetChannelHealthStats(time.Now().Add(-duration))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}
