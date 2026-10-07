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
import { describe, expect, it } from 'vitest'

import {
  AUTH_DEFAULT_SECTION,
  AUTH_SECTION_IDS,
  getAuthSectionNavItems,
} from '../auth/section-registry'
import {
  BILLING_DEFAULT_SECTION,
  BILLING_SECTION_IDS,
  getBillingSectionNavItems,
} from '../billing/section-registry'
import {
  CONTENT_DEFAULT_SECTION,
  CONTENT_SECTION_IDS,
  getContentSectionNavItems,
} from '../content/section-registry'
import {
  getModelsSectionNavItems,
  MODELS_DEFAULT_SECTION,
  MODELS_SECTION_IDS,
} from '../models/section-registry'
import {
  AUTH_DEFAULT_SECTION as navAuthDefault,
  AUTH_SECTION_IDS as navAuthIds,
  BILLING_DEFAULT_SECTION as navBillingDefault,
  BILLING_SECTION_IDS as navBillingIds,
  CONTENT_DEFAULT_SECTION as navContentDefault,
  CONTENT_SECTION_IDS as navContentIds,
  getAuthSectionNavItems as navAuthNavItems,
  getBillingSectionNavItems as navBillingNavItems,
  getContentSectionNavItems as navContentNavItems,
  getModelsSectionNavItems as navModelsNavItems,
  getOperationsSectionNavItems as navOperationsNavItems,
  getPolicySectionNavItems as navPolicyNavItems,
  getSecuritySectionNavItems as navSecurityNavItems,
  getSiteSectionNavItems as navSiteNavItems,
  MODELS_DEFAULT_SECTION as navModelsDefault,
  MODELS_SECTION_IDS as navModelsIds,
  OPERATIONS_DEFAULT_SECTION as navOperationsDefault,
  OPERATIONS_SECTION_IDS as navOperationsIds,
  POLICY_SECTION_IDS as navPolicyIds,
  SECURITY_DEFAULT_SECTION as navSecurityDefault,
  SECURITY_SECTION_IDS as navSecurityIds,
  SITE_DEFAULT_SECTION as navSiteDefault,
  SITE_SECTION_IDS as navSiteIds,
} from '../nav-sections'
import {
  getOperationsSectionNavItems,
  OPERATIONS_DEFAULT_SECTION,
  OPERATIONS_SECTION_IDS,
} from '../operations/section-registry'
import {
  getPolicySectionNavItems,
  POLICY_SECTION_IDS,
} from '../request-policies/section-registry'
import {
  getSecuritySectionNavItems,
  SECURITY_DEFAULT_SECTION,
  SECURITY_SECTION_IDS,
} from '../security/section-registry'
import {
  getSiteSectionNavItems,
  SITE_DEFAULT_SECTION,
  SITE_SECTION_IDS,
} from '../site/section-registry'

// `nav-sections.ts` duplicates the registries' section ids, order and titles so
// that route files and the layout config can read them without pulling every
// settings page into the entry chunk. These guards fail as soon as the two
// drift apart, because the ids drive route validation and the titles drive the
// sidebar.
const t = ((key: string) => key) as unknown as TFunction

const navItems = [
  ['site', navSiteNavItems, getSiteSectionNavItems],
  ['auth', navAuthNavItems, getAuthSectionNavItems],
  ['billing', navBillingNavItems, getBillingSectionNavItems],
  ['models', navModelsNavItems, getModelsSectionNavItems],
  ['request-policies', navPolicyNavItems, getPolicySectionNavItems],
  ['security', navSecurityNavItems, getSecuritySectionNavItems],
  ['content', navContentNavItems, getContentSectionNavItems],
  ['operations', navOperationsNavItems, getOperationsSectionNavItems],
] as const

const sectionIds = [
  ['site', navSiteIds, SITE_SECTION_IDS],
  ['auth', navAuthIds, AUTH_SECTION_IDS],
  ['billing', navBillingIds, BILLING_SECTION_IDS],
  ['models', navModelsIds, MODELS_SECTION_IDS],
  ['request-policies', navPolicyIds, POLICY_SECTION_IDS],
  ['security', navSecurityIds, SECURITY_SECTION_IDS],
  ['content', navContentIds, CONTENT_SECTION_IDS],
  ['operations', navOperationsIds, OPERATIONS_SECTION_IDS],
] as const

const defaultSections = [
  ['site', navSiteDefault, SITE_DEFAULT_SECTION],
  ['auth', navAuthDefault, AUTH_DEFAULT_SECTION],
  ['billing', navBillingDefault, BILLING_DEFAULT_SECTION],
  ['models', navModelsDefault, MODELS_DEFAULT_SECTION],
  ['security', navSecurityDefault, SECURITY_DEFAULT_SECTION],
  ['content', navContentDefault, CONTENT_DEFAULT_SECTION],
  ['operations', navOperationsDefault, OPERATIONS_DEFAULT_SECTION],
] as const

describe('system settings sidebar metadata', () => {
  it.each(navItems)(
    'matches the %s section registry in order, ids and titles',
    (_domain, fromNavSections, fromRegistry) => {
      expect(fromNavSections(t)).toEqual(fromRegistry(t))
    }
  )

  it.each(sectionIds)(
    'exposes the %s section ids used by route validation',
    (_domain, fromNavSections, fromRegistry) => {
      expect([...fromNavSections]).toEqual([...fromRegistry])
    }
  )

  it.each(defaultSections)(
    'exposes the %s default section used by route redirects',
    (_domain, fromNavSections, fromRegistry) => {
      expect(fromNavSections).toBe(fromRegistry)
    }
  )
})
