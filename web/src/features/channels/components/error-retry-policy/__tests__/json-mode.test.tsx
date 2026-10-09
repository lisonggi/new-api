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
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { ErrorRetryPolicyEditor } from '../error-retry-policy-editor'

const POLICY = JSON.stringify({
  enabled: true,
  rules: [
    {
      id: 'rule-1',
      enabled: true,
      action: 'retry',
      status_codes: [500],
      conditions: [],
    },
  ],
})

test('the form tab shows the rules and the JSON tab shows the raw policy', async () => {
  const user = userEvent.setup()
  render(<ErrorRetryPolicyEditor value={POLICY} onChange={() => {}} />)

  // The default form tab renders the rules table.
  expect(document.querySelector('table')).not.toBeNull()

  await user.click(screen.getByRole('tab', { name: 'JSON' }))
  expect(
    screen.getByRole('textbox', { name: 'Error retry policy' })
  ).toHaveValue(POLICY)
})
