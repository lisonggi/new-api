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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useEffect } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import {
  editTagChannels,
  getAllModels,
  getGroups,
  getTagModels,
} from '../../api'
import { ChannelsProvider, useChannels } from '../channels-provider'
import { TagBatchEditDialog } from '../dialogs/tag-batch-edit-dialog'

vi.mock('../../api', () => ({
  editTagChannels: vi.fn(),
  getAllModels: vi.fn(),
  getGroups: vi.fn(),
  getTagModels: vi.fn(),
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
  vi.mocked(getTagModels).mockResolvedValue({ success: true, data: 'gpt-4o' })
  vi.mocked(getAllModels).mockResolvedValue({ success: true, data: [] })
  vi.mocked(getGroups).mockResolvedValue({ success: true, data: ['default'] })
  vi.mocked(editTagChannels).mockResolvedValue({ success: true })
})

test('a proxy address and HTTP/1.1 protocol are sent as a settings patch for the tag', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  const protocol = screen.getByRole('combobox', { name: 'HTTP Protocol' })
  expect(protocol).toHaveTextContent('Keep unchanged')
  await user.click(protocol)
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))
  expect(protocol).toHaveTextContent('HTTP/1.1')

  const proxyMode = screen.getByRole('combobox', { name: 'Proxy Address' })
  expect(
    screen.queryByRole('textbox', { name: 'Proxy Address' })
  ).not.toBeInTheDocument()
  await user.click(proxyMode)
  await user.click(screen.getByRole('option', { name: 'Set' }))
  await user.type(
    screen.getByRole('textbox', { name: 'Proxy Address' }),
    'http://proxy.local:8080'
  )

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(editTagChannels).toHaveBeenCalledWith({
    tag: 'surplus-a',
    models: 'gpt-4o',
    http_protocol: 'http1',
    http2_connection_shards: 1,
    proxy: 'http://proxy.local:8080',
  })
})

test('clearing the proxy sends an empty patch value and hides the address input', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('combobox', { name: 'Proxy Address' }))
  await user.click(screen.getByRole('option', { name: 'Clear' }))
  expect(
    screen.queryByRole('textbox', { name: 'Proxy Address' })
  ).not.toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(editTagChannels).toHaveBeenCalledWith({
    tag: 'surplus-a',
    models: 'gpt-4o',
    proxy: '',
  })
})

test('shard-only patches are sent without touching the protocol', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  const shards = screen.getByRole('combobox', {
    name: 'HTTP/2 Connection Shards',
  })
  expect(shards).toHaveTextContent('Keep unchanged')
  await user.click(shards)
  await user.click(screen.getByRole('option', { name: '4' }))
  expect(shards).toHaveTextContent('4')

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(editTagChannels).toHaveBeenCalledWith({
    tag: 'surplus-a',
    models: 'gpt-4o',
    http2_connection_shards: 4,
  })
})

test('choosing HTTP/1.1 forces a single shard and disables the shard selector', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  const shards = screen.getByRole('combobox', {
    name: 'HTTP/2 Connection Shards',
  })
  await user.click(shards)
  await user.click(screen.getByRole('option', { name: '4' }))
  expect(shards).toHaveTextContent('4')

  await user.click(screen.getByRole('combobox', { name: 'HTTP Protocol' }))
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))
  expect(shards).toHaveTextContent('1')
  expect(shards).toBeDisabled()

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(editTagChannels).toHaveBeenCalledWith({
    tag: 'surplus-a',
    models: 'gpt-4o',
    http_protocol: 'http1',
    http2_connection_shards: 1,
  })
})

test('choosing the set-proxy mode without an address blocks the save', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  await user.click(screen.getByRole('combobox', { name: 'Proxy Address' }))
  await user.click(screen.getByRole('option', { name: 'Set' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))

  expect(editTagChannels).not.toHaveBeenCalled()
})

test('reverting HTTP/1.1 back to keep-unchanged drops the implied shard patch', async () => {
  const user = userEvent.setup()
  await openTagDialog()

  const protocol = screen.getByRole('combobox', { name: 'HTTP Protocol' })
  await user.click(protocol)
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))
  expect(protocol).toHaveTextContent('HTTP/1.1')

  // Changing the mind back must not leave the HTTP/1.1 shard patch behind:
  // saving would otherwise rewrite the shards of every tagged channel without
  // the administrator asking for it.
  await user.click(protocol)
  await user.click(screen.getByRole('option', { name: 'Keep unchanged' }))
  expect(protocol).toHaveTextContent('Keep unchanged')

  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(editTagChannels).toHaveBeenCalledWith({
    tag: 'surplus-a',
    models: 'gpt-4o',
  })
})
