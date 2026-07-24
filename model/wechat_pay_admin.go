package model

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	WechatPayRefundBusinessReserved  = "reserved"
	WechatPayRefundBusinessCommitted = "committed"
	WechatPayRefundBusinessRestored  = "restored"
)

type WechatPayAdminOrderQuery struct {
	Page     int
	PageSize int
	Kind     string
	Status   string
	UserID   int
	Keyword  string
}

type WechatPayAdminOrder struct {
	Kind                string           `json:"kind"`
	UserID              int              `json:"user_id"`
	Username            string           `json:"username"`
	ClientRequestID     string           `json:"client_request_id"`
	OutTradeNo          string           `json:"out_trade_no"`
	AmountFen           int64            `json:"amount_fen"`
	Currency            string           `json:"currency"`
	Status              string           `json:"status"`
	TransactionPreview  string           `json:"transaction_preview,omitempty"`
	ExpiresAt           int64            `json:"expires_at"`
	SuccessTime         int64            `json:"success_time"`
	CreatedAt           int64            `json:"created_at"`
	UpdatedAt           int64            `json:"updated_at"`
	LastCheckedAt       int64            `json:"last_checked_at"`
	CreditQuota         int              `json:"credit_quota,omitempty"`
	SubscriptionOrderID int              `json:"subscription_order_id,omitempty"`
	PlanID              int              `json:"plan_id,omitempty"`
	PlanTitle           string           `json:"plan_title,omitempty"`
	Refund              *WechatPayRefund `json:"refund,omitempty"`
}

