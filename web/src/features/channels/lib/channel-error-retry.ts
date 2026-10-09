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

// Mirror the backend channel error retry policy limits and enums. The editor
// and the form schema share this module so the frontend never emits a policy
// the backend will reject.

export const CHANNEL_ERROR_RETRY_ACTIONS = ['retry', 'stop'] as const
export const CHANNEL_ERROR_RETRY_FIELDS = ['message', 'code', 'type'] as const
export const CHANNEL_ERROR_RETRY_OPERATORS = [
  'equals',
  'contains',
  'not_contains',
] as const

export const CHANNEL_ERROR_RETRY_LIMITS = {
  rules: 32,
  conditionsPerRule: 8,
  statusCodesPerRule: 500,
  statusCodeMin: 100,
  statusCodeMax: 599,
  idBytes: 64,
  nameRunes: 64,
  valueRunes: 512,
  policyBytes: 32 * 1024,
} as const

export type ChannelErrorRetryAction =
  (typeof CHANNEL_ERROR_RETRY_ACTIONS)[number]
export type ChannelErrorRetryField = (typeof CHANNEL_ERROR_RETRY_FIELDS)[number]
export type ChannelErrorRetryOperator =
  (typeof CHANNEL_ERROR_RETRY_OPERATORS)[number]

export interface ChannelErrorRetryCondition {
  field: ChannelErrorRetryField
  operator: ChannelErrorRetryOperator
  value: string
  case_sensitive: boolean
}

export interface ChannelErrorRetryRule {
  id: string
  name?: string
  enabled: boolean
  action: ChannelErrorRetryAction
  status_codes?: number[]
  conditions?: ChannelErrorRetryCondition[]
}

export interface ChannelErrorRetryPolicy {
  enabled: boolean
  rules?: ChannelErrorRetryRule[]
}

export interface ParsedChannelErrorRetryPolicy {
  policy: ChannelErrorRetryPolicy | null
  error: string | null
}

const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

export function channelErrorRetryRuleId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return `rule-${crypto.randomUUID().slice(0, 8)}`
  }
  return `rule-${Math.random().toString(36).slice(2, 10)}`
}

export function newChannelErrorRetryRule(): ChannelErrorRetryRule {
  return {
    id: channelErrorRetryRuleId(),
    name: '',
    enabled: true,
    action: 'retry',
    status_codes: [],
    conditions: [],
  }
}

export function newChannelErrorRetryCondition(): ChannelErrorRetryCondition {
  return {
    field: 'message',
    operator: 'contains',
    value: '',
    case_sensitive: false,
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isAction(value: unknown): value is ChannelErrorRetryAction {
  return (
    typeof value === 'string' &&
    (CHANNEL_ERROR_RETRY_ACTIONS as readonly string[]).includes(value)
  )
}

function isField(value: unknown): value is ChannelErrorRetryField {
  return (
    typeof value === 'string' &&
    (CHANNEL_ERROR_RETRY_FIELDS as readonly string[]).includes(value)
  )
}

function isOperator(value: unknown): value is ChannelErrorRetryOperator {
  return (
    typeof value === 'string' &&
    (CHANNEL_ERROR_RETRY_OPERATORS as readonly string[]).includes(value)
  )
}

function countRunes(value: string): number {
  return [...value].length
}

// hasDuplicateJSONKeys reports whether any object in the raw JSON repeats a key.
// JSON.parse silently keeps the last duplicate, so the strict parser must detect
// duplicates itself to match the backend schema.
function hasDuplicateJSONKeys(raw: string): boolean {
  const stack: Array<Set<string> | null> = []
  let i = 0
  const n = raw.length
  while (i < n) {
    const ch = raw[i]
    if (ch === '"') {
      let j = i + 1
      let value = ''
      while (j < n) {
        const c = raw[j]
        if (c === '\\') {
          value += raw[j + 1] ?? ''
          j += 2
          continue
        }
        if (c === '"') break
        value += c
        j += 1
      }
      j += 1
      let k = j
      while (k < n && /\s/.test(raw[k])) k += 1
      const frame = stack.at(-1) ?? null
      if (raw[k] === ':' && frame) {
        if (frame.has(value)) return true
        frame.add(value)
      }
      i = j
      continue
    }
    if (ch === '{') {
      stack.push(new Set())
    } else if (ch === '[') {
      stack.push(null)
    } else if (ch === '}' || ch === ']') {
      stack.pop()
    }
    i += 1
  }
  return false
}

const CONDITION_KEYS = ['field', 'operator', 'value', 'case_sensitive']
const RULE_KEYS = [
  'id',
  'name',
  'enabled',
  'action',
  'status_codes',
  'conditions',
]
const POLICY_KEYS = ['enabled', 'rules']

function hasUnknownKeys(
  value: Record<string, unknown>,
  allowed: readonly string[]
): boolean {
  return Object.keys(value).some((key) => !allowed.includes(key))
}

// skipJSONValue returns the index just past the JSON value that starts at
// start. It understands strings, escapes and nested objects/arrays so a raw
// top-level field can be sliced out without JSON.parse collapsing duplicates.
function skipJSONValue(raw: string, start: number): number {
  let i = start
  const n = raw.length
  while (i < n && /\s/.test(raw[i])) i += 1
  const ch = raw[i]
  if (ch === '"') {
    i += 1
    while (i < n) {
      if (raw[i] === '\\') {
        i += 2
        continue
      }
      if (raw[i] === '"') {
        i += 1
        break
      }
      i += 1
    }
    return i
  }
  if (ch === '{' || ch === '[') {
    const close = ch === '{' ? '}' : ']'
    i += 1
    while (i < n) {
      const c = raw[i]
      if (/\s/.test(c) || c === ',' || c === ':') {
        i += 1
        continue
      }
      if (c === close) return i + 1
      i = skipJSONValue(raw, i)
    }
    return i
  }
  while (i < n && !/[\s,\]}]/.test(raw[i])) i += 1
  return i
}

