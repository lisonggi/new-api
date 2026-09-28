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
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getSelf } from '@/lib/api'

import { Wallet } from '..'
import { calculateAmount, redeemTopupCode, requestPayment } from '../api'
import type { TopupInfo } from '../types'

const mocks = vi.hoisted(() => ({
  handleServerError: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
  submitPaymentForm: vi.fn(),
}))

vi.mock('../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../api')>()),
  redeemTopupCode: vi.fn(),
  requestPayment: vi.fn(),
  calculateAmount: vi.fn(),
}))
vi.mock('../lib', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib')>()),
  submitPaymentForm: mocks.submitPaymentForm,
}))
vi.mock('@/lib/handle-server-error', () => ({
  handleServerError: mocks.handleServerError,
}))
vi.mock('sonner', () => ({
  toast: { success: mocks.toastSuccess, error: mocks.toastError },
}))

vi.mock('@/lib/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api')>()),
  getSelf: vi.fn(),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { price: 1 } }),
}))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({ currency: undefined }),
}))

vi.mock('@/components/layout', () => {
  const SectionPageLayout = Object.assign(
    (props: { children?: ReactNode }) => <div>{props.children}</div>,
    {
      Title: (props: { children?: ReactNode }) => <h1>{props.children}</h1>,
      Content: (props: { children?: ReactNode }) => <div>{props.children}</div>,
    }
  )
  return { SectionPageLayout }
})

// Explicit online-topup fixture: the redemption section must work next to the
// online topup UI, and the two flows must stay isolated in both directions.
const topupFixture: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  pay_methods: [{ name: 'Alipay', type: 'alipay', min_topup: 1 }],
  min_topup: 1,
  stripe_min_topup: 0,
  amount_options: [10],
  discount: {},
  enable_redemption: true,
}

vi.mock('../hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../hooks')>()
  return {
    ...actual,
    useTopupInfo: () => ({
      topupInfo: topupFixture,
      presetAmounts: [],
      loading: false,
    }),
    useAffiliate: () => ({
      affiliateLink: '',
      loading: false,
      transferQuota: vi.fn(),
      transferring: false,
    }),
    useCreemPayment: () => ({
      processing: false,
      processCreemPayment: vi.fn(),
    }),
    useWaffoPayment: () => ({
      processing: false,
      processWaffoPayment: vi.fn(),
    }),
    useWaffoPancakePayment: () => ({
      processing: false,
      processWaffoPancakePayment: vi.fn(),
    }),
  }
})

vi.mock('../components/affiliate-rewards-card', () => ({
  AffiliateRewardsCard: () => null,
}))
vi.mock('../components/subscription-plans-card', () => ({
  SubscriptionPlansCard: () => null,
}))
vi.mock('../components/wallet-stats-card', () => ({
  WalletStatsCard: () => null,
}))
vi.mock('../components/dialogs/billing-history-dialog', () => ({
  BillingHistoryDialog: () => null,
}))
vi.mock('../components/dialogs/creem-confirm-dialog', () => ({
  CreemConfirmDialog: () => null,
}))
vi.mock('../components/dialogs/transfer-dialog', () => ({
  TransferDialog: () => null,
}))

function renderWallet() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <Wallet />
    </QueryClientProvider>
  )
}

async function submitCode(code: string) {
  await userEvent.type(await screen.findByLabelText('Have a Code?'), code)
  await userEvent.click(screen.getByRole('button', { name: 'Redeem' }))
}

const dialogPayload = {
  title: '兑换成功',
  content: '**好评**立即获得1元兑换码',
  close_button_text: '我知道了',
}

beforeEach(() => {
  vi.mocked(redeemTopupCode).mockReset()
  vi.mocked(requestPayment).mockReset()
  vi.mocked(calculateAmount)
    .mockReset()
    .mockResolvedValue({ success: true, data: '1' })
  vi.mocked(getSelf).mockReset()
  vi.mocked(getSelf).mockResolvedValue({ success: true, data: {} } as never)
  mocks.handleServerError.mockReset()
  mocks.toastSuccess.mockReset()
  mocks.toastError.mockReset()
  mocks.submitPaymentForm.mockReset()
})

