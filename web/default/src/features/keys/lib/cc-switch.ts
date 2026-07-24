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
const CC_SWITCH_APP_CONFIGS = {
  claude: {
    defaultName: 'meimei-claude',
  },
  codex: {
    defaultName: 'meimei-codex',
  },
} as const

export type CCSwitchApp = keyof typeof CC_SWITCH_APP_CONFIGS

type CCSwitchURLParams = {
  app: CCSwitchApp
  apiKey: string
  serverAddress: string
}

export function buildCCSwitchURL(params: CCSwitchURLParams): string {
  const appConfig = CC_SWITCH_APP_CONFIGS[params.app]
  const serverURL = new URL(params.serverAddress.trim())
  if (serverURL.protocol !== 'http:' && serverURL.protocol !== 'https:') {
    throw new TypeError('CC Switch server address must use HTTP or HTTPS')
  }

  serverURL.search = ''
  serverURL.hash = ''
  serverURL.pathname = serverURL.pathname.replace(/\/+$/, '')

  const serverAddress = serverURL.toString().replace(/\/$/, '')
  const endpoint =
    params.app === 'codex' && !serverURL.pathname.endsWith('/v1')
      ? `${serverAddress}/v1`
      : serverAddress
  const searchParams = new URLSearchParams()
  searchParams.set('resource', 'provider')
  searchParams.set('app', params.app)
  searchParams.set('name', appConfig.defaultName)
  searchParams.set('endpoint', endpoint)
  searchParams.set(
    'apiKey',
    params.apiKey.startsWith('sk-') ? params.apiKey : `sk-${params.apiKey}`
  )
  searchParams.set('homepage', serverAddress)
  searchParams.set('enabled', 'true')
  return `ccswitch://v1/import?${searchParams.toString()}`
}
