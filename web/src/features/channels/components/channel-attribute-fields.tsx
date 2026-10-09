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

import { JsonCodeEditor } from '@/components/json-code-editor'
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
  type ChannelAttributeMergeMode,
  type ChannelAttributeProxyMode,
  type ChannelAttributeReplaceMode,
  type ChannelBatchSettingsChanges,
  type HttpProtocolValue,
} from '../lib'
import {
  HttpProtocolSelect,
  HttpShardsSelect,
} from './channel-transport-fields'
import { ErrorRetryPolicyEditor } from './error-retry-policy/error-retry-policy-editor'
import { ModelMappingEditor } from './model-mapping-editor'

type ChannelAttributeFieldsProps = {
  value: ChannelAttributeChanges
  onChange: (next: ChannelAttributeChanges) => void
  disabled?: boolean
}

type ModeSelectProps = {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  disabled: boolean
  options: Array<{ value: string; label: string }>
  className?: string
}

function ModeSelect(props: ModeSelectProps) {
  return (
    <Select
      items={props.options}
      value={props.value}
      onValueChange={(value) => props.onChange(String(value))}
      disabled={props.disabled}
    >
      <SelectTrigger
        id={props.id}
        aria-label={props.label}
        className={props.className ?? 'w-36'}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {props.options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

type BoolSettingRowProps = {
  id: string
  label: string
  description?: string
  enabled: boolean
  onEnabledChange: (value: boolean) => void
  value: boolean
  onValueChange: (value: boolean) => void
  disabled: boolean
}

function BoolSettingRow(props: BoolSettingRowProps) {
  const { t } = useTranslation()
  return (
    <div className='space-y-2'>
      <SettingsSwitchField
        controlId={`${props.id}-scope`}
        checked={props.enabled}
        onCheckedChange={props.onEnabledChange}
        label={props.label}
        disabled={props.disabled}
      />
      <Select
        items={[
          { value: 'true', label: t('Enable') },
          { value: 'false', label: t('Disable') },
        ]}
        value={props.value ? 'true' : 'false'}
        onValueChange={(value) => props.onValueChange(value === 'true')}
        disabled={props.disabled || !props.enabled}
      >
        <SelectTrigger
          id={props.id}
          aria-label={props.label}
          className='w-full'
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value='true'>{t('Enable')}</SelectItem>
            <SelectItem value='false'>{t('Disable')}</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
      {props.description ? (
        <p className='text-muted-foreground text-xs'>{props.description}</p>
      ) : null}
    </div>
  )
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

  const setSetting = <K extends keyof ChannelBatchSettingsChanges>(
    key: K,
    patch: Partial<ChannelBatchSettingsChanges[K]>
  ) => {
    props.onChange({
      ...props.value,
      settings: {
        ...props.value.settings,
        [key]: { ...props.value.settings[key], ...patch },
      },
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
  const settings = props.value.settings
  const replaceAppendRemoveOptions = [
    { value: 'replace', label: t('Replace') },
    { value: 'append', label: t('Append') },
    { value: 'remove', label: t('Remove') },
  ]
  const replaceMergeOptions = [
    { value: 'replace', label: t('Replace') },
    { value: 'merge', label: t('Merge') },
  ]
  const replaceMergeRemoveOptions = [
    { value: 'replace', label: t('Replace') },
    { value: 'merge', label: t('Merge') },
    { value: 'remove', label: t('Remove') },
  ]

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
        <div className='flex gap-2'>
          <ModeSelect
            id='models-mode'
            label={t('Models mode')}
            value={props.value.models.mode}
            onChange={(value) =>
              setField('models', { mode: value as ChannelAttributeReplaceMode })
            }
            disabled={disabled || !props.value.models.enabled}
            options={replaceAppendRemoveOptions}
          />
          <Textarea
            aria-label={t('Models')}
            placeholder={t('Comma-separated model names')}
            value={props.value.models.value}
            onChange={(e) => setField('models', { value: e.target.value })}
            disabled={disabled || !props.value.models.enabled}
            rows={3}
            className='flex-1'
          />
        </div>
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
        <ModeSelect
          id='model-mapping-mode'
          label={t('Model mapping mode')}
          value={props.value.modelMapping.mode}
          onChange={(value) =>
            setField('modelMapping', {
              mode: value as ChannelAttributeMergeMode,
            })
          }
          disabled={disabled || !props.value.modelMapping.enabled}
          options={replaceMergeRemoveOptions}
        />
        <ModelMappingEditor
          value={props.value.modelMapping.value}
          onChange={(value) => setField('modelMapping', { value })}
          disabled={disabled || !props.value.modelMapping.enabled}
        />
        {props.value.modelMapping.mode === 'remove' ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Remove mode deletes the listed request model names from each channel’s mapping; the upstream names are ignored.'
            )}
          </p>
        ) : null}
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
        <div className='flex gap-2'>
          <ModeSelect
            id='groups-mode'
            label={t('Groups mode')}
            value={props.value.groups.mode}
            onChange={(value) =>
              setField('groups', { mode: value as ChannelAttributeReplaceMode })
            }
            disabled={disabled || !props.value.groups.enabled}
            options={replaceAppendRemoveOptions}
          />
          {isLoadingGroups ? (
            <Skeleton className='h-10 flex-1' />
          ) : (
            <MultiSelect
              options={groupOptions}
              selected={props.value.groups.value}
              onChange={(value) => setField('groups', { value })}
              placeholder={t('Select groups (leave empty to keep current)')}
              disabled={disabled || !props.value.groups.enabled}
              className='flex-1'
            />
          )}
        </div>
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

      {/* Channel settings */}
      <div className='space-y-4 border-t pt-4'>
        <div className='space-y-1'>
          <h4 className='text-sm font-medium'>{t('Channel Settings')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Provider-agnostic request handling settings. Each one is only written when you turn it on.'
            )}
          </p>
        </div>

        {/* Reasoning content backfill: one feature shared by both protocols */}
        <div className='space-y-3'>
          <div className='space-y-1'>
            <p className='text-sm font-medium'>
              {t('Reasoning content backfill')}
            </p>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Backfill missing reasoning_content on replayed assistant turns for thinking-mode upstreams. Fills only an empty string and is skipped when body passthrough is enabled. Turn on each protocol it should apply to.'
              )}
            </p>
          </div>

          <BoolSettingRow
            id='reasoning-content-backfill'
            label={t('Chat Completions')}
            enabled={settings.reasoningContentBackfill.enabled}
            onEnabledChange={(value) =>
              setSetting('reasoningContentBackfill', { enabled: value })
            }
            value={settings.reasoningContentBackfill.value}
            onValueChange={(value) =>
              setSetting('reasoningContentBackfill', { value })
            }
            disabled={disabled}
          />

          <BoolSettingRow
            id='responses-reasoning-content-backfill'
            label={t('Responses')}
            enabled={settings.responsesReasoningContentBackfill.enabled}
            onEnabledChange={(value) =>
              setSetting('responsesReasoningContentBackfill', {
                enabled: value,
              })
            }
            value={settings.responsesReasoningContentBackfill.value}
            onValueChange={(value) =>
              setSetting('responsesReasoningContentBackfill', { value })
            }
            disabled={disabled}
          />
        </div>

        <BoolSettingRow
          id='assistant-content-backfill'
          label={t('Assistant content backfill')}
          description={t(
            'Backfill missing content on assistant messages that replay no tool call in Chat Completions. Fills only an empty string and is skipped when body passthrough is enabled.'
          )}
          enabled={settings.assistantContentBackfill.enabled}
          onEnabledChange={(value) =>
            setSetting('assistantContentBackfill', { enabled: value })
          }
          value={settings.assistantContentBackfill.value}
          onValueChange={(value) =>
            setSetting('assistantContentBackfill', { value })
          }
          disabled={disabled}
        />

        <BoolSettingRow
          id='ignore-response-model-mismatch'
          label={t('Ignore response model mismatch')}
          description={t(
            'Skip the mismatch warning when the upstream returns a different model name (for example when the upstream model name is a routing ID).'
          )}
          enabled={settings.ignoreResponseModelMismatch.enabled}
          onEnabledChange={(value) =>
            setSetting('ignoreResponseModelMismatch', { enabled: value })
          }
          value={settings.ignoreResponseModelMismatch.value}
          onValueChange={(value) =>
            setSetting('ignoreResponseModelMismatch', { value })
          }
          disabled={disabled}
        />

        <BoolSettingRow
          id='thinking-to-content'
          label={t('Thinking to Content')}
          description={t('Convert reasoning_content to <think> tag in content')}
          enabled={settings.thinkingToContent.enabled}
          onEnabledChange={(value) =>
            setSetting('thinkingToContent', { enabled: value })
          }
          value={settings.thinkingToContent.value}
          onValueChange={(value) => setSetting('thinkingToContent', { value })}
          disabled={disabled}
        />

        <BoolSettingRow
          id='disable-task-polling-sleep'
          label={t('Skip async task polling delay')}
          description={t(
            'Do not wait one second between polling async tasks for this channel'
          )}
          enabled={settings.disableTaskPollingSleep.enabled}
          onEnabledChange={(value) =>
            setSetting('disableTaskPollingSleep', { enabled: value })
          }
          value={settings.disableTaskPollingSleep.value}
          onValueChange={(value) =>
            setSetting('disableTaskPollingSleep', { value })
          }
          disabled={disabled}
        />

        <BoolSettingRow
          id='system-prompt-override'
          label={t('System prompt override')}
          description={t(
            'Replace the system prompt sent by the client instead of appending to it.'
          )}
          enabled={settings.systemPromptOverride.enabled}
          onEnabledChange={(value) =>
            setSetting('systemPromptOverride', { enabled: value })
          }
          value={settings.systemPromptOverride.value}
          onValueChange={(value) =>
            setSetting('systemPromptOverride', { value })
          }
          disabled={disabled}
        />

        {/* System prompt */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='system-prompt-scope'
            checked={settings.systemPrompt.enabled}
            onCheckedChange={(value) =>
              setSetting('systemPrompt', { enabled: value })
            }
            label={t('System prompt')}
            disabled={disabled}
          />
          <Textarea
            aria-label={t('System prompt')}
            placeholder={t('Leave empty to clear the stored system prompt')}
            value={settings.systemPrompt.value}
            onChange={(e) =>
              setSetting('systemPrompt', { value: e.target.value })
            }
            disabled={disabled || !settings.systemPrompt.enabled}
            rows={3}
          />
        </div>

        {/* First response timeout */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='first-response-timeout-scope'
            checked={settings.modelFirstResponseTimeout.enabled}
            onCheckedChange={(value) =>
              setSetting('modelFirstResponseTimeout', { enabled: value })
            }
            label={t('First response timeout')}
            disabled={disabled}
          />
          <ModeSelect
            id='first-response-timeout-mode'
            label={t('First response timeout mode')}
            value={settings.modelFirstResponseTimeout.mode}
            onChange={(value) =>
              setSetting('modelFirstResponseTimeout', {
                mode: value as 'replace' | 'merge',
              })
            }
            disabled={disabled || !settings.modelFirstResponseTimeout.enabled}
            options={replaceMergeOptions}
          />
          <JsonCodeEditor
            value={settings.modelFirstResponseTimeout.value}
            onChange={(value) =>
              setSetting('modelFirstResponseTimeout', { value })
            }
            disabled={disabled || !settings.modelFirstResponseTimeout.enabled}
            ariaLabel={t('First response timeout')}
            placeholder='{"model":[{"context_tokens":200000,"timeout_ms":3000}]}'
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Per-model time-to-first-byte timeout (milliseconds), tiered by prompt context size. Model keys use the requested model name, not the mapped upstream model.'
            )}
          </p>
        </div>

        {/* Error retry judgment */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='error-retry-policy-scope'
            checked={settings.errorRetryPolicy.enabled}
            onCheckedChange={(value) =>
              setSetting('errorRetryPolicy', { enabled: value })
            }
            label={t('Error retry judgment')}
            disabled={disabled}
          />
          <ErrorRetryPolicyEditor
            value={settings.errorRetryPolicy.value}
            onChange={(value) => setSetting('errorRetryPolicy', { value })}
            disabled={disabled || !settings.errorRetryPolicy.enabled}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Classify upstream HTTP errors as retryable or final for this channel. A miss inherits the global decision.'
            )}
          </p>
        </div>
      </div>
    </>
  )
}
