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
import {
  getAllLogs,
  getUserLogs,
  getAllMidjourneyLogs,
  getUserMidjourneyLogs,
  getAllTaskLogs,
  getUserTaskLogs,
} from '../api'
import { LOG_TYPE_ENUM } from '../constants'
import type {
  FetchLogsConfig,
  GetLogsParams,
  GetLogsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
} from '../types'
import { buildApiParams, buildBaseParams } from './utils'

export function applyCommonLogTypeScope(
  params: GetLogsParams,
  isAdmin: boolean
): GetLogsParams {
  if (!isAdmin) {
    params.type = LOG_TYPE_ENUM.CONSUME
  }
  return params
}

export async function fetchLogsByCategory(
  config: FetchLogsConfig
): Promise<GetLogsResponse> {
  const { logCategory, isAdmin, page, pageSize, searchParams, columnFilters } =
    config

  if (logCategory === 'common') {
    const params = buildApiParams({
      page,
      pageSize,
      searchParams,
      columnFilters,
      isAdmin,
    })
    applyCommonLogTypeScope(params, isAdmin)
    return isAdmin ? await getAllLogs(params) : await getUserLogs(params)
  }

  const baseParams = buildBaseParams({
    page,
    pageSize,
    searchParams,
    useMilliseconds: logCategory === 'drawing',
  })

  const paramsWithFilter = {
    ...baseParams,
    ...(logCategory === 'drawing'
      ? { mj_id: searchParams.filter as string | undefined }
      : {}),
    ...(logCategory === 'task'
      ? { task_id: searchParams.filter as string | undefined }
      : {}),
  }

  if (logCategory === 'drawing') {
    return isAdmin
      ? await getAllMidjourneyLogs(paramsWithFilter as GetMidjourneyLogsParams)
      : await getUserMidjourneyLogs(paramsWithFilter as GetMidjourneyLogsParams)
  }

  return isAdmin
    ? await getAllTaskLogs(paramsWithFilter as GetTaskLogsParams)
    : await getUserTaskLogs(paramsWithFilter as GetTaskLogsParams)
}
