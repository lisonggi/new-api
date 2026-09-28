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
import * as z from 'zod'

import { ERROR_MAPPING_LIMITS } from './types'

// Count Unicode code points to stay consistent with the Go rune count instead
// of UTF-16 string.length.
const countCodePoints = (value: string) => [...value].length

export const errorMappingRuleSchema = z.object({
  name: z
    .string()
    .refine(
      (value) => countCodePoints(value) <= ERROR_MAPPING_LIMITS.maxNameLength,
      { message: 'Name is too long' }
    ),
  keyword: z
    .string()
    .refine((value) => value.trim().length > 0, {
      message: 'Keyword is required',
    })
    .refine(
      (value) =>
        countCodePoints(value) <= ERROR_MAPPING_LIMITS.maxKeywordLength,
      { message: 'Keyword is too long' }
    ),
  case_sensitive: z.boolean(),
  replacement: z
    .string()
    .refine((value) => value.trim().length > 0, {
      message: 'Replacement is required',
    })
    .refine(
      (value) =>
        countCodePoints(value) <= ERROR_MAPPING_LIMITS.maxReplacementLength,
      { message: 'Replacement is too long' }
    ),
})

export type ErrorMappingRuleFormValues = z.infer<typeof errorMappingRuleSchema>
