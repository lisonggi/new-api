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
import { useMutation } from '@tanstack/react-query'
import i18next from 'i18next'
import { useCallback, useRef } from 'react'
import { toast } from 'sonner'

import { formatQuota } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import { redeemTopupCode } from '../api'
import type { RedemptionResult } from '../types'

// ============================================================================
// Redemption Hook
// ============================================================================

export function useRedemption() {
  const inFlightRef = useRef(false)

  const mutation = useMutation({
    mutationFn: async (code: string): Promise<RedemptionResult> => {
      const response = await redeemTopupCode({ key: code })

      // Success is decided by the redemption response alone. The balance
      // refresh is a separate, later step in the page, so a failed refresh
      // can never turn a successful redemption into a failure.
      if (response.success === true && response.data !== undefined) {
        const dialog = response.success_dialog ?? null
        // When the dialog is enabled it replaces the success toast, so the
        // user never sees two success notices at once.
        if (!dialog) {
          toast.success(
            i18next.t('Redemption successful! Added: {{quota}}', {
              quota: formatQuota(response.data),
            })
          )
        }
        return { success: true, quotaAdded: response.data, dialog }
      }

      handleServerError(response, i18next.t('Redemption failed'))
      return { success: false }
    },
    // A retried redemption could replay a consumed code, so never retry it.
    retry: false,
    // The catch below owns the error toast; the query client must not add one.
    meta: { errorToast: false },
  })

  const redeemCode = useCallback(
    async (code: string): Promise<RedemptionResult> => {
      if (!code || code.trim() === '') {
        toast.error(i18next.t('Please enter a redemption code'))
        return { success: false }
      }

      // Synchronous guard: a second submit before React re-renders must not
      // issue another redemption request. `isPending` only updates on render.
      if (inFlightRef.current) {
        return { success: false }
      }
      inFlightRef.current = true
      try {
        return await mutation.mutateAsync(code)
      } catch (error) {
        handleServerError(error, i18next.t('Redemption failed'))
        return { success: false }
      } finally {
        inFlightRef.current = false
      }
    },
    [mutation]
  )

  return {
    redeeming: mutation.isPending,
    redeemCode,
  }
}
