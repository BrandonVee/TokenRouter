import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { PaymentOrder } from '@/types/payment'
import AdminOrdersView from '../AdminOrdersView.vue'

const {
  mockGetOrders,
  mockRefundOrder,
  mockShowError,
  mockShowSuccess,
  mockShowWarning,
} = vi.hoisted(() => ({
  mockGetOrders: vi.fn(),
  mockRefundOrder: vi.fn(),
  mockShowError: vi.fn(),
  mockShowSuccess: vi.fn(),
  mockShowWarning: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => {
  const paymentAPI = {
    getOrders: (...args: unknown[]) => mockGetOrders(...args),
    refundOrder: (...args: unknown[]) => mockRefundOrder(...args),
  }
  return {
    adminPaymentAPI: paymentAPI,
    default: paymentAPI,
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mockShowError,
    showSuccess: mockShowSuccess,
    showWarning: mockShowWarning,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/composables/useBalanceDisplay', () => ({
  useBalanceDisplay: () => ({
    balanceUnitSymbol: '$',
    formatBalanceAmount: (amount: number) => `$${amount.toFixed(2)}`,
  }),
}))

const order: PaymentOrder = {
  id: 42,
  user_id: 7,
  amount: 100,
  pay_amount: 100,
  currency: 'USD',
  fee_rate: 0,
  fee_fixed: 0,
  fee_rate_amount: 0,
  fee_amount: 0,
  payment_type: 'alipay',
  out_trade_no: 'order-42',
  status: 'COMPLETED',
  order_type: 'balance',
  created_at: '2026-08-01T00:00:00Z',
  expires_at: '2026-08-01T00:30:00Z',
  refund_amount: 0,
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const OrderTableStub = {
  props: ['orders'],
  template: `
    <div>
      <div v-for="row in orders" :key="row.id">
        <slot name="actions" :row="row" />
      </div>
    </div>
  `,
}

function mountView() {
  return mount(AdminOrdersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: BaseDialogStub,
        Icon: true,
        OrderStatusBadge: true,
        OrderTable: OrderTableStub,
        Pagination: true,
        Select: true,
      },
    },
  })
}

function findRefundAction(wrapper: ReturnType<typeof mountView>) {
  return wrapper.findAll('button').find((button) => button.text() === 'payment.admin.refund')
}

describe('AdminOrdersView', () => {
  beforeEach(() => {
    mockGetOrders.mockReset()
    mockRefundOrder.mockReset()
    mockShowError.mockReset()
    mockShowSuccess.mockReset()
    mockShowWarning.mockReset()
    mockGetOrders.mockResolvedValue({
      data: { items: [order], total: 1, page: 1, page_size: 20 },
    })
  })

  it('后端要求强制退款时保留弹窗并在显式确认后重试', async () => {
    mockRefundOrder
      .mockResolvedValueOnce({
        data: {
          success: false,
          require_force: true,
          warning: 'insufficient balance, use force',
        },
      })
      .mockResolvedValueOnce({ data: { success: true } })

    const wrapper = mountView()
    await flushPromises()

    const refundAction = findRefundAction(wrapper)
    expect(refundAction).toBeDefined()
    await refundAction!.trigger('click')
    await flushPromises()

    expect(wrapper.find('#force-refund').exists()).toBe(false)
    await wrapper.find('#refund-form').trigger('submit')
    await flushPromises()

    expect(mockRefundOrder).toHaveBeenNthCalledWith(1, 42, {
      amount: 100,
      reason: '',
      deduct_balance: true,
      force: false,
    })
    expect(wrapper.text()).toContain('insufficient balance, use force')
    const forceCheckbox = wrapper.find<HTMLInputElement>('#force-refund')
    expect(forceCheckbox.exists()).toBe(true)
    expect(wrapper.find<HTMLButtonElement>('button[form="refund-form"]').element.disabled).toBe(true)

    await forceCheckbox.setValue(true)
    expect(wrapper.find<HTMLButtonElement>('button[form="refund-form"]').element.disabled).toBe(false)
    await wrapper.find('#refund-form').trigger('submit')
    await flushPromises()

    expect(mockRefundOrder).toHaveBeenNthCalledWith(2, 42, {
      amount: 100,
      reason: '',
      deduct_balance: true,
      force: true,
    })
    expect(wrapper.find('#refund-form').exists()).toBe(false)
    expect(mockShowSuccess).toHaveBeenCalledWith('payment.admin.refundSuccess')

    await findRefundAction(wrapper)!.trigger('click')
    await flushPromises()
    expect(wrapper.find('#force-refund').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('insufficient balance, use force')
  })
})
