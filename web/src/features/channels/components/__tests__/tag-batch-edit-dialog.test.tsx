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
import { useEffect } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import { editTagChannels, getGroups } from '../../api'
import { ChannelsProvider, useChannels } from '../channels-provider'
import { TagBatchEditDialog } from '../dialogs/tag-batch-edit-dialog'

vi.mock('../../api', () => ({
  editTagChannels: vi.fn(),
  getGroups: vi.fn(),
}))

async function openTagDialog() {
  function Harness() {
    const { setCurrentTag } = useChannels()
    useEffect(() => {
      setCurrentTag('surplus-a')
    }, [setCurrentTag])
    return <TagBatchEditDialog open onOpenChange={vi.fn()} />
  }
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <Harness />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  await screen.findByRole('button', { name: 'Save Changes' })
}

beforeEach(() => {
  vi.mocked(getGroups).mockResolvedValue({ success: true, data: ['default'] })
  vi.mocked(editTagChannels).mockResolvedValue({ success: true })
})

test('saving without turning on any attribute sends no request', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editTagChannels).not.toHaveBeenCalled()
})

test('turning on the proxy sends only the proxy and never the model list', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Proxy Address' }))
  await user.type(
    screen.getByRole('textbox', { name: 'Proxy Address' }),
    'http://proxy.local:8080'
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      proxy: 'http://proxy.local:8080',
    })
  )
  const payload = vi.mocked(editTagChannels).mock
    .calls[0][0] as unknown as Record<string, unknown>
  expect(payload).not.toHaveProperty('models')
  expect(payload).not.toHaveProperty('model_mapping')
})

test('turning on the model list sends the list and enables its input', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  const models = screen.getByRole('textbox', { name: 'Models' })
  expect(models).toBeDisabled()

  await user.click(screen.getByRole('switch', { name: 'Models' }))
  expect(models).toBeEnabled()
  await user.type(models, 'gpt-4o,claude-3')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      models: 'gpt-4o,claude-3',
    })
  )
})

test('enabling the model list without a value blocks the save', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Models' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editTagChannels).not.toHaveBeenCalled()
})

test('enabling the proxy without an address blocks the save', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Proxy Address' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editTagChannels).not.toHaveBeenCalled()
})

test('clearing the proxy sends an empty proxy value', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Proxy Address' }))
  await user.click(screen.getByRole('combobox', { name: 'Proxy Address' }))
  await user.click(screen.getByRole('option', { name: 'Clear' }))
  expect(
    screen.queryByRole('textbox', { name: 'Proxy Address' })
  ).not.toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      proxy: '',
    })
  )
})

test('enabling HTTP/1.1 sends the protocol without a shard patch', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'HTTP Protocol' }))
  await user.click(screen.getByRole('combobox', { name: 'HTTP Protocol' }))
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      http_protocol: 'http1',
    })
  )
})

test('enabling shards sends only the shard patch', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(
    screen.getByRole('switch', { name: 'HTTP/2 Connection Shards' })
  )
  await user.click(
    screen.getByRole('combobox', { name: 'HTTP/2 Connection Shards' })
  )
  await user.click(screen.getByRole('option', { name: '4' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      http2_connection_shards: 4,
    })
  )
})

test('HTTP/1.1 forces a single shard when the shard scope is enabled', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(
    screen.getByRole('switch', { name: 'HTTP/2 Connection Shards' })
  )
  const shards = screen.getByRole('combobox', {
    name: 'HTTP/2 Connection Shards',
  })
  await user.click(shards)
  await user.click(screen.getByRole('option', { name: '4' }))
  expect(shards).toHaveTextContent('4')

  await user.click(screen.getByRole('switch', { name: 'HTTP Protocol' }))
  await user.click(screen.getByRole('combobox', { name: 'HTTP Protocol' }))
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))
  expect(shards).toHaveTextContent('1')
  expect(shards).toBeDisabled()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      http_protocol: 'http1',
      http2_connection_shards: 1,
    })
  )
})

test('renaming the tag sends the new tag name', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Tag Name' }))
  const name = screen.getByRole('textbox', { name: 'Tag Name' })
  await user.clear(name)
  await user.type(name, 'surplus-b')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      new_tag: 'surplus-b',
    })
  )
})

test('clearing the tag name disbands the tag', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('switch', { name: 'Tag Name' }))
  await user.clear(screen.getByRole('textbox', { name: 'Tag Name' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  await waitFor(() =>
    expect(editTagChannels).toHaveBeenCalledWith({
      tag: 'surplus-a',
      new_tag: '',
    })
  )
})