type WechatPayAdminOrderPage struct {
	Items    []WechatPayAdminOrder `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type WechatPayAdminOrderEvents struct {
	PaymentNotifications []WechatPayNotification       `json:"payment_notifications"`
	RefundNotifications  []WechatPayRefundNotification `json:"refund_notifications"`
}

func transactionPreview(transactionID *string) string {
	if transactionID == nil {
		return ""
	}
	value := strings.TrimSpace(*transactionID)
	if len(value) <= 8 {
		return value
	}
	return "***" + value[len(value)-8:]
}

func ListWechatPayAdminOrders(query WechatPayAdminOrderQuery) (*WechatPayAdminOrderPage, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 100 {
		query.PageSize = 20
	}
	limit := query.Page * query.PageSize
	keyword := strings.TrimSpace(query.Keyword)
	items := make([]WechatPayAdminOrder, 0, limit*2)
	var total int64

	if query.Kind == "" || query.Kind == WechatPayOrderKindTopUp {
		q := DB.Model(&WechatPayOrder{})
		if query.Status != "" {
			q = q.Where("status = ?", query.Status)
		}
		if query.UserID > 0 {
			q = q.Where("user_id = ?", query.UserID)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("out_trade_no LIKE ? OR client_request_id LIKE ?", like, like)
		}
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return nil, err
		}
		total += count
		var orders []WechatPayOrder
		if err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&orders).Error; err != nil {
			return nil, err
		}
		for _, order := range orders {
			items = append(items, WechatPayAdminOrder{
				Kind:               WechatPayOrderKindTopUp,
				UserID:             order.UserId,
				ClientRequestID:    order.ClientRequestId,
				OutTradeNo:         order.OutTradeNo,
				AmountFen:          order.AmountFen,
				Currency:           order.Currency,
				Status:             order.Status,
				TransactionPreview: transactionPreview(order.WechatTransactionId),
				ExpiresAt:          order.ExpiresAt,
				SuccessTime:        order.SuccessTime,
				CreatedAt:          order.CreatedAt,
				UpdatedAt:          order.UpdatedAt,
				LastCheckedAt:      order.LastCheckedAt,
				CreditQuota:        order.CreditQuota,
			})
		}
	}

	if query.Kind == "" || query.Kind == WechatPayOrderKindSubscription {
		q := DB.Model(&SubscriptionWechatPayOrder{})
		if query.Status != "" {
			q = q.Where("status = ?", query.Status)
		}
		if query.UserID > 0 {
			q = q.Where("user_id = ?", query.UserID)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("out_trade_no LIKE ? OR client_request_id LIKE ?", like, like)
		}
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return nil, err
		}
		total += count
		var orders []SubscriptionWechatPayOrder
		if err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&orders).Error; err != nil {
			return nil, err
		}
		for _, order := range orders {
			items = append(items, WechatPayAdminOrder{
				Kind:                WechatPayOrderKindSubscription,
				UserID:              order.UserId,
				ClientRequestID:     order.ClientRequestId,
				OutTradeNo:          order.OutTradeNo,
				AmountFen:           order.AmountFen,
				Currency:            order.Currency,
				Status:              order.Status,
				TransactionPreview:  transactionPreview(order.WechatTransactionId),
				ExpiresAt:           order.ExpiresAt,
				SuccessTime:         order.SuccessTime,
				CreatedAt:           order.CreatedAt,
				UpdatedAt:           order.UpdatedAt,
				LastCheckedAt:       order.LastCheckedAt,
				SubscriptionOrderID: order.SubscriptionOrderId,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].OutTradeNo > items[j].OutTradeNo
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	start := (query.Page - 1) * query.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + query.PageSize
	if end > len(items) {
		end = len(items)
	}
	items = items[start:end]

	if len(items) > 0 {
		userIDs := make([]int, 0, len(items))
		tradeNos := make([]string, 0, len(items))
		subscriptionOrderIDs := make([]int, 0, len(items))
		for _, item := range items {
			userIDs = append(userIDs, item.UserID)
			tradeNos = append(tradeNos, item.OutTradeNo)
			if item.SubscriptionOrderID > 0 {
				subscriptionOrderIDs = append(subscriptionOrderIDs, item.SubscriptionOrderID)
			}
		}
		var users []User
		if err := DB.Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		usernames := make(map[int]string, len(users))
		for _, user := range users {
			usernames[user.Id] = user.Username
		}
		var refunds []WechatPayRefund
		if err := DB.Where("out_trade_no IN ?", tradeNos).Find(&refunds).Error; err != nil {
			return nil, err
		}
		refundByTrade := make(map[string]*WechatPayRefund, len(refunds))
		for i := range refunds {
			refundByTrade[refunds[i].OutTradeNo] = &refunds[i]
		}
		planByOrder := make(map[int]SubscriptionOrder)
		planTitles := make(map[int]string)
		if len(subscriptionOrderIDs) > 0 {
			var subscriptionOrders []SubscriptionOrder
			if err := DB.Where("id IN ?", subscriptionOrderIDs).Find(&subscriptionOrders).Error; err != nil {
				return nil, err
			}
			planIDs := make([]int, 0, len(subscriptionOrders))
			for _, order := range subscriptionOrders {
				planByOrder[order.Id] = order
				planIDs = append(planIDs, order.PlanId)
			}
			var plans []SubscriptionPlan
			if err := DB.Select("id", "title").Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
				return nil, err
			}
			for _, plan := range plans {
				planTitles[plan.Id] = plan.Title
			}
		}
		for i := range items {
			items[i].Username = usernames[items[i].UserID]
			items[i].Refund = refundByTrade[items[i].OutTradeNo]
			if subscriptionOrder, ok := planByOrder[items[i].SubscriptionOrderID]; ok {
				items[i].PlanID = subscriptionOrder.PlanId
				items[i].PlanTitle = planTitles[subscriptionOrder.PlanId]
			}
		}
	}

	return &WechatPayAdminOrderPage{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func GetWechatPayAdminOrder(kind string, tradeNo string) (*WechatPayAdminOrder, error) {
	if kind != WechatPayOrderKindTopUp && kind != WechatPayOrderKindSubscription {
		return nil, errors.New("invalid order kind")
	}
	item := &WechatPayAdminOrder{Kind: kind}
	if kind == WechatPayOrderKindTopUp {
		order, err := GetWechatPayOrderByTradeNo(tradeNo)
		if err != nil {
			return nil, err
		}
		item.UserID = order.UserId
		item.ClientRequestID = order.ClientRequestId
		item.OutTradeNo = order.OutTradeNo
		item.AmountFen = order.AmountFen
		item.Currency = order.Currency
		item.Status = order.Status
		item.TransactionPreview = transactionPreview(order.WechatTransactionId)
		item.ExpiresAt = order.ExpiresAt
		item.SuccessTime = order.SuccessTime
		item.CreatedAt = order.CreatedAt
		item.UpdatedAt = order.UpdatedAt
		item.LastCheckedAt = order.LastCheckedAt
		item.CreditQuota = order.CreditQuota
	} else {
		order, err := GetSubscriptionWechatPayOrderByTradeNo(tradeNo)
		if err != nil {
			return nil, err
		}
		item.UserID = order.UserId
		item.ClientRequestID = order.ClientRequestId
		item.OutTradeNo = order.OutTradeNo
		item.AmountFen = order.AmountFen
		item.Currency = order.Currency
		item.Status = order.Status
		item.TransactionPreview = transactionPreview(order.WechatTransactionId)
		item.ExpiresAt = order.ExpiresAt
		item.SuccessTime = order.SuccessTime
		item.CreatedAt = order.CreatedAt
		item.UpdatedAt = order.UpdatedAt
		item.LastCheckedAt = order.LastCheckedAt
		item.SubscriptionOrderID = order.SubscriptionOrderId
		var subscriptionOrder SubscriptionOrder
		if err := DB.Where("id = ?", order.SubscriptionOrderId).First(&subscriptionOrder).Error; err != nil {
			return nil, err
		}
		item.PlanID = subscriptionOrder.PlanId
		var plan SubscriptionPlan
		if err := DB.Select("id", "title").Where("id = ?", subscriptionOrder.PlanId).First(&plan).Error; err != nil {
			return nil, err
		}
		item.PlanTitle = plan.Title
	}
	var user User
	if err := DB.Select("id", "username").Where("id = ?", item.UserID).First(&user).Error; err != nil {
		return nil, err
	}
	item.Username = user.Username
	var refund WechatPayRefund
	result := DB.Where("out_trade_no = ?", tradeNo).Limit(1).Find(&refund)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		item.Refund = &refund
	}
	return item, nil
}

func GetWechatPayAdminOrderEvents(tradeNo string, outRefundNo string) (*WechatPayAdminOrderEvents, error) {
	events := &WechatPayAdminOrderEvents{
		PaymentNotifications: []WechatPayNotification{},
		RefundNotifications:  []WechatPayRefundNotification{},
	}
	if err := DB.Where("out_trade_no = ?", tradeNo).Order("received_at DESC, id DESC").Find(&events.PaymentNotifications).Error; err != nil {
		return nil, err
	}
	if outRefundNo != "" {
		if err := DB.Where("out_refund_no = ?", outRefundNo).Order("received_at DESC, id DESC").Find(&events.RefundNotifications).Error; err != nil {
			return nil, err
		}
	}
	return events, nil
}

func GetWechatPayRefundByTradeNo(tradeNo string) (*WechatPayRefund, error) {
	var refund WechatPayRefund
	if err := DB.Where("out_trade_no = ?", tradeNo).First(&refund).Error; err != nil {
		return nil, err
	}
	return &refund, nil
}

func GetWechatPayRefundByOutRefundNo(outRefundNo string) (*WechatPayRefund, error) {
	var refund WechatPayRefund
	if err := DB.Where("out_refund_no = ?", outRefundNo).First(&refund).Error; err != nil {
		return nil, err
	}
	return &refund, nil
}

func PrepareWechatPayRefund(kind string, tradeNo string, reason string, adminID int, outRefundNo string) (*WechatPayRefund, string, error) {
	reason = strings.TrimSpace(reason)
	if (kind != WechatPayOrderKindTopUp && kind != WechatPayOrderKindSubscription) || tradeNo == "" || outRefundNo == "" || adminID <= 0 {
		return nil, "", errors.New("invalid refund request")
	}
	if reason == "" || len([]rune(reason)) > 80 {
		return nil, "", errors.New("退款原因必须为 1 到 80 个字符")
	}
	refund := &WechatPayRefund{
		OrderKind: kind, OutTradeNo: tradeNo, OutRefundNo: outRefundNo,
		RequestedBy: adminID, Reason: reason, Status: WechatPayRefundStatusSubmitting,
		Currency: "CNY", BusinessReservationState: WechatPayRefundBusinessReserved,
		CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	}
	cacheGroup := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing WechatPayRefund
		lookup := tx.Where("out_trade_no = ?", tradeNo).Limit(1).Find(&existing)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected > 0 {
			return errors.New("该订单已经存在退款记录")
		}
		if kind == WechatPayOrderKindTopUp {
			var order WechatPayOrder
			if err := lockForUpdate(tx).Where("out_trade_no = ?", tradeNo).First(&order).Error; err != nil {
				return err
			}
			if order.Status != WechatPayOrderStatusCredited || order.CreditQuota <= 0 {
				return errors.New("仅已入账且未退款的充值订单可以退款")
			}
			var user User
			if err := lockForUpdate(tx).Select("id", "quota", "inviter_id").Where("id = ?", order.UserId).First(&user).Error; err != nil {
				return err
			}
			if user.Quota < order.CreditQuota {
				return errors.New("用户剩余额度不足，无法安全撤销本次充值")
			}
			if err := tx.Model(&user).Update("quota", user.Quota-order.CreditQuota).Error; err != nil {
				return err
			}
			refund.UserId = order.UserId
			refund.AmountFen = order.AmountFen
			refund.TotalFen = order.AmountFen
			refund.Currency = order.Currency
			refund.ReservedQuota = order.CreditQuota
			if user.InviterId > 0 && user.InviterId != user.Id {
				rewardQuota, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(int64(order.CreditQuota)).Mul(decimal.NewFromInt(affiliateTopUpRewardPercent)).Div(decimal.NewFromInt(100)))
				if clamp != nil {
					return clamp
				}
				if rewardQuota > 0 {
					var inviter User
					if err := lockForUpdate(tx).Select("id", "aff_quota", "aff_history").Where("id = ?", user.InviterId).First(&inviter).Error; err != nil {
						return err
					}
					if inviter.AffQuota < rewardQuota || inviter.AffHistoryQuota < rewardQuota {
						return errors.New("邀请奖励余额不足，无法安全撤销本次充值")
					}
					if err := tx.Model(&inviter).Updates(map[string]interface{}{
						"aff_quota": inviter.AffQuota - rewardQuota, "aff_history": inviter.AffHistoryQuota - rewardQuota,
					}).Error; err != nil {
						return err
					}
					refund.AffiliateInviterId = inviter.Id
					refund.ReservedAffiliateQuota = rewardQuota
				}
			}
			return tx.Create(refund).Error
		}

		var payOrder SubscriptionWechatPayOrder
		if err := lockForUpdate(tx).Where("out_trade_no = ?", tradeNo).First(&payOrder).Error; err != nil {
			return err
		}
		if payOrder.Status != WechatPayOrderStatusCredited {
			return errors.New("仅已入账且未退款的套餐订单可以退款")
		}
		var order SubscriptionOrder
		if err := lockForUpdate(tx).Where("id = ?", payOrder.SubscriptionOrderId).First(&order).Error; err != nil {
			return err
		}
		if order.UserSubscriptionId == nil || *order.UserSubscriptionId <= 0 {
			return errors.New("订单缺少套餐实例关联，禁止自动退款")
		}
		var subscription UserSubscription
		if err := lockForUpdate(tx).Where("id = ?", *order.UserSubscriptionId).First(&subscription).Error; err != nil {
			return err
		}
		if subscription.Status != "active" || subscription.AmountUsed != 0 {
			return errors.New("套餐已使用或已失效，禁止自动退款")
		}
		previousGroup, err := getUserGroupByIdTx(tx, subscription.UserId)
		if err != nil {
			return err
		}
		cacheGroup, err = downgradeUserGroupForSubscriptionTx(tx, &subscription, common.GetTimestamp())
		if err != nil {
			return err
		}
		if err := tx.Model(&subscription).Update("status", "refund_pending").Error; err != nil {
			return err
		}
		refund.UserId = payOrder.UserId
		refund.AmountFen = payOrder.AmountFen
		refund.TotalFen = payOrder.AmountFen
		refund.Currency = payOrder.Currency
		refund.UserSubscriptionId = subscription.Id
		refund.PreviousSubscriptionEnd = subscription.EndTime
		refund.PreviousUserGroup = previousGroup
		return tx.Create(refund).Error
	})
	if err != nil {
		return nil, "", err
	}
	return refund, cacheGroup, nil
}

func releaseWechatRefundReservationTx(tx *gorm.DB, refund *WechatPayRefund, failureReason string) (string, error) {
	if refund.BusinessReservationState != WechatPayRefundBusinessReserved {
		return "", nil
	}
	cacheGroup := ""
	if refund.OrderKind == WechatPayOrderKindTopUp {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", refund.UserId).First(&user).Error; err != nil {
			return "", err
		}
		restored, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(int64(user.Quota)).Add(decimal.NewFromInt(int64(refund.ReservedQuota))))
		if clamp != nil {
			return "", clamp
		}
		if err := tx.Model(&user).Update("quota", restored).Error; err != nil {
			return "", err
		}
		if refund.AffiliateInviterId > 0 && refund.ReservedAffiliateQuota > 0 {
			var inviter User
			if err := lockForUpdate(tx).Select("id", "aff_quota", "aff_history").Where("id = ?", refund.AffiliateInviterId).First(&inviter).Error; err != nil {
				return "", err
			}
			affQuota, quotaClamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(int64(inviter.AffQuota)).Add(decimal.NewFromInt(int64(refund.ReservedAffiliateQuota))))
			affHistory, historyClamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(int64(inviter.AffHistoryQuota)).Add(decimal.NewFromInt(int64(refund.ReservedAffiliateQuota))))
			if quotaClamp != nil {
				return "", quotaClamp
			}
			if historyClamp != nil {
				return "", historyClamp
			}
			if err := tx.Model(&inviter).Updates(map[string]interface{}{"aff_quota": affQuota, "aff_history": affHistory}).Error; err != nil {
				return "", err
			}
		}
	} else {
		var subscription UserSubscription
		if err := lockForUpdate(tx).Where("id = ?", refund.UserSubscriptionId).First(&subscription).Error; err != nil {
			return "", err
		}
		if subscription.Status == "refund_pending" {
			if err := tx.Model(&subscription).Updates(map[string]interface{}{"status": "active", "end_time": refund.PreviousSubscriptionEnd}).Error; err != nil {
				return "", err
			}
			var otherActiveCount int64
			if err := tx.Model(&UserSubscription{}).Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''", subscription.UserId, "active", common.GetTimestamp(), subscription.Id).Count(&otherActiveCount).Error; err != nil {
				return "", err
			}
			if otherActiveCount == 0 && refund.PreviousUserGroup != "" {
				if err := tx.Model(&User{}).Where("id = ?", subscription.UserId).Update("group", refund.PreviousUserGroup).Error; err != nil {
					return "", err
				}
				cacheGroup = refund.PreviousUserGroup
			}
		}
	}
	refund.Status = WechatPayRefundStatusFailed
	refund.FailureReason = strings.TrimSpace(failureReason)
	refund.BusinessReservationState = WechatPayRefundBusinessRestored
	refund.UpdatedAt = common.GetTimestamp()
	return cacheGroup, tx.Save(refund).Error
}

func ReleaseWechatPayRefundReservation(outRefundNo string, failureReason string) (string, error) {
	cacheGroup := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var refund WechatPayRefund
		if err := lockForUpdate(tx).Where("out_refund_no = ?", outRefundNo).First(&refund).Error; err != nil {
			return err
		}
		var err error
		cacheGroup, err = releaseWechatRefundReservationTx(tx, &refund, failureReason)
		return err
	})
	return cacheGroup, err
}

func ApplyWechatPayRefundResult(outRefundNo string, wechatRefundID string, providerStatus string, successTime time.Time, failureReason string) (string, error) {
	cacheGroup := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var refund WechatPayRefund
		if err := lockForUpdate(tx).Where("out_refund_no = ?", outRefundNo).First(&refund).Error; err != nil {
			return err
		}
		if wechatRefundID != "" {
			refund.WechatRefundId = &wechatRefundID
		}
		switch strings.ToUpper(strings.TrimSpace(providerStatus)) {
		case "SUCCESS":
			if refund.BusinessReservationState == WechatPayRefundBusinessCommitted {
				return nil
			}
			if refund.BusinessReservationState != WechatPayRefundBusinessReserved {
				return errors.New("退款业务预留状态无效")
			}
			if refund.OrderKind == WechatPayOrderKindTopUp {
				if err := tx.Model(&WechatPayOrder{}).Where("out_trade_no = ?", refund.OutTradeNo).Update("status", WechatPayOrderStatusRefunded).Error; err != nil {
					return err
				}
				if err := tx.Model(&TopUp{}).Where("trade_no = ?", refund.OutTradeNo).Update("status", WechatPayOrderStatusRefunded).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&SubscriptionWechatPayOrder{}).Where("out_trade_no = ?", refund.OutTradeNo).Update("status", WechatPayOrderStatusRefunded).Error; err != nil {
					return err
				}
				if err := tx.Model(&SubscriptionOrder{}).Where("trade_no = ?", refund.OutTradeNo).Update("status", WechatPayOrderStatusRefunded).Error; err != nil {
					return err
				}
				if err := tx.Model(&UserSubscription{}).Where("id = ? AND status = ?", refund.UserSubscriptionId, "refund_pending").Update("status", "cancelled").Error; err != nil {
					return err
				}
			}
			refund.Status = WechatPayRefundStatusSuccess
			refund.BusinessReservationState = WechatPayRefundBusinessCommitted
			refund.FailureReason = ""
			refund.SuccessTime = successTime.Unix()
			if refund.SuccessTime <= 0 {
				refund.SuccessTime = common.GetTimestamp()
			}
		case "PROCESSING":
			refund.Status = WechatPayRefundStatusProcessing
		case "CLOSED", "ABNORMAL":
			var err error
			cacheGroup, err = releaseWechatRefundReservationTx(tx, &refund, failureReason)
			if err != nil {
				return err
			}
			if strings.EqualFold(providerStatus, "closed") {
				refund.Status = WechatPayRefundStatusClosed
			} else {
				refund.Status = WechatPayRefundStatusAbnormal
			}
		default:
			return errors.New("未知的微信退款状态")
		}
		refund.UpdatedAt = common.GetTimestamp()
		refund.LastCheckedAt = common.GetTimestamp()
		return tx.Save(&refund).Error
	})
	return cacheGroup, err
}

func SaveWechatPayRefundNotification(notification *WechatPayRefundNotification) (bool, error) {
	if notification == nil || notification.EventId == "" {
		return false, errors.New("invalid refund notification")
	}
	result := DB.Where("event_id = ?", notification.EventId).FirstOrCreate(notification)
	return result.RowsAffected == 1, result.Error
}

func MarkWechatPayRefundNotificationProcessed(eventID string) error {
	return DB.Model(&WechatPayRefundNotification{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{"processing_status": "processed", "processed_at": common.GetTimestamp()}).Error
}
