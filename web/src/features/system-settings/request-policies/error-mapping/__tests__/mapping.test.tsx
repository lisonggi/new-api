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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'
import { api } from '@/lib/api'

import { ErrorMessageMappingSection } from '..'
import type { ErrorMappingConfig } from '../types'

let queryClient: QueryClient
let serverConfig: ErrorMappingConfig

const makeRule = (
  overrides: Partial<ErrorMappingConfig['rules'][number]> = {}
) => ({
  id: 'rule-1',
  name: 'Reasoning format',
  enabled: true,
  keyword: 'reasoning_content',
  case_sensitive: false,
  replacement: 'Please adjust the request and retry.',
  ...overrides,
})

function renderSection() {
  function Harness() {
    const [actionsContainer, setActionsContainer] =
      useState<HTMLDivElement | null>(null)
    return (
      <QueryClientProvider client={queryClient}>
        <SettingsPageProvider actionsContainer={actionsContainer}>
          <div ref={setActionsContainer} />
          <ErrorMessageMappingSection />
        </SettingsPageProvider>
      </QueryClientProvider>
    )
  }
  return render(<Harness />)
}

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  serverConfig = { enabled: true, rules: [makeRule()] }

  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/error_message_mapping') {
      return { data: { success: true, data: structuredClone(serverConfig) } }
    }
    throw new Error(`unexpected GET ${url}`)
  })
  vi.spyOn(api, 'put').mockImplementation(async (_url, body) => {
    const config = structuredClone(body as ErrorMappingConfig)
    serverConfig = config
    return { data: { success: true, data: config } }
  })
  vi.spyOn(api, 'post').mockImplementation(async () => ({
    data: {
      success: true,
      data: { matched: true, rule_id: 'rule-1', message: 'PREVIEW_RESULT' },
    },
  }))
})

afterEach(() => {
  cleanup()
  queryClient.clear()
})