describe('wallet redemption success dialog', () => {
  it('clears the code and opens the configured dialog even when the balance refresh fails', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })
    vi.mocked(getSelf)
      .mockResolvedValueOnce({ success: true, data: {} } as never)
      .mockRejectedValueOnce(new Error('refresh failed'))

    renderWallet()
    await submitCode('CODE-1')

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('兑换成功')).toBeVisible()
    expect(within(dialog).getByText('好评')).toBeVisible()
    expect(screen.getByLabelText('Have a Code?')).toHaveValue('')
    expect(getSelf).toHaveBeenCalledTimes(2)

    // A failed balance refresh is a follow-up, not a redemption failure: it
    // must not surface an error nor replay the one-shot redemption request.
    expect(redeemTopupCode).toHaveBeenCalledTimes(1)
    expect(mocks.handleServerError).not.toHaveBeenCalled()
    expect(mocks.toastError).not.toHaveBeenCalled()
  })

  it('redeeming a code does not start an online topup payment', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    renderWallet()
    await screen.findByLabelText('Have a Code?')
    // The online topup UI is present next to the redemption section.
    expect(screen.getByLabelText('Custom Amount')).toBeInTheDocument()

    await submitCode('CODE-1')

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('兑换成功')).toBeVisible()
    expect(requestPayment).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Custom Amount')).toBeInTheDocument()
  })

  it('completing an online topup payment does not open the redemption dialog', async () => {
    vi.mocked(requestPayment).mockResolvedValue({
      success: true,
      message: 'success',
      data: { order: 1 },
      url: 'https://pay.example/checkout',
    })

    renderWallet()

    // Real payment method button on the real RechargeFormCard.
    await userEvent.click(await screen.findByRole('button', { name: 'Alipay' }))

    // Real confirmation dialog, then the real confirm action.
    const confirm = await screen.findByRole('alertdialog')
    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Confirm Payment' })
    )

    await waitFor(() => expect(requestPayment).toHaveBeenCalledTimes(1))
    expect(requestPayment).toHaveBeenCalledWith({
      amount: 1,
      payment_method: 'alipay',
    })
    expect(mocks.submitPaymentForm).toHaveBeenCalledWith(
      'https://pay.example/checkout',
      { order: 1 }
    )
    // The balance is refreshed after a successful payment.
    expect(getSelf).toHaveBeenCalledTimes(2)

    // The redemption request is never sent and no redemption dialog appears.
    expect(redeemTopupCode).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByText('兑换成功')).not.toBeInTheDocument()
  })

  it('moves focus into the dialog and back to the redeem control', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    renderWallet()
    await submitCode('CODE-1')
    const dialog = await screen.findByRole('dialog')
    await waitFor(() =>
      expect(dialog.contains(document.activeElement)).toBe(true)
    )

    await userEvent.click(
      within(dialog).getByRole('button', { name: '我知道了' })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Redeem' })).toHaveFocus()
    )
  })

  it('keeps the code and shows no dialog for a rejected redemption', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: false,
      message: '该兑换码已被使用',
    })

    renderWallet()
    await submitCode('EXPIRED-CODE')

    await waitFor(() => expect(redeemTopupCode).toHaveBeenCalledTimes(1))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Have a Code?')).toHaveValue('EXPIRED-CODE')
  })

  it('does not reopen the dialog after it is closed and the wallet is remounted', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    const view = renderWallet()
    await submitCode('CODE-1')
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(
      within(dialog).getByRole('button', { name: '我知道了' })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )

    // A real unmount plus a fresh mount must not resurrect the closed dialog.
    view.unmount()
    renderWallet()
    await screen.findByLabelText('Have a Code?')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('does not open a dialog for a late redemption response after leaving the wallet', async () => {
    type RedeemResult = Awaited<ReturnType<typeof redeemTopupCode>>
    let resolveRedeem: (value: RedeemResult) => void = () => {}
    vi.mocked(redeemTopupCode).mockImplementationOnce(
      () =>
        new Promise<RedeemResult>((resolve) => {
          resolveRedeem = resolve
        })
    )

    const view = renderWallet()
    await submitCode('LATE-CODE')
    view.unmount()

    await act(async () => {
      resolveRedeem({
        success: true,
        data: 500,
        success_dialog: dialogPayload,
      })
    })

    // A concurrently mounted wallet must stay clean.
    renderWallet()
    await screen.findByLabelText('Have a Code?')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the newly configured content on the next redemption', async () => {
    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 500,
      success_dialog: dialogPayload,
    })

    renderWallet()
    await submitCode('CODE-1')
    const firstDialog = await screen.findByRole('dialog')
    await userEvent.click(
      within(firstDialog).getByRole('button', { name: '我知道了' })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )

    vi.mocked(redeemTopupCode).mockResolvedValue({
      success: true,
      data: 300,
      success_dialog: {
        title: '新文案',
        content: '更新后的说明',
        close_button_text: '关闭',
      },
    })
    await submitCode('CODE-2')

    const nextDialog = await screen.findByRole('dialog')
    expect(within(nextDialog).getByText('新文案')).toBeVisible()
    expect(within(nextDialog).getByText('更新后的说明')).toBeVisible()
    expect(
      within(nextDialog).getByRole('button', { name: '关闭' })
    ).toBeVisible()
  })
})
