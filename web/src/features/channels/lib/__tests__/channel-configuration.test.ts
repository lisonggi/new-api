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
import { describe, expect, test } from 'vitest'

import { getChannelConfigurationState } from '../channel-configuration'
import { CHANNEL_FORM_DEFAULT_VALUES } from '../channel-form'

function blocksWith(
  overrides: Partial<typeof CHANNEL_FORM_DEFAULT_VALUES>
): ReturnType<typeof getChannelConfigurationState>['blocks'] {
  return getChannelConfigurationState(
    { ...CHANNEL_FORM_DEFAULT_VALUES, ...overrides },
    {},
    true
  ).blocks
}

describe('channel configuration block detection', () => {
  test('ignore_response_model_mismatch alone marks request processing configured', () => {
    expect(
      blocksWith({ ignore_response_model_mismatch: true }).requestProcessing
    ).toBe('configured')
  })

  test('a channel without request-processing flags leaves the block idle', () => {
    expect(blocksWith({}).requestProcessing).toBe('idle')
  })
})
