package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

type fakeWechatRefundNotifyHandler struct {
	eventID string
	payload []byte
}

func (f *fakeWechatRefundNotifyHandler) ParseNotifyRequest(_ context.Context, _ *http.Request, content interface{}) (*notify.Request, error) {
	if err := common.Unmarshal(f.payload, content); err != nil {
		return nil, err
	}
	return &notify.Request{ID: f.eventID}, nil
}

func TestWechatPayRefundNotifyUsesRefundStatusAndCommitsRefund(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.WechatPayOrder{},
		&model.WechatPayRefund{},
		&model.WechatPayRefundNotification{},
	))

	const (
		userID      = 9501
		tradeNo     = "wechat-refund-notify-trade"
		outRefundNo = "wechat-refund-notify-refund"
	)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.User{
		Id: userID, Username: "wechat-refund-notify-user", Status: common.UserStatusEnabled, Quota: 1000,
	}).Error)
	require.NoError(t, db.Create(&model.TopUp{
		UserId: userID, Amount: 1000, Money: 10, TradeNo: tradeNo,
		PaymentMethod: model.PaymentMethodWechatNative, PaymentProvider: model.PaymentProviderWechatNative,
		CreateTime: now, CompleteTime: now, Status: common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&model.WechatPayOrder{
		UserId: userID, ClientRequestId: "wechat_refund_notify_request", OutTradeNo: tradeNo,
		AmountFen: 1000, CreditQuota: 1000, Currency: "CNY", Status: model.WechatPayOrderStatusCredited,
		CreatedAt: now, UpdatedAt: now, SuccessTime: now,
	}).Error)
	_, _, err := model.PrepareWechatPayRefund(model.WechatPayOrderKindTopUp, tradeNo, "duplicate payment", 1, outRefundNo)
	require.NoError(t, err)

	payload, err := common.Marshal(map[string]interface{}{
		"out_trade_no":  tradeNo,
		"out_refund_no": outRefundNo,
		"refund_id":     "wechat-refund-id",
		"refund_status": "SUCCESS",
		"success_time":  time.Now().UTC().Format(time.RFC3339),
		"amount": map[string]int64{
			"total":  1000,
			"refund": 1000,
		},
	})
	require.NoError(t, err)
	runtime := &wechatPayRuntime{notifyHandler: &fakeWechatRefundNotifyHandler{
		eventID: "wechat-refund-notify-event", payload: payload,
	}}
	originalWebhookEnabled := wechatPayWebhookEnabled
	wechatPayWebhookEnabled = func() bool { return true }
	originalLoader := wechatPayRuntimeLoader
	wechatPayRuntimeLoader = func(context.Context) (*wechatPayRuntime, error) { return runtime, nil }
	t.Cleanup(func() {
		wechatPayWebhookEnabled = originalWebhookEnabled
		wechatPayRuntimeLoader = originalLoader
	})

	request := httptest.NewRequest(http.MethodPost, "/api/payment/wechat/refund/notify", bytes.NewReader([]byte(`{"encrypted":"notification"}`)))
	recorder := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	WechatPayRefundNotify(ctx)
	assert.Equal(t, http.StatusNoContent, recorder.Code)

	refund, err := model.GetWechatPayRefundByOutRefundNo(outRefundNo)
	require.NoError(t, err)
	assert.Equal(t, model.WechatPayRefundStatusSuccess, refund.Status)
	assert.Equal(t, model.WechatPayRefundBusinessCommitted, refund.BusinessReservationState)
	assert.NotNil(t, refund.WechatRefundId)
	assert.Equal(t, "wechat-refund-id", *refund.WechatRefundId)

	order, err := model.GetWechatPayOrderByTradeNo(tradeNo)
	require.NoError(t, err)
	assert.Equal(t, model.WechatPayOrderStatusRefunded, order.Status)
	var notification model.WechatPayRefundNotification
	require.NoError(t, db.Where("event_id = ?", "wechat-refund-notify-event").First(&notification).Error)
	assert.Equal(t, "processed", notification.ProcessingStatus)
}
