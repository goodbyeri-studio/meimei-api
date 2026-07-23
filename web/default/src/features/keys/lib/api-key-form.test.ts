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

import type { TFunction } from 'i18next'

import { getApiKeyFormSchema } from './api-key-form'

const translate = ((key: string) => key) as TFunction

function validForm(name: string) {
  return {
    name,
    unlimited_quota: true,
    model_limits: [],
    allow_ips: '',
    group: 'default',
    cross_group_retry: false,
  }
}

describe('getApiKeyFormSchema', () => {
  test('counts supplementary Unicode characters consistently with the backend', () => {
    const schema = getApiKeyFormSchema(translate)

    assert.equal(schema.safeParse(validForm('😀'.repeat(50))).success, true)
    assert.equal(schema.safeParse(validForm('😀'.repeat(51))).success, false)
  })
})
