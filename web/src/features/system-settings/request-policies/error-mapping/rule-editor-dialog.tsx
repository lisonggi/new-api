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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { SettingsSwitchField } from '../../components/settings-form-layout'
import {
  errorMappingRuleSchema,
  type ErrorMappingRuleFormValues,
} from './schema'
import type { ErrorMappingRule } from './types'

const RULE_FORM_ID = 'error-mapping-rule-form'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: ErrorMappingRule | null
  onSave: (rule: ErrorMappingRule) => void
}

const EMPTY_VALUES: ErrorMappingRuleFormValues = {
  name: '',
  keyword: '',
  case_sensitive: false,
  replacement: '',
}

export function RuleEditorDialog(props: Props) {
  const { t } = useTranslation()
  const isEdit = props.rule !== null
  const form = useForm<ErrorMappingRuleFormValues>({
    resolver: zodResolver(errorMappingRuleSchema),
    defaultValues: EMPTY_VALUES,
  })

  useEffect(() => {
    if (!props.open) return
    if (props.rule) {
      form.reset({
        name: props.rule.name,
        keyword: props.rule.keyword,
        case_sensitive: props.rule.case_sensitive,
        replacement: props.rule.replacement,
      })
      return
    }
    form.reset(EMPTY_VALUES)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.open, props.rule])

  const handleSave = (values: ErrorMappingRuleFormValues) => {
    props.onSave({
      id: props.rule?.id ?? crypto.randomUUID(),
      name: values.name.trim(),
      enabled: props.rule?.enabled ?? true,
      keyword: values.keyword,
      case_sensitive: values.case_sensitive,
      replacement: values.replacement,
    })
    props.onOpenChange(false)
  }

  const errors = form.formState.errors

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={isEdit ? t('Edit mapping rule') : t('Add mapping rule')}
      description={t(
        'When the client-facing error message contains the keyword, the whole message is replaced.'
      )}
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={RULE_FORM_ID}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <form
        id={RULE_FORM_ID}
        onSubmit={form.handleSubmit(handleSave)}
        className='min-w-0 space-y-4'
      >
        <div className='grid gap-1.5'>
          <Label htmlFor='error-mapping-rule-name'>{t('Name')}</Label>
          <Input
            id='error-mapping-rule-name'
            placeholder='reasoning-format'
            aria-invalid={Boolean(errors.name)}
            aria-describedby={
              errors.name ? 'error-mapping-rule-name-error' : undefined
            }
            {...form.register('name')}
          />
          {errors.name ? (
            <p
              id='error-mapping-rule-name-error'
              role='alert'
              className='text-destructive text-sm'
            >
              {t(errors.name.message ?? '')}
            </p>
          ) : null}
        </div>

        <div className='grid gap-1.5'>
          <Label required htmlFor='error-mapping-rule-keyword'>
            {t('Keyword')}
          </Label>
          <Input
            id='error-mapping-rule-keyword'
            placeholder='reasoning_content'
            aria-invalid={Boolean(errors.keyword)}
            aria-describedby={
              errors.keyword
                ? 'error-mapping-rule-keyword-help error-mapping-rule-keyword-error'
                : 'error-mapping-rule-keyword-help'
            }
            {...form.register('keyword')}
          />
          <p
            id='error-mapping-rule-keyword-help'
            className='text-muted-foreground text-xs'
          >
            {t(
              'Matching is a plain “contains” check, not a regular expression.'
            )}
          </p>
          {errors.keyword ? (
            <p
              id='error-mapping-rule-keyword-error'
              role='alert'
              className='text-destructive text-sm'
            >
              {t(errors.keyword.message ?? '')}
            </p>
          ) : null}
        </div>

        <div className='grid gap-1.5'>
          <Label required htmlFor='error-mapping-rule-replacement'>
            {t('Replacement message')}
          </Label>
          <Textarea
            id='error-mapping-rule-replacement'
            rows={3}
            aria-invalid={Boolean(errors.replacement)}
            aria-describedby={
              errors.replacement
                ? 'error-mapping-rule-replacement-error'
                : undefined
            }
            {...form.register('replacement')}
          />
          {errors.replacement ? (
            <p
              id='error-mapping-rule-replacement-error'
              role='alert'
              className='text-destructive text-sm'
            >
              {t(errors.replacement.message ?? '')}
            </p>
          ) : null}
        </div>

        <SettingsSwitchField
          controlId='error-mapping-rule-case-sensitive'
          checked={form.watch('case_sensitive')}
          onCheckedChange={(value) =>
            form.setValue('case_sensitive', value, { shouldDirty: true })
          }
          label={t('Case sensitive')}
          description={t(
            'When off, the keyword is matched after lowercasing both sides.'
          )}
        />
      </form>
    </Dialog>
  )
}
