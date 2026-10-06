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
  buildChannelErrorRetrySetting,
  extractSettingFieldRawValue,
  parseChannelErrorRetryPolicyJSON,
  parseStatusCodesInput,
  serializeChannelErrorRetryPolicy,
  type ChannelErrorRetryPolicy,
} from '../channel-error-retry'
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

const validPolicy: ChannelErrorRetryPolicy = {
  enabled: true,
  rules: [
    {
      id: 'client-parameter',
      name: 'Unsupported parameter',
      enabled: true,
      action: 'stop',
      status_codes: [500],
      conditions: [
        {
          field: 'message',
          operator: 'contains',
          value: 'unsupported parameter',
          case_sensitive: false,
        },
      ],
    },
    {
      id: 'supplier-balance',
      enabled: true,
      action: 'retry',
      status_codes: [402],
      conditions: [
        {
          field: 'code',
          operator: 'equals',
          value: 'insufficient_balance',
          case_sensitive: false,
        },
      ],
    },
  ],
}

describe('channel error retry policy parsing', () => {
  test('treats empty and null values as not configured', () => {
    expect(parseChannelErrorRetryPolicyJSON('').policy).toBeNull()
    expect(parseChannelErrorRetryPolicyJSON(undefined).policy).toBeNull()
    expect(parseChannelErrorRetryPolicyJSON('null').policy).toBeNull()
  })

  test('round trips a valid policy through serialization', () => {
    const parsed = parseChannelErrorRetryPolicyJSON(
      serializeChannelErrorRetryPolicy(validPolicy)
    )
    expect(parsed.error).toBeNull()
    expect(parsed.policy).toEqual(validPolicy)
  })

  test('reports malformed JSON and invalid rules', () => {
    expect(parseChannelErrorRetryPolicyJSON('{not json').error).toBe(
      'invalid_json'
    )
    expect(
      parseChannelErrorRetryPolicyJSON('{"enabled":true,"rules":[]}').policy
    ).toEqual({ enabled: true, rules: [] })
    expect(
      parseChannelErrorRetryPolicyJSON(
        JSON.stringify({
          enabled: true,
          rules: [{ id: 'r1', enabled: true, action: 'inherit' }],
        })
      ).error
    ).toContain('action')
  })

  test('rejects invalid status codes, duplicate codes and empty matchers', () => {
    const base = { id: 'r1', enabled: true, action: 'retry' as const }
    expect(
      parseChannelErrorRetryPolicyJSON(
        JSON.stringify({
          enabled: true,
          rules: [{ ...base, status_codes: [600] }],
        })
      ).error
    ).toContain('invalid code')
    expect(
      parseChannelErrorRetryPolicyJSON(
        JSON.stringify({
          enabled: true,
          rules: [{ ...base, status_codes: [500, 500] }],
        })
      ).error
    ).toContain('duplicate')
    expect(
      parseChannelErrorRetryPolicyJSON(
        JSON.stringify({ enabled: true, rules: [base] })
      ).error
    ).toContain('empty matcher')
  })

  test('parses comma separated status codes and drops blanks and duplicates', () => {
    expect(parseStatusCodesInput('400, 500, 400, , 503')).toEqual({
      codes: [400, 500, 503],
      invalidTokens: [],
    })
  })

  test('reports status code tokens that are not a single code in range', () => {
    expect(parseStatusCodesInput('200, abc, 99, 600, 500-599')).toEqual({
      codes: [200],
      invalidTokens: ['abc', '99', '600', '500-599'],
    })
  })

  test('preserves an invalid stored policy instead of dropping it', () => {
    const invalidObject = JSON.stringify({
      enabled: true,
      rules: [{ id: 'r1', enabled: true, action: 'retry' }],
    })
    expect(buildChannelErrorRetrySetting(invalidObject)).toEqual(
      JSON.parse(invalidObject)
    )
    expect(buildChannelErrorRetrySetting('{broken')).toBe('{broken')
    expect(buildChannelErrorRetrySetting('')).toBeUndefined()
  })
})

describe('error_retry_policy channel setting round trip', () => {
  test('new channels default to empty and omit the field', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.error_retry_policy).toBe('')
    expect(
      'error_retry_policy' in
        JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
    ).toBe(false)
  })

  test('preserves a configured policy through create, update and reload', () => {
    const channel = channelWithSetting(
      JSON.stringify({ error_retry_policy: validPolicy })
    )
    const values = transformChannelToFormDefaults(channel)
    expect(
      parseChannelErrorRetryPolicyJSON(values.error_retry_policy).policy
    ).toEqual(validPolicy)

    const payloads = [
      transformFormDataToCreatePayload(values).channel,
      transformFormDataToUpdatePayload(values, channel.id),
    ]
    for (const payload of payloads) {
      assert(typeof payload.setting === 'string')
      expect(JSON.parse(payload.setting).error_retry_policy).toEqual(
        validPolicy
      )
      const reloaded = transformChannelToFormDefaults({
        ...channel,
        setting: payload.setting,
      })
      expect(
        parseChannelErrorRetryPolicyJSON(reloaded.error_retry_policy).policy
      ).toEqual(validPolicy)
    }
  })

  test('omits the field only when the form value is actually empty', () => {
    const empty = { ...CHANNEL_FORM_DEFAULT_VALUES, error_retry_policy: '' }
    expect('error_retry_policy' in JSON.parse(buildSettingJSON(empty))).toBe(
      false
    )
  })
})