describe('error message mapping settings', () => {
  it('renders the saved rules and the scope note', async () => {
    serverConfig = {
      enabled: true,
      rules: [
        makeRule(),
        makeRule({ id: 'rule-2', name: 'Tool choice', keyword: 'tool_choice' }),
      ],
    }
    renderSection()
    expect(await screen.findByText('reasoning_content')).toBeVisible()
    expect(screen.getByText('tool_choice')).toBeVisible()
    expect(
      screen.getByRole('switch', { name: 'Enable error message mapping' })
    ).toBeChecked()
  })

  it('saving a toggled rule writes the whole draft config', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    const [url, body] = vi.mocked(api.put).mock.calls[0]
    expect(url).toBe('/api/option/error_message_mapping')
    expect((body as ErrorMappingConfig).rules[0].enabled).toBe(false)
  })

  it('adds a rule through the dialog with the default flags', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add mapping rule' })
    )
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByRole('textbox', { name: /Keyword/ }), {
      target: { value: 'tool_choice' },
    })
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: /Replacement/ }),
      { target: { value: 'Tools are not available here.' } }
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(screen.getAllByText('tool_choice').length).toBeGreaterThan(0)

    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    const body = vi.mocked(api.put).mock.calls[0][1] as ErrorMappingConfig
    expect(body.rules).toHaveLength(2)
    expect(body.rules[1]).toMatchObject({
      keyword: 'tool_choice',
      enabled: true,
      case_sensitive: false,
    })
    expect(body.rules[1].id).not.toBe('')
  })

  it('rejects a keyword longer than the rune limit', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add mapping rule' })
    )
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByRole('textbox', { name: /Keyword/ }), {
      target: { value: 'a'.repeat(257) },
    })
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: /Replacement/ }),
      { target: { value: 'x' } }
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByText('Keyword is too long')).toBeVisible()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('counts the keyword limit in Unicode code points, not UTF-16 units', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add mapping rule' })
    )
    const dialog = await screen.findByRole('dialog')
    // 256 emoji is exactly at the rune limit even though its UTF-16 length is
    // 512; 257 is one over. This fails if the limit uses string.length.
    fireEvent.change(within(dialog).getByRole('textbox', { name: /Keyword/ }), {
      target: { value: '😀'.repeat(257) },
    })
    fireEvent.change(
      within(dialog).getByRole('textbox', { name: /Replacement/ }),
      { target: { value: 'x' } }
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByText('Keyword is too long')).toBeVisible()

    fireEvent.change(within(dialog).getByRole('textbox', { name: /Keyword/ }), {
      target: { value: '😀'.repeat(256) },
    })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
  })

  it('associates field errors with the keyword and replacement inputs', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add mapping rule' })
    )
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

    const keywordError = await within(dialog).findByText('Keyword is required')
    const replacementError = await within(dialog).findByText(
      'Replacement is required'
    )
    const keyword = within(dialog).getByRole('textbox', { name: /Keyword/ })
    const replacement = within(dialog).getByRole('textbox', {
      name: /Replacement/,
    })

    expect(keyword).toHaveAttribute('aria-invalid', 'true')
    expect(replacement).toHaveAttribute('aria-invalid', 'true')
    expect(keyword).toHaveAttribute(
      'aria-describedby',
      `error-mapping-rule-keyword-help ${keywordError.id}`
    )
    expect(replacement).toHaveAttribute('aria-describedby', replacementError.id)
  })

  it('lists the four supported API endpoints', async () => {
    renderSection()
    await screen.findByText('reasoning_content')
    for (const endpoint of [
      'POST /v1/chat/completions',
      'POST /v1/messages',
      'POST /v1/responses',
      'POST /v1/responses/compact',
    ]) {
      expect(screen.getByText(endpoint)).toBeVisible()
    }
  })

  it('reorders rules with the explicit up and down buttons', async () => {
    serverConfig = {
      enabled: true,
      rules: [
        makeRule({ id: 'first', name: 'First', keyword: 'first_keyword' }),
        makeRule({ id: 'second', name: 'Second', keyword: 'second_keyword' }),
      ],
    }
    renderSection()
    await screen.findByText('first_keyword')
    const up = screen.getAllByRole('button', { name: 'Move rule up' })
    expect(up[0]).toBeDisabled()
    await userEvent.click(up[1])
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    const body = vi.mocked(api.put).mock.calls[0][1] as ErrorMappingConfig
    expect(body.rules.map((rule) => rule.id)).toEqual(['second', 'first'])
  })

  it('keeps the draft when the save is rejected', async () => {
    vi.mocked(api.put).mockResolvedValue({
      data: { success: false, message: 'Save rejected' },
    })
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).not.toBeChecked()
    expect(screen.getByText('reasoning_content')).toBeVisible()
  })

  it('does not let a background refetch overwrite a dirty draft', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    serverConfig = {
      enabled: true,
      rules: [
        makeRule({ id: 'other', name: 'Other', keyword: 'refetched_keyword' }),
      ],
    }
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['error-message-mapping'],
      })
    })
    expect(screen.queryByText('refetched_keyword')).not.toBeInTheDocument()
    expect(screen.getByText('reasoning_content')).toBeVisible()
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).not.toBeChecked()
  })

  it('previews the current draft and marks the result stale after an edit', async () => {
    serverConfig = {
      enabled: true,
      rules: [
        makeRule({
          id: 'alpha',
          name: 'Alpha',
          keyword: 'alpha_error',
          replacement: 'Replaced by alpha.',
        }),
      ],
    }
    renderSection()
    const message = await screen.findByLabelText('Sample error message')
    fireEvent.change(message, { target: { value: 'saw alpha_error here' } })
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    const panel = await screen.findByRole('status')
    expect(within(panel).getByText('Matched a rule')).toBeVisible()
    expect(within(panel).getByText('PREVIEW_RESULT')).toBeVisible()

    // The preview request carries the current draft, not a stale snapshot.
    const [, posted] = vi.mocked(api.post).mock.calls[0]
    expect(posted).toMatchObject({ message: 'saw alpha_error here' })
    expect(
      (posted as { config: ErrorMappingConfig }).config.rules[0]
    ).toMatchObject({ keyword: 'alpha_error', enabled: true })

    await userEvent.click(
      screen.getByRole('switch', { name: 'Enable rule Alpha' })
    )
    expect(
      within(panel).getByText('The draft changed after this preview.')
    ).toBeVisible()
  })

  it('ignores a late preview response after the draft changed', async () => {
    let resolvePreview: (() => void) | null = null
    vi.mocked(api.post).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePreview = () =>
            resolve({
              data: {
                success: true,
                data: { matched: true, rule_id: 'rule-1', message: 'LATE' },
              },
            })
        })
    )
    renderSection()
    const message = await screen.findByLabelText('Sample error message')
    fireEvent.change(message, { target: { value: 'reasoning_content' } })
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    await userEvent.click(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    )
    await act(async () => {
      resolvePreview?.()
      await Promise.resolve()
    })
    expect(screen.queryByText('LATE')).not.toBeInTheDocument()
    expect(screen.queryByText('Matched a rule')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Run preview' })).toBeEnabled()
  })

  it('D review: saving an older draft preserves edits made while the request is pending', async () => {
    let finishSave: () => void = () => {
      throw new Error('Save was not started')
    }
    vi.mocked(api.put).mockImplementation((_url, body) => {
      const submitted = structuredClone(body as ErrorMappingConfig)
      return new Promise((resolve) => {
        finishSave = () => {
          serverConfig = submitted
          resolve({ data: { success: true, data: submitted } })
        }
      })
    })
    renderSection()
    const toggle = await screen.findByRole('switch', {
      name: 'Enable rule Reasoning format',
    })
    await userEvent.click(toggle)
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    await userEvent.click(toggle)
    expect(toggle).toBeChecked()
    await act(async () => finishSave())
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).toBeChecked()
  })

  it('D review: a failed background refresh keeps the dirty editor visible', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    vi.mocked(api.get).mockRejectedValue(new Error('Refresh unavailable'))
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['error-message-mapping'],
      })
    })
    await waitFor(() =>
      expect(queryClient.getQueryState(['error-message-mapping'])?.status).toBe(
        'error'
      )
    )
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).not.toBeChecked()
  })

  it('keeps a successfully saved rule visible when the follow-up refresh fails', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    vi.mocked(api.get).mockRejectedValue(new Error('Refresh unavailable'))
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(serverConfig.rules[0].enabled).toBe(false)
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).not.toBeChecked()
  })

  it('shows the saved config after leaving during a save and returning', async () => {
    let finishSave: () => void = () => {
      throw new Error('Save not started')
    }
    vi.mocked(api.put).mockImplementation((_url, body) => {
      const submitted = structuredClone(body as ErrorMappingConfig)
      return new Promise((resolve) => {
        finishSave = () => {
          serverConfig = submitted
          resolve({ data: { success: true, data: submitted } })
        }
      })
    })
    const view = renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    view.unmount()
    await act(async () => finishSave())
    await waitFor(() => expect(queryClient.isMutating()).toBe(0))
    expect(serverConfig.rules[0].enabled).toBe(false)
    renderSection()
    expect(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    ).not.toBeChecked()
  })

  it('marks the old preview stale when a refresh replaces the clean draft', async () => {
    renderSection()
    fireEvent.change(await screen.findByLabelText('Sample error message'), {
      target: { value: 'reasoning_content' },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    await screen.findByText('PREVIEW_RESULT')
    serverConfig = { enabled: false, rules: [] }
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['error-message-mapping'],
      })
    })
    expect(await screen.findByText('No mapping rules yet')).toBeVisible()
    expect(
      screen.getByText('The draft changed after this preview.')
    ).toBeVisible()
  })

  it('ignores an in-flight preview after a refresh replaces the clean draft and allows a new preview', async () => {
    let finishPreview: () => void = () => {
      throw new Error('Preview not started')
    }
    vi.mocked(api.post).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishPreview = () =>
            resolve({
              data: {
                success: true,
                data: {
                  matched: true,
                  rule_id: 'rule-1',
                  message: 'OLD_PREVIEW',
                },
              },
            })
        })
    )
    renderSection()
    fireEvent.change(await screen.findByLabelText('Sample error message'), {
      target: { value: 'reasoning_content' },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    expect(screen.getByRole('button', { name: 'Run preview' })).toBeDisabled()
    serverConfig = { enabled: false, rules: [] }
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: ['error-message-mapping'],
      })
    })
    await screen.findByText('No mapping rules yet')
    expect(screen.getByRole('button', { name: 'Run preview' })).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    expect(await screen.findByText('PREVIEW_RESULT')).toBeVisible()
    expect(vi.mocked(api.post).mock.calls[1][1]).toEqual({
      config: { enabled: false, rules: [] },
      message: 'reasoning_content',
    })
    await act(async () => finishPreview())
    expect(screen.queryByText('OLD_PREVIEW')).not.toBeInTheDocument()
    expect(screen.getByText('PREVIEW_RESULT')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Run preview' })).toBeEnabled()
  })

  it('saves an empty ruleset and keeps the empty editor usable', async () => {
    serverConfig = { enabled: false, rules: [] }
    renderSection()
    await screen.findByText('No mapping rules yet')
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(serverConfig.rules).toEqual([])
    expect(screen.getByText('No mapping rules yet')).toBeVisible()
  })

  it('resets to the successful save after a failed refresh', async () => {
    renderSection()
    await userEvent.click(
      await screen.findByRole('switch', {
        name: 'Enable rule Reasoning format',
      })
    )
    vi.mocked(api.get).mockRejectedValue(new Error('Refresh unavailable'))
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isMutating()).toBe(0))
    await userEvent.click(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Reset' }))
    expect(
      screen.getByRole('switch', { name: 'Enable rule Reasoning format' })
    ).not.toBeChecked()
  })

  it('disables adding a rule once the rule limit is reached', async () => {
    serverConfig = {
      enabled: true,
      rules: Array.from({ length: 100 }, (_, index) =>
        makeRule({
          id: `rule-${index}`,
          name: `Rule ${index}`,
          keyword: `keyword_${index}`,
        })
      ),
    }
    renderSection()
    expect(await screen.findByText('keyword_0')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Add mapping rule' })
    ).toBeDisabled()
  })

  it('marks a finished preview stale when the sample message changes', async () => {
    renderSection()
    const message = await screen.findByLabelText('Sample error message')
    fireEvent.change(message, { target: { value: 'reasoning_content' } })
    await userEvent.click(screen.getByRole('button', { name: 'Run preview' }))
    const panel = await screen.findByRole('status')
    expect(within(panel).getByText('Matched a rule')).toBeVisible()

    fireEvent.change(message, { target: { value: 'something else' } })
    expect(
      within(panel).getByText('The draft changed after this preview.')
    ).toBeVisible()
  })
})
