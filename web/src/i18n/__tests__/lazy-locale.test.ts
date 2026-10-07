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

import i18n, { i18nReady } from '@/i18n/config'

/**
 * Only English is bundled in the entry chunk; every other locale is a separate
 * chunk served through an i18next backend.
 *
 * Regression: `changeLanguage` must finish loading that chunk before it
 * resolves. The previous hand-rolled loader keyed off `i18n.resolvedLanguage`,
 * which is always `en` while only English is bundled, so the bundle was never
 * requested and every non-English visitor got an English UI.
 */
describe('on-demand locale loading', () => {
  const ON_DEMAND_LOCALES = ['zhCN', 'zhTW', 'fr', 'ru', 'ja', 'vi']

  it('has the bundled English namespace loaded once i18nReady resolves', async () => {
    await i18nReady

    expect(i18n.isInitialized).toBe(true)
    expect(i18n.hasResourceBundle(i18n.language, 'translation')).toBe(true)
  })

  it('loads the Chinese chunk when the language switches to zhCN', async () => {
    await i18n.changeLanguage('zhCN')

    expect(i18n.hasResourceBundle('zhCN', 'translation')).toBe(true)
    expect(i18n.getResource('zhCN', 'translation', 'Save')).toBe('保存')
  })

  it('loads the Traditional Chinese chunk for zhTW instead of the Simplified one', async () => {
    const zhTW = (await import('@/i18n/locales/zh-TW.json')).default
      .translation as Record<string, string>

    await i18n.changeLanguage('zhTW')

    expect(i18n.hasResourceBundle('zhTW', 'translation')).toBe(true)
    expect(i18n.getResource('zhTW', 'translation', 'Save')).toBe(zhTW['Save'])
  })

  it('loads a chunk for every remaining on-demand locale', async () => {
    for (const lng of ON_DEMAND_LOCALES) {
      await i18n.changeLanguage(lng)

      expect(i18n.hasResourceBundle(lng, 'translation')).toBe(true)
    }
  })

  it('resolves without fetching a chunk for English', async () => {
    await i18n.changeLanguage('en')

    expect(i18n.hasResourceBundle('en', 'translation')).toBe(true)
  })
})
