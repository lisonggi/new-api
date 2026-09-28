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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, within } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { Home } from '../index'

const statusData = {
  system_name: 'iioooo',
  logo: '',
  announcements_enabled: false,
  demo_site_enabled: false,
  docs_link: '',
  user_agreement_enabled: false,
  privacy_policy_enabled: false,
}

// Admin-configured home page content ('' renders the default minimal landing).
let homePageContent = ''

function renderHome() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: Home }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  homePageContent = ''
  localStorage.clear()
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: null },
  }))
  vi.spyOn(api, 'get').mockImplementation(async (url: string) => {
    if (url === '/api/home_page_content') {
      return { data: { success: true, data: homePageContent } }
    }
    if (url === '/api/status') {
      return { data: { success: true, data: statusData } }
    }
    return { data: { success: true, data: '' } }
  })
})

test('signed-out visitors see the minimal landing with logo, system name, slogan and auth links', async () => {
  renderHome()

  const heading = await screen.findByRole('heading', {
    level: 1,
    name: 'iioooo',
  })
  expect(heading).toBeInTheDocument()

  const main = screen.getByRole('main')

  const logo = within(main).getByRole('img', { name: 'iioooo' })
  expect(logo).toHaveAttribute('src', '/iioooo.logo.svg')

  expect(
    within(main).getByText('Ultra-fast relay, one step ahead')
  ).toBeInTheDocument()

  const signIn = within(main).getByRole('button', { name: 'Sign in' })
  expect(signIn).toHaveAttribute('href', '/sign-in')
  const signUp = within(main).getByRole('button', { name: 'Sign up' })
  expect(signUp).toHaveAttribute('href', '/sign-up')

  expect(
    within(main).queryByRole('button', { name: 'Go to Dashboard' })
  ).not.toBeInTheDocument()
})

test('signed-in users see the dashboard CTA instead of the auth buttons', async () => {
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 1, username: 'song', role: 1 },
    },
  }))

  renderHome()

  await screen.findByRole('heading', { level: 1, name: 'iioooo' })
  const main = screen.getByRole('main')

  const dashboard = within(main).getByRole('button', {
    name: 'Go to Dashboard',
  })
  expect(dashboard).toHaveAttribute('href', '/dashboard')

  expect(
    within(main).queryByRole('button', { name: 'Sign in' })
  ).not.toBeInTheDocument()
  expect(
    within(main).queryByRole('button', { name: 'Sign up' })
  ).not.toBeInTheDocument()
})

test('admin-configured markdown home page content replaces the minimal landing', async () => {
  homePageContent = '# Custom Welcome'

  renderHome()

  expect(
    await screen.findByRole('heading', { level: 1, name: 'Custom Welcome' })
  ).toBeInTheDocument()
  expect(
    screen.queryByText('Ultra-fast relay, one step ahead')
  ).not.toBeInTheDocument()
})
