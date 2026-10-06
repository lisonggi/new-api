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
  ArrowLeftRight,
  FileText,
  Link2Off,
  LockKeyhole,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { TruncatedCell } from '@/components/data-table'
import { RuleListTable } from '@/components/rule-list-table'
import { StatusBadge, StatusBadgeList } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { policyLabel } from '../../request-policies/policy-label'
import type { AffinityRule, CacheStats, SessionMode } from './types'

interface SessionRulesTableProps {
  rules: AffinityRule[]
  cacheStats: CacheStats | null
  enabled?: boolean
  globalSessionMode?: SessionMode | ''
  onEdit: (rule: AffinityRule) => void
  onDelete: (rule: AffinityRule) => void
  onClearCache: (name: string) => void
  onFillTemplates: () => void
}

export function SessionRulesTable(props: SessionRulesTableProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  return (
    <RuleListTable
      ariaLabel={t('Session rules table')}
      tableClassName='min-w-[960px] table-fixed [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4 [&_code]:font-mono!'
      rules={props.rules}
      getRowKey={(rule) => rule.id ?? rule.name}
      empty={{
        title: t('No rules yet'),
        description: t('Fill a CLI template or add a blank rule.'),
        actionLabel: t('Fill Templates'),
        actionIcon: FileText,
        onAction: props.onFillTemplates,
      }}
      actions={{
        name: (rule) => rule.name,
        editLabel: t('Edit Rule'),
        deleteLabel: t('Delete Rule'),
        headerClassName: 'w-24 text-right',
        onEdit: (rule) => props.onEdit(rule),
        onDelete: (rule) => props.onDelete(rule),
        extraMenuItems: (rule) =>
          rule.include_rule_name ? (
            <>
              <DropdownMenuItem onClick={() => props.onClearCache(rule.name)}>
                <Trash2 aria-hidden='true' />
                {t('Clear cache for this rule')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          ) : null,
      }}
      columns={[
        {
          id: 'name',
          header: t('Name'),
          className: 'w-[20%]',
          cell: (rule) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              <TruncatedCell tabIndex={0}>{rule.name || '—'}</TruncatedCell>
              <div
                className='text-muted-foreground min-w-0'
                data-table-text='secondary'
                aria-label={t('Model Regex')}
              >
                <TruncatedCell
                  tabIndex={0}
                  tooltipContent={
                    (rule.model_regex || []).join('\n') || t('All models')
                  }
                >
                  <code>
                    {(rule.model_regex || []).join(' · ') || t('All models')}
                  </code>
                </TruncatedCell>
              </div>
            </div>
          ),
        },
        {
          id: 'key-sources',
          header: t('Key Sources'),
          className: 'w-[22%]',
          cell: (rule) => (
            <div className='flex min-w-0 flex-col gap-1.5'>
              {(rule.key_sources || []).map((source) => (
                <div
                  key={`${source.type}:${source.path || source.key}`}
                  className='flex min-w-0 items-center gap-2'
                >
                  <Badge variant='secondary' className='shrink-0'>
                    {source.type}
                  </Badge>
                  <TruncatedCell
                    tabIndex={0}
                    tooltipContent={
                      source.type === 'gjson' ? source.path : source.key
                    }
                  >
                    <code>
                      {source.type === 'gjson' ? source.path : source.key}
                    </code>
                  </TruncatedCell>
                </div>
              ))}
              {!rule.key_sources?.length ? (
                <span className='text-muted-foreground'>—</span>
              ) : null}
            </div>
          ),
        },
        {
          id: 'session',
          header: t('Session behavior'),
          cell: (rule) => {
            let mode =
              rule.session_mode ||
              (rule.skip_retry_on_failure ? 'strict' : 'prefer')
            let source = t('Rule override')
            if (mode === 'inherit') {
              mode = props.globalSessionMode || 'prefer'
              source = t('Inherit global default')
            }
            if (props.enabled === false) {
              mode = 'off'
              source = t('Disabled globally')
            }
            let icon = ArrowLeftRight
            if (mode === 'strict') icon = LockKeyhole
            if (mode === 'off') icon = Link2Off
            return (
              <div className='flex min-w-0 flex-col gap-1.5'>
                <StatusBadge
                  label={policyLabel(t, mode)}
                  icon={icon}
                  variant='neutral'
                  type='text'
                  copyable={false}
                  className='h-auto whitespace-normal [&>span]:overflow-visible [&>span]:whitespace-normal'
                />
                <span
                  data-table-text='secondary'
                  className='text-muted-foreground whitespace-normal'
                >
                  {source}
                </span>
              </div>
            )
          },
        },
        {
          id: 'ttl',
          header: t('TTL'),
          className: 'w-24',
          cellClassName: 'text-muted-foreground',
          cell: (rule) => (
            <TruncatedCell tabIndex={0}>
              {rule.ttl_seconds > 0
                ? `${formatNumber(rule.ttl_seconds, locale)} ${t('seconds')}`
                : t('Global default')}
            </TruncatedCell>
          ),
        },
        {
          id: 'scope',
          header: t('Scope'),
          className: 'w-32',
          cell: (rule) => {
            const items = [
              rule.include_using_group && t('Group'),
              rule.include_model_name && t('Model'),
              rule.include_rule_name && t('Rule'),
            ].filter(Boolean) as string[]
            return (
              <StatusBadgeList
                items={items}
                max={3}
                className='flex-wrap gap-1.5'
                renderItem={(item) => <Badge variant='outline'>{item}</Badge>}
              />
            )
          },
        },
        {
          id: 'cache',
          header: t('Cache'),
          className: 'w-16 text-right',
          cellClassName: 'text-right tabular-nums',
          cell: (rule) => {
            if (!rule.include_rule_name) return t('N/A')
            if (!props.cacheStats) return '—'
            return formatNumber(
              props.cacheStats.by_rule_name?.[rule.name] || 0,
              locale
            )
          },
        },
      ]}
    />
  )
}
