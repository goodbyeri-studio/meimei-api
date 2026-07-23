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
import { flexRender, type Row } from '@tanstack/react-table'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'

import { isTagAggregateRow, parseGroupsList, parseModelsList } from '../lib'
import type { Channel } from '../types'
import { ChannelRowActionsLayoutContext } from './channel-row-actions-context'
import { useChannels } from './channels-provider'
import { EditableGroupRatioBadge } from './editable-group-ratio-badge'

const SENSITIVE_MASK = '••••'

function ChannelCardComponent({
  row,
  isSelected,
  groupRatios,
  canEditGroupRatios,
}: {
  row: Row<Channel>
  isSelected: boolean
  groupRatios: Record<string, number>
  canEditGroupRatios: boolean
}) {
  const { t } = useTranslation()
  const { sensitiveVisible } = useChannels()
  const isTagRow = isTagAggregateRow(row.original)
  const cells = row.getAllCells()

  const renderCell = (id: string) => {
    const cell = cells.find((c) => c.column.id === id)
    if (!cell || !cell.column.columnDef.cell) {
      return null
    }
    return flexRender(cell.column.columnDef.cell, cell.getContext())
  }

  const groups = parseGroupsList(row.original.group ?? '')
  const modelNames = new Set(parseModelsList(row.original.models ?? ''))
  if (isTagRow) {
    const children = (row.original as Channel & { children?: Channel[] })
      .children
    for (const channel of children ?? []) {
      for (const model of parseModelsList(channel.models ?? '')) {
        modelNames.add(model)
      }
    }
  }

  const baseUrl = row.original.base_url?.trim() || '-'
  let endpoint = baseUrl
  if (baseUrl !== '-') {
    try {
      endpoint = new URL(baseUrl).host || baseUrl
    } catch {
      endpoint = baseUrl
    }
  }

  const selectCell = renderCell('select')
  const typeCell = renderCell('type')
  const nameCell = renderCell('name')
  const statusCell = renderCell('status')
  const actionsCell = renderCell('actions')
  const priorityCell = renderCell('priority')
  const weightCell = renderCell('weight')
  const balanceCell = renderCell('balance')
  const responseCell = renderCell('response_time')
  const testCell = renderCell('test_time')
  const tagCell = row.original.tag ? renderCell('tag') : null

  const labelClass =
    'text-muted-foreground text-[11px] font-medium leading-none select-none'

  return (
    <ChannelRowActionsLayoutContext.Provider value='card'>
      <div
        data-state={isSelected ? 'selected' : undefined}
        className='grid min-w-0 grid-cols-1 items-center gap-x-4 gap-y-4 sm:grid-cols-2 xl:grid-cols-[minmax(200px,1.25fr)_minmax(170px,1fr)_minmax(145px,.85fr)_minmax(190px,1fr)_auto]'
      >
        <div className='min-w-0 sm:col-span-2 xl:col-span-1'>
          <div className='flex min-w-0 items-center gap-2'>
            {!isTagRow && selectCell && (
              <span className='shrink-0'>{selectCell}</span>
            )}
            <div className='min-w-0 flex-1 overflow-hidden'>{typeCell}</div>
            <div className='shrink-0'>{statusCell}</div>
          </div>
          <div className='mt-2 flex min-w-0 items-baseline gap-2'>
            {!isTagRow && (
              <span className='text-muted-foreground shrink-0 font-mono text-[11px]'>
                #{sensitiveVisible ? row.original.id : SENSITIVE_MASK}
              </span>
            )}
            <div className='min-w-0 flex-1 text-sm'>{nameCell}</div>
          </div>
        </div>

        <div className='min-w-0'>
          <div className={labelClass}>{t('Group')}</div>
          <div className='mt-2 flex min-h-5 min-w-0 flex-wrap items-center gap-1'>
            {groups.length > 0 ? (
              groups.map((group) => (
                <EditableGroupRatioBadge
                  key={group}
                  group={group}
                  label={sensitiveVisible ? undefined : SENSITIVE_MASK}
                  ratio={sensitiveVisible ? groupRatios[group] : null}
                  groupRatios={groupRatios}
                  editable={canEditGroupRatios && sensitiveVisible}
                />
              ))
            ) : (
              <span className='text-muted-foreground text-sm'>-</span>
            )}
          </div>
          <div className='mt-3 flex min-w-0 items-center gap-2'>
            <span className={labelClass}>{t('Models')}</span>
            <StatusBadge
              label={t('{{count}} models', { count: modelNames.size })}
              variant='neutral'
              size='sm'
              copyable={false}
              showDot={false}
            />
            {tagCell && (
              <div className='min-w-0 overflow-hidden'>{tagCell}</div>
            )}
          </div>
        </div>

        <div className='min-w-0'>
          <div className={labelClass}>{t('Used / Remaining')}</div>
          <div className='mt-2 min-h-5 min-w-0 overflow-hidden text-sm'>
            {balanceCell ?? <span className='text-muted-foreground'>-</span>}
          </div>
          <div className='mt-3 min-w-0'>
            <div className={labelClass}>{t('Base URL')}</div>
            <div
              className='text-muted-foreground mt-1.5 truncate font-mono text-xs'
              title={sensitiveVisible ? baseUrl : undefined}
            >
              {sensitiveVisible ? endpoint : SENSITIVE_MASK}
            </div>
          </div>
        </div>

        <div className='grid min-w-0 grid-cols-2 items-center gap-x-4 gap-y-2'>
          <span className={labelClass}>{t('Priority')}</span>
          <span className={labelClass}>{t('Weight')}</span>
          <div className='min-w-0'>{priorityCell}</div>
          <div className='min-w-0'>{weightCell}</div>
          <span className={labelClass}>{t('Response')}</span>
          <span className={labelClass}>{t('Last Tested')}</span>
          <div className='min-w-0 overflow-hidden text-sm'>
            {responseCell ?? <span className='text-muted-foreground'>-</span>}
          </div>
          <div className='min-w-0 overflow-hidden text-sm'>
            {testCell ?? <span className='text-muted-foreground'>-</span>}
          </div>
        </div>

        <div className='flex justify-end sm:col-span-2 xl:col-span-1 xl:justify-start'>
          {actionsCell}
        </div>
      </div>
    </ChannelRowActionsLayoutContext.Provider>
  )
}

export const ChannelCard = memo(ChannelCardComponent)
