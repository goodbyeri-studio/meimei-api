package controller

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"gorm.io/gorm"
)

type adminWechatRefundRequest struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

type adminWechatReconcileRequest struct {
	Kind string `json:"kind"`
}

func AdminListWechatPayOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	userID, _ := strconv.Atoi(c.Query("user_id"))
	result, err := model.ListWechatPayAdminOrders(model.WechatPayAdminOrderQuery{
		Page: page, PageSize: pageSize, UserID: userID,
		Kind: strings.TrimSpace(c.Query("kind")), Status: strings.TrimSpace(c.Query("status")),
		Keyword: strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func AdminGetWechatPayOrder(c *gin.Context) {
	kind := strings.TrimSpace(c.Query("kind"))
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	order, err := model.GetWechatPayAdminOrder(kind, tradeNo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	outRefundNo := ""
	if order.Refund != nil {
		outRefundNo = order.Refund.OutRefundNo
	}
	events, err := model.GetWechatPayAdminOrderEvents(tradeNo, outRefundNo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"order": order, "events": events}})
}

func wechatRefundNotifyURL(paymentNotifyURL string) (string, error) {
	parsed, err := url.Parse(paymentNotifyURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("微信支付回调地址无效")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(path, "/notify") {
		path = strings.TrimSuffix(path, "/notify")
	}
	parsed.Path = path + "/refund/notify"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func applyWechatRefundResponse(response *refunddomestic.Refund, failureReason string) (string, error) {
	if response == nil || response.OutRefundNo == nil || response.OutTradeNo == nil || response.Status == nil || response.Amount == nil || response.Amount.Refund == nil || response.Amount.Total == nil {
		return "", errors.New("微信退款响应字段不完整")
	}
	localRefund, err := model.GetWechatPayRefundByOutRefundNo(*response.OutRefundNo)
	if err != nil {
		return "", err
	}
	if localRefund.OutTradeNo != *response.OutTradeNo || localRefund.AmountFen != *response.Amount.Refund || localRefund.TotalFen != *response.Amount.Total {
		return "", errors.New("微信退款订单号或金额与本地退款记录不一致")
	}
	if response.Amount.Currency != nil && localRefund.Currency != *response.Amount.Currency {
		return "", errors.New("微信退款币种与本地退款记录不一致")
	}
	refundID := ""
	if response.RefundId != nil {
		refundID = *response.RefundId
	}
	successTime := time.Time{}
	if response.SuccessTime != nil {
		successTime = *response.SuccessTime
	}
	return model.ApplyWechatPayRefundResult(*response.OutRefundNo, refundID, string(*response.Status), successTime, failureReason)
}

type wechatRefundNotificationResource struct {
	Mchid               *string    `json:"mchid"`
	OutTradeNo          *string    `json:"out_trade_no"`
	TransactionID       *string    `json:"transaction_id"`
	OutRefundNo         *string    `json:"out_refund_no"`
	RefundID            *string    `json:"refund_id"`
	RefundStatus        *string    `json:"refund_status"`
	SuccessTime         *time.Time `json:"success_time"`
	UserReceivedAccount *string    `json:"user_received_account"`
	Amount              *struct {
		Total       *int64 `json:"total"`
		Refund      *int64 `json:"refund"`
		PayerTotal  *int64 `json:"payer_total"`
		PayerRefund *int64 `json:"payer_refund"`
	} `json:"amount"`
}

func AdminRequestWechatPayRefund(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	request := adminWechatRefundRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	runtime, err := wechatPayRuntimeLoader(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	outRefundNo := fmt.Sprintf("RF%d%s", time.Now().UnixMilli(), common.GetRandomString(8))
	refund, cacheGroup, err := model.PrepareWechatPayRefund(strings.TrimSpace(request.Kind), tradeNo, request.Reason, c.GetInt("id"), outRefundNo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if cacheGroup != "" {
		_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
	}
	notifyURL, err := wechatRefundNotifyURL(runtime.config.NotifyURL)
	if err != nil {
		cacheGroup, _ = model.ReleaseWechatPayRefundReservation(outRefundNo, err.Error())
		if cacheGroup != "" {
			_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
		}
		common.ApiError(c, err)
		return
	}
	response, _, err := runtime.refundService.Create(c.Request.Context(), refunddomestic.CreateRequest{
		OutTradeNo:  core.String(tradeNo),
		OutRefundNo: core.String(outRefundNo),
		Reason:      core.String(strings.TrimSpace(request.Reason)),
		NotifyUrl:   core.String(notifyURL),
		Amount: &refunddomestic.AmountReq{
			Refund: core.Int64(refund.AmountFen), Total: core.Int64(refund.TotalFen), Currency: core.String(refund.Currency),
		},
	})
	if err != nil {
		cacheGroup, releaseErr := model.ReleaseWechatPayRefundReservation(outRefundNo, err.Error())
		if cacheGroup != "" {
			_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
		}
		if releaseErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("微信退款申请失败且业务预留恢复失败 refund_no=%s error=%q", outRefundNo, releaseErr.Error()))
		}
		common.ApiError(c, err)
		return
	}
	cacheGroup, err = applyWechatRefundResponse(response, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if cacheGroup != "" {
		_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
	}
	recordManageAudit(c, "payment.wechat_refund", map[string]interface{}{
		"trade_no": tradeNo, "refund_no": outRefundNo, "kind": request.Kind, "amount_fen": refund.AmountFen,
	})
	updated, _ := model.GetWechatPayRefundByOutRefundNo(outRefundNo)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

func reconcileWechatRefund(c *gin.Context, refund *model.WechatPayRefund) error {
	runtime, err := wechatPayRuntimeLoader(c.Request.Context())
	if err != nil {
		return err
	}
	response, _, err := runtime.refundService.QueryByOutRefundNo(c.Request.Context(), refunddomestic.QueryByOutRefundNoRequest{
		OutRefundNo: core.String(refund.OutRefundNo),
	})
	if err != nil {
		return err
	}
	cacheGroup, err := applyWechatRefundResponse(response, "微信退款查询返回失败状态")
	if cacheGroup != "" {
		_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
	}
	return err
}

func AdminReconcileWechatPayOrder(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	request := adminWechatReconcileRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	refund, refundErr := model.GetWechatPayRefundByTradeNo(tradeNo)
	if refundErr == nil && refund.Status != model.WechatPayRefundStatusSuccess {
		if err := reconcileWechatRefund(c, refund); err != nil {
			common.ApiError(c, err)
			return
		}
	} else if refundErr != nil && !errors.Is(refundErr, gorm.ErrRecordNotFound) {
		common.ApiError(c, refundErr)
		return
	} else if request.Kind == model.WechatPayOrderKindTopUp {
		order, err := model.GetWechatPayOrderByTradeNo(tradeNo)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		reconcileWechatPayOrder(c.Request.Context(), order)
	} else if request.Kind == model.WechatPayOrderKindSubscription {
		order, err := model.GetSubscriptionWechatPayOrderByTradeNo(tradeNo)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		reconcileSubscriptionWechatPayOrder(c.Request.Context(), order)
	} else {
		common.ApiErrorMsg(c, "不支持的订单类型")
		return
	}
	recordManageAudit(c, "payment.wechat_reconcile", map[string]interface{}{"trade_no": tradeNo, "kind": request.Kind})
	order, err := model.GetWechatPayAdminOrder(request.Kind, tradeNo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}

func WechatPayRefundNotify(c *gin.Context) {
	if !wechatPayWebhookEnabled() {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	runtime, err := wechatPayRuntimeLoader(c.Request.Context())
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wechatPayNotifyMaxBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	bodyDigest := fmt.Sprintf("%x", sha256.Sum256(body))
	response := &wechatRefundNotificationResource{}
	notifyRequest, err := runtime.notifyHandler.ParseNotifyRequest(c.Request.Context(), c.Request, response)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信退款回调验签或解密失败 body_digest=%s error=%q", bodyDigest, err.Error()))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if notifyRequest.ID == "" || response.OutTradeNo == nil || response.OutRefundNo == nil || response.RefundStatus == nil || response.Amount == nil || response.Amount.Total == nil || response.Amount.Refund == nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	localRefund, err := model.GetWechatPayRefundByOutRefundNo(*response.OutRefundNo)
	if err != nil || localRefund.OutTradeNo != *response.OutTradeNo || localRefund.TotalFen != *response.Amount.Total || localRefund.AmountFen != *response.Amount.Refund {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信退款回调订单或金额不匹配 event_id=%s refund_no=%s", notifyRequest.ID, *response.OutRefundNo))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	notification := &model.WechatPayRefundNotification{
		EventId: notifyRequest.ID, OutRefundNo: *response.OutRefundNo, BodyDigest: bodyDigest,
		ProcessingStatus: "processing", ReceivedAt: common.GetTimestamp(),
	}
	if _, err := model.SaveWechatPayRefundNotification(notification); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	refundID := ""
	if response.RefundID != nil {
		refundID = *response.RefundID
	}
	successTime := time.Time{}
	if response.SuccessTime != nil {
		successTime = *response.SuccessTime
	}
	cacheGroup, err := model.ApplyWechatPayRefundResult(*response.OutRefundNo, refundID, *response.RefundStatus, successTime, "微信退款回调返回失败状态")
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信退款回调处理失败 event_id=%s refund_no=%s error=%q", notifyRequest.ID, *response.OutRefundNo, err.Error()))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if cacheGroup != "" {
		refund, lookupErr := model.GetWechatPayRefundByOutRefundNo(*response.OutRefundNo)
		if lookupErr == nil {
			_ = model.UpdateUserGroupCache(refund.UserId, cacheGroup)
		}
	}
	if err := model.MarkWechatPayRefundNotificationProcessed(notifyRequest.ID); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.AbortWithStatus(http.StatusNoContent)
}
