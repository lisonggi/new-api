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
  Plus,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  DataTableRowActionMenu,
  StaticDataTable,
  TruncatedCell,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { Badge } from '@/components/ui/badge'
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

import type { ErrorMappingRule } from './types'

interface ErrorMappingRulesTableProps {
  rules: ErrorMappingRule[]
  onAdd: () => void
  onEdit: (index: number) => void
  onDelete: (index: number) => void
  onToggle: (index: number, enabled: boolean) => void
  onMove: (index: number, direction: -1 | 1) => void
}

export function ErrorMappingRulesTable(props: ErrorMappingRulesTableProps) {
  const { t } = useTranslation()

  const ruleLabel = (rule: ErrorMappingRule) => rule.name || rule.keyword

  if (props.rules.length === 0) {
    return (
      <EmptyState
        icon={ListFilter}
        title={t('No mapping rules yet')}
        description={t(
          'Add a rule to replace a client-facing error message when it contains a keyword.'
        )}
        className='min-h-56'
        action={
          <Button variant='outline' onClick={props.onAdd}>
            <Plus aria-hidden='true' />
            {t('Add mapping rule')}
          </Button>
        }
      />
    )
  }

  return (
    <StaticDataTable
      className='focus-visible:outline-ring overflow-x-auto rounded-none border-0 focus-visible:-outline-offset-2'
      containerProps={{
        role: 'region',
        'aria-label': t('Error message mapping rules table'),
        tabIndex: 0,
      }}
      tableProps={{ withContainer: false }}
      tableClassName='min-w-[880px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_code]:font-mono!'
      headerRowClassName='bg-muted/35 hover:bg-muted/35'
      data={props.rules}
      getRowKey={(rule) => rule.id}
      columns={[
        {
          id: 'name',
          header: t('Rule'),
          className: 'w-[24%]',
          cell: (rule, index) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              <TruncatedCell tabIndex={0}>
                {rule.name || rule.keyword || '—'}
              </TruncatedCell>
              <div className='text-muted-foreground min-w-0 text-xs'>
                <TruncatedCell tabIndex={0}>{rule.id}</TruncatedCell>
              </div>
              <span className='sr-only'>
                {t('Priority {{priority}}', { priority: index + 1 })}
              </span>
            </div>
          ),
        },
        {
          id: 'keyword',
          header: t('Keyword'),
          className: 'w-[20%]',
          cell: (rule) => (
            <TruncatedCell tabIndex={0} tooltipContent={rule.keyword}>
              <code>{rule.keyword}</code>
            </TruncatedCell>
          ),
        },
        {
          id: 'replacement',
          header: t('Replacement message'),
          cell: (rule) => (
            <TruncatedCell tabIndex={0} tooltipContent={rule.replacement}>
              {rule.replacement}
            </TruncatedCell>
          ),
        },
        {
          id: 'matching',
          header: t('Matching'),
          className: 'w-32',
          cell: (rule) => (
            <Badge variant='outline'>
              {rule.case_sensitive
                ? t('Case sensitive')
                : t('Case insensitive')}
            </Badge>
          ),
        },
        {
          id: 'enabled',
          header: t('Enabled'),
          className: 'w-20',
          cell: (rule, index) => (
            <Switch
              checked={rule.enabled}
              onCheckedChange={(checked) => props.onToggle(index, checked)}
              aria-label={t('Enable rule {{name}}', {
                name: ruleLabel(rule),
              })}
            />
          ),
        },
        {
          id: 'actions',
          header: t('Actions'),
          className: 'w-40 text-right',
          cell: (rule, index) => (
            <div
              role='group'
              aria-label={t('Actions for {{name}}', { name: ruleLabel(rule) })}
              className='flex items-center justify-end gap-0.5'
            >
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Move rule up')}
                      disabled={index === 0}
                      onClick={() => props.onMove(index, -1)}
                    >
                      <ArrowUp aria-hidden='true' />
                    </Button>
                  }
                />
                <TooltipContent>{t('Move rule up')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Move rule down')}
                      disabled={index === props.rules.length - 1}
                      onClick={() => props.onMove(index, 1)}
                    >
                      <ArrowDown aria-hidden='true' />
                    </Button>
                  }
                />
                <TooltipContent>{t('Move rule down')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={t('Edit mapping rule')}
                      onClick={() => props.onEdit(index)}
                    >
                      <Edit aria-hidden='true' />
                    </Button>
                  }
                />
                <TooltipContent>{t('Edit mapping rule')}</TooltipContent>
              </Tooltip>
              <DataTableRowActionMenu ariaLabel={t('More actions')}>
                <DropdownMenuGroup>
                  <DropdownMenuItem
                    variant='destructive'
                    onClick={() => props.onDelete(index)}
                  >
                    <Trash2 aria-hidden='true' />
                    {t('Delete mapping rule')}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DataTableRowActionMenu>
            </div>
          ),
        },
      ]}
    />
  )
}
