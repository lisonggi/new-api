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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Eye } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { RedemptionSuccessDialog } from '@/components/redemption-success-dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import { SettingsCard } from '../../components/settings-card'
import {
  SettingsForm,
  SettingsSwitchField,
} from '../../components/settings-form-layout'
import { SettingsPageFormActions } from '../../components/settings-page-context'
import { getRedemptionSuccessDialog, saveRedemptionSuccessDialog } from './api'
import {
  getRedemptionDialogSchema,
  type RedemptionDialogFormValues,
} from './lib/schema'
import {
  REDEMPTION_DIALOG_LIMITS,
  type RedemptionSuccessDialogConfig,
} from './types'

// Value equality for the persisted config, so the editor only treats a new
// server value as a real replacement when something actually changed.
function sameRedemptionDialogConfig(
  a: RedemptionSuccessDialogConfig,
  b: RedemptionSuccessDialogConfig
) {
  return (
    a.enabled === b.enabled &&
    a.title === b.title &&
    a.content === b.content &&
    a.close_button_text === b.close_button_text
  )
}

export function RedemptionSuccessDialogSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['redemption-success-dialog'],
    queryFn: getRedemptionSuccessDialog,
    staleTime: Infinity,
  })

  if (query.isPending) return <LoadingState />
  // Only a failed *initial* load hides the editor. A background refetch failure
  // keeps the cached config and the user's unsaved draft on screen.
  if (query.isError && query.data === undefined) {
    return (
      <ErrorState
        title={t('Failed to load settings')}
        onRetry={() => void query.refetch()}
      />
    )
  }
  if (query.data === undefined) return <LoadingState />

  return <RedemptionSuccessDialogEditor config={query.data} />
}

