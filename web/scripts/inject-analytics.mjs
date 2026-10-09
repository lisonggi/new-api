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
/**
 * Build-time analytics injection for the dashboard frontend.
 *
 * The backend used to inject these tags into the HTML it served. The frontend
 * is now built and deployed independently, so the injection happens here
 * instead: `dist/index.html` (and, because this runs before prerender.mjs, every
 * pre-rendered page cloned from it) gets the Umami and Google Analytics tags.
 *
 * Reads the same environment variables as before:
 *   UMAMI_WEBSITE_ID, UMAMI_SCRIPT_URL, GOOGLE_ANALYTICS_ID
 * The matching `VITE_`-prefixed names are also accepted for convenience.
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const distDir = resolve(__dirname, '..', 'dist')
const indexPath = join(distDir, 'index.html')

function env(name) {
  return process.env[name] || process.env[`VITE_${name}`] || ''
}

function umamiSnippet() {
  const siteId = env('UMAMI_WEBSITE_ID')
  const scriptUrl = env('UMAMI_SCRIPT_URL') || 'https://analytics.umami.is/script.js'
  const script = siteId
    ? `<script defer src="${scriptUrl}" data-website-id="${siteId}"></script>`
    : ''
  return `${script}<!--Umami QuantumNous-->\n`
}

function googleAnalyticsSnippet() {
  const gaId = env('GOOGLE_ANALYTICS_ID')
  const script = gaId
    ? `<script async src="https://www.googletagmanager.com/gtag/js?id=${gaId}"></script>` +
      `<script>window.dataLayer = window.dataLayer || [];` +
      `function gtag(){dataLayer.push(arguments);}` +
      `gtag('js', new Date());` +
      `gtag('config', '${gaId}');</script>`
    : ''
  return `${script}<!--Google Analytics QuantumNous-->\n`
}

let html = readFileSync(indexPath, 'utf8')
html = html.split('<!--umami-->\n').join(umamiSnippet())
html = html.split('<!--Google Analytics-->\n').join(googleAnalyticsSnippet())
writeFileSync(indexPath, html)

console.log('[inject-analytics] updated dist/index.html')
