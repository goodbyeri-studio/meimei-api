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

import { buildCCSwitchURL, CC_SWITCH_APP_CONFIGS } from './cc-switch'

const serverAddress = 'https://api.meimei.example'

describe('CC Switch provider URL', () => {
  test('uses the Codex endpoint and default provider name without a model', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'codex',
        name: CC_SWITCH_APP_CONFIGS.codex.defaultName,
        apiKey: 'sk-codex-key',
        serverAddress,
      })
    )

    assert.equal(url.protocol, 'ccswitch:')
    assert.equal(url.searchParams.get('app'), 'codex')
    assert.equal(url.searchParams.get('name'), 'meimei-codex')
    assert.equal(url.searchParams.get('endpoint'), `${serverAddress}/v1`)
    assert.equal(url.searchParams.get('apiKey'), 'sk-codex-key')
    assert.equal(url.searchParams.has('model'), false)
  })

  test('uses the Claude endpoint and default provider name without a model', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'claude',
        name: CC_SWITCH_APP_CONFIGS.claude.defaultName,
        apiKey: 'sk-claude-key',
        serverAddress,
      })
    )

    assert.equal(url.searchParams.get('app'), 'claude')
    assert.equal(url.searchParams.get('name'), 'meimei-claude')
    assert.equal(url.searchParams.get('endpoint'), serverAddress)
    assert.equal(url.searchParams.get('apiKey'), 'sk-claude-key')
    assert.equal(url.searchParams.has('model'), false)
  })
})
