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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createElement } from 'react'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { handleServerError } from '@/lib/handle-server-error'

import { redeemTopupCode } from '../../api'
import type { RedemptionResult } from '../../types'
import { useRedemption } from '../use-redemption'

vi.mock('../../api', () => ({ redeemTopupCode: vi.fn() }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const dialogPayload = {
  title: '兑换成功',
  content: '**好评**立即获得1元兑换码',
  close_button_text: '我知道了',
}

function renderRedemptionHook() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return renderHook(() => useRedemption(), {
    wrapper: (props) =>
      createElement(
        QueryClientProvider,
        { client: queryClient },
        props.children
      ),
  })
}

async function redeem(code: string): Promise<RedemptionResult> {
  const hook = renderRedemptionHook()
  let result: RedemptionResult | undefined
  await act(async () => {
    result = await hook.result.current.redeemCode(code)
  })
  if (!result) {
    throw new Error('redeemCode did not resolve')
  }
  return result
}

beforeEach(() => {
  vi.mocked(redeemTopupCode).mockReset()
  vi.mocked(handleServerError).mockReset()
})

describe('useRedemption', () => {
  it('ignores a second submission while the first redemption is pending', async () => {
    const hook = renderRedemptionHook()
    let finish: (value: { success: true; data: number }) => void = () => {
      throw new Error('Request not started')
    }
    vi.mocked(redeemTopupCode).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    vi.mocked(redeemTopupCode).mockResolvedValue({ success: false })
    let first: Promise<RedemptionResult>
    let second: Promise<RedemptionResult>
    await act(async () => {
      first = hook.result.current.redeemCode('CODE-1')
      second = hook.result.current.redeemCode('CODE-1')
    })
    await waitFor(() => expect(redeemTopupCode).toHaveBeenCalledTimes(1))
    await act(async () => {
      finish({ success: true, data: 500 })
      await Promise.all([first, second])
    })
  })

  it('returns the success dialog and skips the success toast when enabled', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const result = await redeem('CODE-1')

    expect(result).toEqual({
      success: true,
      quotaAdded: 500,
      dialog: dialogPayload,
    })
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('keeps the success toast when no dialog is configured', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({ success: true, data: 500 })

    const result = await redeem('CODE-2')

    expect(result).toEqual({ success: true, quotaAdded: 500, dialog: null })
    expect(toast.success).toHaveBeenCalledTimes(1)
  })

  it('reports a failure without a dialog when the redemption is rejected', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: false,
      message: 'invalid code',
    })

    const result = await redeem('CODE-3')

    expect(result).toEqual({ success: false })
    expect(toast.success).not.toHaveBeenCalled()
    expect(handleServerError).toHaveBeenCalledTimes(1)
  })

  it('rejects a blank code before calling the API', async () => {
    const result = await redeem('   ')

    expect(result).toEqual({ success: false })
    expect(redeemTopupCode).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledTimes(1)
  })

  it('reports a network rejection once and allows a retry on the same hook', async () => {
    const hook = renderRedemptionHook()
    vi.mocked(redeemTopupCode).mockRejectedValueOnce(new Error('network down'))

    let failed: RedemptionResult | undefined
    await act(async () => {
      failed = await hook.result.current.redeemCode('CODE-4')
    })
    expect(failed).toEqual({ success: false })
    expect(handleServerError).toHaveBeenCalledTimes(1)

    // The in-flight guard must be released after a rejection.
    vi.mocked(redeemTopupCode).mockResolvedValue({ success: true, data: 100 })
    let retried: RedemptionResult | undefined
    await act(async () => {
      retried = await hook.result.current.redeemCode('CODE-4')
    })
    expect(retried).toEqual({ success: true, quotaAdded: 100, dialog: null })
    expect(redeemTopupCode).toHaveBeenCalledTimes(2)
  })
})
