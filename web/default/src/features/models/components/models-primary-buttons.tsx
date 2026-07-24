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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus,
  MoreHorizontal,
  RefreshCw,
  List,
  Building2,
  AlertCircle,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import { syncDeepKeyCatalog } from '../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../lib'
import { useModels } from './models-provider'

export function ModelsPrimaryButtons() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { setOpen, setCurrentRow } = useModels()
  const [deepKeySyncOpen, setDeepKeySyncOpen] = useState(false)

  const deepKeySyncMutation = useMutation({
    mutationFn: syncDeepKeyCatalog,
    onSuccess: async (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to sync DeepKey catalog'))
        return
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() }),
      ])
      setDeepKeySyncOpen(false)
      toast.success(
        t(
          'DeepKey catalog synced: {{total}} total, {{available}} available, {{created}} created, {{updated}} updated',
          response.data
        )
      )
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to sync DeepKey catalog'))
    },
  })

  const handleCreateModel = () => {
    setCurrentRow(null)
    setOpen('create-model')
  }

  const handleMissingModels = () => {
    setOpen('missing-models')
  }

  const handleSync = () => {
    setOpen('sync-wizard')
  }

  const handlePrefillGroups = () => {
    setOpen('prefill-groups')
  }

  const handleManageVendors = () => {
    setOpen('create-vendor') // Will be a separate vendors management dialog
  }

  return (
    <div className='flex items-center gap-2'>
      {/* Create Model */}
      <Button onClick={handleCreateModel} size='sm'>
        <Plus className='h-4 w-4' />
        {t('Add Model')}
      </Button>

      <Button
        variant='outline'
        size='sm'
        onClick={() => setDeepKeySyncOpen(true)}
        disabled={deepKeySyncMutation.isPending}
      >
        <RefreshCw
          className={cn(
            'h-4 w-4',
            deepKeySyncMutation.isPending && 'animate-spin'
          )}
        />
        {t('Sync DeepKey catalog')}
      </Button>

      {/* More Actions */}
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant='outline' size='sm' />}>
          <MoreHorizontal className='h-4 w-4' />
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-56'>
          <DropdownMenuItem onClick={handleMissingModels}>
            {t('Missing Models')}
            <DropdownMenuShortcut>
              <AlertCircle className='h-4 w-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuItem onClick={handleSync}>
            {t('Sync Upstream')}
            <DropdownMenuShortcut>
              <RefreshCw className='h-4 w-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem onClick={handlePrefillGroups}>
            {t('Prefill Groups')}
            <DropdownMenuShortcut>
              <List className='h-4 w-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>

          <DropdownMenuItem onClick={handleManageVendors}>
            {t('Manage Vendors')}
            <DropdownMenuShortcut>
              <Building2 className='h-4 w-4' />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog open={deepKeySyncOpen} onOpenChange={setDeepKeySyncOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Sync DeepKey catalog')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Refresh all DeepKey catalog models now? Only models backed by an enabled local DeepKey channel will be published, and manual unpublish decisions will be preserved.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deepKeySyncMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deepKeySyncMutation.mutate()}
              disabled={deepKeySyncMutation.isPending}
            >
              {deepKeySyncMutation.isPending ? t('Syncing...') : t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
