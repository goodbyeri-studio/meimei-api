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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  Ban,
  CheckCircle2,
  CircleDollarSign,
  Edit3,
  Loader2,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  deleteManagedGroup,
  disableManagedGroup,
  getChannelHealth,
  getManagedGroups,
  getWechatOrders,
  reconcileWechatOrder,
  refundWechatOrder,
  restoreManagedGroup,
  saveManagedGroup,
} from './api'
import type { ManagedGroup, WechatOrder } from './types'

const orderKinds = [
  { value: 'all', label: 'All types' },
  { value: 'topup', label: 'Wallet top-up' },
  { value: 'subscription', label: 'Subscription' },
]

const healthWindows = [
  { value: '1h', label: 'Last hour' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
] as const

function formatTime(timestamp?: number) {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-'
}

function getOrderStatusLabel(status: string) {
  switch (status) {
    case 'pending':
      return 'Pending'
    case 'credited':
      return 'Paid'
    case 'refunded':
      return 'Refunded'
    case 'closed':
      return 'Closed'
    case 'failed':
      return 'Failed'
    default:
      return 'Unknown'
  }
}

function statusVariant(status: string) {
  if (status === 'credited') return 'default' as const
  if (status === 'failed') return 'destructive' as const
  if (status === 'pending') return 'secondary' as const
  return 'outline' as const
}

function healthVariant(requestCount: number, successRate: number) {
  if (requestCount === 0) return 'outline' as const
  if (successRate >= 99) return 'default' as const
  if (successRate >= 95) return 'secondary' as const
  return 'destructive' as const
}

function PaymentOrdersPanel({ isRoot }: { isRoot: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [kind, setKind] = useState('all')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [refundOrder, setRefundOrder] = useState<WechatOrder | null>(null)
  const [refundReason, setRefundReason] = useState('')

  const ordersQuery = useQuery({
    queryKey: ['admin-operations', 'wechat-orders', page, kind, keyword],
    queryFn: () =>
      getWechatOrders({
        page,
        page_size: 20,
        kind: kind === 'all' ? undefined : kind,
        keyword: keyword || undefined,
      }),
  })

  const reconcileMutation = useMutation({
    mutationFn: (order: WechatOrder) =>
      reconcileWechatOrder(order.out_trade_no, order.kind),
    onSuccess: () => {
      toast.success(t('Reconciliation completed'))
      queryClient.invalidateQueries({
        queryKey: ['admin-operations', 'wechat-orders'],
      })
    },
  })

  const refundMutation = useMutation({
    mutationFn: () => {
      if (!refundOrder) throw new Error(t('Order not found'))
      return refundWechatOrder(
        refundOrder.out_trade_no,
        refundOrder.kind,
        refundReason.trim()
      )
    },
    onSuccess: () => {
      toast.success(t('Refund submitted'))
      setRefundOrder(null)
      setRefundReason('')
      queryClient.invalidateQueries({
        queryKey: ['admin-operations', 'wechat-orders'],
      })
    },
  })

  const items = ordersQuery.data?.items ?? []
  const totalPages = Math.max(
    1,
    Math.ceil(
      (ordersQuery.data?.total ?? 0) / (ordersQuery.data?.page_size ?? 20)
    )
  )

  return (
    <div className='flex min-h-0 flex-col gap-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Select
          items={orderKinds.map((item) => ({
            value: item.value,
            label: t(item.label),
          }))}
          value={kind}
          onValueChange={(value) => {
            if (!value) return
            setKind(value)
            setPage(1)
          }}
        >
          <SelectTrigger className='w-44'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {orderKinds.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {t(item.label)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        <Input
          className='w-full sm:w-72'
          value={keywordInput}
          onChange={(event) => setKeywordInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              setKeyword(keywordInput.trim())
              setPage(1)
            }
          }}
          placeholder={t('Search order number or request ID')}
        />
        <Button
          variant='outline'
          onClick={() => {
            setKeyword(keywordInput.trim())
            setPage(1)
          }}
        >
          <Search />
          {t('Search')}
        </Button>
        <Button
          variant='ghost'
          size='icon'
          aria-label={t('Refresh')}
          title={t('Refresh')}
          onClick={() => ordersQuery.refetch()}
        >
          <RefreshCw className={ordersQuery.isFetching ? 'animate-spin' : ''} />
        </Button>
      </div>

      <div className='min-h-0 overflow-auto rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Order')}</TableHead>
              <TableHead>{t('Customer')}</TableHead>
              <TableHead>{t('Type')}</TableHead>
              <TableHead>{t('Amount')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Created at')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((order) => (
              <TableRow key={`${order.kind}:${order.out_trade_no}`}>
                <TableCell className='max-w-64 truncate font-mono text-xs'>
                  {order.out_trade_no}
                </TableCell>
                <TableCell>
                  <div>{order.username || `#${order.user_id}`}</div>
                  <div className='text-muted-foreground text-xs'>
                    ID {order.user_id}
                  </div>
                </TableCell>
                <TableCell>
                  {order.kind === 'topup'
                    ? t('Wallet top-up')
                    : order.plan_title || t('Subscription')}
                </TableCell>
                <TableCell>
                  {order.currency} {(order.amount_fen / 100).toFixed(2)}
                </TableCell>
                <TableCell>
                  <div className='flex flex-col items-start gap-1'>
                    <Badge variant={statusVariant(order.status)}>
                      {t(getOrderStatusLabel(order.status))}
                    </Badge>
                    {order.refund ? (
                      <span className='text-muted-foreground text-xs'>
                        {t('Refund')}: {order.refund.status}
                      </span>
                    ) : null}
                  </div>
                </TableCell>
                <TableCell>{formatTime(order.created_at)}</TableCell>
                <TableCell>
                  <div className='flex justify-end gap-1'>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      title={t('Reconcile')}
                      aria-label={t('Reconcile')}
                      disabled={reconcileMutation.isPending}
                      onClick={() => reconcileMutation.mutate(order)}
                    >
                      <RefreshCw />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      title={t('Refund')}
                      aria-label={t('Refund')}
                      disabled={
                        !isRoot ||
                        order.status !== 'credited' ||
                        Boolean(order.refund)
                      }
                      onClick={() => setRefundOrder(order)}
                    >
                      <RotateCcw />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {!ordersQuery.isLoading && items.length === 0 ? (
          <Empty className='min-h-64 border-0'>
            <EmptyHeader>
              <EmptyMedia variant='icon'>
                <CircleDollarSign />
              </EmptyMedia>
              <EmptyTitle>{t('No payment orders')}</EmptyTitle>
              <EmptyDescription>
                {t(
                  'Payment orders will appear here after customers create them.'
                )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : null}
      </div>

      <div className='flex items-center justify-between'>
        <span className='text-muted-foreground text-sm'>
          {t('Page {{current}} of {{total}}', {
            current: page,
            total: totalPages,
          })}
        </span>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={page <= 1}
            onClick={() => setPage((value) => Math.max(1, value - 1))}
          >
            {t('Previous')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={page >= totalPages}
            onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
          >
            {t('Next')}
          </Button>
        </div>
      </div>

      <Dialog
        open={refundOrder !== null}
        onOpenChange={(open) => !open && setRefundOrder(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Submit WeChat refund')}</DialogTitle>
            <DialogDescription>
              {t(
                'The customer balance or unused subscription will be reserved before the refund is sent to WeChat Pay.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <Label htmlFor='refund-reason'>{t('Refund reason')}</Label>
            <Input
              id='refund-reason'
              maxLength={80}
              value={refundReason}
              onChange={(event) => setRefundReason(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setRefundOrder(null)}
              disabled={refundMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              disabled={!refundReason.trim() || refundMutation.isPending}
              onClick={() => refundMutation.mutate()}
            >
              {refundMutation.isPending ? (
                <Loader2 className='animate-spin' />
              ) : (
                <RotateCcw />
              )}
              {t('Submit refund')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function GroupManagementPanel({ isRoot }: { isRoot: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editingGroup, setEditingGroup] = useState<ManagedGroup | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [ratio, setRatio] = useState('1')
  const [deleteGroup, setDeleteGroup] = useState<ManagedGroup | null>(null)

  const groupsQuery = useQuery({
    queryKey: ['admin-operations', 'groups'],
    queryFn: getManagedGroups,
  })

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['admin-operations', 'groups'] })

  const saveMutation = useMutation({
    mutationFn: saveManagedGroup,
    onSuccess: () => {
      toast.success(t('Group saved'))
      setEditingGroup(null)
      setIsCreating(false)
      refresh()
    },
  })
  const disableMutation = useMutation({
    mutationFn: (group: ManagedGroup) =>
      group.disabled
        ? restoreManagedGroup(group.name)
        : disableManagedGroup(group.name, t('Disabled by administrator')),
    onSuccess: refresh,
  })
  const deleteMutation = useMutation({
    mutationFn: (group: ManagedGroup) => deleteManagedGroup(group.name),
    onSuccess: () => {
      toast.success(t('Group deleted'))
      setDeleteGroup(null)
      refresh()
    },
  })

  const openEditor = (group?: ManagedGroup) => {
    setEditingGroup(group ?? null)
    setIsCreating(!group)
    setName(group?.name ?? '')
    setDescription(group?.description ?? '')
    setRatio(String(group?.ratio ?? 1))
  }

  const groups = groupsQuery.data ?? []

  return (
    <div className='flex min-h-0 flex-col gap-3'>
      <div className='flex items-center justify-between gap-2'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Disabled groups are immediately hidden from customer API key creation. Permanent deletion is blocked while references exist.'
          )}
        </p>
        <Button disabled={!isRoot} onClick={() => openEditor()}>
          <Plus />
          {t('Add group')}
        </Button>
      </div>
      <div className='min-h-0 overflow-auto rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Group')}</TableHead>
              <TableHead>{t('Ratio')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('References')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {groups.map((group) => {
              const refs = group.references
              const blocking =
                refs.users +
                refs.tokens +
                refs.channels +
                refs.subscription_plans +
                refs.active_subscriptions
              return (
                <TableRow key={group.name}>
                  <TableCell>
                    <div className='font-medium'>{group.name}</div>
                    <div className='text-muted-foreground max-w-80 truncate text-xs'>
                      {group.description || '-'}
                    </div>
                  </TableCell>
                  <TableCell>{group.ratio.toFixed(4)}</TableCell>
                  <TableCell>
                    <Badge variant={group.disabled ? 'destructive' : 'outline'}>
                      {group.disabled ? t('Disabled') : t('Enabled')}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className='text-xs'>
                      {t('Users')}: {refs.users} · {t('API Keys')}:{' '}
                      {refs.tokens} · {t('Channels')}: {refs.channels}
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Plans')}: {refs.subscription_plans} ·{' '}
                      {t('Active subscriptions')}: {refs.active_subscriptions}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='flex justify-end gap-1'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        title={t('Edit')}
                        aria-label={t('Edit')}
                        disabled={!isRoot}
                        onClick={() => openEditor(group)}
                      >
                        <Edit3 />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        title={group.disabled ? t('Restore') : t('Disable')}
                        aria-label={
                          group.disabled ? t('Restore') : t('Disable')
                        }
                        disabled={!isRoot || disableMutation.isPending}
                        onClick={() => disableMutation.mutate(group)}
                      >
                        {group.disabled ? <CheckCircle2 /> : <Ban />}
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        title={t('Delete')}
                        aria-label={t('Delete')}
                        disabled={
                          !isRoot || blocking > 0 || group.name === 'default'
                        }
                        onClick={() => setDeleteGroup(group)}
                      >
                        <Trash2 />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>

      <Dialog
        open={isCreating || editingGroup !== null}
        onOpenChange={(open) => {
          if (!open) {
            setIsCreating(false)
            setEditingGroup(null)
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {isCreating ? t('Add group') : t('Edit group')}
            </DialogTitle>
            <DialogDescription>
              {t('Changes to the ratio apply to new usage immediately.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3'>
            <div className='space-y-1.5'>
              <Label htmlFor='group-name'>{t('Group name')}</Label>
              <Input
                id='group-name'
                value={name}
                disabled={!isCreating}
                maxLength={64}
                onChange={(event) => setName(event.target.value)}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='group-description'>{t('Description')}</Label>
              <Input
                id='group-description'
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='group-ratio'>{t('Ratio')}</Label>
              <Input
                id='group-ratio'
                type='number'
                min='0.0001'
                max='1000'
                step='0.0001'
                value={ratio}
                onChange={(event) => setRatio(event.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => {
                setIsCreating(false)
                setEditingGroup(null)
              }}
            >
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                !name.trim() ||
                !Number.isFinite(Number(ratio)) ||
                Number(ratio) <= 0 ||
                saveMutation.isPending
              }
              onClick={() =>
                saveMutation.mutate({
                  name: name.trim(),
                  description: description.trim(),
                  ratio: Number(ratio),
                })
              }
            >
              {saveMutation.isPending ? (
                <Loader2 className='animate-spin' />
              ) : null}
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteGroup !== null}
        onOpenChange={(open) => !open && setDeleteGroup(null)}
        title={t('Delete group')}
        desc={t(
          'This permanently removes the group configuration. This action cannot be undone.'
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => deleteGroup && deleteMutation.mutate(deleteGroup)}
      />
    </div>
  )
}

function ChannelHealthPanel() {
  const { t } = useTranslation()
  const [window, setWindow] = useState<'1h' | '24h' | '7d'>('24h')
  const healthQuery = useQuery({
    queryKey: ['admin-operations', 'channel-health', window],
    queryFn: () => getChannelHealth(window),
    refetchInterval: 60_000,
  })
  const items = [...(healthQuery.data ?? [])].sort((left, right) => {
    if (left.error_count !== right.error_count) {
      return right.error_count - left.error_count
    }
    return left.success_rate - right.success_rate
  })

  return (
    <div className='flex min-h-0 flex-col gap-3'>
      <div className='flex items-center justify-between'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Success rate is calculated from consume and error logs in the selected period.'
          )}
        </p>
        <Select
          items={healthWindows.map((item) => ({
            value: item.value,
            label: t(item.label),
          }))}
          value={window}
          onValueChange={(value) => value && setWindow(value as typeof window)}
        >
          <SelectTrigger className='w-44'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {healthWindows.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {t(item.label)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      <div className='min-h-0 overflow-auto rounded-lg border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Channel')}</TableHead>
              <TableHead>{t('Group')}</TableHead>
              <TableHead>{t('Requests')}</TableHead>
              <TableHead>{t('Success rate')}</TableHead>
              <TableHead>{t('Errors')}</TableHead>
              <TableHead>{t('Average latency')}</TableHead>
              <TableHead>{t('Last request')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.channel_id}>
                <TableCell>
                  <div className='font-medium'>{item.channel_name}</div>
                  <div className='text-muted-foreground text-xs'>
                    #{item.channel_id}
                  </div>
                </TableCell>
                <TableCell>{item.channel_group || '-'}</TableCell>
                <TableCell>{item.request_count}</TableCell>
                <TableCell>
                  <Badge
                    variant={healthVariant(
                      item.request_count,
                      item.success_rate
                    )}
                  >
                    {item.request_count === 0
                      ? t('No data')
                      : `${item.success_rate.toFixed(2)}%`}
                  </Badge>
                </TableCell>
                <TableCell>{item.error_count}</TableCell>
                <TableCell>{item.average_latency_ms} ms</TableCell>
                <TableCell>{formatTime(item.last_request_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

export function AdminOperations() {
  const { t } = useTranslation()
  const role = useAuthStore((state) => state.auth.user?.role)
  const isRoot = role === ROLE.SUPER_ADMIN

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Operations Center')}</span>
          {!isRoot ? <Badge variant='outline'>{t('Read only')}</Badge> : null}
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='payments' className='h-full min-h-0 gap-4'>
          <TabsList variant='line'>
            <TabsTrigger value='payments'>
              <CircleDollarSign />
              {t('Payments')}
            </TabsTrigger>
            <TabsTrigger value='groups'>
              <Activity />
              {t('Groups')}
            </TabsTrigger>
            <TabsTrigger value='health'>
              <Activity />
              {t('Channel health')}
            </TabsTrigger>
          </TabsList>
          <TabsContent value='payments' className='min-h-0 overflow-auto'>
            <PaymentOrdersPanel isRoot={isRoot} />
          </TabsContent>
          <TabsContent value='groups' className='min-h-0 overflow-auto'>
            <GroupManagementPanel isRoot={isRoot} />
          </TabsContent>
          <TabsContent value='health' className='min-h-0 overflow-auto'>
            <ChannelHealthPanel />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
