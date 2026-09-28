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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { RedemptionSuccessDialog } from '@/components/redemption-success-dialog'

describe('RedemptionSuccessDialog', () => {
  it('renders the configured title as Markdown content and the close button', async () => {
    const onOpenChange = vi.fn()
    render(
      <RedemptionSuccessDialog
        open
        onOpenChange={onOpenChange}
        dialog={{
          title: '兑换成功',
          content: '**好评**立即获得1元兑换码\n\n- 第一项',
          closeButtonText: '我知道了',
        }}
        description='Added: 1.00'
      />
    )

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('兑换成功')).toBeVisible()
    expect(within(dialog).getByText('好评')).toBeVisible()
    expect(within(dialog).getByText('第一项')).toBeVisible()
    expect(within(dialog).getByText('Added: 1.00')).toBeVisible()
    expect(
      within(dialog).queryByText(
        'Preview only. It does not save the draft or redeem a code.'
      )
    ).not.toBeInTheDocument()

    await userEvent.click(
      within(dialog).getByRole('button', { name: '我知道了' })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('closes the dialog when Escape is pressed', async () => {
    const onOpenChange = vi.fn()
    render(
      <RedemptionSuccessDialog
        open
        onOpenChange={onOpenChange}
        dialog={{ title: '兑换成功', content: '正文', closeButtonText: '好的' }}
      />
    )

    await screen.findByRole('dialog')
    await userEvent.keyboard('{Escape}')
    expect(onOpenChange.mock.calls[0]?.[0]).toBe(false)
  })

  it('marks a preview and falls back to default labels for blank values', async () => {
    render(
      <RedemptionSuccessDialog
        open
        onOpenChange={vi.fn()}
        dialog={{ title: '', content: '', closeButtonText: '' }}
        preview
      />
    )

    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(
        'Preview only. It does not save the draft or redeem a code.'
      )
    ).toBeVisible()
    expect(
      within(dialog).getAllByRole('button', { name: 'Close' }).length
    ).toBeGreaterThanOrEqual(1)
  })

  it('renders nothing without dialog content', () => {
    render(
      <RedemptionSuccessDialog open onOpenChange={vi.fn()} dialog={null} />
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders headings, links, quotes, code, tables and images', async () => {
    render(
      <RedemptionSuccessDialog
        open
        onOpenChange={vi.fn()}
        dialog={{
          title: '兑换成功',
          content: [
            '# 标题',
            '',
            '**加粗** 与 [文档](https://example.com/docs)',
            '',
            '> 引用内容',
            '',
            '- 列表项',
            '',
            '`行内代码`',
            '',
            '| 列A | 列B |',
            '| --- | --- |',
            '| 1 | 2 |',
            '',
            '![示意图](https://example.com/diagram.png)',
          ].join('\n'),
          closeButtonText: '我知道了',
        }}
      />
    )

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: '标题' })).toBeVisible()
    expect(within(dialog).getByText('加粗')).toBeVisible()
    expect(within(dialog).getByText('引用内容')).toBeVisible()
    expect(within(dialog).getByText('列表项')).toBeVisible()
    expect(within(dialog).getByText('行内代码')).toBeVisible()

    const link = within(dialog).getByRole('link', { name: '文档' })
    expect(link).toHaveAttribute('href', 'https://example.com/docs')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')

    expect(
      within(dialog).getByRole('columnheader', { name: '列A' })
    ).toBeVisible()
    expect(within(dialog).getByRole('cell', { name: '1' })).toBeVisible()
    expect(within(dialog).getByRole('img', { name: '示意图' })).toHaveAttribute(
      'src',
      'https://example.com/diagram.png'
    )
  })

  it('sanitizes scripts, event handlers and dangerous URLs in Markdown', async () => {
    render(
      <RedemptionSuccessDialog
        open
        onOpenChange={vi.fn()}
        dialog={{
          title: 'Safe',
          content:
            '<script>window.__redemptionPwned = 1</script>\n\n' +
            '<img src="x" onerror="window.__redemptionPwned = 2">\n\n' +
            '[bad](javascript:alert%281%29)\n\n' +
            '[good](https://example.com/safe)',
          closeButtonText: 'OK',
        }}
      />
    )

    const dialog = await screen.findByRole('dialog')
    expect(dialog.querySelector('script')).toBeNull()
    expect(dialog.querySelector('[onerror]')).toBeNull()
    expect(dialog.querySelector('a[href^="javascript:"]')).toBeNull()
    expect(within(dialog).getByRole('link', { name: 'good' })).toHaveAttribute(
      'href',
      'https://example.com/safe'
    )
    expect(
      (window as unknown as { __redemptionPwned?: number }).__redemptionPwned
    ).toBeUndefined()
  })
})
