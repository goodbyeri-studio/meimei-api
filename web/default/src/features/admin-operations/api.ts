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
import { api } from '@/lib/api'

import type {
  ChannelHealth,
  ManagedGroup,
  WechatOrderKind,
  WechatOrderPage,
} from './types'

const adminReadHeaders = { 'Cache-Control': 'no-cache' }

export async function getWechatOrders(params: {
  page: number
  page_size: number
  kind?: string
  status?: string
  keyword?: string
}) {
  const response = await api.get('/api/admin/operations/wechat/orders', {
    params,
    headers: adminReadHeaders,
  })
  return response.data.data as WechatOrderPage
}

export async function reconcileWechatOrder(
  tradeNo: string,
  kind: WechatOrderKind
) {
  const response = await api.post(
    `/api/admin/operations/wechat/orders/${tradeNo}/reconcile`,
    { kind }
  )
  return response.data
}

export async function refundWechatOrder(
  tradeNo: string,
  kind: WechatOrderKind,
  reason: string
) {
  const response = await api.post(
    `/api/admin/operations/wechat/orders/${tradeNo}/refund`,
    { kind, reason }
  )
  return response.data
}

export async function getManagedGroups() {
  const response = await api.get('/api/admin/operations/groups', {
    headers: adminReadHeaders,
  })
  return response.data.data as ManagedGroup[]
}

export async function saveManagedGroup(payload: {
  name: string
  description: string
  ratio: number
}) {
  const response = await api.put('/api/admin/operations/groups', payload)
  return response.data
}

export async function disableManagedGroup(name: string, reason: string) {
  const response = await api.post(
    `/api/admin/operations/groups/${encodeURIComponent(name)}/disable`,
    { reason }
  )
  return response.data
}

export async function restoreManagedGroup(name: string) {
  const response = await api.post(
    `/api/admin/operations/groups/${encodeURIComponent(name)}/restore`
  )
  return response.data
}

export async function deleteManagedGroup(name: string) {
  const response = await api.delete(
    `/api/admin/operations/groups/${encodeURIComponent(name)}`
  )
  return response.data
}

export async function getChannelHealth(window: '1h' | '24h' | '7d') {
  const response = await api.get('/api/admin/operations/channel-health', {
    params: { window },
    headers: adminReadHeaders,
  })
  return response.data.data as ChannelHealth[]
}
