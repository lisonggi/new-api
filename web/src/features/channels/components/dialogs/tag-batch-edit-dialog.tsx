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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { editTagChannels, getGroups } from '../../api'
import { FIELD_DESCRIPTIONS, FIELD_PLACEHOLDERS } from '../../constants'
import {
  HTTP_PROTOCOL_AUTO,
  HTTP_PROTOCOL_HTTP1,
  channelsQueryKeys,
  type HttpProtocolValue,
} from '../../lib'
import type { TagOperationParams } from '../../types'
import {
  HttpProtocolSelect,
  HttpShardsSelect,
} from '../channel-transport-fields'
import { useChannels } from '../channels-provider'
import { ModelMappingEditor } from '../model-mapping-editor'

type TagBatchEditDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type ProxyMode = 'set' | 'clear'

// Each attribute can only be sent when its scope switch is turned on. This is
// the single source of truth for "should this field be written", replacing the
// old implicit "empty string means keep" convention that silently overwrote
// every tagged channel's model list.
type ScopeKey =
  | 'tag'
  | 'models'
  | 'modelMapping'
  | 'groups'
  | 'httpProtocol'
  | 'shards'
  | 'proxy'

const EMPTY_SCOPE: Record<ScopeKey, boolean> = {
  tag: false,
  models: false,
  modelMapping: false,
  groups: false,
  httpProtocol: false,
  shards: false,
  proxy: false,
}

