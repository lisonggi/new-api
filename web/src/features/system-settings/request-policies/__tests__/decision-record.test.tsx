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
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { PolicyEvent } from '../api'
import { PolicyDecisionRecord } from '../decision-record'

function retryEvent(audit: PolicyEvent['decision']['audit']): PolicyEvent {
  return {
    attempt: 1,
    elapsed_ms: 12,
    status: 500,
    upstream_status: 400,
    decision: {
      action: 'stop',
      reason: 'channel_error_rule_stop',
      source: 'channel_rule',
      rule_id: 'r1',
      audit,
    },
  }
}

test('shows both the raw upstream status and the status the rules matched', () => {
  render(
    <PolicyDecisionRecord
      events={[
        retryEvent({
          status: 'matched',
          upstream_status: 400,
          matched_status: 500,
        }),
      ]}
    />
  )

  expect(screen.getByText(/Upstream HTTP 400/)).toBeTruthy()
  expect(screen.getByText(/Matched HTTP 500/)).toBeTruthy()
})

test('omits the matched status for an audit recorded before it existed', () => {
  render(
    <PolicyDecisionRecord
      events={[retryEvent({ status: 'miss', upstream_status: 400 })]}
    />
  )

  expect(screen.getByText(/Upstream HTTP 400/)).toBeTruthy()
  expect(screen.queryByText(/Matched HTTP/)).toBeNull()
})
