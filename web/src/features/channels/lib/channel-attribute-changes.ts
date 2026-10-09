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
import { isValidChannelErrorRetryPolicyJSON } from './channel-error-retry'
import {
  HTTP_PROTOCOL_AUTO,
  isOptionalModelFirstResponseTimeout,
  type HttpProtocolValue,
} from './channel-form'

export type ChannelAttributeProxyMode = 'set' | 'clear'
/** Replace overwrites the value; append unions with the current one. */
export type ChannelAttributeReplaceMode = 'replace' | 'append'
/** Replace overwrites the value; merge overlays it on the current one. */
export type ChannelAttributeMergeMode = 'replace' | 'merge'

export type ChannelAttributeTimeoutValue = Record<
  string,
  Array<{ context_tokens: number; timeout_ms: number }>
>

/** The "channel settings" group shared by both batch edit dialogs. */
export type ChannelBatchSettingsChanges = {
  reasoningContentBackfill: { enabled: boolean; value: boolean }
  responsesReasoningContentBackfill: { enabled: boolean; value: boolean }
  assistantContentBackfill: { enabled: boolean; value: boolean }
  ignoreResponseModelMismatch: { enabled: boolean; value: boolean }
  thinkingToContent: { enabled: boolean; value: boolean }
  systemPrompt: { enabled: boolean; value: string }
  systemPromptOverride: { enabled: boolean; value: boolean }
  disableTaskPollingSleep: { enabled: boolean; value: boolean }
  modelFirstResponseTimeout: {
    enabled: boolean
    mode: ChannelAttributeMergeMode
    value: string
  }
  errorRetryPolicy: { enabled: boolean; value: string }
}

/**
 * The attribute changes shared by the tag batch edit dialog and the
 * selected-channels batch edit dialog. Every attribute is opt-in: an attribute
 * is only written when `enabled` is true, which keeps "only change what the
 * administrator asked for" true for both entry points.
 */
export type ChannelAttributeChanges = {
  models: { enabled: boolean; mode: ChannelAttributeReplaceMode; value: string }
  modelMapping: {
    enabled: boolean
    mode: ChannelAttributeMergeMode
    value: string
  }
  groups: {
    enabled: boolean
    mode: ChannelAttributeReplaceMode
    value: string[]
  }
  httpProtocol: { enabled: boolean; value: HttpProtocolValue }
  shards: { enabled: boolean; value: string }
  proxy: {
    enabled: boolean
    mode: ChannelAttributeProxyMode
    address: string
  }
  settings: ChannelBatchSettingsChanges
}

/** The `settings` payload of a batch edit request (only enabled keys appear). */
export type ChannelAttributeSettingsParams = {
  reasoning_content_backfill?: boolean
  responses_reasoning_content_backfill?: boolean
  assistant_content_backfill?: boolean
  ignore_response_model_mismatch?: boolean
  thinking_to_content?: boolean
  system_prompt?: string
  system_prompt_override?: boolean
  disable_task_polling_sleep?: boolean
  model_first_response_timeout?: ChannelAttributeTimeoutValue
  model_first_response_timeout_mode?: ChannelAttributeMergeMode
  error_retry_policy?: unknown
}

/** The request fields built from an enabled attribute set. */
export type ChannelAttributeParams = {
  models?: string
  models_mode?: ChannelAttributeReplaceMode
  model_mapping?: string
  model_mapping_mode?: ChannelAttributeMergeMode
  groups?: string
  groups_mode?: ChannelAttributeReplaceMode
  http_protocol?: string
  http2_connection_shards?: number
  proxy?: string
  settings?: ChannelAttributeSettingsParams
}

export function emptyChannelAttributeChanges(): ChannelAttributeChanges {
  return {
    models: { enabled: false, mode: 'replace', value: '' },
    modelMapping: { enabled: false, mode: 'replace', value: '' },
    groups: { enabled: false, mode: 'replace', value: [] },
    httpProtocol: { enabled: false, value: HTTP_PROTOCOL_AUTO },
    shards: { enabled: false, value: '1' },
    proxy: { enabled: false, mode: 'set', address: '' },
    settings: {
      reasoningContentBackfill: { enabled: false, value: true },
      responsesReasoningContentBackfill: { enabled: false, value: true },
      assistantContentBackfill: { enabled: false, value: true },
      ignoreResponseModelMismatch: { enabled: false, value: true },
      thinkingToContent: { enabled: false, value: true },
      systemPrompt: { enabled: false, value: '' },
      systemPromptOverride: { enabled: false, value: true },
      disableTaskPollingSleep: { enabled: false, value: true },
      modelFirstResponseTimeout: { enabled: false, mode: 'replace', value: '' },
      errorRetryPolicy: { enabled: false, value: '' },
    },
  }
}

