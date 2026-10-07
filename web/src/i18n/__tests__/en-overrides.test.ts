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
import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import enOverrides from '@/i18n/locales/en-overrides.json'

/**
 * Only English is bundled in the entry chunk, and only the entries whose value
 * differs from their key carry information: i18next returns the key itself when
 * a translation is missing, so the identity entries are pure bundle weight
 * (~500 KB of the entry chunk for the 69 entries that say something).
 *
 * Regenerate with `node scripts/generate-en-overrides.mjs` after editing
 * en.json. This suite is what catches the two files drifting apart.
 */
describe('bundled English overrides', () => {
  const source = (en as { translation: Record<string, string> }).translation
  const bundled = (enOverrides as { translation: Record<string, string> })
    .translation

  it('contains exactly the entries whose value differs from their key', () => {
    const expected = Object.fromEntries(
      Object.entries(source).filter(([key, value]) => value !== key)
    )

    expect(bundled).toEqual(expected)
  })

  it('renders a dropped identity entry as its own key', async () => {
    const identityKey = Object.keys(source).find(
      (key) => source[key] === key && !key.includes('.')
    )
    expect(identityKey).toBeTruthy()
    expect(bundled).not.toHaveProperty(identityKey as string)

    const { default: i18n, i18nReady } = await import('@/i18n/config')
    await i18nReady
    await i18n.changeLanguage('en')

    expect(i18n.t(identityKey as string)).toBe(identityKey)
  })
})
