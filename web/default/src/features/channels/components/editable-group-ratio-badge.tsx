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
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, Save, X } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupBadge } from '@/components/group-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import type { SystemOptionsResponse } from '@/features/system-settings/types'

type EditableGroupRatioBadgeProps = {
  group: string
  ratio?: number | null
  groupRatios: Record<string, number>
  editable: boolean
  label?: string
}

export function EditableGroupRatioBadge(props: EditableGroupRatioBadgeProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()
  const inputId = useId()

  const displayRatio = props.ratio ?? (props.editable ? 1 : null)
  const badge = (
    <GroupBadge
      group={props.group}
      label={props.label}
      ratio={displayRatio}
      size='sm'
    />
  )

  if (!props.editable || displayRatio == null) {
    return badge
  }

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (nextOpen) {
      setDraft(String(displayRatio))
      setError(null)
    }
  }

  const handleSave = async () => {
    if (updateOption.isPending) return

    const value = Number(draft.trim())
    if (draft.trim() === '' || !Number.isFinite(value)) {
      setError(t('Please enter a valid ratio'))
      return
    }
    if (value < 0) {
      setError(t('Value must be at least 0'))
      return
    }

    const nextRatios = {
      ...props.groupRatios,
      [props.group]: value,
    }
    const serializedRatios = JSON.stringify(nextRatios)
    try {
      const result = await updateOption.mutateAsync({
        key: 'GroupRatio',
        value: serializedRatios,
      })
      if (result.success) {
        queryClient.setQueryData<SystemOptionsResponse>(
          ['system-options'],
          (current) => {
            if (!current) return current
            const hasGroupRatio = current.data.some(
              (option) => option.key === 'GroupRatio'
            )
            const data = hasGroupRatio
              ? current.data.map((option) =>
                  option.key === 'GroupRatio'
                    ? { ...option, value: serializedRatios }
                    : option
                )
              : [
                  ...current.data,
                  { key: 'GroupRatio', value: serializedRatios },
                ]
            return { ...current, data }
          }
        )
        setOpen(false)
      }
    } catch {
      // useUpdateOption displays the request error.
    }
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <button
            type='button'
            className='group/ratio focus-visible:ring-ring/50 inline-flex max-w-full min-w-0 cursor-pointer rounded-md text-left outline-none focus-visible:ring-2'
            aria-label={`${t('Edit')} ${t('Ratio')}`}
            onClick={(event) => event.stopPropagation()}
          />
        }
      >
        {badge}
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-[min(20rem,calc(100vw-2rem))] p-3'
      >
        <PopoverHeader>
          <PopoverTitle>{t('Ratio')}</PopoverTitle>
          <PopoverDescription>{t('Group ratios')}</PopoverDescription>
        </PopoverHeader>
        <div className='space-y-2'>
          <Label htmlFor={inputId}>{t('Ratio')}</Label>
          <div className='flex items-center gap-2'>
            <Input
              id={inputId}
              type='number'
              min='0'
              step='0.01'
              inputMode='decimal'
              value={draft}
              onChange={(event) => {
                setDraft(event.target.value)
                setError(null)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault()
                  void handleSave()
                }
              }}
              aria-invalid={error != null}
            />
            <span className='text-muted-foreground shrink-0 text-sm'>x</span>
          </div>
          {error && (
            <p className='text-destructive text-xs' role='alert'>
              {error}
            </p>
          )}
        </div>
        <div className='flex justify-end gap-2 pt-1'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => setOpen(false)}
            disabled={updateOption.isPending}
          >
            <X className='size-4' />
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            size='sm'
            onClick={() => void handleSave()}
            disabled={updateOption.isPending}
          >
            {updateOption.isPending ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Save className='size-4' />
            )}
            {t('Save')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