// extractSettingFieldRawValue returns the raw JSON text of a top-level field in
// a channel setting string, preserving duplicate keys and unknown nested fields
// that JSON.parse would collapse. ambiguous is true when the field appears more
// than once or under a non-canonical spelling, which the form must surface as
// invalid instead of normalizing.
export function extractSettingFieldRawValue(
  setting: string | undefined,
  key: string
): { value: string | null; ambiguous: boolean } {
  const result: { value: string | null; ambiguous: boolean } = {
    value: null,
    ambiguous: false,
  }
  if (!setting) return result
  let i = 0
  const n = setting.length
  while (i < n && /\s/.test(setting[i])) i += 1
  if (setting[i] !== '{') return result
  i += 1
  let matches = 0
  while (i < n) {
    while (i < n && /[\s,]/.test(setting[i])) i += 1
    if (i >= n || setting[i] !== '"') break
    const keyStart = i
    i = skipJSONValue(setting, i)
    let parsedKey: string
    try {
      parsedKey = JSON.parse(setting.slice(keyStart, i)) as string
    } catch {
      break
    }
    while (i < n && /\s/.test(setting[i])) i += 1
    if (setting[i] !== ':') break
    i += 1
    const valueStart = i
    i = skipJSONValue(setting, i)
    if (parsedKey === key) {
      matches += 1
      result.value = setting.slice(valueStart, i).trim()
      if (matches > 1) result.ambiguous = true
    } else if (parsedKey.toLowerCase() === key.toLowerCase()) {
      result.ambiguous = true
    }
  }
  return result
}

function validateCondition(
  value: unknown,
  path: string
): { condition: ChannelErrorRetryCondition; error: string | null } {
  const empty: ChannelErrorRetryCondition = {
    field: 'message',
    operator: 'contains',
    value: '',
    case_sensitive: false,
  }
  if (!isRecord(value)) {
    return { condition: empty, error: `${path}: not an object` }
  }
  if (hasUnknownKeys(value, CONDITION_KEYS)) {
    return { condition: empty, error: `${path}: unknown field` }
  }
  if (!isField(value.field)) {
    return { condition: empty, error: `${path}.field: invalid` }
  }
  if (!isOperator(value.operator)) {
    return { condition: empty, error: `${path}.operator: invalid` }
  }
  if (typeof value.value !== 'string' || value.value.trim() === '') {
    return { condition: empty, error: `${path}.value: required` }
  }
  if (value.value.includes('\u0000')) {
    return { condition: empty, error: `${path}.value: invalid` }
  }
  if (countRunes(value.value) > CHANNEL_ERROR_RETRY_LIMITS.valueRunes) {
    return { condition: empty, error: `${path}.value: too long` }
  }
  const caseSensitive =
    value.case_sensitive === undefined ? false : value.case_sensitive
  if (typeof caseSensitive !== 'boolean') {
    return { condition: empty, error: `${path}.case_sensitive: not a bool` }
  }
  return {
    condition: {
      field: value.field,
      operator: value.operator,
      value: value.value,
      case_sensitive: caseSensitive,
    },
    error: null,
  }
}

