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
import {
  ArrowDown,
  ArrowUp,
  Edit,
  ListFilter,
  Trash2,
  type LucideIcon,
} from 'lucide-react'
import type { Key, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTableRowActionMenu,
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuGroup,
  DropdownMenuItem,
} from '@/components/ui/dropdown-menu'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

/**
 * Shared shell for the ordered "rule list" tables (channel affinity, error
 * message mapping, per-channel error retry). It owns the empty state, the static
 * table chrome, the optional enabled switch and the row action cluster
 * (move up/down, edit, overflow delete). Features pass their own columns and
 * handlers; they must not re-assemble this shell.
 */
type RuleListEmptyState = {
  title: string
  description: string
  actionLabel: string
  actionIcon: LucideIcon
  onAction: () => void
  /** Overrides the empty-state min height; defaults to `min-h-56`. */
  minHeightClassName?: string
}

type RuleListActions<TRule> = {
  /** Row label used by the action group and enabled switch accessible names. */
  name: (rule: TRule) => string
  editLabel: string
  deleteLabel: string
  onEdit: (rule: TRule, index: number) => void
  onDelete: (rule: TRule, index: number) => void
  /** Renders the move up/down buttons when provided. */
  onMove?: (index: number, direction: -1 | 1) => void
  /** Extra overflow menu items rendered above the delete item. */
  extraMenuItems?: (rule: TRule, index: number) => ReactNode
  /** Actions header width class; defaults to `w-40 text-right`. */
  headerClassName?: string
}

type RuleListEnabledColumn<TRule> = {
  isEnabled: (rule: TRule) => boolean
  onToggle: (rule: TRule, index: number, enabled: boolean) => void
}

interface RuleListTableProps<TRule> {
  ariaLabel: string
  rules: TRule[]
  getRowKey: (rule: TRule, index: number) => Key
  /** Feature columns; the enabled and actions columns are appended. */
  columns: StaticDataTableColumn<TRule>[]
  tableClassName: string
  empty: RuleListEmptyState
  actions: RuleListActions<TRule>
  enabled?: RuleListEnabledColumn<TRule>
  disabled?: boolean
}

export function RuleListTable<TRule>(props: RuleListTableProps<TRule>) {
  const { t } = useTranslation()

  if (props.rules.length === 0) {
    const EmptyIcon = props.empty.actionIcon
    return (
      <EmptyState
        icon={ListFilter}
        title={props.empty.title}
        description={props.empty.description}
        className={props.empty.minHeightClassName ?? 'min-h-56'}
        action={
          <Button
            type='button'
            variant='outline'
            disabled={props.disabled}
            onClick={props.empty.onAction}
          >
            <EmptyIcon aria-hidden='true' />
            {props.empty.actionLabel}
          </Button>
        }
      />
    )
  }

  const columns: StaticDataTableColumn<TRule>[] = [...props.columns]
  const enabled = props.enabled
  if (enabled) {
    columns.push({
      id: 'enabled',
      header: t('Enabled'),
      className: 'w-20',
      cell: (rule, index) => (
        <Switch
          checked={enabled.isEnabled(rule)}
          disabled={props.disabled}
          aria-label={t('Enable rule {{name}}', {
            name: props.actions.name(rule),
          })}
          onCheckedChange={(checked) => enabled.onToggle(rule, index, checked)}
        />
      ),
    })
  }

  const onMove = props.actions.onMove
  const onEdit = props.actions.onEdit
  const onDelete = props.actions.onDelete
  columns.push({
    id: 'actions',
    header: t('Actions'),
    className: props.actions.headerClassName ?? 'w-40 text-right',
    cell: (rule, index) => (
      <div
        role='group'
        aria-label={t('Actions for {{name}}', {
          name: props.actions.name(rule),
        })}
        className='flex items-center justify-end gap-0.5'
      >
        {onMove ? (
          <>
            <RuleActionButton
              icon={ArrowUp}
              label={t('Move rule up')}
              disabled={props.disabled || index === 0}
              onClick={() => onMove(index, -1)}
            />
            <RuleActionButton
              icon={ArrowDown}
              label={t('Move rule down')}
              disabled={props.disabled || index === props.rules.length - 1}
              onClick={() => onMove(index, 1)}
            />
          </>
        ) : null}
        <RuleActionButton
          icon={Edit}
          label={props.actions.editLabel}
          disabled={props.disabled}
          onClick={() => onEdit(rule, index)}
        />
        <DataTableRowActionMenu ariaLabel={t('More actions')}>
          <DropdownMenuGroup>
            {props.actions.extraMenuItems?.(rule, index)}
            <DropdownMenuItem
              variant='destructive'
              disabled={props.disabled}
              onClick={() => onDelete(rule, index)}
            >
              <Trash2 aria-hidden='true' />
              {props.actions.deleteLabel}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DataTableRowActionMenu>
      </div>
    ),
  })

  return (
    <StaticDataTable
      className='focus-visible:outline-ring overflow-x-auto rounded-none border-0 focus-visible:-outline-offset-2'
      containerProps={{
        role: 'region',
        'aria-label': props.ariaLabel,
        tabIndex: 0,
      }}
      tableProps={{ withContainer: false }}
      tableClassName={props.tableClassName}
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.rules}
      getRowKey={props.getRowKey}
      columns={columns}
    />
  )
}

type RuleActionButtonProps = {
  icon: LucideIcon
  label: string
  disabled?: boolean
  onClick: () => void
}

function RuleActionButton(props: RuleActionButtonProps) {
  const Icon = props.icon
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='icon'
            aria-label={props.label}
            disabled={props.disabled}
            onClick={props.onClick}
          >
            <Icon aria-hidden='true' />
          </Button>
        }
      />
      <TooltipContent>{props.label}</TooltipContent>
    </Tooltip>
  )
}
