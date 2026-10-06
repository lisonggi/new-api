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
import { Plus } from 'lucide-react'
import { expect, test, vi } from 'vitest'

import { DropdownMenuItem } from '@/components/ui/dropdown-menu'

import { RuleListTable } from '../rule-list-table'

type Rule = { id: string; name: string; enabled: boolean }

const RULES: Rule[] = [
  { id: 'r1', name: 'First', enabled: true },
  { id: 'r2', name: 'Second', enabled: false },
]

type Handlers = {
  onAdd: () => void
  onEdit: (rule: Rule, index: number) => void
  onDelete: (rule: Rule, index: number) => void
  onToggle: (rule: Rule, index: number, enabled: boolean) => void
  onMove: (index: number, direction: -1 | 1) => void
  onClearCache: (rule: Rule, index: number) => void
}

function renderRuleList(options: { rules?: Rule[]; disabled?: boolean } = {}) {
  const handlers: Handlers = {
    onAdd: vi.fn<() => void>(),
    onEdit: vi.fn<(rule: Rule, index: number) => void>(),
    onDelete: vi.fn<(rule: Rule, index: number) => void>(),
    onToggle: vi.fn<(rule: Rule, index: number, enabled: boolean) => void>(),
    onMove: vi.fn<(index: number, direction: -1 | 1) => void>(),
    onClearCache: vi.fn<(rule: Rule, index: number) => void>(),
  }
  render(
    <RuleListTable<Rule>
      ariaLabel='Test rules table'
      tableClassName='min-w-[400px]'
      rules={options.rules ?? RULES}
      getRowKey={(rule) => rule.id}
      disabled={options.disabled}
      empty={{
        title: 'No rules yet',
        description: 'Add one.',
        actionLabel: 'Add rule',
        actionIcon: Plus,
        onAction: handlers.onAdd,
      }}
      enabled={{
        isEnabled: (rule) => rule.enabled,
        onToggle: handlers.onToggle,
      }}
      actions={{
        name: (rule) => rule.name,
        editLabel: 'Edit rule',
        deleteLabel: 'Delete rule',
        onEdit: handlers.onEdit,
        onDelete: handlers.onDelete,
        onMove: handlers.onMove,
        extraMenuItems: (rule, index) => (
          <DropdownMenuItem onClick={() => handlers.onClearCache(rule, index)}>
            Clear cache
          </DropdownMenuItem>
        ),
      }}
      columns={[{ id: 'name', header: 'Name', cell: (rule) => rule.name }]}
    />
  )
  return handlers
}

test('renders the feature columns inside the configured region', () => {
  renderRuleList()

  expect(
    screen.getByRole('region', { name: 'Test rules table' })
  ).toBeInTheDocument()
  expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument()
  expect(
    screen.getByRole('columnheader', { name: 'Enabled' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('columnheader', { name: 'Actions' })
  ).toBeInTheDocument()
  expect(screen.getByText('First')).toBeInTheDocument()
})

test('renders the empty state and calls the action when there are no rules', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList({ rules: [] })

  expect(screen.queryByRole('region')).not.toBeInTheDocument()
  expect(screen.getByText('No rules yet')).toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Add rule' }))
  expect(handlers.onAdd).toHaveBeenCalledTimes(1)
})

test('disabling the list disables the empty-state action', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList({ rules: [], disabled: true })

  const add = screen.getByRole('button', { name: 'Add rule' })
  expect(add).toBeDisabled()

  await user.click(add)
  expect(handlers.onAdd).not.toHaveBeenCalled()
})

test('the enabled switch reflects the row and reports the row name', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList()

  const enabled = screen.getByRole('switch', { name: 'Enable rule First' })
  expect(enabled).toBeChecked()
  expect(
    screen.getByRole('switch', { name: 'Enable rule Second' })
  ).not.toBeChecked()

  await user.click(screen.getByRole('switch', { name: 'Enable rule Second' }))
  expect(handlers.onToggle).toHaveBeenCalledWith(RULES[1], 1, true)
})

test('move up is disabled on the first row and move down on the last', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList()

  const up = screen.getAllByRole('button', { name: 'Move rule up' })
  const down = screen.getAllByRole('button', { name: 'Move rule down' })
  expect(up[0]).toBeDisabled()
  expect(up[1]).toBeEnabled()
  expect(down[0]).toBeEnabled()
  expect(down[1]).toBeDisabled()

  await user.click(up[1])
  expect(handlers.onMove).toHaveBeenCalledWith(1, -1)
})

test('edit reports the rule together with its index', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList()

  await user.click(screen.getAllByRole('button', { name: 'Edit rule' })[1])
  expect(handlers.onEdit).toHaveBeenCalledWith(RULES[1], 1)
})

test('extra menu items and delete are reachable from the row overflow menu', async () => {
  const user = userEvent.setup()
  const handlers = renderRuleList()

  await user.click(screen.getAllByRole('button', { name: 'More actions' })[0])
  await user.click(screen.getByRole('menuitem', { name: 'Clear cache' }))
  expect(handlers.onClearCache).toHaveBeenCalledWith(RULES[0], 0)

  await user.click(screen.getAllByRole('button', { name: 'More actions' })[0])
  await user.click(screen.getByRole('menuitem', { name: 'Delete rule' }))
  expect(handlers.onDelete).toHaveBeenCalledWith(RULES[0], 0)
})

test('disabling the list disables the switch and the row actions', () => {
  renderRuleList({ disabled: true })

  expect(
    screen.getByRole('switch', { name: 'Enable rule First' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(screen.getAllByRole('button', { name: 'Edit rule' })[0]).toBeDisabled()
})
