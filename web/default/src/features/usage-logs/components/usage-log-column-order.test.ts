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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getCommonLogColumnOrder } from './usage-log-column-order'

describe('usage log column visibility', () => {
  test('keeps billing columns in the regular user view', () => {
    assert.deepEqual(getCommonLogColumnOrder(false), [
      'created_at',
      'token_name',
      'group',
      'type',
      'model_name',
      'prompt_tokens',
      'completion_tokens',
      'cache_tokens',
      'use_time',
      'quota',
      'content',
    ])
  })

  test('keeps user and channel controls in the admin view', () => {
    const columns = getCommonLogColumnOrder(true)

    assert.equal(columns[0], 'created_at')
    assert.equal(columns[1], 'user')
    assert.equal(columns[2], 'channel')
    assert.ok(columns.includes('quota'))
    assert.ok(columns.includes('content'))
  })
})
