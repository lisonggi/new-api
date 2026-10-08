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
import { HTTP_PROTOCOL_AUTO, type HttpProtocolValue } from './channel-form'

export type ChannelAttributeProxyMode = 'set' | 'clear'

/**
 * The attribute changes shared by the tag batch edit dialog and the
 * selected-channels batch edit dialog. Every attribute is opt-in: an attribute
 * is only written when `enabled` is true, which keeps "only change what the
 * administrator asked for" true for both entry points.
 */
export type ChannelAttributeChanges = {
  models: { enabled: boolean; value: string }
  modelMapping: { enabled: boolean; value: string }
  groups: { enabled: boolean; value: string[] }
  httpProtocol: { enabled: boolean; value: HttpProtocolValue }
  shards: { enabled: boolean; value: string }
  proxy: {
    enabled: boolean
    mode: ChannelAttributeProxyMode
    address: string
  }
}

export function emptyChannelAttributeChanges(): ChannelAttributeChanges {
  return {
    models: { enabled: false, value: '' },
    modelMapping: { enabled: false, value: '' },
    groups: { enabled: false, value: [] },
    httpProtocol: { enabled: false, value: HTTP_PROTOCOL_AUTO },
    shards: { enabled: false, value: '1' },
    proxy: { enabled: false, mode: 'set', address: '' },
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
  return null
}

/**
 * Builds the request fields for the enabled attributes only. An attribute that
 * is not turned on never appears here, so the request can never change it.
 */
export function buildChannelAttributeParams(
  changes: ChannelAttributeChanges
): Record<string, string | number> {
  const params: Record<string, string | number> = {}
  if (changes.models.enabled) {
    params.models = changes.models.value.trim()
  }
  if (changes.modelMapping.enabled) {
    params.model_mapping = changes.modelMapping.value.trim()
  }
  if (changes.groups.enabled) {
    params.groups = changes.groups.value.join(',')
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
  return params
}
