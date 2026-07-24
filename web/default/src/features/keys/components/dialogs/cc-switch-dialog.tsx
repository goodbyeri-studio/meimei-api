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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

import {
  buildCCSwitchURL,
  CC_SWITCH_APP_CONFIGS,
  type CCSwitchApp,
} from '../../lib/cc-switch'

function getServerAddress(): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw)
      if (status.server_address) return status.server_address
    }
  } catch {
    /* empty */
  }
  return window.location.origin
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  tokenKey: string
}

type LaunchState = 'idle' | 'requested' | 'invalid-server'

export function CCSwitchDialog(props: Props) {
  const { t } = useTranslation()
  const [app, setApp] = useState<CCSwitchApp>('codex')
  const [name, setName] = useState<string>(
    CC_SWITCH_APP_CONFIGS.codex.defaultName
  )
  const [launchState, setLaunchState] = useState<LaunchState>('idle')

  useEffect(() => {
    if (props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setApp('codex')
      setName(CC_SWITCH_APP_CONFIGS.codex.defaultName)
      setLaunchState('idle')
    }
  }, [props.open])

  const currentConfig = CC_SWITCH_APP_CONFIGS[app]

  const handleAppChange = (val: string) => {
    const appVal = val as CCSwitchApp
    setApp(appVal)
    setName(CC_SWITCH_APP_CONFIGS[appVal].defaultName)
    setLaunchState('idle')
  }

  const handleSubmit = () => {
    let url: string
    try {
      const key = props.tokenKey.startsWith('sk-')
        ? props.tokenKey
        : `sk-${props.tokenKey}`
      const providerName = name.trim() || currentConfig.defaultName
      url = buildCCSwitchURL({
        app,
        name: providerName,
        apiKey: key,
        serverAddress: getServerAddress(),
      })
    } catch {
      setLaunchState('invalid-server')
      return
    }

    setLaunchState('requested')
    try {
      window.location.assign(url)
    } catch {
      // Do not log the deep link because it contains the API key.
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Import to CC Switch')}
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {launchState === 'idle' ? t('Cancel') : t('Close')}
          </Button>
          <Button onClick={handleSubmit}>
            {launchState === 'requested' ? t('Retry') : t('Open CC Switch')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <FieldGroup className='gap-4'>
          <FieldSet>
            <FieldLegend variant='label'>{t('Application')}</FieldLegend>
            <RadioGroup
              value={app}
              onValueChange={handleAppChange}
              className='flex gap-4'
            >
              {(
                Object.entries(CC_SWITCH_APP_CONFIGS) as [
                  CCSwitchApp,
                  (typeof CC_SWITCH_APP_CONFIGS)[CCSwitchApp],
                ][]
              ).map(([key, cfg]) => (
                <div key={key} className='flex items-center gap-2'>
                  <RadioGroupItem value={key} id={`app-${key}`} />
                  <Label htmlFor={`app-${key}`} className='cursor-pointer'>
                    {cfg.label}
                  </Label>
                </div>
              ))}
            </RadioGroup>
          </FieldSet>

          <Field>
            <FieldLabel htmlFor='cc-switch-name'>{t('Name')}</FieldLabel>
            <Input
              id='cc-switch-name'
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={currentConfig.defaultName}
            />
          </Field>
        </FieldGroup>

        {launchState === 'requested' && (
          <Alert>
            <AlertTitle>{t('Tried to open CC Switch')}</AlertTitle>
            <AlertDescription>
              {t(
                'If CC Switch did not open, make sure it is installed and try again.'
              )}{' '}
              <a href='https://ccswitch.io' target='_blank' rel='noreferrer'>
                {t('Download CC Switch')}
              </a>
            </AlertDescription>
          </Alert>
        )}

        {launchState === 'invalid-server' && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Error')}</AlertTitle>
            <AlertDescription>
              {t(
                'Unable to open CC Switch because the server address is invalid. Please contact your administrator.'
              )}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </Dialog>
  )
}
