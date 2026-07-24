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
export type WechatOrderKind = 'topup' | 'subscription'

export interface WechatRefund {
  out_refund_no: string
  status: string
  reason: string
  failure_reason?: string
  amount_fen: number
  created_at: number
  success_time: number
}

export interface WechatOrder {
  kind: WechatOrderKind
  user_id: number
  username: string
  out_trade_no: string
  amount_fen: number
  currency: string
  status: string
  created_at: number
  success_time: number
  last_checked_at: number
  plan_title?: string
  refund?: WechatRefund
}

export interface WechatOrderPage {
  items: WechatOrder[]
  total: number
  page: number
  page_size: number
}

export interface GroupReferences {
  users: number
  tokens: number
  channels: number
  subscription_plans: number
  active_subscriptions: number
  auto_group: boolean
  user_usable_group: boolean
  group_ratio_overrides: number
  special_usable_mappings: number
}

export interface ManagedGroup {
  name: string
  description: string
  ratio: number
  disabled: boolean
  reason?: string
  updated_at?: number
  references: GroupReferences
}

export interface ChannelHealth {
  channel_id: number
  channel_name: string
  channel_group: string
  channel_status: number
  request_count: number
  success_count: number
  error_count: number
  success_rate: number
  average_latency_ms: number
  last_request_at: number
}
