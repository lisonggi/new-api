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
import { render } from '@testing-library/react'
import { expect, test } from 'vitest'

import { ErrorRetryPolicyEditor } from '../error-retry-policy-editor'

// An unbreakable value like a long model name or URL: with an auto table
// layout it widens the column and pushes the row actions out of the dialog.
const LONG_VALUE = 'a'.repeat(200)

function policyWithConditionValue(value: string): string {
  return JSON.stringify({
    enabled: true,
    rules: [
      {
        id: 'rule-1',
        enabled: true,
        action: 'retry',
        status_codes: [],
        conditions: [
          {
            field: 'message',
            operator: 'contains',
            value,
            case_sensitive: false,
          },
        ],
      },
    ],
  })
}

test('a long rule condition keeps the rules table at a fixed layout instead of widening it', () => {
  const { container } = render(
    <ErrorRetryPolicyEditor
      value={policyWithConditionValue(LONG_VALUE)}
      onChange={() => {}}
    />
  )

  const table = container.querySelector('table')
  expect(table).not.toBeNull()
  expect(table).toHaveClass('table-fixed')
})

// The rules table is also rendered inside the batch edit dialog (max-w-2xl,
// ~614px content). A min-width larger than that forces a horizontal scroll
// that pushes the row action buttons out of view, so it must stay small
// enough to fit.
test('the rules table min-width fits the batch edit dialog so row actions stay visible', () => {
  const { container } = render(
    <ErrorRetryPolicyEditor
      value={policyWithConditionValue(LONG_VALUE)}
      onChange={() => {}}
    />
  )

  const table = container.querySelector('table')
  expect(table).not.toBeNull()
  const minWidthPx = Number(
    table?.className.match(/min-w-\[(\d+)px\]/)?.[1] ?? Number.NaN
  )
  expect(minWidthPx).toBeLessThanOrEqual(600)
})
