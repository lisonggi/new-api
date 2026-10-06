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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { TruncatedCell } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { RuleListTable } from '@/components/rule-list-table'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSwitchField } from '@/features/system-settings/components/settings-form-layout'

import {
  CHANNEL_ERROR_RETRY_FIELDS,
  CHANNEL_ERROR_RETRY_LIMITS,
  CHANNEL_ERROR_RETRY_OPERATORS,
  channelErrorRetryRuleId,
  newChannelErrorRetryCondition,
  newChannelErrorRetryRule,
  parseChannelErrorRetryPolicyJSON,
  parseStatusCodesInput,
  serializeChannelErrorRetryPolicy,
  type ChannelErrorRetryCondition,
  type ChannelErrorRetryField,
  type ChannelErrorRetryOperator,
  type ChannelErrorRetryPolicy,
  type ChannelErrorRetryRule,
} from '../../lib/channel-error-retry'

interface EditorProps {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

export function ErrorRetryPolicyEditor(props: EditorProps) {
  const { t } = useTranslation()
  const parsed = useMemo(
    () => parseChannelErrorRetryPolicyJSON(props.value),
    [props.value]
  )
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteIndex, setDeleteIndex] = useState<number | null>(null)

  if (parsed.error) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Stored error retry policy is invalid')}</AlertTitle>
        <AlertDescription className='space-y-2'>
          <p>
            {t(
              'The saved policy cannot be parsed and will block saving until fixed or cleared. It is never silently replaced.'
            )}
          </p>
          <p className='font-mono text-xs'>{parsed.error}</p>
          <Textarea
            aria-label={t('Raw error retry policy JSON')}
            rows={6}
            disabled={props.disabled}
            value={props.value}
            onChange={(event) => props.onChange(event.target.value)}
          />
          {props.disabled ? null : (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => props.onChange('')}
            >
              {t('Clear invalid policy')}
            </Button>
          )}
        </AlertDescription>
      </Alert>
    )
  }

  const policy: ChannelErrorRetryPolicy = parsed.policy ?? {
    enabled: false,
    rules: [],
  }
  const rules = policy.rules ?? []

  const emit = (next: ChannelErrorRetryPolicy) => {
    props.onChange(serializeChannelErrorRetryPolicy(next))
  }

  const handleSaveRule = (rule: ChannelErrorRetryRule) => {
    const nextRules = [...rules]
    if (editingIndex === null) {
      nextRules.push(rule)
    } else {
      nextRules[editingIndex] = rule
    }
    emit({ ...policy, rules: nextRules })
    setEditingIndex(null)
  }

  const handleMove = (index: number, direction: -1 | 1) => {
    const target = index + direction
    if (target < 0 || target >= rules.length) return
    const nextRules = [...rules]
    ;[nextRules[index], nextRules[target]] = [
      nextRules[target],
      nextRules[index],
    ]
    emit({ ...policy, rules: nextRules })
  }

  const handleDelete = () => {
    if (deleteIndex === null) return
    emit({
      ...policy,
      rules: rules.filter((_rule, index) => index !== deleteIndex),
    })
    setDeleteIndex(null)
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='text-muted-foreground text-xs'>
        {t(
          'The first enabled matching rule decides retry or stop. A miss inherits the global decision.'
        )}
      </div>

      <SettingsSwitchField
        controlId='channel-error-retry-enabled'
        checked={policy.enabled}
        disabled={props.disabled}
        onCheckedChange={(checked) => emit({ ...policy, enabled: checked })}
        label={t('Enable error retry judgment')}
        description={t(
          'When off, every request keeps using the global retry decision.'
        )}
      />

      <RuleListTable
        ariaLabel={t('Error retry rules table')}
        tableClassName='min-w-[760px] [&_th]:px-4 [&_th]:text-muted-foreground [&_td]:px-4 [&_td]:py-4'
        rules={rules}
        getRowKey={(rule) => rule.id}
        disabled={props.disabled}
        empty={{
          title: t('No error retry rules yet'),
          description: t(
            'Add a rule to classify an upstream error as retryable or final for this channel.'
          ),
          actionLabel: t('Add error retry rule'),
          actionIcon: Plus,
          minHeightClassName: 'min-h-48',
          onAction: () => {
            setEditingIndex(null)
            setDialogOpen(true)
          },
        }}
        enabled={{
          isEnabled: (rule) => rule.enabled,
          onToggle: (rule, index, enabled) => {
            const nextRules = [...rules]
            nextRules[index] = { ...rule, enabled }
            emit({ ...policy, rules: nextRules })
          },
        }}
        actions={{
          name: (rule) => rule.name || rule.id,
          editLabel: t('Edit error retry rule'),
          deleteLabel: t('Delete error retry rule'),
          onEdit: (_rule, index) => {
            setEditingIndex(index)
            setDialogOpen(true)
          },
          onDelete: (_rule, index) => setDeleteIndex(index),
          onMove: handleMove,
        }}
        columns={[
          {
            id: 'rule',
            header: t('Rule'),
            cell: (rule, index) => (
              <div className='flex min-w-0 flex-col gap-1'>
                <TruncatedCell tabIndex={0}>
                  {rule.name || rule.id}
                </TruncatedCell>
                <span className='text-muted-foreground text-xs'>
                  {t('Priority {{priority}}', { priority: index + 1 })}
                </span>
              </div>
            ),
          },
          {
            id: 'match',
            header: t('Match'),
            cell: (rule) => (
              <TruncatedCell
                tabIndex={0}
                tooltipContent={describeRuleMatch(rule, t)}
              >
                {describeRuleMatch(rule, t)}
              </TruncatedCell>
            ),
          },
          {
            id: 'action',
            header: t('Action'),
            className: 'w-32',
            cell: (rule) => (
              <Badge variant={rule.action === 'stop' ? 'secondary' : 'outline'}>
                {rule.action === 'stop'
                  ? t('Stop retrying')
                  : t('Allow retrying')}
              </Badge>
            ),
          },
        ]}
      />

      {rules.length > 0 ? (
        <Button
          type='button'
          variant='outline'
          className='self-start'
          disabled={props.disabled}
          onClick={() => {
            setEditingIndex(null)
            setDialogOpen(true)
          }}
        >
          <Plus aria-hidden='true' />
          {t('Add error retry rule')}
        </Button>
      ) : null}

      <ErrorRetryRuleDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        rule={editingIndex === null ? null : rules[editingIndex]}
        onSave={handleSaveRule}
      />

      <ConfirmDialog
        open={deleteIndex !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteIndex(null)
        }}
        title={t('Delete error retry rule?')}
        desc={t('This removes the rule from the channel policy.')}
        destructive
        handleConfirm={handleDelete}
      />
    </div>
  )
}

