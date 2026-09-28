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
export type RedemptionSuccessDialogConfig = {
  enabled: boolean
  title: string
  content: string
  close_button_text: string
}

// Limits mirrored from the backend `setting/redemption_dialog` package.
// Character limits are Unicode code points, matching the Go rune count.
export const REDEMPTION_DIALOG_LIMITS = {
  maxTitleLength: 80,
  maxContentLength: 4000,
  maxCloseButtonLength: 20,
} as const

export const EMPTY_REDEMPTION_DIALOG_CONFIG: RedemptionSuccessDialogConfig = {
  enabled: false,
  title: '',
  content: '',
  close_button_text: '',
}
