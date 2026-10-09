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

export type ErrorMappingRule = {
  id: string
  name: string
  enabled: boolean
  keywords: string[]
  case_sensitive: boolean
  replacement: string
}

export type ErrorMappingConfig = {
  enabled: boolean
  rules: ErrorMappingRule[]
}

export type ErrorMappingPreviewResult = {
  matched: boolean
  rule_id: string
  message: string
}

// Limits mirrored from the backend `setting/error_mapping` package. Character
// limits are Unicode code points, matching the Go rune count.
export const ERROR_MAPPING_LIMITS = {
  maxRules: 100,
  maxIdLength: 64,
  maxNameLength: 80,
  maxKeywordLength: 256,
  maxReplacementLength: 2048,
} as const

// HTTP entries whose final client-facing errors are mapped. Kept in sync with
// the frozen phase B scope and rendered literally in the settings UI.
export const ERROR_MAPPING_ENDPOINTS = [
  'POST /v1/chat/completions',
  'POST /v1/messages',
  'POST /v1/responses',
  'POST /v1/responses/compact',
] as const

export const DEFAULT_ERROR_MAPPING_CONFIG: ErrorMappingConfig = {
  enabled: false,
  rules: [],
}
