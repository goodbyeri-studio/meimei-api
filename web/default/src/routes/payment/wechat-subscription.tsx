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
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { CircleCheck, Clock3, Loader2, RefreshCw, ScanLine } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { getSubscriptionWechatPayStatus } from '@/features/subscriptions/api'

const searchSchema = z.object({
  trade_no: z.string().min(1),
})

const paymentWindowDataSchema = z.object({
  trade_no: z.string().min(1),
  code_url: z.string().min(1),
  amount_fen: z.number().int().positive(),
  expires_at: z.number().int().positive(),
})

export const Route = createFileRoute('/payment/wechat-subscription')({
  validateSearch: searchSchema,
  component: WechatSubscriptionPaymentPage,
})

function WechatSubscriptionPaymentPage() {
  const { t } = useTranslation()
  const { trade_no } = Route.useSearch()
  const [paymentWindowData] = useState(() => {
    if (typeof window === 'undefined' || !window.name) return null
    try {
      const result = paymentWindowDataSchema.safeParse(JSON.parse(window.name))
      return result.success ? result.data : null
    } catch {
      return null
    }
  })
  const paymentData =
    paymentWindowData?.trade_no === trade_no ? paymentWindowData : null
  const notified = useRef(false)
  const statusQuery = useQuery({
    queryKey: ['subscription', 'wechat-payment', trade_no],
    queryFn: async () => {
      const response = await getSubscriptionWechatPayStatus(trade_no)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Payment request failed'))
      }
      return response.data
    },
    refetchInterval: (query) =>
      query.state.data?.status === 'pending' ? 2000 : false,
    retry: 2,
    enabled: paymentData !== null,
  })

  const status = statusQuery.data?.status || 'pending'
  const expired =
    status === 'pending' &&
    paymentData !== null &&
    paymentData.expires_at * 1000 <= Date.now()
  const completed = status === 'credited'
  const closed = status === 'failed' || status === 'closed' || expired

  useEffect(() => {
    window.name = ''
  }, [])

  useEffect(() => {
    if (!completed || notified.current) return
    notified.current = true
    window.opener?.postMessage(
      { type: 'subscription-wechat-paid', tradeNo: trade_no },
      window.location.origin
    )
  }, [completed, trade_no])

  let content
  if (!paymentData) {
    content = (
      <div className='flex flex-col items-center gap-4 py-8 text-center'>
        <p className='text-muted-foreground'>{t('Payment request failed')}</p>
      </div>
    )
  } else if (completed) {
    content = (
      <div className='flex flex-col items-center gap-4 py-8 text-center'>
        <CircleCheck className='h-16 w-16 text-emerald-500' />
        <p className='text-lg font-semibold'>{t('Payment completed')}</p>
      </div>
    )
  } else if (closed) {
    content = (
      <div className='flex flex-col items-center gap-4 py-8 text-center'>
        <Clock3 className='text-muted-foreground h-14 w-14' />
        <p className='font-medium'>{t('This QR code has expired')}</p>
      </div>
    )
  } else if (statusQuery.isError) {
    content = (
      <div className='flex flex-col items-center gap-4 py-8 text-center'>
        <p className='text-muted-foreground'>{t('Payment request failed')}</p>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void statusQuery.refetch()}
        >
          <RefreshCw className='h-4 w-4' />
          {t('Refresh payment status')}
        </Button>
      </div>
    )
  } else {
    content = (
      <div className='flex flex-col items-center gap-5 py-5'>
        <div className='rounded-xl border bg-white p-4 shadow-sm'>
          <QRCodeSVG
            value={paymentData.code_url}
            size={244}
            level='M'
            marginSize={0}
            title={t('WeChat Pay QR code')}
          />
        </div>
        <div className='text-center'>
          <p className='text-3xl font-semibold tracking-tight'>
            ¥{(paymentData.amount_fen / 100).toFixed(2)}
          </p>
          <p className='text-muted-foreground mt-2 flex items-center justify-center gap-2 text-sm'>
            {statusQuery.isFetching ? (
              <Loader2 className='h-3.5 w-3.5 animate-spin' />
            ) : (
              <ScanLine className='h-3.5 w-3.5' />
            )}
            {t('Waiting for payment')}
          </p>
        </div>
      </div>
    )
  }

  return (
    <main className='bg-muted/30 flex min-h-screen items-center justify-center p-4'>
      <Card className='w-full max-w-md shadow-lg'>
        <CardHeader className='items-center border-b text-center'>
          <div className='flex h-10 w-10 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600'>
            <ScanLine className='h-5 w-5' />
          </div>
          <h1 className='text-xl font-semibold'>{t('WeChat Pay')}</h1>
          <p className='text-muted-foreground text-sm'>
            {t('Use WeChat to scan the QR code and complete payment.')}
          </p>
        </CardHeader>
        <CardContent className='space-y-4 p-5'>
          {content}
          <Button
            variant={completed ? 'default' : 'outline'}
            className='w-full'
            onClick={() => window.close()}
          >
            {t('Close')}
          </Button>
        </CardContent>
      </Card>
    </main>
  )
}
