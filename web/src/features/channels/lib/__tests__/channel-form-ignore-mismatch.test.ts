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

describe('ignore_response_model_mismatch channel setting', () => {
  test('new channels default to the warning enabled', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.ignore_response_model_mismatch).toBe(
      false
    )
    expect(
      JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
        .ignore_response_model_mismatch
    ).toBe(false)
  })

  test.each([undefined, false, true])(
    'preserves setting %s through create, update and reload',
    (enabled) => {
      const channel = channelWithSetting(
        JSON.stringify({ ignore_response_model_mismatch: enabled })
      )
      const values = transformChannelToFormDefaults(channel)
      const payloads = [
        transformFormDataToCreatePayload(values).channel,
        transformFormDataToUpdatePayload(values, channel.id),
      ]
      for (const payload of payloads) {
        assert(typeof payload.setting === 'string')
        expect(JSON.parse(payload.setting).ignore_response_model_mismatch).toBe(
          enabled === true
        )
        expect(
          transformChannelToFormDefaults({
            ...channel,
            setting: payload.setting,
          }).ignore_response_model_mismatch
        ).toBe(enabled === true)
      }
    }
  )

  test('coerces non-boolean values to false instead of leaking them into the form', () => {
    const stringFalse = channelWithSetting(
      JSON.stringify({ ignore_response_model_mismatch: 'false' })
    )
    expect(
      transformChannelToFormDefaults(stringFalse).ignore_response_model_mismatch
    ).toBe(false)

    const numeric = channelWithSetting(
      JSON.stringify({ ignore_response_model_mismatch: 1 })
    )
    expect(
      transformChannelToFormDefaults(numeric).ignore_response_model_mismatch
    ).toBe(false)
  })
})
