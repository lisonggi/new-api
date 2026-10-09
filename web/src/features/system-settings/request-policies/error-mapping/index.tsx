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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import { SettingsCard } from '../../components/settings-card'
import {
  SettingsForm,
  SettingsSwitchField,
} from '../../components/settings-form-layout'
import { SettingsPageFormActions } from '../../components/settings-page-context'
import {
  getErrorMessageMapping,
  previewErrorMessageMapping,
  saveErrorMessageMapping,
} from './api'
import { RuleEditorDialog } from './rule-editor-dialog'
import { ErrorMappingRulesTable } from './rules-table'
import {
  ERROR_MAPPING_ENDPOINTS,
  ERROR_MAPPING_LIMITS,
  type ErrorMappingConfig,
  type ErrorMappingPreviewResult,
  type ErrorMappingRule,
} from './types'

type PreviewState = {
  message: string
  result: ErrorMappingPreviewResult
  stale: boolean
}

// Value equality for the persisted config, so the editor only treats a new
// server value as a real replacement when something actually changed.
function sameErrorMessageMappingConfig(
  a: ErrorMappingConfig,
  b: ErrorMappingConfig
) {
  if (a.enabled !== b.enabled || a.rules.length !== b.rules.length) {
    return false
  }
  return a.rules.every((rule, index) => {
    const other = b.rules[index]
    return (
      rule.id === other.id &&
      rule.name === other.name &&
      rule.enabled === other.enabled &&
      rule.keywords.length === other.keywords.length &&
      rule.keywords.every((keyword, i) => keyword === other.keywords[i]) &&
      rule.case_sensitive === other.case_sensitive &&
      rule.replacement === other.replacement
    )
  })
}

export function ErrorMessageMappingSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['error-message-mapping'],
    queryFn: getErrorMessageMapping,
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

  return <ErrorMessageMappingEditor config={query.data} />
}

