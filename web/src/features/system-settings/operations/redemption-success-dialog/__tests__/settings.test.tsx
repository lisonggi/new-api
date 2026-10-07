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
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'
import { api } from '@/lib/api'

import { RedemptionSuccessDialogSection } from '..'
import type { RedemptionSuccessDialogConfig } from '../types'

let queryClient: QueryClient
let serverConfig: RedemptionSuccessDialogConfig

function renderSection() {
  function Harness() {
    const [actionsContainer, setActionsContainer] =
      useState<HTMLDivElement | null>(null)
    return (
      <QueryClientProvider client={queryClient}>
        <SettingsPageProvider actionsContainer={actionsContainer}>
          <div ref={setActionsContainer} />
          <RedemptionSuccessDialogSection />
        </SettingsPageProvider>
      </QueryClientProvider>
    )
  }
  return render(<Harness />)
}

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  serverConfig = {
    enabled: true,
    title: '兑换成功',
    content: '**好评**立即获得1元兑换码',
    close_button_text: '我知道了',
  }

  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/redemption_success_dialog') {
      return { data: { success: true, data: structuredClone(serverConfig) } }
    }
    throw new Error(`unexpected GET ${url}`)
  })
  vi.spyOn(api, 'put').mockImplementation(async (_url, body) => {
    const config = structuredClone(body as RedemptionSuccessDialogConfig)
    serverConfig = config
    return { data: { success: true, data: config } }
  })
})

afterEach(() => {
  cleanup()
  queryClient.clear()
})

