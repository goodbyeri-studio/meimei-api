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
    assert.equal(url.searchParams.get('homepage'), serverAddress)
    assert.equal(url.searchParams.get('resource'), 'provider')
    assert.equal(url.searchParams.get('enabled'), 'true')
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

  test('normalizes trailing slashes before adding the Codex API path', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'codex',
        name: 'meimei-codex',
        apiKey: 'sk-test',
        serverAddress: `${serverAddress}/`,
      })
    )

    assert.equal(url.searchParams.get('endpoint'), `${serverAddress}/v1`)
    assert.equal(url.searchParams.get('homepage'), serverAddress)
  })

  test('does not duplicate an existing Codex API path', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'codex',
        name: 'meimei-codex',
        apiKey: 'sk-test',
        serverAddress: `${serverAddress}/v1/`,
      })
    )

    assert.equal(url.searchParams.get('endpoint'), `${serverAddress}/v1`)
  })

  test('removes query and fragment data from the server address', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'codex',
        name: 'meimei-codex',
        apiKey: 'sk-test',
        serverAddress: `${serverAddress}/relay/?source=console#settings`,
      })
    )

    assert.equal(url.searchParams.get('endpoint'), `${serverAddress}/relay/v1`)
    assert.equal(url.searchParams.get('homepage'), `${serverAddress}/relay`)
  })

  test('encodes provider names and API keys without changing their values', () => {
    const url = new URL(
      buildCCSwitchURL({
        app: 'claude',
        name: '美美 & Claude',
        apiKey: 'sk-a+b&c%20',
        serverAddress,
      })
    )

    assert.equal(url.searchParams.get('name'), '美美 & Claude')
    assert.equal(url.searchParams.get('apiKey'), 'sk-a+b&c%20')
  })

  test('rejects non-HTTP server addresses', () => {
    assert.throws(
      () =>
        buildCCSwitchURL({
          app: 'codex',
          name: 'meimei-codex',
          apiKey: 'sk-test',
          serverAddress: 'javascript:alert(1)',
        }),
      /HTTP or HTTPS/
    )
  })
})
