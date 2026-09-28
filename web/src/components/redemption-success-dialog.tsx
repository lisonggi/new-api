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
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'

export type RedemptionSuccessDialogContent = {
  title: string
  content: string
  closeButtonText: string
}

type RedemptionSuccessDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  dialog: RedemptionSuccessDialogContent | null
  description?: React.ReactNode
  preview?: boolean
}

/**
 * Shared redemption-success dialog. The wallet renders it after a successful
 * redemption; the admin settings section renders the same component as a
 * client-side preview so both paths share one Markdown implementation.
 */
export function RedemptionSuccessDialog(props: RedemptionSuccessDialogProps) {
  const { t } = useTranslation()

  if (!props.dialog) {
    return null
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.dialog.title || t('Redemption successful')}
      description={props.description}
      contentClassName='sm:max-w-xl'
      footer={
        <Button onClick={() => props.onOpenChange(false)}>
          {props.dialog.closeButtonText || t('Close')}
        </Button>
      }
    >
      <RichContent
        mode='markdown'
        breaks
        content={props.dialog.content}
        className='text-sm'
      />
      {props.preview ? (
        <p className='text-muted-foreground mt-3 text-xs'>
          {t('Preview only. It does not save the draft or redeem a code.')}
        </p>
      ) : null}
    </Dialog>
  )
}
