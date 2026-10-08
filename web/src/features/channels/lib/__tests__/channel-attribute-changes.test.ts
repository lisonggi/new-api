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
import { expect, test } from 'vitest'

import {
  buildChannelAttributeParams,
  emptyChannelAttributeChanges,
  validateChannelAttributeChanges,
} from '../channel-attribute-changes'

test('a fresh change set turns every attribute off', () => {
  const changes = emptyChannelAttributeChanges()
  expect(changes.models.enabled).toBe(false)
  expect(changes.modelMapping.enabled).toBe(false)
  expect(changes.groups.enabled).toBe(false)
  expect(changes.httpProtocol.enabled).toBe(false)
  expect(changes.shards.enabled).toBe(false)
  expect(changes.proxy.enabled).toBe(false)
  expect(buildChannelAttributeParams(changes)).toEqual({})
})

test('only enabled attributes are built into the request params', () => {
  const changes = emptyChannelAttributeChanges()
  changes.models = { enabled: true, value: ' gpt-4o,claude-3 ' }
  changes.proxy = { enabled: true, mode: 'set', address: 'http://p.local:1' }
  expect(buildChannelAttributeParams(changes)).toEqual({
    models: 'gpt-4o,claude-3',
    proxy: 'http://p.local:1',
  })
})

test('a proxy clear is sent as an empty value', () => {
  const changes = emptyChannelAttributeChanges()
  changes.proxy = { enabled: true, mode: 'clear', address: '' }
  expect(buildChannelAttributeParams(changes)).toEqual({ proxy: '' })
})

test('protocol and shards are built only when enabled', () => {
  const changes = emptyChannelAttributeChanges()
  changes.httpProtocol = { enabled: true, value: 'http1' }
  changes.shards = { enabled: true, value: '4' }
  expect(buildChannelAttributeParams(changes)).toEqual({
    http_protocol: 'http1',
    http2_connection_shards: 4,
  })
})

test('an enabled attribute with an empty value is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.models = { enabled: true, value: '   ' }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model list is required'
  )

  const mapping = emptyChannelAttributeChanges()
  mapping.modelMapping = { enabled: true, value: '' }
  expect(validateChannelAttributeChanges(mapping)).toBe(
    'Model mapping is required'
  )

  const groups = emptyChannelAttributeChanges()
  groups.groups = { enabled: true, value: [] }
  expect(validateChannelAttributeChanges(groups)).toBe(
    'Select at least one group'
  )

  const proxy = emptyChannelAttributeChanges()
  proxy.proxy = { enabled: true, mode: 'set', address: '' }
  expect(validateChannelAttributeChanges(proxy)).toBe(
    'Proxy address is required'
  )
})

test('invalid model mapping JSON is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.modelMapping = { enabled: true, value: '{not json' }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model mapping must be valid JSON'
  )
})

test('a valid change set passes validation', () => {
  const changes = emptyChannelAttributeChanges()
  changes.models = { enabled: true, value: 'gpt-4o' }
  changes.modelMapping = { enabled: true, value: '{"gpt-4o":"upstream"}' }
  changes.proxy = { enabled: true, mode: 'clear', address: '' }
  expect(validateChannelAttributeChanges(changes)).toBeNull()
})
