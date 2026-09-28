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
import { useCallback, useRef, useState } from 'react'

import type { RedemptionSuccessDialogContent } from '@/components/redemption-success-dialog'

import { useRedemption } from './use-redemption'

type UseRedemptionFlowResult = {
  code: string
  setCode: (code: string) => void
  redeeming: boolean
  submit: () => Promise<void>
  dialog: RedemptionSuccessDialogContent | null
  dialogOpen: boolean
  setDialogOpen: (open: boolean) => void
  quotaAdded: number
}

/**
 * Wallet redemption flow: submit a code, open the configured dialog on success,
 * then refresh the balance as an independent follow-up. Kept separate from the
 * page so the ordering guarantees are directly testable.
 */
export function useRedemptionFlow(
  refreshUser: () => void | Promise<void>
): UseRedemptionFlowResult {
  const { redeeming, redeemCode } = useRedemption()
  const [code, setCode] = useState('')
  const [dialog, setDialog] = useState<RedemptionSuccessDialogContent | null>(
    null
  )
  const [dialogOpen, setDialogOpen] = useState(false)
  const [quotaAdded, setQuotaAdded] = useState(0)

  // Latest callback without re-creating submit on every page render.
  const refreshRef = useRef(refreshUser)
  refreshRef.current = refreshUser

  const submit = useCallback(async () => {
    if (!code) return

    const result = await redeemCode(code)
    if (!result.success) return

    setCode('')

    // Opening the dialog is driven by this redemption's success response, not
    // by a balance change or a re-render, so a refresh never reopens it.
    if (result.dialog) {
      setQuotaAdded(result.quotaAdded)
      setDialog({
        title: result.dialog.title,
        content: result.dialog.content,
        closeButtonText: result.dialog.close_button_text,
      })
      setDialogOpen(true)
    }

    // Balance refresh is an independent follow-up: a failed refresh must not
    // roll back the successful redemption above.
    try {
      await refreshRef.current()
    } catch {
      // The page owns background refresh errors; redemption already succeeded.
    }
  }, [code, redeemCode])

  return {
    code,
    setCode,
    redeeming,
    submit,
    dialog,
    dialogOpen,
    setDialogOpen,
    quotaAdded,
  }
}