function RedemptionSuccessDialogEditor(props: {
  config: RedemptionSuccessDialogConfig
}) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const schema = useMemo(
    () => getRedemptionDialogSchema(t, (value) => formatNumber(value, locale)),
    [t, locale]
  )
  const form = useForm<RedemptionDialogFormValues>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues: props.config,
  })
  const [previewOpen, setPreviewOpen] = useState(false)
  // Last saved document. Draft dirtiness is measured against this instead of
  // RHF's isDirty, which is relative to the values captured at mount and would
  // mis-handle edits made after a save.
  const [baseline, setBaseline] = useState<RedemptionSuccessDialogConfig>(
    props.config
  )
  const values = form.watch()
  const dirty = !sameRedemptionDialogConfig(values, baseline)
  const hasErrors = Object.keys(form.formState.errors).length > 0
  // Set synchronously before the request starts so a background refetch that
  // lands while the save is in flight cannot replace the draft.
  const pendingSaveRef = useRef(false)

  const saveMutation = useMutation({
    mutationFn: saveRedemptionSuccessDialog,
    // Cache updates must run even if the editor unmounted mid-save, so they
    // live on the mutation lifecycle instead of the per-call callbacks.
    onSuccess: (saved) => {
      pendingSaveRef.current = false
      queryClient.setQueryData(['redemption-success-dialog'], saved)
      void queryClient.invalidateQueries({
        queryKey: ['redemption-success-dialog'],
      })
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error) => {
      pendingSaveRef.current = false
      handleServerError(error)
    },
    meta: { errorToast: false },
  })

  // Adopt an external saved value only when no save is in flight and the draft
  // still matches our baseline (no unsaved edits). A background refetch must
  // not overwrite a draft, and an edit back to the old baseline is still an
  // unsaved edit.
  useEffect(() => {
    if (pendingSaveRef.current) return
    const current = form.getValues()
    if (!sameRedemptionDialogConfig(current, baseline)) return
    if (sameRedemptionDialogConfig(current, props.config)) return
    setBaseline(props.config)
    form.reset(props.config)
  }, [props.config, baseline, form])

  const onSubmit = (submitted: RedemptionDialogFormValues) => {
    pendingSaveRef.current = true
    saveMutation.mutate(submitted, {
      onSuccess: (saved) => {
        setBaseline(saved)
        // Only replace the values the user submitted. Edits made while the
        // request was in flight stay on screen and keep the draft dirty.
        if (sameRedemptionDialogConfig(form.getValues(), submitted)) {
          form.reset(saved)
        }
        toast.success(t('Saved'))
      },
    })
  }

  return (
    <>
      <Form {...form}>
        <SettingsPageFormActions
          onSave={form.handleSubmit(onSubmit)}
          onReset={() => form.reset(baseline)}
          isSaving={saveMutation.isPending}
          isSaveDisabled={hasErrors}
          isResetDisabled={!dirty}
        />
        <SettingsForm
          className='min-w-0'
          onSubmit={form.handleSubmit(onSubmit)}
        >
          <SettingsCard
            title={t('Redemption success dialog')}
            description={t(
              'Show a dialog after a redemption code is successfully redeemed. The content is rendered as Markdown.'
            )}
            className='shadow-none'
          >
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchField
                  controlId='redemption-dialog-enabled'
                  checked={field.value}
                  onCheckedChange={(checked) => {
                    field.onChange(checked)
                    // Enabling the dialog makes the visible fields required, so
                    // revalidate the whole form instead of only this field.
                    void form.trigger()
                  }}
                  label={t('Enable redemption success dialog')}
                  description={t(
                    'When off, a successful redemption keeps the original success message.'
                  )}
                />
              )}
            />

            <FormField
              control={form.control}
              name='title'
              render={({ field }) => (
                <FormItem className='mt-5'>
                  <FormLabel>{t('Dialog title')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                  <p
                    className='text-muted-foreground text-xs tabular-nums'
                    aria-hidden='true'
                  >
                    {t('{{current}}/{{max}} characters', {
                      current: formatNumber([...values.title].length, locale),
                      max: formatNumber(
                        REDEMPTION_DIALOG_LIMITS.maxTitleLength,
                        locale
                      ),
                    })}
                  </p>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='content'
              render={({ field }) => (
                <FormItem className='mt-5'>
                  <FormLabel>{t('Dialog content (Markdown)')}</FormLabel>
                  <FormControl>
                    <Textarea rows={10} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'The content is rendered as Markdown: headings, bold, italics, lists, quotes, links, images, code and basic tables are supported.'
                    )}
                  </FormDescription>
                  <FormMessage />
                  <p
                    className='text-muted-foreground text-xs tabular-nums'
                    aria-hidden='true'
                  >
                    {t('{{current}}/{{max}} characters', {
                      current: formatNumber([...values.content].length, locale),
                      max: formatNumber(
                        REDEMPTION_DIALOG_LIMITS.maxContentLength,
                        locale
                      ),
                    })}
                  </p>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='close_button_text'
              render={({ field }) => (
                <FormItem className='mt-5'>
                  <FormLabel>{t('Close button text')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                  <p
                    className='text-muted-foreground text-xs tabular-nums'
                    aria-hidden='true'
                  >
                    {t('{{current}}/{{max}} characters', {
                      current: formatNumber(
                        [...values.close_button_text].length,
                        locale
                      ),
                      max: formatNumber(
                        REDEMPTION_DIALOG_LIMITS.maxCloseButtonLength,
                        locale
                      ),
                    })}
                  </p>
                </FormItem>
              )}
            />

            <div className='mt-5'>
              <Button
                type='button'
                variant='outline'
                onClick={() => setPreviewOpen(true)}
              >
                <Eye aria-hidden='true' data-icon='inline-start' />
                {t('Preview dialog')}
              </Button>
            </div>
          </SettingsCard>
        </SettingsForm>
      </Form>

      <RedemptionSuccessDialog
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        dialog={{
          title: values.title,
          content: values.content,
          closeButtonText: values.close_button_text,
        }}
        preview
      />
    </>
  )
}