describe('error_retry_policy schema validation', () => {
  function validForm() {
    return {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'review',
      key: 'sk-test',
      models: 'gpt-4o',
      type: 1,
      error_retry_policy: serializeChannelErrorRetryPolicy(validPolicy),
    }
  }

  test('accepts an empty and a valid policy', () => {
    expect(
      channelFormSchema.safeParse({
        ...validForm(),
        error_retry_policy: '',
      }).success
    ).toBe(true)
    expect(channelFormSchema.safeParse(validForm()).success).toBe(true)
  })

  test('rejects a rule with no matcher and an unknown action', () => {
    const emptyMatcher = serializeChannelErrorRetryPolicy({
      enabled: true,
      rules: [{ id: 'r1', enabled: true, action: 'retry' }],
    })
    expect(
      channelFormSchema.safeParse({
        ...validForm(),
        error_retry_policy: emptyMatcher,
      }).success
    ).toBe(false)
  })

  test('rejects a NUL byte in a rule name or a condition value', () => {
    // The backend rejects NUL bytes in both places, so the form must not emit
    // a policy the server would refuse.
    const nulName =
      '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","name":"a\\u0000b","status_codes":[500]}]}'
    expect(
      channelFormSchema.safeParse({
        ...validForm(),
        error_retry_policy: nulName,
      }).success
    ).toBe(false)

    const nulValue =
      '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","conditions":[{"field":"message","operator":"contains","value":"a\\u0000b"}]}]}'
    expect(
      channelFormSchema.safeParse({
        ...validForm(),
        error_retry_policy: nulValue,
      }).success
    ).toBe(false)
  })

  test('rejects a policy that carries rules without saying whether it is enabled', () => {
    // The backend refuses this too: a policy that never states whether it is
    // enabled is never evaluated and never reported in the decision audit, so
    // the configured rules would silently never run.
    const rulesWithoutEnabled =
      '{"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[500]}]}'
    expect(
      channelFormSchema.safeParse({
        ...validForm(),
        error_retry_policy: rulesWithoutEnabled,
      }).success
    ).toBe(false)
  })
})

describe('error_retry_policy strict preservation regressions', () => {
  test('rejects duplicate keys anywhere in the policy', () => {
    const duplicateAction =
      '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","action":"stop","status_codes":[500]}]}'
    expect(parseChannelErrorRetryPolicyJSON(duplicateAction).error).toBe(
      'duplicate_key'
    )
  })

  test('rejects unknown policy, rule and condition fields', () => {
    expect(
      parseChannelErrorRetryPolicyJSON('{"enabled":true,"extra":1}').error
    ).toBe('unknown_field')
    expect(
      parseChannelErrorRetryPolicyJSON(
        '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","status_codes":[500],"extra":1}]}'
      ).error
    ).toContain('unknown field')
    expect(
      parseChannelErrorRetryPolicyJSON(
        '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","conditions":[{"field":"message","operator":"contains","value":"x","extra":1}]}]}'
      ).error
    ).toContain('unknown field')
  })

  test('preserves a non-object stored policy and blocks saving it unrelated', () => {
    const channel = channelWithSetting(
      JSON.stringify({ error_retry_policy: 'broken' })
    )
    const values = transformChannelToFormDefaults(channel)
    expect(values.error_retry_policy).toBe('"broken"')
    expect(channelFormSchema.safeParse(values).success).toBe(false)
  })

  test('preserves duplicate keys inside a stored policy and blocks saving it', () => {
    const setting =
      '{"error_retry_policy":{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","action":"stop","status_codes":[500]}]}}'
    const values = transformChannelToFormDefaults(channelWithSetting(setting))
    expect(
      parseChannelErrorRetryPolicyJSON(values.error_retry_policy).error
    ).toBe('duplicate_key')
    expect(channelFormSchema.safeParse(values).success).toBe(false)
  })

  test('preserves the raw policy value including unknown nested fields', () => {
    const raw =
      '{"enabled":true,"rules":[{"id":"r1","enabled":true,"action":"retry","conditions":[{"field":"message","operator":"contains","value":"x","extra":1}]}]}'
    const extracted = extractSettingFieldRawValue(
      `{"error_retry_policy":${raw},"proxy":"http://a"}`,
      'error_retry_policy'
    )
    expect(extracted.ambiguous).toBe(false)
    expect(extracted.value).toBe(raw)
  })

  test('flags a duplicated top-level policy field as ambiguous', () => {
    const extracted = extractSettingFieldRawValue(
      '{"error_retry_policy":{"enabled":true},"error_retry_policy":{"enabled":false}}',
      'error_retry_policy'
    )
    expect(extracted.ambiguous).toBe(true)
  })
})
