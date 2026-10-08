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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSwitchField } from '@/features/system-settings/components/settings-form-layout'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getGroups } from '../api'
import { FIELD_DESCRIPTIONS, FIELD_PLACEHOLDERS } from '../constants'
import {
  HTTP_PROTOCOL_AUTO,
  HTTP_PROTOCOL_HTTP1,
  type ChannelAttributeChanges,
  type ChannelAttributeProxyMode,
  type HttpProtocolValue,
} from '../lib'
import {
  HttpProtocolSelect,
  HttpShardsSelect,
} from './channel-transport-fields'
import { ModelMappingEditor } from './model-mapping-editor'

type ChannelAttributeFieldsProps = {
  value: ChannelAttributeChanges
  onChange: (next: ChannelAttributeChanges) => void
  disabled?: boolean
}

/**
 * The attribute rows shared by the tag batch edit dialog and the
 * selected-channels batch edit dialog. Each row has an explicit "change this"
 * switch (default off); only a row that is turned on is written by the caller.
 */
export function ChannelAttributeFields(props: ChannelAttributeFieldsProps) {
  const { t } = useTranslation()

  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })

  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    const allGroups = new Set([...groupsData.data, ...props.value.groups.value])
    return [...allGroups].map((group) => ({
      value: group,
      label: group,
    }))
  }, [groupsData, props.value.groups.value])

  const setField = <K extends keyof ChannelAttributeChanges>(
    key: K,
    patch: Partial<ChannelAttributeChanges[K]>
  ) => {
    props.onChange({
      ...props.value,
      [key]: { ...props.value[key], ...patch },
    } as ChannelAttributeChanges)
  }

  const handleProtocolChange = (next: HttpProtocolValue) => {
    const changes: ChannelAttributeChanges = {
      ...props.value,
      httpProtocol: { ...props.value.httpProtocol, value: next },
    }
    if (next === HTTP_PROTOCOL_HTTP1) {
      // HTTP/1.1 always uses a single connection shard.
      changes.shards = { ...changes.shards, value: '1' }
    }
    props.onChange(changes)
  }

  const handleShardsChange = (value: string) => {
    const changes: ChannelAttributeChanges = {
      ...props.value,
      shards: { ...props.value.shards, value },
    }
    if (Number(value) > 1) {
      // More than one shard only means anything for HTTP/2, so it lifts an
      // HTTP/1.1 pin instead of storing a contradictory combination.
      changes.httpProtocol = {
        ...changes.httpProtocol,
        value: HTTP_PROTOCOL_AUTO,
      }
    }
    props.onChange(changes)
  }

  const disabled = props.disabled === true

  return (
    <>
      {/* Models */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='models-scope'
          checked={props.value.models.enabled}
          onCheckedChange={(value) => setField('models', { enabled: value })}
          label={t('Models')}
          disabled={disabled}
        />
        <Textarea
          aria-label={t('Models')}
          placeholder={t('Comma-separated model names')}
          value={props.value.models.value}
          onChange={(e) => setField('models', { value: e.target.value })}
          disabled={disabled || !props.value.models.enabled}
          rows={3}
        />
      </div>

      {/* Model Mapping */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='model-mapping-scope'
          checked={props.value.modelMapping.enabled}
          onCheckedChange={(value) =>
            setField('modelMapping', { enabled: value })
          }
          label={t('Model Mapping')}
          disabled={disabled}
        />
        <ModelMappingEditor
          value={props.value.modelMapping.value}
          onChange={(value) => setField('modelMapping', { value })}
          disabled={disabled || !props.value.modelMapping.enabled}
        />
      </div>

      {/* Groups */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='groups-scope'
          checked={props.value.groups.enabled}
          onCheckedChange={(value) => setField('groups', { enabled: value })}
          label={t('Groups')}
          disabled={disabled}
        />
        {isLoadingGroups ? (
          <Skeleton className='h-10 w-full' />
        ) : (
          <MultiSelect
            options={groupOptions}
            selected={props.value.groups.value}
            onChange={(value) => setField('groups', { value })}
            placeholder={t('Select groups (leave empty to keep current)')}
            disabled={disabled || !props.value.groups.enabled}
          />
        )}
        <p className='text-muted-foreground text-xs'>
          {t('User groups that can access channels with this tag')}
        </p>
      </div>

      {/* HTTP Protocol */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='http-protocol-scope'
          checked={props.value.httpProtocol.enabled}
          onCheckedChange={(value) =>
            setField('httpProtocol', { enabled: value })
          }
          label={t('HTTP Protocol')}
          disabled={disabled}
        />
        <HttpProtocolSelect
          id='http-protocol'
          aria-label={t('HTTP Protocol')}
          className='w-full'
          value={props.value.httpProtocol.value}
          onValueChange={(value) =>
            handleProtocolChange(value as HttpProtocolValue)
          }
          disabled={disabled || !props.value.httpProtocol.enabled}
        />
        <p className='text-muted-foreground text-xs'>
          {t(FIELD_DESCRIPTIONS.HTTP_PROTOCOL)}
        </p>
      </div>

      {/* HTTP/2 Connection Shards */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='http2-connection-shards-scope'
          checked={props.value.shards.enabled}
          onCheckedChange={(value) => setField('shards', { enabled: value })}
          label={t('HTTP/2 Connection Shards')}
          disabled={disabled}
        />
        <HttpShardsSelect
          id='http2-connection-shards'
          aria-label={t('HTTP/2 Connection Shards')}
          className='w-full'
          value={props.value.shards.value}
          onValueChange={handleShardsChange}
          disabled={
            disabled ||
            !props.value.shards.enabled ||
            props.value.httpProtocol.value === HTTP_PROTOCOL_HTTP1
          }
        />
        <p className='text-muted-foreground text-xs'>
          {props.value.httpProtocol.value === HTTP_PROTOCOL_HTTP1
            ? t(FIELD_DESCRIPTIONS.HTTP2_CONNECTION_SHARDS_HTTP1)
            : t(FIELD_DESCRIPTIONS.HTTP2_CONNECTION_SHARDS)}
        </p>
      </div>

      {/* Proxy */}
      <div className='space-y-2'>
        <SettingsSwitchField
          controlId='proxy-scope'
          checked={props.value.proxy.enabled}
          onCheckedChange={(value) => setField('proxy', { enabled: value })}
          label={t('Proxy Address')}
          disabled={disabled}
        />
        <Select
          items={[
            { value: 'set', label: t('Set') },
            { value: 'clear', label: t('Clear') },
          ]}
          value={props.value.proxy.mode}
          onValueChange={(value) =>
            setField('proxy', { mode: value as ChannelAttributeProxyMode })
          }
          disabled={disabled || !props.value.proxy.enabled}
        >
          <SelectTrigger
            id='proxy-mode'
            aria-label={t('Proxy Address')}
            className='w-full'
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value='set'>{t('Set')}</SelectItem>
              <SelectItem value='clear'>{t('Clear')}</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
        {props.value.proxy.mode === 'set' ? (
          <Input
            aria-label={t('Proxy Address')}
            placeholder={t(FIELD_PLACEHOLDERS.PROXY)}
            value={props.value.proxy.address}
            onChange={(e) => setField('proxy', { address: e.target.value })}
            disabled={disabled || !props.value.proxy.enabled}
          />
        ) : null}
        <p className='text-muted-foreground text-xs'>
          {t(FIELD_DESCRIPTIONS.PROXY)}
        </p>
      </div>
    </>
  )
}
