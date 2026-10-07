#!/usr/bin/env node
/**
 * Regenerate src/i18n/locales/en-overrides.json from en.json.
 *
 * This project uses English source strings as translation keys, so almost every
 * entry in en.json maps a key onto itself. i18next already returns the key when
 * a translation is missing, which makes those identity entries dead weight:
 * ~500 KB of the entry chunk for 69 entries that carry information.
 *
 * Only entries whose value differs from the key are bundled. Run this after
 * editing en.json; src/i18n/__tests__/en-overrides.test.ts fails if the two
 * files drift apart.
 *
 * Usage: node scripts/generate-en-overrides.mjs
 */
import { readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const localesDir = path.join(here, '..', 'src', 'i18n', 'locales')

const source = JSON.parse(
  readFileSync(path.join(localesDir, 'en.json'), 'utf8')
).translation
const overrides = Object.fromEntries(
  Object.entries(source).filter(([key, value]) => value !== key)
)

writeFileSync(
  path.join(localesDir, 'en-overrides.json'),
  `${JSON.stringify({ translation: overrides }, null, 2)}\n`
)

console.log(
  `en-overrides.json: kept ${Object.keys(overrides).length} of ${
    Object.keys(source).length
  } entries`
)
