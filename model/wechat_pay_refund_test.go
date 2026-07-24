package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createCreditedWechatTopUpForRefund(t *testing.T, userID int, tradeNo string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: userID, Username: tradeNo + "-user", Status: common.UserStatusEnabled, Quota: 1000,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: userID, Amount: 1000, Money: 10, TradeNo: tradeNo,
		PaymentMethod: PaymentMethodWechatNative, PaymentProvider: PaymentProviderWechatNative,
		CreateTime: common.GetTimestamp(), CompleteTime: common.GetTimestamp(), Status: common.TopUpStatusSuccess,
	}).Error)
	require.NoError(t, DB.Create(&WechatPayOrder{
		UserId: userID, ClientRequestId: tradeNo + "-request", OutTradeNo: tradeNo,
		AmountFen: 1000, CreditQuota: 1000, Currency: "CNY", Status: WechatPayOrderStatusCredited,
		CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(), SuccessTime: common.GetTimestamp(),
	}).Error)
}

func TestWechatPayTopUpRefundReservationAndSuccessAreIdempotent(t *testing.T) {
	truncateTables(t)
	createCreditedWechatTopUpForRefund(t, 9401, "wechat-refund-success")

	refund, _, err := PrepareWechatPayRefund(
		WechatPayOrderKindTopUp, "wechat-refund-success", "duplicate payment", 1, "refund-success",
	)
	require.NoError(t, err)
	assert.Equal(t, 1000, refund.ReservedQuota)
	var user User
	require.NoError(t, DB.Where("id = ?", 9401).First(&user).Error)
	assert.Zero(t, user.Quota)

	_, err = ApplyWechatPayRefundResult("refund-success", "wechat-refund-id", "SUCCESS", time.Now(), "")
	require.NoError(t, err)
	_, err = ApplyWechatPayRefundResult("refund-success", "wechat-refund-id", "SUCCESS", time.Now(), "")
	require.NoError(t, err)

	stored, err := GetWechatPayRefundByOutRefundNo("refund-success")
	require.NoError(t, err)
	assert.Equal(t, WechatPayRefundStatusSuccess, stored.Status)
	assert.Equal(t, WechatPayRefundBusinessCommitted, stored.BusinessReservationState)
	require.NoError(t, DB.Where("id = ?", 9401).First(&user).Error)
	assert.Zero(t, user.Quota)
	order, err := GetWechatPayOrderByTradeNo("wechat-refund-success")
	require.NoError(t, err)
	assert.Equal(t, WechatPayOrderStatusRefunded, order.Status)
}

func TestWechatPayTopUpRefundFailureRestoresReservationOnce(t *testing.T) {
	truncateTables(t)
	createCreditedWechatTopUpForRefund(t, 9402, "wechat-refund-failed")

	_, _, err := PrepareWechatPayRefund(
		WechatPayOrderKindTopUp, "wechat-refund-failed", "operator cancellation", 1, "refund-failed",
	)
	require.NoError(t, err)
	_, err = ApplyWechatPayRefundResult("refund-failed", "wechat-refund-id-2", "CLOSED", time.Time{}, "closed by WeChat")
	require.NoError(t, err)
	_, err = ApplyWechatPayRefundResult("refund-failed", "wechat-refund-id-2", "CLOSED", time.Time{}, "closed by WeChat")
	require.NoError(t, err)

	var user User
	require.NoError(t, DB.Where("id = ?", 9402).First(&user).Error)
	assert.Equal(t, 1000, user.Quota)
	stored, err := GetWechatPayRefundByOutRefundNo("refund-failed")
	require.NoError(t, err)
	assert.Equal(t, WechatPayRefundStatusClosed, stored.Status)
	assert.Equal(t, WechatPayRefundBusinessRestored, stored.BusinessReservationState)
}
