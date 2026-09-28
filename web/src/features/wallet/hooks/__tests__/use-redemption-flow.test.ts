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
import { act, renderHook } from '@testing-library/react'
import { createElement } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { redeemTopupCode } from '../../api'
import { useRedemptionFlow } from '../use-redemption-flow'

vi.mock('../../api', () => ({ redeemTopupCode: vi.fn() }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

function renderFlow(refreshUser: () => void | Promise<void>) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return renderHook(() => useRedemptionFlow(refreshUser), {
    wrapper: (props) =>
      createElement(
        QueryClientProvider,
        { client: queryClient },
        props.children
      ),
  })
}

const dialogPayload = {
  title: '兑换成功',
  content: '**好评**立即获得1元兑换码',
  close_button_text: '我知道了',
}

beforeEach(() => {
  vi.mocked(redeemTopupCode).mockReset()
})

describe('useRedemptionFlow', () => {
  it('clears the code, opens the dialog and refreshes the balance on success', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('CODE-1'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.code).toBe('')
    expect(hook.result.current.dialog).toEqual({
      title: '兑换成功',
      content: '**好评**立即获得1元兑换码',
      closeButtonText: '我知道了',
    })
    expect(hook.result.current.dialogOpen).toBe(true)
    expect(hook.result.current.quotaAdded).toBe(500)
    expect(refreshUser).toHaveBeenCalledTimes(1)
  })

  it('keeps the success dialog and cleared code when the balance refresh fails', async () => {
    const refreshUser = vi.fn().mockRejectedValue(new Error('refresh failed'))
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('CODE-1'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.dialogOpen).toBe(true)
    expect(hook.result.current.dialog).not.toBeNull()
    expect(hook.result.current.code).toBe('')
  })

  it('keeps the typed code and shows no dialog when the redemption fails', async () => {
    const refreshUser = vi.fn()
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: false,
      message: 'invalid code',
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('BAD-CODE'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.code).toBe('BAD-CODE')
    expect(hook.result.current.dialogOpen).toBe(false)
    expect(hook.result.current.dialog).toBeNull()
    expect(refreshUser).not.toHaveBeenCalled()
  })

  it('refreshes without opening a dialog when none is configured', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(redeemTopupCode).mockResolvedValue({ success: true, data: 100 })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('CODE-2'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.dialogOpen).toBe(false)
    expect(hook.result.current.code).toBe('')
    expect(refreshUser).toHaveBeenCalledTimes(1)
  })

  it('does not reopen the dialog after it is closed and the page re-renders', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('CODE-1'))
    await act(async () => {
      await hook.result.current.submit()
    })
    act(() => hook.result.current.setDialogOpen(false))
    hook.rerender()

    expect(hook.result.current.dialogOpen).toBe(false)
  })

  it('shows the newly configured dialog on the next successful redemption', async () => {
    const refreshUser = vi.fn().mockResolvedValue(undefined)
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('CODE-1'))
    await act(async () => {
      await hook.result.current.submit()
    })
    act(() => hook.result.current.setDialogOpen(false))

    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 300,
      success_dialog: {
        title: '新文案',
        content: '更新后的说明',
        close_button_text: '关闭',
      },
    })
    act(() => hook.result.current.setCode('CODE-2'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.dialogOpen).toBe(true)
    expect(hook.result.current.quotaAdded).toBe(300)
    expect(hook.result.current.dialog).toEqual({
      title: '新文案',
      content: '更新后的说明',
      closeButtonText: '关闭',
    })
  })

  it('does not open a dialog for an expired code even when the dialog is enabled', async () => {
    const refreshUser = vi.fn()
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: false,
      message: '该兑换码已被使用',
    })

    const hook = renderFlow(refreshUser)
    act(() => hook.result.current.setCode('EXPIRED-CODE'))
    await act(async () => {
      await hook.result.current.submit()
    })

    expect(hook.result.current.dialogOpen).toBe(false)
    expect(hook.result.current.dialog).toBeNull()
    expect(refreshUser).not.toHaveBeenCalled()
  })
})
