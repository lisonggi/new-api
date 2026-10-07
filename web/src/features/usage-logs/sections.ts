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
 * Section ids for `/usage-logs`, mirroring `section-registry.tsx`.
 *
 * Route files need them in `beforeLoad`, and TanStack Router only code-splits
 * the route `component`, so importing the registry from a route file pulled
 * every usage-log section into the entry chunk. `__tests__/sections.test.ts`
 * fails when the two drift apart.
 */
export const USAGE_LOGS_SECTION_IDS = [
  'common',
  'drawing',
  'task',
] as const

export const USAGE_LOGS_DEFAULT_SECTION = 'common'

export type UsageLogsSectionId = (typeof USAGE_LOGS_SECTION_IDS)[number]

/**
 * Mirrors the guard in `section-registry.tsx`. Route files need it in
 * `beforeLoad`, so the light module must expose it too.
 */
export function isUsageLogsSectionId(s: string): s is UsageLogsSectionId {
  return (USAGE_LOGS_SECTION_IDS as readonly string[]).includes(s)
}