export function TagBatchEditDialog(props: TagBatchEditDialogProps) {
  const { t } = useTranslation()
  const { currentTag } = useChannels()
  const queryClient = useQueryClient()
  const [isSaving, setIsSaving] = useState(false)

  const [newTag, setNewTag] = useState('')
  const [models, setModels] = useState('')
  const [modelMapping, setModelMapping] = useState('')
  const [groups, setGroups] = useState<string[]>([])
  const [httpProtocol, setHttpProtocol] =
    useState<HttpProtocolValue>(HTTP_PROTOCOL_AUTO)
  const [shards, setShards] = useState('1')
  const [proxyMode, setProxyMode] = useState<ProxyMode>('set')
  const [proxyAddress, setProxyAddress] = useState('')

  const [scope, setScope] = useState<Record<ScopeKey, boolean>>({
    ...EMPTY_SCOPE,
  })

  const { data: groupsData, isLoading: isLoadingGroups } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })

  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    const allGroups = new Set([...groupsData.data, ...groups])
    return [...allGroups].map((group) => ({
      value: group,
      label: group,
    }))
  }, [groupsData, groups])

  useEffect(() => {
    if (props.open && currentTag) {
      setNewTag(currentTag)
    }
  }, [props.open, currentTag])

  const setScopeValue = (key: ScopeKey, value: boolean) => {
    setScope((current) => ({ ...current, [key]: value }))
  }

  const handleProtocolChange = (value: HttpProtocolValue) => {
    setHttpProtocol(value)
    if (value === HTTP_PROTOCOL_HTTP1) {
      // HTTP/1.1 always uses a single connection shard.
      setShards('1')
    }
  }

  const handleShardsChange = (value: string) => {
    setShards(value)
    if (Number(value) > 1) {
      // More than one shard only means anything for HTTP/2, so it lifts an
      // HTTP/1.1 pin instead of storing a contradictory combination.
      setHttpProtocol(HTTP_PROTOCOL_AUTO)
    }
  }

  const handleSave = async () => {
    if (!currentTag) return

    if (scope.models && !models.trim()) {
      toast.error(t('Model list is required'))
      return
    }
    if (scope.modelMapping && !modelMapping.trim()) {
      toast.error(t('Model mapping is required'))
      return
    }
    if (scope.modelMapping) {
      try {
        JSON.parse(modelMapping)
      } catch {
        toast.error(t('Model mapping must be valid JSON'))
        return
      }
    }
    if (scope.groups && groups.length === 0) {
      toast.error(t('Select at least one group'))
      return
    }
    if (scope.proxy && proxyMode === 'set' && !proxyAddress.trim()) {
      toast.error(t('Proxy address is required'))
      return
    }

    setIsSaving(true)
    try {
      const params: Record<string, string | number | undefined> = {
        tag: currentTag,
      }

      if (scope.tag && newTag !== currentTag) {
        params.new_tag = newTag
      }
      if (scope.models) {
        params.models = models.trim()
      }
      if (scope.modelMapping) {
        params.model_mapping = modelMapping.trim()
      }
      if (scope.groups) {
        params.groups = groups.join(',')
      }
      if (scope.httpProtocol) {
        params.http_protocol = httpProtocol
      }
      if (scope.shards) {
        params.http2_connection_shards = Number(shards)
      }
      if (scope.proxy) {
        params.proxy = proxyMode === 'clear' ? '' : proxyAddress.trim()
      }

      // Nothing but the tag selector was chosen: there is nothing to write.
      if (Object.keys(params).length === 1) {
        toast.warning(t('No changes made'))
        return
      }

      const response = await editTagChannels(
        params as unknown as TagOperationParams
      )
      if (response.success) {
        toast.success(t('Tag updated successfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        handleClose()
      } else {
        handleServerError(response, t('Failed to update tag'))
      }
    } catch (error: unknown) {
      handleServerError(error, t('Failed to update tag'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleClose = () => {
    setNewTag('')
    setModels('')
    setModelMapping('')
    setGroups([])
    setHttpProtocol(HTTP_PROTOCOL_AUTO)
    setShards('1')
    setProxyMode('set')
    setProxyAddress('')
    setScope({ ...EMPTY_SCOPE })
    props.onOpenChange(false)
  }

  if (!currentTag) return null

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleClose}
      title={t('Batch Edit by Tag')}
      description={
        <>
          {t('Edit all channels with tag:')}
          <strong>{currentTag}</strong>
        </>
      }
      contentClassName='max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={handleClose} disabled={isSaving}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave} disabled={isSaving}>
            {isSaving ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : null}
            {isSaving ? t('Saving...') : t('Save Changes')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <Alert>
          <AlertCircle className='h-4 w-4' />
          <AlertDescription>
            {t(
              "Only the attributes you turn on are changed. Everything else keeps each channel's current value."
            )}
          </AlertDescription>
        </Alert>

        {/* Tag Name */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='tag-name-scope'
            checked={scope.tag}
            onCheckedChange={(value) => setScopeValue('tag', value)}
            label={t('Tag Name')}
            disabled={isSaving}
          />
          <Input
            aria-label={t('Tag Name')}
            placeholder={t('Enter new tag name (leave empty to disband tag)')}
            value={newTag}
            onChange={(e) => setNewTag(e.target.value)}
            disabled={isSaving || !scope.tag}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave empty to disband the tag')}
          </p>
        </div>

        {/* Models */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='models-scope'
            checked={scope.models}
            onCheckedChange={(value) => setScopeValue('models', value)}
            label={t('Models')}
            disabled={isSaving}
          />
          <Textarea
            aria-label={t('Models')}
            placeholder={t('Comma-separated model names')}
            value={models}
            onChange={(e) => setModels(e.target.value)}
            disabled={isSaving || !scope.models}
            rows={3}
          />
        </div>

        {/* Model Mapping */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='model-mapping-scope'
            checked={scope.modelMapping}
            onCheckedChange={(value) => setScopeValue('modelMapping', value)}
            label={t('Model Mapping')}
            disabled={isSaving}
          />
          <ModelMappingEditor
            value={modelMapping}
            onChange={setModelMapping}
            disabled={isSaving || !scope.modelMapping}
          />
        </div>

        {/* Groups */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='groups-scope'
            checked={scope.groups}
            onCheckedChange={(value) => setScopeValue('groups', value)}
            label={t('Groups')}
            disabled={isSaving}
          />
          {isLoadingGroups ? (
            <Skeleton className='h-10 w-full' />
          ) : (
            <MultiSelect
              options={groupOptions}
              selected={groups}
              onChange={setGroups}
              placeholder={t('Select groups (leave empty to keep current)')}
              disabled={isSaving || !scope.groups}
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
            checked={scope.httpProtocol}
            onCheckedChange={(value) => setScopeValue('httpProtocol', value)}
            label={t('HTTP Protocol')}
            disabled={isSaving}
          />
          <HttpProtocolSelect
            id='http-protocol'
            aria-label={t('HTTP Protocol')}
            className='w-full'
            value={httpProtocol}
            onValueChange={(value) =>
              handleProtocolChange(value as HttpProtocolValue)
            }
            disabled={isSaving || !scope.httpProtocol}
          />
          <p className='text-muted-foreground text-xs'>
            {t(FIELD_DESCRIPTIONS.HTTP_PROTOCOL)}
          </p>
        </div>

        {/* HTTP/2 Connection Shards */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='http2-connection-shards-scope'
            checked={scope.shards}
            onCheckedChange={(value) => setScopeValue('shards', value)}
            label={t('HTTP/2 Connection Shards')}
            disabled={isSaving}
          />
          <HttpShardsSelect
            id='http2-connection-shards'
            aria-label={t('HTTP/2 Connection Shards')}
            className='w-full'
            value={shards}
            onValueChange={handleShardsChange}
            disabled={
              isSaving || !scope.shards || httpProtocol === HTTP_PROTOCOL_HTTP1
            }
          />
          <p className='text-muted-foreground text-xs'>
            {httpProtocol === HTTP_PROTOCOL_HTTP1
              ? t(FIELD_DESCRIPTIONS.HTTP2_CONNECTION_SHARDS_HTTP1)
              : t(FIELD_DESCRIPTIONS.HTTP2_CONNECTION_SHARDS)}
          </p>
        </div>

        {/* Proxy */}
        <div className='space-y-2'>
          <SettingsSwitchField
            controlId='proxy-scope'
            checked={scope.proxy}
            onCheckedChange={(value) => setScopeValue('proxy', value)}
            label={t('Proxy Address')}
            disabled={isSaving}
          />
          <Select
            items={[
              { value: 'set', label: t('Set') },
              { value: 'clear', label: t('Clear') },
            ]}
            value={proxyMode}
            onValueChange={(value) => setProxyMode(value as ProxyMode)}
            disabled={isSaving || !scope.proxy}
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
          {proxyMode === 'set' ? (
            <Input
              aria-label={t('Proxy Address')}
              placeholder={t(FIELD_PLACEHOLDERS.PROXY)}
              value={proxyAddress}
              onChange={(e) => setProxyAddress(e.target.value)}
              disabled={isSaving || !scope.proxy}
            />
          ) : null}
          <p className='text-muted-foreground text-xs'>
            {t(FIELD_DESCRIPTIONS.PROXY)}
          </p>
        </div>
      </div>
    </Dialog>
  )
}
