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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { editChannelBatch, getGroups } from '../../api'
import { ChannelBatchEditDialog } from '../dialogs/channel-batch-edit-dialog'

vi.mock('../../api', () => ({
  editChannelBatch: vi.fn(),
  getGroups: vi.fn(),
}))

async function openDialog(ids: number[] = [1, 2]) {
  const onSaved = vi.fn()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelBatchEditDialog
        open
        onOpenChange={vi.fn()}
        ids={ids}
        onSaved={onSaved}
      />
    </QueryClientProvider>
  )
  await screen.findByRole('button', { name: 'Save Changes' })
  return { onSaved }
}

beforeEach(() => {
  vi.mocked(getGroups).mockResolvedValue({ success: true, data: ['default'] })
  vi.mocked(editChannelBatch).mockResolvedValue({ success: true })
})

test('saving without turning on any attribute sends no request', async () => {
  const user = userEvent.setup()
  await openDialog()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editChannelBatch).not.toHaveBeenCalled()
})

test('turning on the proxy sends the selected ids and only the proxy', async () => {
  const user = userEvent.setup()
  await openDialog([7, 8])

  await user.click(screen.getByRole('switch', { name: 'Proxy Address' }))
  await user.type(
    screen.getByRole('textbox', { name: 'Proxy Address' }),
    'http://p.local:1'
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editChannelBatch).toHaveBeenCalledWith({
      ids: [7, 8],
      proxy: 'http://p.local:1',
    })
  )
  const payload = vi.mocked(editChannelBatch).mock
    .calls[0][0] as unknown as Record<string, unknown>
  expect(payload).not.toHaveProperty('models')
  expect(payload).not.toHaveProperty('settings')
})

test('turning on the model list sends the list for the selected ids', async () => {
  const user = userEvent.setup()
  await openDialog([3])

  const models = screen.getByRole('textbox', { name: 'Models' })
  expect(models).toBeDisabled()
  await user.click(screen.getByRole('switch', { name: 'Models' }))
  await user.type(models, 'gpt-4o')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editChannelBatch).toHaveBeenCalledWith({
      ids: [3],
      models: 'gpt-4o',
    })
  )
})

test('append mode is sent with the model list', async () => {
  const user = userEvent.setup()
  await openDialog([5])

  await user.click(screen.getByRole('switch', { name: 'Models' }))
  await user.click(screen.getByRole('combobox', { name: 'Models mode' }))
  await user.click(screen.getByRole('option', { name: 'Append' }))
  await user.type(screen.getByRole('textbox', { name: 'Models' }), 'gpt-4.1')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editChannelBatch).toHaveBeenCalledWith({
      ids: [5],
      models: 'gpt-4.1',
      models_mode: 'append',
    })
  )
})

test('a channel setting is sent inside settings, never as a column', async () => {
  const user = userEvent.setup()
  await openDialog([9])

  await user.click(screen.getByRole('switch', { name: 'Chat Completions' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editChannelBatch).toHaveBeenCalledWith({
      ids: [9],
      settings: { reasoning_content_backfill: true },
    })
  )
  const payload = vi.mocked(editChannelBatch).mock
    .calls[0][0] as unknown as Record<string, unknown>
  expect(payload).not.toHaveProperty('models')
  expect(payload).not.toHaveProperty('proxy')
})

test('the assistant content backfill setting is sent inside settings', async () => {
  const user = userEvent.setup()
  await openDialog([9])

  await user.click(
    screen.getByRole('switch', { name: 'Assistant content backfill' })
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editChannelBatch).toHaveBeenCalledWith({
      ids: [9],
      settings: { assistant_content_backfill: true },
    })
  )
})

test('enabling an attribute without a value blocks the save', async () => {
  const user = userEvent.setup()
  await openDialog()

  await user.click(screen.getByRole('switch', { name: 'Models' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editChannelBatch).not.toHaveBeenCalled()
})
