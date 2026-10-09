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
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { DEFAULT_LOGO, DEFAULT_SYSTEM_NAME } from '@/lib/constants'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

const STORAGE_KEY = 'system-config-storage'

/**
 * Branding is fixed at build time. A snapshot persisted by an older session can
 * still hold a previous name/logo, which would flash in the header and favicon
 * before `/api/status` refreshes the store, so rehydration must overwrite it.
 */
const staleSnapshot = {
  state: {
    config: {
      systemName: 'New API',
      logo: 'https://example.com/old.png',
      currency: { ...DEFAULT_CURRENCY_CONFIG },
    },
    loadedLogoUrl: 'https://example.com/old.png',
  },
  version: 0,
}

describe('system config store branding', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  afterEach(() => {
    window.localStorage.clear()
    useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  })

  it('replaces a stale persisted name and logo with the build constants on rehydrate', async () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(staleSnapshot))

    await useSystemConfigStore.persist.rehydrate()

    expect(useSystemConfigStore.getState().config.systemName).toBe(
      DEFAULT_SYSTEM_NAME
    )
    expect(useSystemConfigStore.getState().config.logo).toBe(DEFAULT_LOGO)
  })

  it('does not throw and keeps build branding when the persisted state is malformed', async () => {
    window.localStorage.setItem(STORAGE_KEY, '{not valid json')

    await useSystemConfigStore.persist.rehydrate()

    expect(useSystemConfigStore.getState().config.systemName).toBe(
      DEFAULT_SYSTEM_NAME
    )
    expect(useSystemConfigStore.getState().config.logo).toBe(DEFAULT_LOGO)
  })

  it('resets branding when the persisted config omits the branding fields entirely', async () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          config: { currency: { ...DEFAULT_CURRENCY_CONFIG } },
          loadedLogoUrl: '/old.svg',
        },
        version: 0,
      })
    )

    await useSystemConfigStore.persist.rehydrate()

    expect(useSystemConfigStore.getState().config.systemName).toBe(
      DEFAULT_SYSTEM_NAME
    )
    expect(useSystemConfigStore.getState().config.logo).toBe(DEFAULT_LOGO)
    // A stale loadedLogoUrl must not equal the fresh logo, otherwise the logo
    // would be treated as already loaded and never re-preloaded.
    expect(useSystemConfigStore.getState().loadedLogoUrl).not.toBe(DEFAULT_LOGO)
  })

  it('preserves a persisted currency config through rehydration', async () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        state: {
          config: {
            systemName: 'New API',
            logo: '/old.svg',
            currency: {
              ...DEFAULT_CURRENCY_CONFIG,
              quotaDisplayType: 'CNY',
              usdExchangeRate: 7.2,
            },
          },
          loadedLogoUrl: '/old.svg',
        },
        version: 0,
      })
    )

    await useSystemConfigStore.persist.rehydrate()

    expect(useSystemConfigStore.getState().config.currency.quotaDisplayType).toBe(
      'CNY'
    )
    expect(
      useSystemConfigStore.getState().config.currency.usdExchangeRate
    ).toBe(7.2)
  })
})
