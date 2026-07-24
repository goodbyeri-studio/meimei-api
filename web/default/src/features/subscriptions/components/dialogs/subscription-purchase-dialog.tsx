/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Crown, Package, ScanLine } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { GroupBadge } from '@/components/group-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { WechatNativeDialog } from '@/features/wallet/components/dialogs/wechat-native-dialog'
import { formatQuotaCNY } from '@/lib/format'

import { useSubscriptionWechatPayment } from '../../hooks/use-subscription-wechat-payment'
import { formatPlanPrice, formatResetPeriod } from '../../lib'
import type { PlanRecord } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableWechatPay?: boolean
  purchaseLimit?: number
  purchaseCount?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const [paying, setPaying] = useState(false)
  const wechatPayment = useSubscriptionWechatPayment()
  const creditedTradeNo = useRef<string | null>(null)
  const onPurchaseSuccess = props.onPurchaseSuccess

  useEffect(() => {
    if (
      wechatPayment.order?.status === 'credited' &&
      wechatPayment.order.trade_no !== creditedTradeNo.current
    ) {
      creditedTradeNo.current = wechatPayment.order.trade_no
      toast.success(t('Subscription purchased successfully'))
      void onPurchaseSuccess?.()
    }
  }, [onPurchaseSuccess, t, wechatPayment.order])

  const plan = props.plan?.plan
  if (!plan) return null

  const hasWechatPay = props.enableWechatPay && plan.currency === 'CNY'
  const totalAmount = Number(plan.total_amount || 0)
  const price = formatPlanPrice(plan)
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)

  const handlePayWechat = async () => {
    if (limitReached) return
    setPaying(true)
    try {
      const started = await wechatPayment.start(plan.id)
      if (started) props.onOpenChange(false)
    } finally {
      setPaying(false)
    }
  }

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={
          <>
            <Crown className='h-5 w-5' />
            {t('Purchase Subscription')}
          </>
        }
        contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'
        titleClassName='flex items-center gap-2'
        contentHeight='auto'
        bodyClassName='space-y-4'
      >
        <div className='space-y-3 sm:space-y-4'>
          <div className='bg-muted/50 space-y-2.5 rounded-lg border p-3 sm:space-y-3 sm:p-4'>
            <div className='flex justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Plan Name')}
              </span>
              <span className='max-w-[200px] truncate text-sm font-medium'>
                {plan.title}
              </span>
            </div>
            {formatResetPeriod(plan, t) !== t('No Reset') && (
              <div className='flex justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Reset Period')}
                </span>
                <span className='text-sm'>{formatResetPeriod(plan, t)}</span>
              </div>
            )}
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Plan Quota')}
              </span>
              <span className='flex items-center gap-1 text-sm'>
                <Package className='h-3.5 w-3.5' />
                {totalAmount > 0 ? formatQuotaCNY(totalAmount) : t('Unlimited')}
              </span>
            </div>
            {plan.upgrade_group && (
              <div className='flex items-center justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('Upgrade Group')}
                </span>
                <GroupBadge group={plan.upgrade_group} />
              </div>
            )}
            <Separator />
            <div className='flex items-center justify-between'>
              <span className='text-sm font-medium'>{t('Amount Due')}</span>
              <span className='text-primary text-lg font-bold'>{price}</span>
            </div>
          </div>

          {limitReached && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t('Purchase limit reached')} ({props.purchaseCount}/
                {props.purchaseLimit})
              </AlertDescription>
            </Alert>
          )}

          {hasWechatPay ? (
            <Button
              className='w-full'
              onClick={handlePayWechat}
              disabled={paying || wechatPayment.creating || limitReached}
            >
              <ScanLine className='h-4 w-4' />
              {t('WeChat Pay')}
            </Button>
          ) : (
            <Alert variant='destructive'>
              <AlertDescription>
                {t(
                  'No payment methods available. Please contact administrator.'
                )}
              </AlertDescription>
            </Alert>
          )}
        </div>
      </Dialog>

      <WechatNativeDialog
        open={wechatPayment.open}
        onOpenChange={wechatPayment.setOpen}
        order={wechatPayment.order}
        refreshing={wechatPayment.refreshing}
        refreshError={wechatPayment.refreshError}
        onRefresh={() => void wechatPayment.refresh()}
      />
    </>
  )
}
