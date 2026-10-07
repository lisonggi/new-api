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
import type { TFunction } from 'i18next'

/**
 * Sidebar navigation metadata for `/system-settings/*`.
 *
 * The section registries under `features/system-settings/*` own the section
 * components, so importing them from the layout config pulled every settings
 * page into the entry chunk. Only the ids and titles are needed for the
 * sidebar, so they live here, free of component imports.
 *
 * `__tests__/nav-sections.test.ts` compares these lists with the registries
 * and fails when a section is added, renamed, reordered or moved.
 */

type SectionNavMeta = readonly { id: string; titleKey: string }[]

const SITE_SECTIONS = [
  { id: 'system-info', titleKey: 'System Information' },
  { id: 'notice', titleKey: 'System Notice' },
  { id: 'header-navigation', titleKey: 'Header navigation' },
  { id: 'sidebar-modules', titleKey: 'Sidebar modules' },
] as const

const AUTH_SECTIONS = [
  { id: 'basic-auth', titleKey: 'Basic Authentication' },
  { id: 'oauth', titleKey: 'OAuth Integrations' },
  { id: 'passkey', titleKey: 'Passkey Authentication' },
  { id: 'bot-protection', titleKey: 'Bot Protection' },
  { id: 'custom-oauth', titleKey: 'Custom OAuth' },
] as const

const BILLING_SECTIONS = [
  { id: 'quota', titleKey: 'Quota Settings' },
  { id: 'currency', titleKey: 'Currency & Display' },
  { id: 'model-pricing', titleKey: 'Model Pricing' },
  { id: 'group-pricing', titleKey: 'Group Pricing' },
  { id: 'payment', titleKey: 'Payment Gateway' },
  { id: 'checkin', titleKey: 'Check-in Rewards' },
] as const

const MODELS_SECTIONS = [
  { id: 'global', titleKey: 'Global Model Configuration' },
  { id: 'gemini', titleKey: 'Gemini' },
  { id: 'claude', titleKey: 'Claude' },
  { id: 'grok', titleKey: 'Grok' },
  { id: 'model-deployment', titleKey: 'Model Deployment' },
] as const

const REQUEST_POLICY_SECTIONS = [
  { id: 'filtering', titleKey: 'Request checks' },
  { id: 'routing', titleKey: 'Sessions and retries' },
  { id: 'health', titleKey: 'Channel health' },
  { id: 'error-mapping', titleKey: 'Error message mapping' },
] as const

const SECURITY_SECTIONS = [
  { id: 'rate-limit', titleKey: 'Rate Limiting' },
  { id: 'ssrf', titleKey: 'SSRF Protection' },
  { id: 'token-limits', titleKey: 'Token Limits' },
] as const

const CONTENT_SECTIONS = [
  { id: 'dashboard', titleKey: 'Data Dashboard' },
  { id: 'announcements', titleKey: 'Announcements' },
  { id: 'api-info', titleKey: 'API Addresses' },
  { id: 'faq', titleKey: 'FAQ' },
  { id: 'uptime-kuma', titleKey: 'Uptime Kuma' },
  { id: 'chat', titleKey: 'Chat Presets' },
  { id: 'drawing', titleKey: 'Drawing' },
] as const

const OPERATIONS_SECTIONS = [
  { id: 'behavior', titleKey: 'System Behavior' },
  { id: 'redemption-dialog', titleKey: 'Redemption success dialog' },
  { id: 'alerts', titleKey: 'Monitoring & Alerts' },
  { id: 'email', titleKey: 'SMTP Email' },
  { id: 'worker', titleKey: 'Worker Proxy' },
  { id: 'logs', titleKey: 'Log Maintenance' },
  { id: 'performance', titleKey: 'Performance' },
  { id: 'update-checker', titleKey: 'System maintenance' },
] as const

/**
 * Route files need the section ids and the default section in `beforeLoad`
 * (TanStack only code-splits `component`), so they must not import the section
 * registries. These mirror `sectionIds` / `defaultSection` of each registry.
 */
export const SITE_SECTION_IDS = SITE_SECTIONS.map((section) => section.id)
export const AUTH_SECTION_IDS = AUTH_SECTIONS.map((section) => section.id)
export const BILLING_SECTION_IDS = BILLING_SECTIONS.map((section) => section.id)
export const MODELS_SECTION_IDS = MODELS_SECTIONS.map((section) => section.id)
export const POLICY_SECTION_IDS = REQUEST_POLICY_SECTIONS.map(
  (section) => section.id
)
export const SECURITY_SECTION_IDS = SECURITY_SECTIONS.map(
  (section) => section.id
)
export const CONTENT_SECTION_IDS = CONTENT_SECTIONS.map((section) => section.id)
export const OPERATIONS_SECTION_IDS = OPERATIONS_SECTIONS.map(
  (section) => section.id
)

export const SITE_DEFAULT_SECTION = 'system-info'
export const AUTH_DEFAULT_SECTION = 'basic-auth'
export const BILLING_DEFAULT_SECTION = 'quota'
export const MODELS_DEFAULT_SECTION = 'global'
export const POLICY_DEFAULT_SECTION = 'routing'
export const SECURITY_DEFAULT_SECTION = 'rate-limit'
export const CONTENT_DEFAULT_SECTION = 'dashboard'
export const OPERATIONS_DEFAULT_SECTION = 'behavior'

function buildNavItems(
  t: TFunction,
  sections: SectionNavMeta,
  basePath: string
) {
  return sections.map((section) => ({
    title: t(section.titleKey),
    url: `${basePath}/${section.id}`,
  }))
}

export function getSiteSectionNavItems(t: TFunction) {
  return buildNavItems(t, SITE_SECTIONS, '/system-settings/site')
}

export function getAuthSectionNavItems(t: TFunction) {
  return buildNavItems(t, AUTH_SECTIONS, '/system-settings/auth')
}

export function getBillingSectionNavItems(t: TFunction) {
  return buildNavItems(t, BILLING_SECTIONS, '/system-settings/billing')
}

export function getModelsSectionNavItems(t: TFunction) {
  return buildNavItems(t, MODELS_SECTIONS, '/system-settings/models')
}

export function getPolicySectionNavItems(t: TFunction) {
  return buildNavItems(
    t,
    REQUEST_POLICY_SECTIONS,
    '/system-settings/request-policies'
  )
}

export function getSecuritySectionNavItems(t: TFunction) {
  return buildNavItems(t, SECURITY_SECTIONS, '/system-settings/security')
}

export function getContentSectionNavItems(t: TFunction) {
  return buildNavItems(t, CONTENT_SECTIONS, '/system-settings/content')
}

export function getOperationsSectionNavItems(t: TFunction) {
  return buildNavItems(t, OPERATIONS_SECTIONS, '/system-settings/operations')
}
