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
import { assert, describe, expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  buildSettingJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

function channelWithSetting(setting: string) {
  return channelSchema.parse({
    id: 1,
    name: 'review',
    key: '',
    type: 1,
    status: 1,
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    setting,
  })
}

const validConfig = {
  'deepseek-v4.1': [
    { context_tokens: 32000, timeout_ms: 1000 },
    { context_tokens: 200000, timeout_ms: 3000 },
  ],
}

describe('model_first_response_timeout channel setting', () => {
  test('new channels default to empty and omit the field from serialized JSON', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.model_first_response_timeout).toBe('')
    expect(
      'model_first_response_timeout' in
        JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
    ).toBe(false)
  })

  test('preserves a valid config through create, update and reload', () => {
    const channel = channelWithSetting(
      JSON.stringify({ model_first_response_timeout: validConfig })
    )
    const values = transformChannelToFormDefaults(channel)
    expect(values.model_first_response_timeout).toBe(
      JSON.stringify(validConfig, null, 2)
    )

    const payloads = [
      transformFormDataToCreatePayload(values).channel,
      transformFormDataToUpdatePayload(values, channel.id),
    ]
    for (const payload of payloads) {
      assert(typeof payload.setting === 'string')
      expect(JSON.parse(payload.setting).model_first_response_timeout).toEqual(
        validConfig
      )
      expect(
        transformChannelToFormDefaults({
          ...channel,
          setting: payload.setting,
        }).model_first_response_timeout
      ).toBe(JSON.stringify(validConfig, null, 2))
    }
  })

  test('omits the field when the form value is empty or invalid JSON', () => {
    const empty = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      model_first_response_timeout: '',
    }
    expect(
      'model_first_response_timeout' in JSON.parse(buildSettingJSON(empty))
    ).toBe(false)

    const invalid = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      model_first_response_timeout: '{not valid json',
    }
    expect(
      'model_first_response_timeout' in JSON.parse(buildSettingJSON(invalid))
    ).toBe(false)
  })

  test('coerces non-object stored values to empty string instead of leaking them', () => {
    const array = channelWithSetting(
      JSON.stringify({ model_first_response_timeout: [1, 2, 3] })
    )
    expect(
      transformChannelToFormDefaults(array).model_first_response_timeout
    ).toBe('')

    const scalar = channelWithSetting(
      JSON.stringify({ model_first_response_timeout: 'hello' })
    )
    expect(
      transformChannelToFormDefaults(scalar).model_first_response_timeout
    ).toBe('')
  })
})

function validForm() {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'review',
    key: 'sk-test',
    models: 'gpt-4o',
    type: 1,
  }
}

describe('model_first_response_timeout schema validation', () => {
  test('accepts an empty value', () => {
    expect(channelFormSchema.safeParse(validForm()).success).toBe(true)
  })

  test('accepts strictly ascending positive tiers', () => {
    const result = channelFormSchema.safeParse({
      ...validForm(),
      model_first_response_timeout: JSON.stringify(validConfig),
    })
    expect(result.success).toBe(true)
  })

  test('rejects unsorted tiers', () => {
    const unsorted = {
      m: [
        { context_tokens: 200000, timeout_ms: 3000 },
        { context_tokens: 32000, timeout_ms: 1000 },
      ],
    }
    const result = channelFormSchema.safeParse({
      ...validForm(),
      model_first_response_timeout: JSON.stringify(unsorted),
    })
    expect(result.success).toBe(false)
  })

  test('rejects duplicate context bounds', () => {
    const duplicate = {
      m: [
        { context_tokens: 32000, timeout_ms: 1000 },
        { context_tokens: 32000, timeout_ms: 2000 },
      ],
    }
    const result = channelFormSchema.safeParse({
      ...validForm(),
      model_first_response_timeout: JSON.stringify(duplicate),
    })
    expect(result.success).toBe(false)
  })

  test('rejects a non-positive timeout', () => {
    const zero = { m: [{ context_tokens: 32000, timeout_ms: 0 }] }
    const result = channelFormSchema.safeParse({
      ...validForm(),
      model_first_response_timeout: JSON.stringify(zero),
    })
    expect(result.success).toBe(false)
  })

  test('rejects an overflowing timeout', () => {
    const overflow = {
      m: [{ context_tokens: 32000, timeout_ms: 18446744073710 }],
    }
    const result = channelFormSchema.safeParse({
      ...validForm(),
      model_first_response_timeout: JSON.stringify(overflow),
    })
    expect(result.success).toBe(false)
  })
})