describe('redemption success dialog settings', () => {
  it('renders the saved dialog fields', async () => {
    renderSection()
    expect(
      await screen.findByRole('switch', {
        name: 'Enable redemption success dialog',
      })
    ).toBeChecked()
    expect(screen.getByLabelText('Dialog title')).toHaveValue('兑换成功')
    expect(screen.getByLabelText('Dialog content (Markdown)')).toHaveValue(
      '**好评**立即获得1元兑换码'
    )
    expect(screen.getByLabelText('Close button text')).toHaveValue('我知道了')
  })

  it('requires every visible field when the dialog is enabled', async () => {
    serverConfig = {
      enabled: false,
      title: '',
      content: '',
      close_button_text: '',
    }
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable redemption success dialog',
      })
    )
    expect(await screen.findByText('Title is required')).toBeVisible()
    expect(screen.getByText('Content is required')).toBeVisible()
    expect(screen.getByText('Close button text is required')).toBeVisible()
    expect(screen.getByLabelText('Dialog title')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('counts the content limit in Unicode code points and blocks over-limit saves', async () => {
    serverConfig = {
      enabled: true,
      title: 'Title',
      content: 'body',
      close_button_text: 'OK',
    }
    renderSection()
    const content = await screen.findByLabelText('Dialog content (Markdown)')
    fireEvent.change(content, { target: { value: '😀'.repeat(4001) } })
    expect(
      await screen.findByText('Content is too long (max 4,000 characters)')
    ).toBeVisible()
    expect(content).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()

    fireEvent.change(content, { target: { value: '😀'.repeat(4000) } })
    await waitFor(() =>
      expect(
        screen.queryByText('Content is too long (max 4,000 characters)')
      ).not.toBeInTheDocument()
    )
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  })

  it('saves the whole config through the dedicated endpoint', async () => {
    renderSection()
    const toggle = await screen.findByRole('switch', {
      name: 'Enable redemption success dialog',
    })
    await userEvent.click(toggle)
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    const [url, body] = vi.mocked(api.put).mock.calls[0]
    expect(url).toBe('/api/option/redemption_success_dialog')
    expect(body).toEqual({
      enabled: false,
      title: '兑换成功',
      content: '**好评**立即获得1元兑换码',
      close_button_text: '我知道了',
    })
  })

  it('previews the draft Markdown in the shared dialog', async () => {
    renderSection()
    await screen.findByRole('switch', {
      name: 'Enable redemption success dialog',
    })
    await userEvent.click(
      screen.getByRole('button', { name: 'Preview dialog' })
    )
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('兑换成功')).toBeVisible()
    // Markdown is loaded on demand, so its content resolves asynchronously.
    expect(await within(dialog).findByText('好评')).toBeVisible()
    expect(
      within(dialog).getByText(
        'Preview only. It does not save the draft or redeem a code.'
      )
    ).toBeVisible()
    expect(
      within(dialog).getByRole('button', { name: '我知道了' })
    ).toBeVisible()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('does not let a background refetch overwrite a dirty draft', async () => {
    renderSection()
    const toggle = await screen.findByRole('switch', {
      name: 'Enable redemption success dialog',
    })
    await userEvent.click(toggle)
    serverConfig = {
      enabled: true,
      title: 'Refetched title',
      content: 'refetched content',
      close_button_text: 'Refetched',
    }
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['redemption-success-dialog'],
      })
    })
    expect(screen.getByLabelText('Dialog title')).toHaveValue('兑换成功')
    expect(toggle).not.toBeChecked()
  })

  it('keeps the draft and stays dirty when the save fails', async () => {
    renderSection()
    const title = await screen.findByLabelText('Dialog title')
    await userEvent.clear(title)
    await userEvent.type(title, 'Unsaved title')
    vi.mocked(api.put).mockImplementationOnce(async () => ({
      data: { success: false, message: 'server rejected', data: serverConfig },
    }))
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))

    expect(screen.getByLabelText('Dialog title')).toHaveValue('Unsaved title')
    expect(screen.getByRole('button', { name: 'Reset' })).toBeEnabled()
  })

  it('keeps edits made while a save is in flight', async () => {
    type PutResult = Awaited<ReturnType<typeof api.put>>
    let resolvePut: (value: PutResult) => void = () => {}
    vi.mocked(api.put).mockImplementationOnce(
      () => new Promise<PutResult>((resolve) => (resolvePut = resolve))
    )
    renderSection()
    const title = await screen.findByLabelText('Dialog title')
    await userEvent.clear(title)
    await userEvent.type(title, 'First')
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))

    await userEvent.type(screen.getByLabelText('Dialog title'), ' edited')
    await act(async () => {
      resolvePut({
        data: { success: true, data: { ...serverConfig, title: 'First' } },
      })
    })

    expect(screen.getByLabelText('Dialog title')).toHaveValue('First edited')
  })

  it.each([false, true])(
    'preserves an edit back to the original title while saving (refetch before PUT response: %s)',
    async (refetchBeforeResponse) => {
      type PutResult = Awaited<ReturnType<typeof api.put>>
      let resolvePut: (value: PutResult) => void = () => {
        throw new Error('PUT not started')
      }
      vi.mocked(api.put).mockImplementationOnce(
        () =>
          new Promise<PutResult>((resolve) => {
            resolvePut = resolve
          })
      )
      renderSection()
      const title = await screen.findByLabelText('Dialog title')
      const originalTitle = serverConfig.title
      fireEvent.change(title, { target: { value: 'Submitted title' } })
      await userEvent.click(
        screen.getByRole('button', { name: 'Save Changes' })
      )
      await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))

      // Editing remains enabled during a save. Returning to the old baseline is
      // still a new unsaved edit relative to the in-flight document.
      fireEvent.change(title, { target: { value: originalTitle } })
      serverConfig = { ...serverConfig, title: 'Submitted title' }
      if (refetchBeforeResponse) {
        // The server can commit before the PUT response reaches the browser.
        // A background GET must not replace edits made after submission.
        await act(async () => {
          await queryClient.invalidateQueries({
            queryKey: ['redemption-success-dialog'],
          })
        })
        // Give the user another interaction while the PUT response is still
        // pending; this keeps GET rendering separate from PUT completion.
        await userEvent.click(title)
      }
      await act(async () => {
        resolvePut({ data: { success: true, data: serverConfig } })
      })
      await waitFor(() =>
        expect(
          screen.getByRole('button', { name: 'Save Changes' })
        ).toBeEnabled()
      )

      expect(screen.getByLabelText('Dialog title')).toHaveValue(originalTitle)
      expect(screen.getByRole('button', { name: 'Reset' })).toBeEnabled()
      await userEvent.click(screen.getByRole('button', { name: 'Reset' }))
      expect(screen.getByLabelText('Dialog title')).toHaveValue(
        'Submitted title'
      )
    }
  )

  it('updates the cached config when the editor unmounts mid-save', async () => {
    type PutResult = Awaited<ReturnType<typeof api.put>>
    let resolvePut: (value: PutResult) => void = () => {}
    vi.mocked(api.put).mockImplementationOnce(
      () => new Promise<PutResult>((resolve) => (resolvePut = resolve))
    )
    const view = renderSection()
    const title = await screen.findByLabelText('Dialog title')
    await userEvent.clear(title)
    await userEvent.type(title, 'Saved later')
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))

    view.unmount()
    const saved = { ...serverConfig, title: 'Saved later' }
    await act(async () => {
      resolvePut({ data: { success: true, data: saved } })
    })

    expect(queryClient.getQueryData(['redemption-success-dialog'])).toEqual(
      saved
    )
  })

  it('resets the draft to the last saved config', async () => {
    renderSection()
    const title = await screen.findByLabelText('Dialog title')
    await userEvent.clear(title)
    await userEvent.type(title, 'Changed')

    await userEvent.click(screen.getByRole('button', { name: 'Reset' }))

    expect(screen.getByLabelText('Dialog title')).toHaveValue('兑换成功')
    expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()
  })
})
