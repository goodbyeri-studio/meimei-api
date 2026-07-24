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
import { AlertCircle, ScanLine, WalletCards } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'

import { useWechatNativePayment } from '../hooks/use-wechat-native-payment'
import { formatCurrency } from '../lib'
import type { PresetAmount, TopupInfo } from '../types'
import { WechatNativeDialog } from './dialogs/wechat-native-dialog'

interface BalanceRechargeCardProps {
  topupInfo: TopupInfo | null
  presetAmounts: PresetAmount[]
  loading?: boolean
  onRechargeSuccess?: () => void | Promise<void>
}

export function BalanceRechargeCard(props: BalanceRechargeCardProps) {
  const { t } = useTranslation()
  const wechatPayment = useWechatNativePayment()
  const creditedTradeNo = useRef<string | null>(null)
  const onRechargeSuccess = props.onRechargeSuccess

  useEffect(() => {
    if (
      wechatPayment.order?.status !== 'credited' ||
      wechatPayment.order.trade_no === creditedTradeNo.current
    ) {
      return
    }

    creditedTradeNo.current = wechatPayment.order.trade_no
    toast.success(t('Balance recharged successfully'))
    void onRechargeSuccess?.()
  }, [onRechargeSuccess, t, wechatPayment.order])

  const wechatEnabled = !!props.topupInfo?.enable_wechat_pay

  return (
    <>
      <TitledCard
        title={t('Balance Recharge')}
        description={t('Choose a recharge amount and pay with WeChat')}
        icon={<WalletCards className='h-4 w-4' />}
        iconTone='success'
        disableHoverEffect
        contentClassName='space-y-4 sm:space-y-5'
      >
        {!wechatEnabled && !props.loading ? (
          <div
            className='border-destructive/40 bg-destructive/5 text-destructive flex items-center gap-2 rounded-md border px-3 py-2 text-sm'
            role='alert'
          >
            <AlertCircle className='size-4 shrink-0' aria-hidden='true' />
            {t('WeChat Pay is unavailable. Please contact administrator.')}
          </div>
        ) : null}

        {props.loading ? (
          <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
            {['one', 'two', 'three', 'four'].map((key) => (
              <Skeleton key={key} className='h-40 w-full rounded-lg' />
            ))}
          </div>
        ) : (
          <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
            {props.presetAmounts.map((preset) => (
              <div
                key={preset.value}
                className='bg-card flex min-h-40 flex-col rounded-lg border p-3.5 sm:p-4'
              >
                <div className='text-muted-foreground text-xs'>
                  {t('Recharge Amount')}
                </div>
                <div className='text-primary mt-2 text-2xl font-bold'>
                  ¥{formatCurrency(preset.value)}
                </div>
                <div className='text-muted-foreground mt-2 text-xs'>
                  {t('Balance credited')}: ¥{formatCurrency(preset.value)}
                </div>
                <div className='mt-auto pt-4'>
                  <Button
                    className='w-full'
                    disabled={!wechatEnabled || wechatPayment.creating}
                    onClick={() =>
                      void wechatPayment.processWechatNativePayment(
                        preset.value
                      )
                    }
                  >
                    <ScanLine aria-hidden='true' />
                    {t('Recharge Now')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </TitledCard>

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
