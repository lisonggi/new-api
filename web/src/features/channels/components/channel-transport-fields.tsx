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
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  HTTP_PROTOCOL_AUTO,
  HTTP_PROTOCOL_HTTP1,
  KEEP_UNCHANGED,
  httpShardItems,
} from '../lib'

/**
 * Props forwarded to the trigger so a surrounding form wrapper keeps its label
 * association and validation wiring (`id`, `aria-*`, `className`).
 */
type TransportTriggerProps = Omit<
  ComponentProps<typeof SelectTrigger>,
  'children' | 'disabled'
>

type HttpProtocolSelectProps = {
  value: string
  onValueChange: (value: string) => void
  disabled?: boolean
  /** Adds a "keep unchanged" option for batch editing. */
  allowUnchanged?: boolean
} & TransportTriggerProps

/**
 * HTTP protocol selector shared by the single-channel form and the tag batch
 * edit dialog. Callers that switch to HTTP/1.1 are responsible for resetting
 * the connection shards.
 */
export function HttpProtocolSelect(props: HttpProtocolSelectProps) {
  const { t } = useTranslation()
  const { value, onValueChange, disabled, allowUnchanged, ...triggerProps } =
    props
  const items = [
    ...(allowUnchanged
      ? [{ value: KEEP_UNCHANGED, label: t('Keep unchanged') }]
      : []),
    { value: HTTP_PROTOCOL_AUTO, label: t('Auto') },
    { value: HTTP_PROTOCOL_HTTP1, label: t('HTTP/1.1') },
  ]

  return (
    <Select
      items={items}
      value={value}
      onValueChange={(next) => {
        // Base UI reports null when a selection is cleared; both callers keep
        // a concrete value, so ignore it instead of emptying the setting.
        if (next === null) return
        onValueChange(next)
      }}
      disabled={disabled}
    >
      <SelectTrigger disabled={disabled} {...triggerProps}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

type HttpShardsSelectProps = {
  value: string
  onValueChange: (value: string) => void
  disabled?: boolean
  /** Adds a "keep unchanged" option for batch editing. */
  allowUnchanged?: boolean
} & TransportTriggerProps

/**
 * HTTP/2 connection shard selector shared by the single-channel form and the
 * tag batch edit dialog. The caller disables it while HTTP/1.1 is selected.
 */
export function HttpShardsSelect(props: HttpShardsSelectProps) {
  const { t } = useTranslation()
  const { value, onValueChange, disabled, allowUnchanged, ...triggerProps } =
    props
  const items = allowUnchanged
    ? [{ value: KEEP_UNCHANGED, label: t('Keep unchanged') }, ...httpShardItems]
    : httpShardItems

  return (
    <Select
      items={items}
      value={value}
      onValueChange={(next) => {
        // Base UI reports null when a selection is cleared; both callers keep
        // a concrete value, so ignore it instead of emptying the setting.
        if (next === null) return
        onValueChange(next)
      }}
      disabled={disabled}
    >
      <SelectTrigger disabled={disabled} {...triggerProps}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
