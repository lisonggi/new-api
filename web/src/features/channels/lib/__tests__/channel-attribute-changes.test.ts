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
  expect(changes.settings.reasoningContentBackfill.enabled).toBe(false)
  expect(changes.settings.modelFirstResponseTimeout.enabled).toBe(false)
  expect(changes.settings.errorRetryPolicy.enabled).toBe(false)
  expect(buildChannelAttributeParams(changes)).toEqual({})
})

test('only enabled attributes are built into the request params', () => {
  const changes = emptyChannelAttributeChanges()
  changes.models = { enabled: true, mode: 'append', value: ' gpt-4o,claude-3 ' }
  changes.proxy = { enabled: true, mode: 'set', address: 'http://p.local:1' }
  expect(buildChannelAttributeParams(changes)).toEqual({
    models: 'gpt-4o,claude-3',
    models_mode: 'append',
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

test('model mapping and groups carry their replace/merge mode', () => {
  const changes = emptyChannelAttributeChanges()
  changes.modelMapping = { enabled: true, mode: 'merge', value: '{"a":"b"}' }
  changes.groups = { enabled: true, mode: 'append', value: ['vip'] }
  expect(buildChannelAttributeParams(changes)).toEqual({
    model_mapping: '{"a":"b"}',
    model_mapping_mode: 'merge',
    groups: 'vip',
    groups_mode: 'append',
  })
})

test('channel settings are only built when enabled', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.reasoningContentBackfill = { enabled: true, value: true }
  changes.settings.assistantContentBackfill = { enabled: true, value: true }
  changes.settings.systemPrompt = { enabled: true, value: 'be nice' }
  changes.settings.modelFirstResponseTimeout = {
    enabled: true,
    mode: 'merge',
    value: '{"gpt-4o":[{"context_tokens":1000,"timeout_ms":500}]}',
  }
  expect(buildChannelAttributeParams(changes)).toEqual({
    settings: {
      reasoning_content_backfill: true,
      assistant_content_backfill: true,
      system_prompt: 'be nice',
      model_first_response_timeout: {
        'gpt-4o': [{ context_tokens: 1000, timeout_ms: 500 }],
      },
      model_first_response_timeout_mode: 'merge',
    },
  })
})

test('an enabled attribute with an empty value is rejected', () => {
  const models = emptyChannelAttributeChanges()
  models.models = { enabled: true, mode: 'replace', value: '   ' }
  expect(validateChannelAttributeChanges(models)).toBe('Model list is required')

  const mapping = emptyChannelAttributeChanges()
  mapping.modelMapping = { enabled: true, mode: 'replace', value: '' }
  expect(validateChannelAttributeChanges(mapping)).toBe(
    'Model mapping is required'
  )

  const groups = emptyChannelAttributeChanges()
  groups.groups = { enabled: true, mode: 'replace', value: [] }
  expect(validateChannelAttributeChanges(groups)).toBe(
    'Select at least one group'
  )

  const proxy = emptyChannelAttributeChanges()
  proxy.proxy = { enabled: true, mode: 'set', address: '' }
  expect(validateChannelAttributeChanges(proxy)).toBe(
    'Proxy address is required'
  )

  const timeout = emptyChannelAttributeChanges()
  timeout.settings.modelFirstResponseTimeout = {
    enabled: true,
    mode: 'replace',
    value: '',
  }
  expect(validateChannelAttributeChanges(timeout)).toBe(
    'Model first response timeout must be a JSON object mapping model names to tiers'
  )

  const policy = emptyChannelAttributeChanges()
  policy.settings.errorRetryPolicy = { enabled: true, value: '' }
  expect(validateChannelAttributeChanges(policy)).toBe(
    'Error retry policy must be a valid policy object'
  )
})

test('invalid model mapping JSON is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.modelMapping = { enabled: true, mode: 'replace', value: '{not json' }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model mapping must be valid JSON'
  )
})

test('an out-of-range first response timeout tier is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.modelFirstResponseTimeout = {
    enabled: true,
    mode: 'replace',
    value: '{"gpt-4o":[{"context_tokens":0,"timeout_ms":500}]}',
  }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model first response timeout must be a JSON object mapping model names to tiers'
  )
})

test('a valid change set passes validation', () => {
  const changes = emptyChannelAttributeChanges()
  changes.models = { enabled: true, mode: 'replace', value: 'gpt-4o' }
  changes.modelMapping = {
    enabled: true,
    mode: 'merge',
    value: '{"gpt-4o":"upstream"}',
  }
  changes.proxy = { enabled: true, mode: 'clear', address: '' }
  changes.settings.errorRetryPolicy = {
    enabled: true,
    value: '{"enabled":true}',
  }
  expect(validateChannelAttributeChanges(changes)).toBeNull()
})

test('an array first response timeout value is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.modelFirstResponseTimeout = {
    enabled: true,
    mode: 'replace',
    value: '[1,2,3]',
  }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model first response timeout must be a JSON object mapping model names to tiers'
  )
})

test('a first response timeout tier with an empty list is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.modelFirstResponseTimeout = {
    enabled: true,
    mode: 'replace',
    value: '{"gpt-4o":[]}',
  }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Model first response timeout must be a JSON object mapping model names to tiers'
  )
})

test('a retry policy with an unknown field is rejected', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.errorRetryPolicy = {
    enabled: true,
    value: '{"enabled":true,"unknown":1}',
  }
  expect(validateChannelAttributeChanges(changes)).toBe(
    'Error retry policy must be a valid policy object'
  )
})

test('an empty system prompt is sent as an explicit clear', () => {
  const changes = emptyChannelAttributeChanges()
  changes.settings.systemPrompt = { enabled: true, value: '' }
  expect(buildChannelAttributeParams(changes)).toEqual({
    settings: { system_prompt: '' },
  })
})