function validateRule(
  value: unknown,
  path: string
): { rule: ChannelErrorRetryRule; error: string | null } {
  const empty = newChannelErrorRetryRule()
  if (!isRecord(value)) {
    return { rule: empty, error: `${path}: not an object` }
  }
  if (hasUnknownKeys(value, RULE_KEYS)) {
    return { rule: empty, error: `${path}: unknown field` }
  }
  if (typeof value.id !== 'string' || !ID_PATTERN.test(value.id)) {
    return { rule: empty, error: `${path}.id: invalid` }
  }
  if (!isAction(value.action)) {
    return { rule: empty, error: `${path}.action: invalid` }
  }
  if (value.name !== undefined && typeof value.name !== 'string') {
    return { rule: empty, error: `${path}.name: not a string` }
  }
  if (typeof value.name === 'string' && value.name.includes('\u0000')) {
    return { rule: empty, error: `${path}.name: invalid` }
  }
  if (
    typeof value.name === 'string' &&
    countRunes(value.name) > CHANNEL_ERROR_RETRY_LIMITS.nameRunes
  ) {
    return { rule: empty, error: `${path}.name: too long` }
  }
  const enabled = value.enabled === undefined ? false : value.enabled
  if (typeof enabled !== 'boolean') {
    return { rule: empty, error: `${path}.enabled: not a bool` }
  }

  const statusCodes: number[] = []
  if (value.status_codes !== undefined) {
    if (!Array.isArray(value.status_codes)) {
      return { rule: empty, error: `${path}.status_codes: not an array` }
    }
    if (
      value.status_codes.length > CHANNEL_ERROR_RETRY_LIMITS.statusCodesPerRule
    ) {
      return { rule: empty, error: `${path}.status_codes: too many` }
    }
    const seen = new Set<number>()
    for (const code of value.status_codes) {
      if (
        !Number.isInteger(code) ||
        code < CHANNEL_ERROR_RETRY_LIMITS.statusCodeMin ||
        code > CHANNEL_ERROR_RETRY_LIMITS.statusCodeMax
      ) {
        return { rule: empty, error: `${path}.status_codes: invalid code` }
      }
      if (seen.has(code)) {
        return { rule: empty, error: `${path}.status_codes: duplicate` }
      }
      seen.add(code)
      statusCodes.push(code)
    }
  }

  const conditions: ChannelErrorRetryCondition[] = []
  if (value.conditions !== undefined) {
    if (!Array.isArray(value.conditions)) {
      return { rule: empty, error: `${path}.conditions: not an array` }
    }
    if (
      value.conditions.length > CHANNEL_ERROR_RETRY_LIMITS.conditionsPerRule
    ) {
      return { rule: empty, error: `${path}.conditions: too many` }
    }
    for (let index = 0; index < value.conditions.length; index += 1) {
      const parsed = validateCondition(
        value.conditions[index],
        `${path}.conditions[${index}]`
      )
      if (parsed.error) {
        return { rule: empty, error: parsed.error }
      }
      conditions.push(parsed.condition)
    }
  }

  if (statusCodes.length === 0 && conditions.length === 0) {
    return { rule: empty, error: `${path}: empty matcher` }
  }

  const rule: ChannelErrorRetryRule = {
    id: value.id,
    enabled,
    action: value.action,
    status_codes: statusCodes,
    conditions,
  }
  if (typeof value.name === 'string' && value.name !== '') {
    rule.name = value.name
  }
  return { rule, error: null }
}

// parseChannelErrorRetryPolicyJSON strictly validates a stored policy string.
// An empty string means "not configured" (inherit). A malformed or invalid
// value returns a bounded error string so the editor can surface it instead of
// silently replacing it with an empty policy.
export function parseChannelErrorRetryPolicyJSON(
  raw: string | undefined | null
): ParsedChannelErrorRetryPolicy {
  if (!raw || raw.trim() === '' || raw.trim() === 'null') {
    return { policy: null, error: null }
  }
  if (hasDuplicateJSONKeys(raw)) {
    return { policy: null, error: 'duplicate_key' }
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return { policy: null, error: 'invalid_json' }
  }
  if (!isRecord(parsed)) {
    return { policy: null, error: 'not_object' }
  }
  if (hasUnknownKeys(parsed, POLICY_KEYS)) {
    return { policy: null, error: 'unknown_field' }
  }
  const enabled = parsed.enabled === undefined ? false : parsed.enabled
  if (typeof enabled !== 'boolean') {
    return { policy: null, error: 'enabled: not a bool' }
  }
  const rules: ChannelErrorRetryRule[] = []
  if (parsed.rules !== undefined) {
    if (!Array.isArray(parsed.rules)) {
      return { policy: null, error: 'rules: not an array' }
    }
    if (parsed.rules.length > CHANNEL_ERROR_RETRY_LIMITS.rules) {
      return { policy: null, error: 'rules: too many' }
    }
    const seenIds = new Set<string>()
    for (let index = 0; index < parsed.rules.length; index += 1) {
      const parsedRule = validateRule(parsed.rules[index], `rules[${index}]`)
      if (parsedRule.error) {
        return { policy: null, error: parsedRule.error }
      }
      if (seenIds.has(parsedRule.rule.id)) {
        return { policy: null, error: `rules[${index}].id: duplicate` }
      }
      seenIds.add(parsedRule.rule.id)
      rules.push(parsedRule.rule)
    }
  }
  if (rules.length > 0 && parsed.enabled === undefined) {
    // Mirror the backend contract: a policy that carries rules must state
    // whether it is enabled, otherwise the configured rules would silently never
    // run and the decision audit would not report it.
    return { policy: null, error: 'enabled: required' }
  }
  return { policy: { enabled, rules }, error: null }
}

