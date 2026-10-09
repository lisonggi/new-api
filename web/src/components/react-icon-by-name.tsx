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
import { useEffect, useState } from 'react'
import type { IconBaseProps, IconType } from 'react-icons'

type IconPackModule = Record<string, unknown>
type IconPackLoader = () => Promise<IconPackModule>

// The dashboard resolves a react-icons component by name (e.g. the built-in
// "SiAlipay" / "SiStripe" / "SiWechat" / "LuCreditCard" payment icons) using the
// prefix match below. Bundling every react-icons pack inflated `dist` by ~40 MB,
// so the supported packs are curated here; add a pack and its prefix entry in
// ICON_PACK_CANDIDATES when a new icon source is genuinely needed.
const ICON_PACK_LOADERS = {
  bi: () => import('react-icons/bi').then((module) => module as IconPackModule),
  fa6: () =>
    import('react-icons/fa6').then((module) => module as IconPackModule),
  fi: () => import('react-icons/fi').then((module) => module as IconPackModule),
  hi2: () =>
    import('react-icons/hi2').then((module) => module as IconPackModule),
  io5: () =>
    import('react-icons/io5').then((module) => module as IconPackModule),
  lu: () => import('react-icons/lu').then((module) => module as IconPackModule),
  md: () => import('react-icons/md').then((module) => module as IconPackModule),
  si: () => import('react-icons/si').then((module) => module as IconPackModule),
} satisfies Record<string, IconPackLoader>

type IconPackId = keyof typeof ICON_PACK_LOADERS

const ICON_PACK_CACHE = new Map<IconPackId, Promise<IconPackModule>>()

const ICON_PACK_CANDIDATES: Array<[RegExp, IconPackId[]]> = [
  [/^Bi[A-Z0-9]/, ['bi']],
  [/^Fa[A-Z0-9]/, ['fa6']],
  [/^Fi[A-Z0-9]/, ['fi']],
  [/^Hi[A-Z0-9]/, ['hi2']],
  [/^Io[A-Z0-9]/, ['io5']],
  [/^Lu[A-Z0-9]/, ['lu']],
  [/^Md[A-Z0-9]/, ['md']],
  [/^Si[A-Z0-9]/, ['si']],
]

function normalizeIconName(name: string | null | undefined): string | null {
  const trimmed = name?.trim()
  if (!trimmed || !/^[A-Z][A-Za-z0-9]*$/.test(trimmed)) return null
  return trimmed
}

function getCandidatePacks(iconName: string): IconPackId[] {
  return (
    ICON_PACK_CANDIDATES.find(([pattern]) => pattern.test(iconName))?.[1] ?? []
  )
}

function loadIconPack(packId: IconPackId): Promise<IconPackModule> {
  const cached = ICON_PACK_CACHE.get(packId)
  if (cached) return cached

  const promise = ICON_PACK_LOADERS[packId]()
  ICON_PACK_CACHE.set(packId, promise)
  return promise
}

function isIconComponent(value: unknown): value is IconType {
  return typeof value === 'function'
}

async function resolveReactIcon(iconName: string): Promise<IconType | null> {
  for (const packId of getCandidatePacks(iconName)) {
    try {
      const icon = (await loadIconPack(packId))[iconName]
      if (isIconComponent(icon)) return icon
    } catch {
      // Missing chunks or unknown packs should behave the same as unknown names.
    }
  }
  return null
}

type ReactIconByNameProps = IconBaseProps & {
  name?: string | null
}

type ResolvedIconState = {
  iconName: string
  Icon: IconType | null
}

export function ReactIconByName({ name, ...props }: ReactIconByNameProps) {
  const iconName = normalizeIconName(name)
  const [resolvedIcon, setResolvedIcon] = useState<ResolvedIconState | null>(
    null
  )

  useEffect(() => {
    let cancelled = false

    if (!iconName) return

    void resolveReactIcon(iconName).then((Icon) => {
      if (!cancelled) setResolvedIcon({ iconName, Icon })
    })

    return () => {
      cancelled = true
    }
  }, [iconName])

  if (!iconName || resolvedIcon?.iconName !== iconName || !resolvedIcon.Icon) {
    return null
  }

  const Icon = resolvedIcon.Icon

  return <Icon {...props} />
}