function ErrorMessageMappingEditor(props: { config: ErrorMappingConfig }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [draft, setDraft] = useState<ErrorMappingConfig>(props.config)
  const [dirty, setDirty] = useState(false)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [deletingIndex, setDeletingIndex] = useState<number | null>(null)
  const [previewMessage, setPreviewMessage] = useState('')
  const [preview, setPreview] = useState<PreviewState | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)

  const baselineRef = useRef<ErrorMappingConfig>(props.config)
  const previewSeqRef = useRef(0)
  const draftSeqRef = useRef(0)

  const saveMutation = useMutation({
    mutationFn: saveErrorMessageMapping,
    // Cache updates must run even if the editor unmounted mid-save, so they
    // live on the mutation lifecycle instead of the per-call callbacks.
    onSuccess: (saved) => {
      queryClient.setQueryData(['error-message-mapping'], saved)
      void queryClient.invalidateQueries({
        queryKey: ['error-message-mapping'],
      })
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error) => handleServerError(error),
    meta: { errorToast: false },
  })

  const previewMutation = useMutation({
    mutationFn: (variables: {
      config: ErrorMappingConfig
      message: string
      seq: number
    }) =>
      previewErrorMessageMapping(variables.config, variables.message).then(
        (result) => ({ result, seq: variables.seq })
      ),
    meta: { errorToast: false },
  })

  // A background refetch must not overwrite an unsaved draft, and adopting a
  // genuinely different server config must expire any preview built on the old
  // draft.
  useEffect(() => {
    if (dirty) return
    if (sameErrorMessageMappingConfig(baselineRef.current, props.config)) return
    draftSeqRef.current += 1
    previewSeqRef.current += 1
    setPreviewLoading(false)
    setPreview((current) => (current ? { ...current, stale: true } : current))
    baselineRef.current = props.config
    setDraft(props.config)
  }, [props.config, dirty])

  // A draft or sample change makes an earlier preview stale and invalidates its
  // in-flight request. Clearing the busy flag here prevents a superseded
  // request's finally from leaving the preview button stuck disabled, and the
  // sequence guard keeps the late response from describing the older input.
  const invalidatePreview = () => {
    previewSeqRef.current += 1
    setPreviewLoading(false)
    setPreview((current) => (current ? { ...current, stale: true } : current))
  }

  const updateDraft = (
    updater: (current: ErrorMappingConfig) => ErrorMappingConfig
  ) => {
    draftSeqRef.current += 1
    invalidatePreview()
    setDraft(updater)
    setDirty(true)
  }

  const openAdd = () => {
    if (draft.rules.length >= ERROR_MAPPING_LIMITS.maxRules) return
    setEditingIndex(null)
    setEditorOpen(true)
  }

  const openEdit = (index: number) => {
    setEditingIndex(index)
    setEditorOpen(true)
  }

  const handleSaveRule = (rule: ErrorMappingRule) => {
    updateDraft((current) => {
      const rules = [...current.rules]
      if (editingIndex === null) rules.push(rule)
      else rules[editingIndex] = rule
      return { ...current, rules }
    })
  }

  const handleDelete = () => {
    if (deletingIndex === null) return
    const index = deletingIndex
    updateDraft((current) => ({
      ...current,
      rules: current.rules.filter((_, i) => i !== index),
    }))
    setDeletingIndex(null)
  }

  const handleToggle = (index: number, enabled: boolean) => {
    updateDraft((current) => ({
      ...current,
      rules: current.rules.map((rule, i) =>
        i === index ? { ...rule, enabled } : rule
      ),
    }))
  }

  const handleMove = (index: number, direction: -1 | 1) => {
    const target = index + direction
    if (target < 0 || target >= draft.rules.length) return
    updateDraft((current) => {
      const rules = [...current.rules]
      const [moved] = rules.splice(index, 1)
      rules.splice(target, 0, moved)
      return { ...current, rules }
    })
  }

  const handleToggleEnabled = (enabled: boolean) => {
    updateDraft((current) => ({ ...current, enabled }))
  }

  const handleReset = () => {
    draftSeqRef.current += 1
    previewSeqRef.current += 1
    setPreview(null)
    setPreviewLoading(false)
    setDraft(baselineRef.current)
    setDirty(false)
  }

  const handleSave = () => {
    if (draft.rules.length > ERROR_MAPPING_LIMITS.maxRules) {
      toast.error(t('Too many rules'))
      return
    }
    // Only apply the response to the draft the user submitted. Edits made while
    // the request is in flight stay on screen and keep the form dirty.
    const submittedSeq = draftSeqRef.current
    saveMutation.mutate(draft, {
      onSuccess: (saved) => {
        baselineRef.current = saved
        if (draftSeqRef.current === submittedSeq) {
          setDraft(saved)
          setDirty(false)
        }
        toast.success(t('Saved'))
      },
    })
  }

  const runPreview = () => {
    const seq = previewSeqRef.current + 1
    previewSeqRef.current = seq
    setPreviewLoading(true)
    previewMutation.mutate(
      { config: draft, message: previewMessage, seq },
      {
        onSuccess: ({ result, seq: responseSeq }) => {
          if (responseSeq !== previewSeqRef.current) return
          setPreview({ message: previewMessage, result, stale: false })
        },
        onError: (error, variables) => {
          if (variables.seq !== previewSeqRef.current) return
          handleServerError(error)
        },
        onSettled: (_data, _error, variables) => {
          if (variables.seq !== previewSeqRef.current) return
          setPreviewLoading(false)
        },
      }
    )
  }

  const editingRule =
    editingIndex === null ? null : (draft.rules[editingIndex] ?? null)

  return (
    <>
      <SettingsPageFormActions
        onSave={handleSave}
        onReset={handleReset}
        isSaving={saveMutation.isPending}
        isResetDisabled={!dirty}
      />
      <SettingsForm
        className='min-w-0'
        onSubmit={(event) => {
          event.preventDefault()
          handleSave()
        }}
      >
        <SettingsCard
          title={t('Error message mapping')}
          description={t(
            'Replace the client-facing error message of supported APIs when it contains any of the keywords. Internal logs, retries and channel health keep the original error.'
          )}
          className='shadow-none'
        >
          <SettingsSwitchField
            controlId='error-message-mapping-enabled'
            checked={draft.enabled}
            onCheckedChange={handleToggleEnabled}
            label={t('Enable error message mapping')}
            description={t(
              'When off, every supported API returns its original error message.'
            )}
          />
          <p className='text-muted-foreground mt-3 text-xs'>
            {t(
              'Only errors from the supported API endpoints are mapped. Error frames already dropped by a protocol converter cannot be rewritten, and other entries such as WebSocket, Gemini native, image and audio are not covered.'
            )}
          </p>
          <ul className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs'>
            {ERROR_MAPPING_ENDPOINTS.map((endpoint) => (
              <li key={endpoint}>
                <code>{endpoint}</code>
              </li>
            ))}
          </ul>
        </SettingsCard>

        <section
          aria-label={t('Mapping rules')}
          className='bg-card min-w-0 overflow-hidden rounded-xl border'
        >
          <div className='flex flex-wrap items-center justify-between gap-3 px-4 py-4 sm:px-5'>
            <div className='flex flex-col gap-1'>
              <div className='flex items-center gap-2'>
                <h3 className='text-base font-semibold'>
                  {t('Mapping rules')}
                </h3>
                <Badge variant='secondary' className='tabular-nums'>
                  {draft.rules.length}
                </Badge>
              </div>
              <p className='text-muted-foreground text-sm'>
                {t('The first enabled rule that matches wins.')}
              </p>
            </div>
            <Button
              variant='outline'
              onClick={openAdd}
              disabled={draft.rules.length >= ERROR_MAPPING_LIMITS.maxRules}
            >
              <Plus aria-hidden='true' data-icon='inline-start' />
              {t('Add mapping rule')}
            </Button>
          </div>
          <ErrorMappingRulesTable
            rules={draft.rules}
            onAdd={openAdd}
            onEdit={openEdit}
            onDelete={setDeletingIndex}
            onToggle={handleToggle}
            onMove={handleMove}
          />
        </section>

        <SettingsCard
          title={t('Preview')}
          description={t(
            'Preview the current draft rules against a sample client-facing error message. Preview never saves.'
          )}
          className='shadow-none'
        >
          <div className='grid gap-1.5'>
            <Label htmlFor='error-mapping-preview-message'>
              {t('Sample error message')}
            </Label>
            <Textarea
              id='error-mapping-preview-message'
              rows={3}
              value={previewMessage}
              onChange={(event) => {
                setPreviewMessage(event.target.value)
                invalidatePreview()
              }}
              aria-describedby='error-mapping-preview-message-help'
              placeholder='reasoning_content is not supported'
            />
            <p
              id='error-mapping-preview-message-help'
              className='text-muted-foreground text-xs'
            >
              {t(
                'The preview uses the text as the raw error message: it has no local request id and is not passed through protocol masking.'
              )}
            </p>
          </div>
          <div className='mt-3 flex items-center gap-2'>
            <Button
              variant='outline'
              onClick={() => void runPreview()}
              disabled={previewLoading}
              aria-busy={previewLoading}
            >
              <RefreshCw
                aria-hidden='true'
                className={cn(
                  previewLoading && 'animate-spin motion-reduce:animate-none'
                )}
              />
              {t('Run preview')}
            </Button>
          </div>
          {preview ? (
            <div
              role='status'
              className={cn(
                'mt-4 rounded-lg border p-3 text-sm',
                preview.stale && 'opacity-60'
              )}
            >
              <p className='font-medium'>
                {preview.result.matched
                  ? t('Matched a rule')
                  : t('No rule matched')}
              </p>
              {preview.result.matched ? (
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Rule id: {{id}}', { id: preview.result.rule_id })}
                </p>
              ) : null}
              <p className='mt-2'>
                <span className='text-muted-foreground'>
                  {t('Client-visible message')}:
                </span>{' '}
                <span className='font-medium'>{preview.result.message}</span>
              </p>
              {preview.stale ? (
                <p className='text-muted-foreground mt-2 text-xs'>
                  {t('The draft changed after this preview.')}
                </p>
              ) : null}
            </div>
          ) : null}
        </SettingsCard>
      </SettingsForm>

      <RuleEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        rule={editingRule}
        onSave={handleSaveRule}
      />

      {deletingIndex !== null && draft.rules[deletingIndex] ? (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setDeletingIndex(null)}
          title={t('Delete mapping rule')}
          desc={t(
            'Delete rule “{{name}}”? Save your changes to apply the removal.',
            {
              name:
                draft.rules[deletingIndex].name ||
                draft.rules[deletingIndex].keywords.join(', '),
            }
          )}
          confirmText={t('Delete mapping rule')}
          handleConfirm={handleDelete}
          destructive
        />
      ) : null}
    </>
  )
}
