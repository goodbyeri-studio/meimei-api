package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type defaultSubscriptionPlan struct {
	title    string
	subtitle string
	amount   int64
}

var defaultCNYSubscriptionPlans = []defaultSubscriptionPlan{
	{title: "轻量包", subtitle: "适合少量体验与临时调用", amount: 10},
	{title: "入门包", subtitle: "适合个人日常轻量使用", amount: 20},
	{title: "标准包", subtitle: "适合稳定的日常开发调用", amount: 50},
	{title: "进阶包", subtitle: "适合高频开发与内容生产", amount: 100},
	{title: "专业包", subtitle: "适合专业项目持续使用", amount: 200},
	{title: "团队包", subtitle: "适合小型团队协作调用", amount: 500},
	{title: "商务包", subtitle: "适合业务系统规模化接入", amount: 1000},
	{title: "企业包", subtitle: "适合企业级高并发场景", amount: 2000},
}

// seedDefaultCNYSubscriptionPlans initializes a brand-new catalog and upgrades
// only untouched legacy built-ins to the current permanent-duration contract.
func seedDefaultCNYSubscriptionPlans() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&SubscriptionPlan{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			plans := make([]SubscriptionPlan, 0, len(defaultCNYSubscriptionPlans))
			for index, item := range defaultCNYSubscriptionPlans {
				quota, clamp := common.QuotaFromDecimalChecked(
					decimal.NewFromInt(item.amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
				)
				if clamp != nil || quota <= 0 {
					return errors.New("默认套餐额度超出允许范围")
				}
				plans = append(plans, SubscriptionPlan{
					Title:               item.title,
					Subtitle:            item.subtitle,
					PriceAmount:         float64(item.amount),
					Currency:            "CNY",
					DurationUnit:        SubscriptionDurationPermanent,
					DurationValue:       0,
					Enabled:             true,
					SortOrder:           len(defaultCNYSubscriptionPlans) - index,
					AllowBalancePay:     common.GetPointer(true),
					AllowWalletOverflow: common.GetPointer(true),
					TotalAmount:         int64(quota),
					QuotaResetPeriod:    SubscriptionResetNever,
				})
			}
			return tx.Create(&plans).Error
		}

		// Upgrade only untouched built-in monthly plans. Matching the complete
		// business fingerprint avoids changing administrator-managed plans.
		for index, item := range defaultCNYSubscriptionPlans {
			quota, clamp := common.QuotaFromDecimalChecked(
				decimal.NewFromInt(item.amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
			)
			if clamp != nil || quota <= 0 {
				return errors.New("默认套餐额度超出允许范围")
			}
			legacyPlanQuery := tx.Model(&SubscriptionPlan{}).
				Where("title = ? AND subtitle = ? AND currency = ? AND price_amount = ? AND duration_unit = ? AND duration_value = ? AND custom_seconds = ? AND enabled = ? AND sort_order = ? AND allow_balance_pay = ? AND allow_wallet_overflow = ? AND stripe_price_id = ? AND creem_product_id = ? AND waffo_pancake_product_id = ? AND max_purchase_per_user = ? AND upgrade_group = ? AND downgrade_group = ? AND total_amount = ? AND quota_reset_period = ? AND quota_reset_custom_seconds = ?",
					item.title, item.subtitle, "CNY", float64(item.amount), SubscriptionDurationMonth, 1, 0,
					true, len(defaultCNYSubscriptionPlans)-index, true, true, "", "", "", 0, "", "",
					int64(quota), SubscriptionResetNever, 0)
			var legacyPlanIds []int
			if err := legacyPlanQuery.Pluck("id", &legacyPlanIds).Error; err != nil {
				return err
			}
			if len(legacyPlanIds) == 0 {
				continue
			}
			if err := tx.Model(&SubscriptionPlan{}).
				Where("id IN ?", legacyPlanIds).
				Updates(map[string]interface{}{
					"duration_unit":  SubscriptionDurationPermanent,
					"duration_value": 0,
					"custom_seconds": 0,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&UserSubscription{}).
				Where("plan_id IN ? AND status = ?", legacyPlanIds, "active").
				Updates(map[string]interface{}{
					"end_time":        0,
					"next_reset_time": 0,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