function describeRuleMatch(
  rule: ChannelErrorRetryRule,
  t: (key: string) => string
): string {
  const parts: string[] = []
  if (rule.status_codes && rule.status_codes.length > 0) {
    parts.push(`HTTP ${rule.status_codes.join(', ')}`)
  }
  for (const condition of rule.conditions ?? []) {
    parts.push(
      `${condition.field} ${condition.operator} "${condition.value}"${
        condition.case_sensitive ? '' : ` (${t('case insensitive')})`
      }`
    )
  }
  return parts.join(' · ')
}

interface RuleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: ChannelErrorRetryRule | null
  onSave: (rule: ChannelErrorRetryRule) => void
}

interface ConditionDraft extends ChannelErrorRetryCondition {
  key: string
}

function ErrorRetryRuleDialog(props: RuleDialogProps) {
  const { t } = useTranslation()
  const isEdit = props.rule !== null
  const [name, setName] = useState('')
  const [action, setAction] = useState<'retry' | 'stop'>('retry')
  const [statusInput, setStatusInput] = useState('')
  const [conditions, setConditions] = useState<ConditionDraft[]>([])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!props.open) return
    if (props.rule) {
      setName(props.rule.name ?? '')
      setAction(props.rule.action)
      setStatusInput((props.rule.status_codes ?? []).join(', '))
      setConditions(
        (props.rule.conditions ?? []).map((condition) => ({
          ...condition,
          key: channelErrorRetryRuleId(),
        }))
      )
    } else {
      setName('')
      setAction('retry')
      setStatusInput('')
      setConditions([])
    }
    setError(null)
  }, [props.open, props.rule])

  const handleSave = () => {
    const { codes, invalidTokens } = parseStatusCodesInput(statusInput)
    if (invalidTokens.length > 0) {
      setError(t('HTTP status codes must be integers between 100 and 599.'))
      return
    }
    if (codes.length > CHANNEL_ERROR_RETRY_LIMITS.statusCodesPerRule) {
      setError(t('Too many HTTP status codes.'))
      return
    }
    if (conditions.length > CHANNEL_ERROR_RETRY_LIMITS.conditionsPerRule) {
      setError(t('Too many conditions.'))
      return
    }
    if (conditions.some((condition) => condition.value.trim() === '')) {
      setError(t('Every condition needs a value.'))
      return
    }
    if (codes.length === 0 && conditions.length === 0) {
      setError(t('Add at least one HTTP status code or condition.'))
      return
    }
    if ([...name].length > CHANNEL_ERROR_RETRY_LIMITS.nameRunes) {
      setError(t('Rule name is too long.'))
      return
    }
    props.onSave({
      id: props.rule?.id ?? newChannelErrorRetryRule().id,
      name: name.trim(),
      enabled: props.rule?.enabled ?? true,
      action,
      status_codes: codes,
      conditions: conditions.map((condition) => ({
        field: condition.field,
        operator: condition.operator,
        value: condition.value,
        case_sensitive: condition.case_sensitive,
      })),
    })
    props.onOpenChange(false)
  }

  const updateCondition = (
    index: number,
    patch: Partial<ChannelErrorRetryCondition>
  ) => {
    setConditions((current) =>
      current.map((condition, i) =>
        i === index ? { ...condition, ...patch } : condition
      )
    )
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEdit ? t('Edit error retry rule') : t('Add error retry rule')}
      description={t(
        'The status code is the value after status-code mapping. The message is the text before display mapping.'
      )}
      contentClassName='sm:max-w-2xl'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleSave}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='min-w-0 space-y-4'>
        <div className='grid gap-1.5'>
          <Label htmlFor='channel-error-retry-rule-name'>{t('Name')}</Label>
          <Input
            id='channel-error-retry-rule-name'
            value={name}
            placeholder={t('Supplier balance')}
            onChange={(event) => setName(event.target.value)}
          />
        </div>

        <div className='grid gap-1.5'>
          <Label htmlFor='channel-error-retry-rule-action'>{t('Action')}</Label>
          <Select
            items={[
              { value: 'retry', label: t('Allow retrying') },
              { value: 'stop', label: t('Stop retrying') },
            ]}
            value={action}
            onValueChange={(value) =>
              setAction(value === 'stop' ? 'stop' : 'retry')
            }
          >
            <SelectTrigger id='channel-error-retry-rule-action'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='retry'>{t('Allow retrying')}</SelectItem>
                <SelectItem value='stop'>{t('Stop retrying')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div className='grid gap-1.5'>
          <Label htmlFor='channel-error-retry-rule-status'>
            {t('HTTP status codes (after status-code mapping)')}
          </Label>
          <Input
            id='channel-error-retry-rule-status'
            value={statusInput}
            placeholder='400, 500'
            onChange={(event) => setStatusInput(event.target.value)}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Comma separated. Leave empty to match any status code.')}
          </p>
        </div>

        <div className='space-y-2'>
          <div className='flex items-center justify-between'>
            <Label>{t('Conditions')}</Label>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() =>
                setConditions((current) => [
                  ...current,
                  {
                    ...newChannelErrorRetryCondition(),
                    key: channelErrorRetryRuleId(),
                  },
                ])
              }
            >
              <Plus aria-hidden='true' />
              {t('Add condition')}
            </Button>
          </div>
          {conditions.length === 0 ? (
            <p className='text-muted-foreground text-xs'>
              {t('Conditions are combined with AND.')}
            </p>
          ) : (
            conditions.map((condition, index) => (
              <div
                key={condition.key}
                className='grid items-end gap-2 sm:grid-cols-[1fr_1fr_2fr_auto_auto]'
              >
                <Select
                  items={CHANNEL_ERROR_RETRY_FIELDS.map((field) => ({
                    value: field,
                    label: t(fieldLabelKey(field)),
                  }))}
                  value={condition.field}
                  onValueChange={(value) =>
                    updateCondition(index, {
                      field: value as ChannelErrorRetryField,
                    })
                  }
                >
                  <SelectTrigger
                    aria-label={t('Condition field {{index}}', {
                      index: index + 1,
                    })}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {CHANNEL_ERROR_RETRY_FIELDS.map((field) => (
                        <SelectItem key={field} value={field}>
                          {t(fieldLabelKey(field))}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>

                <Select
                  items={CHANNEL_ERROR_RETRY_OPERATORS.map((operator) => ({
                    value: operator,
                    label: t(operatorLabelKey(operator)),
                  }))}
                  value={condition.operator}
                  onValueChange={(value) =>
                    updateCondition(index, {
                      operator: value as ChannelErrorRetryOperator,
                    })
                  }
                >
                  <SelectTrigger
                    aria-label={t('Condition operator {{index}}', {
                      index: index + 1,
                    })}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {CHANNEL_ERROR_RETRY_OPERATORS.map((operator) => (
                        <SelectItem key={operator} value={operator}>
                          {t(operatorLabelKey(operator))}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>

                <Input
                  aria-label={t('Condition value {{index}}', {
                    index: index + 1,
                  })}
                  value={condition.value}
                  onChange={(event) =>
                    updateCondition(index, { value: event.target.value })
                  }
                />

                <div className='flex items-center gap-2 pb-2'>
                  <Switch
                    checked={condition.case_sensitive}
                    aria-label={t('Case sensitive')}
                    onCheckedChange={(checked) =>
                      updateCondition(index, { case_sensitive: checked })
                    }
                  />
                  <span className='text-xs'>{t('Case')}</span>
                </div>

                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Remove condition {{index}}', {
                    index: index + 1,
                  })}
                  onClick={() =>
                    setConditions((current) =>
                      current.filter((_c, i) => i !== index)
                    )
                  }
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              </div>
            ))
          )}
        </div>

        {error ? (
          <p role='alert' className='text-destructive text-sm'>
            {error}
          </p>
        ) : null}
      </div>
    </Dialog>
  )
}

function fieldLabelKey(field: ChannelErrorRetryField): string {
  switch (field) {
    case 'message':
      return 'Retry judgment message (before display mapping)'
    case 'code':
      return 'Standardized error code'
    case 'type':
      return 'Standardized error type'
  }
}

function operatorLabelKey(operator: ChannelErrorRetryOperator): string {
  switch (operator) {
    case 'equals':
      return 'Equals'
    case 'contains':
      return 'Contains'
    case 'not_contains':
      return 'Does not contain'
  }
}