export function serializeChannelErrorRetryPolicy(
  policy: ChannelErrorRetryPolicy
): string {
  const rules = (policy.rules ?? []).map((rule) => {
    const serialized: Record<string, unknown> = {
      id: rule.id,
      enabled: rule.enabled,
      action: rule.action,
    }
    if (rule.name?.trim()) {
      serialized.name = rule.name.trim()
    }
    if (rule.status_codes && rule.status_codes.length > 0) {
      serialized.status_codes = rule.status_codes
    }
    if (rule.conditions && rule.conditions.length > 0) {
      serialized.conditions = rule.conditions
    }
    return serialized
  })
  return JSON.stringify({ enabled: policy.enabled, rules })
}

// buildChannelErrorRetrySetting returns the value to store in the channel
// setting object: undefined when the field is untouched/empty, the canonical
// policy JSON when valid, and otherwise the original stored value verbatim so
// an existing broken policy is never silently dropped. The backend rejects the
// save with a bounded error when the value is invalid.
export function buildChannelErrorRetrySetting(
  raw: string | undefined
): unknown {
  if (!raw || raw.trim() === '') {
    return undefined
  }
  const parsed = parseChannelErrorRetryPolicyJSON(raw)
  if (!parsed.error && parsed.policy) {
    return parsed.policy
  }
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

export function isChannelErrorRetryPolicyConfigured(
  raw: string | undefined
): boolean {
  if (!raw || raw.trim() === '') return false
  const parsed = parseChannelErrorRetryPolicyJSON(raw)
  // An invalid stored value must surface as configured so the editor shows it
  // instead of hiding a broken policy. An explicit null/absent value is not
  // configured, matching the backend (null means inherit the global decision).
  if (parsed.error) return true
  if (!parsed.policy) return false
  return Boolean(
    parsed.policy.enabled ||
    (parsed.policy.rules && parsed.policy.rules.length > 0)
  )
}

export function channelErrorRetryPolicyUtf8Bytes(policy: unknown): number {
  const encoded = JSON.stringify(policy)
  if (typeof TextEncoder !== 'undefined') {
    return new TextEncoder().encode(encoded).length
  }
  return encoded.length
}

export function isValidChannelErrorRetryPolicyJSON(
  value: string | undefined
): boolean {
  if (!value || value.trim() === '') return true
  const parsed = parseChannelErrorRetryPolicyJSON(value)
  if (parsed.error) return false
  // An explicit null/absent value is valid: the backend treats it as "no
  // policy / inherit the global decision".
  if (!parsed.policy) return true
  return (
    channelErrorRetryPolicyUtf8Bytes(parsed.policy) <=
    CHANNEL_ERROR_RETRY_LIMITS.policyBytes
  )
}

export interface ParsedStatusCodesInput {
  codes: number[]
  /** Tokens that are not a single HTTP status code inside the allowed range. */
  invalidTokens: string[]
}

// parseStatusCodesInput splits a comma-separated status-code list once, keeps
// the valid codes deduplicated in order, and reports invalid tokens so the
// editor can reject a typo instead of silently dropping it.
export function parseStatusCodesInput(value: string): ParsedStatusCodesInput {
  const codes: number[] = []
  const invalidTokens: string[] = []
  const seen = new Set<number>()
  for (const part of value.split(',')) {
    const trimmed = part.trim()
    if (!trimmed) continue
    const code = Number(trimmed)
    if (
      !Number.isInteger(code) ||
      code < CHANNEL_ERROR_RETRY_LIMITS.statusCodeMin ||
      code > CHANNEL_ERROR_RETRY_LIMITS.statusCodeMax
    ) {
      invalidTokens.push(trimmed)
      continue
    }
    if (seen.has(code)) continue
    seen.add(code)
    codes.push(code)
  }
  return { codes, invalidTokens }
}
