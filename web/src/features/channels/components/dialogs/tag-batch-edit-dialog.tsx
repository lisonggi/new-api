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
import { useQueryClient } from '@tanstack/react-query'
import { AlertCircle, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SettingsSwitchField } from '@/features/system-settings/components/settings-form-layout'
import { handleServerError } from '@/lib/handle-server-error'

import { editTagChannels } from '../../api'
import {
  buildChannelAttributeParams,
  channelsQueryKeys,
  emptyChannelAttributeChanges,
  validateChannelAttributeChanges,
  type ChannelAttributeChanges,
} from '../../lib'
import type { TagOperationParams } from '../../types'
import { ChannelAttributeFields } from '../channel-attribute-fields'
import { useChannels } from '../channels-provider'

type TagBatchEditDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TagBatchEditDialog(props: TagBatchEditDialogProps) {
  const { t } = useTranslation()
  const { currentTag } = useChannels()
  const queryClient = useQueryClient()
  const [isSaving, setIsSaving] = useState(false)
  const [tagEnabled, setTagEnabled] = useState(false)
  const [newTag, setNewTag] = useState('')
  const [changes, setChanges] = useState<ChannelAttributeChanges>(() =>
    emptyChannelAttributeChanges()
  )

  useEffect(() => {
    if (props.open && currentTag) {
      setNewTag(currentTag)
    }
  }, [props.open, currentTag])

  const handleSave = async () => {
    if (!currentTag) return

    const errorKey = validateChannelAttributeChanges(changes)
    if (errorKey) {
      toast.error(t(errorKey))
      return
    }

    setIsSaving(true)
    try {
      const params: TagOperationParams = {
        tag: currentTag,
        ...buildChannelAttributeParams(changes),
      }
      if (tagEnabled && newTag !== currentTag) {
        params.new_tag = newTag
      }

      // Nothing but the tag selector was chosen: there is nothing to write.
      if (Object.keys(params).length === 1) {
        toast.warning(t('No changes made'))
        return
      }

      const response = await editTagChannels(params)
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
    setTagEnabled(false)
    setNewTag('')
    setChanges(emptyChannelAttributeChanges())
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
            checked={tagEnabled}
            onCheckedChange={setTagEnabled}
            label={t('Tag Name')}
            disabled={isSaving}
          />
          <Input
            aria-label={t('Tag Name')}
            placeholder={t('Enter new tag name (leave empty to disband tag)')}
            value={newTag}
            onChange={(e) => setNewTag(e.target.value)}
            disabled={isSaving || !tagEnabled}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave empty to disband the tag')}
          </p>
        </div>

        <ChannelAttributeFields
          value={changes}
          onChange={setChanges}
          disabled={isSaving}
        />
      </div>
    </Dialog>
  )
}
