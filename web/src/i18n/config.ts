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
import i18n, { type BackendModule } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { convertDetectedLanguage } from './languages'
import enOverrides from './locales/en-overrides.json'

export type LocaleResource = { translation: Record<string, string> }

/*
 * Only the English entries that differ from their key are bundled.
 *
 * English source strings are the keys, so `en.json` is almost entirely
 * key-to-itself mappings. i18next already returns the key when a translation is
 * missing, so bundling them added ~500 KB to the entry chunk to carry the 69
 * entries that actually say something (for example
 * `auth.resetPasswordConfirm.confirm` maps to "Confirm reset password").
 * `en` stays the fallback chain, and the overrides are what keep it from
 * rendering a raw key.
 *
 * Regenerate with `node scripts/generate-en-overrides.mjs`.
 *
 * Every other locale is a separate on-demand chunk. Bundling all seven added
 * ~3.9 MB of translations to the entry chunk, so every visitor downloaded six
 * languages they never selected before the first paint.
 */
const LOCALE_LOADERS = {
  zhCN: () => import('./locales/zh.json'),
  fr: () => import('./locales/fr.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
  zhTW: () => import('./locales/zh-TW.json'),
} satisfies Record<string, () => Promise<{ default: LocaleResource }>>

export type OnDemandLocale = keyof typeof LOCALE_LOADERS

function isOnDemandLocale(lng: string): lng is OnDemandLocale {
  return lng in LOCALE_LOADERS
}

/**
 * Resolve the chunk for an i18next language value.
 *
 * The project codes are matched as-is first: `convertDetectedLanguage` only
 * understands BCP-47 tags, so feeding it `zhTW` would map it onto `zhCN` and
 * serve Simplified Chinese to Traditional Chinese visitors. It is therefore
 * only consulted for values i18next never normalised (e.g. `zh-CN`).
 */
function localeLoaderFor(language: string) {
  if (isOnDemandLocale(language)) return LOCALE_LOADERS[language]
  const converted = convertDetectedLanguage(language)
  return isOnDemandLocale(converted) ? LOCALE_LOADERS[converted] : undefined
}

/**
 * Serve the on-demand locale chunks through an i18next backend.
 *
 * i18next awaits this `read` inside `changeLanguage`, so every language switch
 * — the language switcher, the profile preference card, the post-login
 * restore, and the initial detection — loads its own chunk without the caller
 * having to remember to. Loading the bundles by hand instead meant any missed
 * `await` silently left the UI on English, because adding a resource bundle
 * emits `added`, not `languageChanged`, and react-i18next only re-renders on
 * the latter.
 */
const onDemandLocaleBackend: BackendModule = {
  type: 'backend',
  init() {
    /* no backend options to initialise */
  },
  read(language, namespace, callback) {
    // `en` is already bundled, but `partialBundledLanguages` can ask for it.
    if (language === 'en') {
      callback(null, enOverrides.translation)
      return
    }

    const loader = localeLoaderFor(language)
    if (!loader) {
      // No chunk for this language; let i18next fall back to bundled English.
      callback(null, false)
      return
    }

    void (async () => {
      try {
        const bundle = await loader()
        callback(
          null,
          bundle.default[namespace as keyof LocaleResource] ?? false
        )
      } catch (error: unknown) {
        callback(error as Error, false)
      }
    })()
  },
}

/*
 * Awaiting the initialization promise guarantees the detected locale chunk is
 * registered before the first render, otherwise non-English visitors see one
 * frame of untranslated keys.
 */
export const i18nReady: Promise<unknown> = i18n
  .use(onDemandLocaleBackend)
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { en: enOverrides as LocaleResource },
    fallbackLng: 'en',
    supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
    load: 'currentOnly',
    // English is bundled while every other locale comes from the backend above.
    partialBundledLanguages: true,
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      // Browsers report `zh-CN`/`zh-TW`/`zh`; map them onto our `zhCN`/`zhTW`
      // codes (non-Chinese codes pass through for normal supportedLngs matching).
      convertDetectedLanguage,
    },
  })

export default i18n
