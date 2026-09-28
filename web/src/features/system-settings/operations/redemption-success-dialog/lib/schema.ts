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
import type { TFunction } from 'i18next'
import * as z from 'zod'

import { REDEMPTION_DIALOG_LIMITS } from '../types'

// Mirrors the backend `setting/redemption_dialog` validation: rune limits
// always apply, and when the dialog is enabled every visible field must not be
// blank.
export function getRedemptionDialogSchema(
  t: TFunction,
  formatLimit: (value: number) => string
) {
  return z
    .object({
      enabled: z.boolean(),
      title: z.string(),
      content: z.string(),
      close_button_text: z.string(),
    })
    .superRefine((values, ctx) => {
      if ([...values.title].length > REDEMPTION_DIALOG_LIMITS.maxTitleLength) {
        ctx.addIssue({
          code: 'custom',
          path: ['title'],
          message: t('Title is too long (max {{max}} characters)', {
            max: formatLimit(REDEMPTION_DIALOG_LIMITS.maxTitleLength),
          }),
        })
      } else if (values.enabled && values.title.trim() === '') {
        ctx.addIssue({
          code: 'custom',
          path: ['title'],
          message: t('Title is required'),
        })
      }

      if (
        [...values.content].length > REDEMPTION_DIALOG_LIMITS.maxContentLength
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['content'],
          message: t('Content is too long (max {{max}} characters)', {
            max: formatLimit(REDEMPTION_DIALOG_LIMITS.maxContentLength),
          }),
        })
      } else if (values.enabled && values.content.trim() === '') {
        ctx.addIssue({
          code: 'custom',
          path: ['content'],
          message: t('Content is required'),
        })
      }

      if (
        [...values.close_button_text].length >
        REDEMPTION_DIALOG_LIMITS.maxCloseButtonLength
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['close_button_text'],
          message: t('Close button text is too long (max {{max}} characters)', {
            max: formatLimit(REDEMPTION_DIALOG_LIMITS.maxCloseButtonLength),
          }),
        })
      } else if (values.enabled && values.close_button_text.trim() === '') {
        ctx.addIssue({
          code: 'custom',
          path: ['close_button_text'],
          message: t('Close button text is required'),
        })
      }
    })
}

export type RedemptionDialogFormValues = z.infer<
  ReturnType<typeof getRedemptionDialogSchema>
>
