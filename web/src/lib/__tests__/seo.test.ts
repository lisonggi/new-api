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
import { afterEach, describe, expect, it } from 'vitest'

import { syncCanonicalLink } from '@/lib/seo'

function canonicalLinks(): NodeListOf<HTMLLinkElement> {
  return document.head.querySelectorAll('link[rel="canonical"]')
}

describe('syncCanonicalLink', () => {
  afterEach(() => {
    canonicalLinks().forEach((link) => link.remove())
  })

  it('creates a canonical link when the document has none', () => {
    syncCanonicalLink('https://io.iioooo.com/about')

    const links = canonicalLinks()
    expect(links).toHaveLength(1)
    expect(links[0]).toHaveAttribute('href', 'https://io.iioooo.com/about')
  })

  it('updates the existing canonical link instead of adding another', () => {
    const existing = document.createElement('link')
    existing.rel = 'canonical'
    existing.href = 'https://io.iioooo.com/'
    document.head.appendChild(existing)

    syncCanonicalLink('https://io.iioooo.com/pricing')

    const links = canonicalLinks()
    expect(links).toHaveLength(1)
    expect(links[0]).toHaveAttribute('href', 'https://io.iioooo.com/pricing')
  })
})
