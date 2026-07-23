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
import i18next from 'i18next'
import { toast } from 'sonner'

import { updateGroupRatio } from '@/features/system-settings/api'
import type {
  SystemOptionsResponse,
  UpdateGroupRatioRequest,
} from '@/features/system-settings/types'

export function useUpdateGroupRatio() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateGroupRatioRequest) => updateGroupRatio(request),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || i18next.t('Failed to update setting'))
        return
      }

      const serializedRatios = JSON.stringify(response.data)
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
            : [...current.data, { key: 'GroupRatio', value: serializedRatios }]
          return { ...current, data }
        }
      )
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      queryClient.invalidateQueries({ queryKey: ['user-groups'] })
      toast.success(i18next.t('Setting updated successfully'))
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Failed to update setting'))
    },
  })
}
