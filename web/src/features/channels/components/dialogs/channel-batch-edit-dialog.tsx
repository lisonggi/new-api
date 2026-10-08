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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { handleServerError } from '@/lib/handle-server-error'

import { editChannelBatch } from '../../api'
import {
  buildChannelAttributeParams,
  channelsQueryKeys,
  emptyChannelAttributeChanges,
  validateChannelAttributeChanges,
  type ChannelAttributeChanges,
} from '../../lib'
import { ChannelAttributeFields } from '../channel-attribute-fields'

type ChannelBatchEditDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Channel ids captured when the dialog was opened. */
  ids: number[]
  /** Called after a successful save, before the parent clears the selection. */
  onSaved: () => void
}

/**
 * Batch edit the attributes of an explicit set of selected channels. Each
 * attribute is opt-in (default keep unchanged), so an edit only writes what the
 * administrator turned on.
 */
export function ChannelBatchEditDialog(props: ChannelBatchEditDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [isSaving, setIsSaving] = useState(false)
  const [changes, setChanges] = useState<ChannelAttributeChanges>(() =>
    emptyChannelAttributeChanges()
  )

  const handleSave = async () => {
    if (props.ids.length === 0) return

    const errorKey = validateChannelAttributeChanges(changes)
    if (errorKey) {
      toast.error(t(errorKey))
      return
    }

    const attributeParams = buildChannelAttributeParams(changes)
    if (Object.keys(attributeParams).length === 0) {
      toast.warning(t('No changes made'))
      return
    }

    setIsSaving(true)
    try {
      const response = await editChannelBatch({
        ids: props.ids,
        ...attributeParams,
      })
      if (response.success) {
        toast.success(t('Channels updated successfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        handleClose()
        props.onSaved()
      } else {
        handleServerError(response, t('Failed to update channels'))
      }
    } catch (error: unknown) {
      handleServerError(error, t('Failed to update channels'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleClose = () => {
    setChanges(emptyChannelAttributeChanges())
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleClose}
      title={t('Batch Edit Selected Channels')}
      description={
        <>
          {t('Edit the selected channels:')}
          <strong>{props.ids.length}</strong>
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

        <ChannelAttributeFields
          value={changes}
          onChange={setChanges}
          disabled={isSaving}
        />
      </div>
    </Dialog>
  )
}