/**
 * Returns the i18n key of the first validation error for an enabled attribute,
 * or null when the changes can be submitted.
 */
export function validateChannelAttributeChanges(
  changes: ChannelAttributeChanges
): string | null {
  if (changes.models.enabled && !changes.models.value.trim()) {
    return 'Model list is required'
  }
  if (changes.modelMapping.enabled && !changes.modelMapping.value.trim()) {
    return 'Model mapping is required'
  }
  if (changes.modelMapping.enabled) {
    try {
      JSON.parse(changes.modelMapping.value)
    } catch {
      return 'Model mapping must be valid JSON'
    }
  }
  if (changes.groups.enabled && changes.groups.value.length === 0) {
    return 'Select at least one group'
  }
  if (
    changes.proxy.enabled &&
    changes.proxy.mode === 'set' &&
    !changes.proxy.address.trim()
  ) {
    return 'Proxy address is required'
  }
  const timeout = changes.settings.modelFirstResponseTimeout
  if (
    timeout.enabled &&
    (!timeout.value.trim() ||
      !isOptionalModelFirstResponseTimeout(timeout.value))
  ) {
    return 'Model first response timeout must be a JSON object mapping model names to tiers'
  }
  const policy = changes.settings.errorRetryPolicy
  if (
    policy.enabled &&
    (!policy.value.trim() || !isValidChannelErrorRetryPolicyJSON(policy.value))
  ) {
    return 'Error retry policy must be a valid policy object'
  }
  return null
}

/**
 * Builds the request fields for the enabled attributes only. An attribute that
 * is not turned on never appears here, so the request can never change it.
 */
export function buildChannelAttributeParams(
  changes: ChannelAttributeChanges
): ChannelAttributeParams {
  const params: ChannelAttributeParams = {}
  if (changes.models.enabled) {
    params.models = changes.models.value.trim()
    // "replace" is the default and stays implicit, so a plain edit request is
    // unchanged from before the mode existed.
    if (changes.models.mode === 'append') {
      params.models_mode = 'append'
    }
  }
  if (changes.modelMapping.enabled) {
    params.model_mapping = changes.modelMapping.value.trim()
    if (changes.modelMapping.mode === 'merge') {
      params.model_mapping_mode = 'merge'
    }
  }
  if (changes.groups.enabled) {
    params.groups = changes.groups.value.join(',')
    if (changes.groups.mode === 'append') {
      params.groups_mode = 'append'
    }
  }
  if (changes.httpProtocol.enabled) {
    params.http_protocol = changes.httpProtocol.value
  }
  if (changes.shards.enabled) {
    params.http2_connection_shards = Number(changes.shards.value)
  }
  if (changes.proxy.enabled) {
    params.proxy =
      changes.proxy.mode === 'clear' ? '' : changes.proxy.address.trim()
  }

  const settings: ChannelAttributeSettingsParams = {}
  const s = changes.settings
  if (s.reasoningContentBackfill.enabled) {
    settings.reasoning_content_backfill = s.reasoningContentBackfill.value
  }
  if (s.responsesReasoningContentBackfill.enabled) {
    settings.responses_reasoning_content_backfill =
      s.responsesReasoningContentBackfill.value
  }
  if (s.assistantContentBackfill.enabled) {
    settings.assistant_content_backfill = s.assistantContentBackfill.value
  }
  if (s.ignoreResponseModelMismatch.enabled) {
    settings.ignore_response_model_mismatch =
      s.ignoreResponseModelMismatch.value
  }
  if (s.thinkingToContent.enabled) {
    settings.thinking_to_content = s.thinkingToContent.value
  }
  if (s.systemPrompt.enabled) {
    settings.system_prompt = s.systemPrompt.value
  }
  if (s.systemPromptOverride.enabled) {
    settings.system_prompt_override = s.systemPromptOverride.value
  }
  if (s.disableTaskPollingSleep.enabled) {
    settings.disable_task_polling_sleep = s.disableTaskPollingSleep.value
  }
  if (s.modelFirstResponseTimeout.enabled) {
    settings.model_first_response_timeout = JSON.parse(
      s.modelFirstResponseTimeout.value
    ) as ChannelAttributeTimeoutValue
    if (s.modelFirstResponseTimeout.mode === 'merge') {
      settings.model_first_response_timeout_mode = 'merge'
    }
  }
  if (s.errorRetryPolicy.enabled) {
    settings.error_retry_policy = JSON.parse(s.errorRetryPolicy.value)
  }
  if (Object.keys(settings).length > 0) {
    params.settings = settings
  }
  return params
}
