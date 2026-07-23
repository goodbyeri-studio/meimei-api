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

import { parseGroupRatiosOption } from './group-ratios'

describe('parseGroupRatiosOption', () => {
  test('accepts a finite non-negative ratio map', () => {
    assert.deepEqual(parseGroupRatiosOption('{"default":1,"vip":1.5}'), {
      valid: true,
      ratios: { default: 1, vip: 1.5 },
    })
  })

  test('rejects missing, malformed, and structurally invalid values', () => {
    for (const value of [
      undefined,
      '',
      'not-json',
      '[]',
      'null',
      '{"default":"1"}',
      '{"default":-1}',
      '{"":1}',
    ]) {
      assert.deepEqual(parseGroupRatiosOption(value), {
        valid: false,
        ratios: {},
      })
    }
  })
})
