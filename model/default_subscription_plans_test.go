package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDefaultSubscriptionPlansIsIdempotent(t *testing.T) {
	truncateTables(t)

	require.NoError(t, seedDefaultCNYSubscriptionPlans())
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Order("sort_order desc").Find(&plans).Error)
	require.Len(t, plans, len(defaultCNYSubscriptionPlans))
	assert.Equal(t, "轻量包", plans[0].Title)
	assert.Equal(t, "CNY", plans[0].Currency)
	assert.Equal(t, float64(10), plans[0].PriceAmount)
	assert.Greater(t, plans[0].TotalAmount, int64(0))
	assert.Equal(t, SubscriptionDurationPermanent, plans[0].DurationUnit)
	assert.Zero(t, plans[0].DurationValue)
}

func TestSeedDefaultSubscriptionPlansMigratesOnlyUntouchedBuiltIns(t *testing.T) {
	truncateTables(t)

	quota, clamp := common.QuotaFromDecimalChecked(
		decimal.NewFromInt(10).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
	require.Nil(t, clamp)
	legacyDefault := &SubscriptionPlan{
		Title:            "轻量包",
		Subtitle:         "适合少量体验与临时调用",
		PriceAmount:      10,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      int64(quota),
		QuotaResetPeriod: SubscriptionResetNever,
	}
	modifiedPlan := &SubscriptionPlan{
		Title:            "入门包",
		Subtitle:         "管理员修改过的说明",
		PriceAmount:      20,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      int64(quota) * 2,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(legacyDefault).Error)
	require.NoError(t, DB.Create(modifiedPlan).Error)
	legacySubscription := &UserSubscription{
		UserId:    1001,
		PlanId:    legacyDefault.Id,
		StartTime: time.Now().Add(-time.Hour).Unix(),
		EndTime:   time.Now().Add(30 * 24 * time.Hour).Unix(),
		Status:    "active",
	}
	require.NoError(t, DB.Create(legacySubscription).Error)

	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var migrated SubscriptionPlan
	require.NoError(t, DB.First(&migrated, legacyDefault.Id).Error)
	assert.Equal(t, SubscriptionDurationPermanent, migrated.DurationUnit)
	assert.Zero(t, migrated.DurationValue)
	var migratedSubscription UserSubscription
	require.NoError(t, DB.First(&migratedSubscription, legacySubscription.Id).Error)
	assert.Zero(t, migratedSubscription.EndTime)

	var preserved SubscriptionPlan
	require.NoError(t, DB.First(&preserved, modifiedPlan.Id).Error)
	assert.Equal(t, SubscriptionDurationMonth, preserved.DurationUnit)
	assert.Equal(t, 1, preserved.DurationValue)
}

func TestPermanentSubscriptionParticipatesInActiveBilling(t *testing.T) {
	truncateTables(t)

	user := &User{Username: "permanent-subscription-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:            "永久套餐",
		PriceAmount:      10,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationPermanent,
		DurationValue:    0,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)

	tx := DB.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })
	subscription, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "test")
	require.NoError(t, err)
	require.NoError(t, tx.Commit().Error)
	assert.Zero(t, subscription.EndTime)

	active, err := HasActiveUserSubscription(user.Id)
	require.NoError(t, err)
	assert.True(t, active)

	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	consume, err := PreConsumeUserSubscription("permanent-subscription-request", user.Id, "test-model", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, subscription.Id, consume.UserSubscriptionId)
	assert.EqualValues(t, 100, consume.PreConsumed)

	expired, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Zero(t, expired)
}

func TestSeedDefaultSubscriptionPlansPreservesAdminCatalog(t *testing.T) {
	truncateTables(t)

	customPlan := &SubscriptionPlan{
		Title:            "管理员自定义套餐",
		PriceAmount:      88,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(customPlan).Error)
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Find(&plans).Error)
	require.Len(t, plans, 1)
	assert.Equal(t, customPlan.Title, plans[0].Title)
}

func TestCreatePendingSubscriptionOrderReservesPurchaseLimit(t *testing.T) {
	truncateTables(t)

	user := &User{Username: "subscription-reservation-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:              "限购套餐",
		PriceAmount:        10,
		Currency:           "CNY",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		Enabled:            true,
		MaxPurchasePerUser: 1,
		QuotaResetPeriod:   SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)

	first := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-reservation-first",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, CreatePendingSubscriptionOrder(first))

	second := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-reservation-second",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.ErrorIs(t, CreatePendingSubscriptionOrder(second), ErrSubscriptionPurchaseLimit)

	require.NoError(t, DB.Model(first).Update("status", common.TopUpStatusFailed).Error)
	require.NoError(t, CreatePendingSubscriptionOrder(second))
}

func TestBalancePurchaseHonorsPendingReservation(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username: "subscription-balance-reservation-user",
		Status:   common.UserStatusEnabled,
		Quota:    common.MaxQuota,
	}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:              "余额限购套餐",
		PriceAmount:        10,
		Currency:           "CNY",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		Enabled:            true,
		MaxPurchasePerUser: 1,
		QuotaResetPeriod:   SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, CreatePendingSubscriptionOrder(&SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-balance-reservation-pending",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}))

	require.ErrorIs(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id), ErrSubscriptionPurchaseLimit)

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	assert.Equal(t, common.MaxQuota, storedUser.Quota)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, user.Id))
}
