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
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { CommonLogsStats } from '../common-logs-stats'
import { UsageLogsProvider } from '../usage-logs-provider'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  getRouteApi: () => ({ useSearch: () => ({}) }),
}))

let statData: Record<string, unknown> = {}

function renderStats() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <CommonLogsStats />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  statData = { quota: 0, rpm: 0, tpm: 0 }
  vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
    if (url.includes('/stat')) {
      return { data: { success: true, data: statData } }
    }
    return { data: { success: true, data: '' } }
  })
})

test('renders the cache hit rate as a percentage with four decimal places', async () => {
  statData = { quota: 1000, rpm: 5, tpm: 2000, cache_tokens: 1234, prompt_tokens: 10000 }

  renderStats()

  expect(await screen.findByText('12.3400%')).toBeInTheDocument()
  expect(screen.getByText('Cache Hit Rate')).toBeInTheDocument()
})

test('rounds the cache hit rate to four decimal places', async () => {
  statData = { quota: 0, rpm: 0, tpm: 0, cache_tokens: 1, prompt_tokens: 3 }

  renderStats()

  expect(await screen.findByText('33.3333%')).toBeInTheDocument()
})

test('renders 0.0000% when there are no prompt tokens', async () => {
  statData = { quota: 0, rpm: 0, tpm: 0, cache_tokens: 0, prompt_tokens: 0 }

  renderStats()

  expect(await screen.findByText('0.0000%')).toBeInTheDocument()
})
