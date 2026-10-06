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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { TruncatedCell } from '@/components/data-table'
import { RuleListTable } from '@/components/rule-list-table'
import { Badge } from '@/components/ui/badge'

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

  return (
    <RuleListTable
      ariaLabel={t('Error message mapping rules table')}
      tableClassName='min-w-[880px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_code]:font-mono!'
      rules={props.rules}
      getRowKey={(rule) => rule.id}
      empty={{
        title: t('No mapping rules yet'),
        description: t(
          'Add a rule to replace a client-facing error message when it contains a keyword.'
        ),
        actionLabel: t('Add mapping rule'),
        actionIcon: Plus,
        onAction: props.onAdd,
      }}
      enabled={{
        isEnabled: (rule) => rule.enabled,
        onToggle: (_rule, index, enabled) => props.onToggle(index, enabled),
      }}
      actions={{
        name: ruleLabel,
        editLabel: t('Edit mapping rule'),
        deleteLabel: t('Delete mapping rule'),
        onEdit: (_rule, index) => props.onEdit(index),
        onDelete: (_rule, index) => props.onDelete(index),
        onMove: props.onMove,
      }}
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
      ]}
    />
  )
}
