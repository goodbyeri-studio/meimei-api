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
import { tryJsonParse } from '@/features/system-settings/utils/json-parser'

export type GroupRatiosParseResult =
  | { valid: true; ratios: Record<string, number> }
  | { valid: false; ratios: Record<string, never> }

export function parseGroupRatiosOption(
  value: string | null | undefined
): GroupRatiosParseResult {
  const result = tryJsonParse<unknown>(value)
  if (
    !result.success ||
    typeof result.data !== 'object' ||
    result.data === null ||
    Array.isArray(result.data)
  ) {
    return { valid: false, ratios: {} }
  }

  const entries = Object.entries(result.data)
  const valid = entries.every(
    ([group, ratio]) =>
      group.trim() !== '' &&
      typeof ratio === 'number' &&
      Number.isFinite(ratio) &&
      ratio >= 0
  )
  if (!valid) {
    return { valid: false, ratios: {} }
  }
  return { valid: true, ratios: Object.fromEntries(entries) }
}
