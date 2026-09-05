import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import type { InvoiceRequestDetail, InvoiceRequestStatus } from '@/types/invoice'
import AdminInvoiceRequestsView from '../AdminInvoiceRequestsView.vue'

const { mockList, mockGet, mockUploadAttachment, mockShowError, mockShowSuccess } = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockGet: vi.fn(),
  mockUploadAttachment: vi.fn(),
  mockShowError: vi.fn(),
  mockShowSuccess: vi.fn(),
}))

vi.mock('@/api/admin/invoice', () => {
  const adminInvoiceAPI = {
    list: (...args: unknown[]) => mockList(...args),
    get: (...args: unknown[]) => mockGet(...args),
    uploadAttachment: (...args: unknown[]) => mockUploadAttachment(...args),
  }
  return { adminInvoiceAPI, default: adminInvoiceAPI }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mockShowError, showSuccess: mockShowSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

// 组装一份已审批的申请详情，明细金额与申请总额故意不一致，用于校验按明细汇总。
function buildDetail(status: InvoiceRequestStatus): InvoiceRequestDetail {
  return {
    request: {
      id: 9,
      user_id: 7,
      request_no: 'INV-9',
      status,
      currency: 'CNY',
      total_amount: 999,
      invoice_type: 'ENTERPRISE',
      invoice_title: '示例公司',
      tax_id: 'TAX-1',
      recipient_email: 'billing@example.com',
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z',
    },
    items: [
      { id: 1, payment_order_id: 11, order_no: 'ORDER-1', order_type: 'balance', currency: 'CNY', pay_amount: 100, credited_amount: 100 },
      { id: 2, payment_order_id: 12, order_no: 'ORDER-2', order_type: 'subscription', currency: 'CNY', pay_amount: 200, credited_amount: 200 },
    ],
    attachments: [],
    deliveries: [],
  }
}

async function openDetail(wrapper: VueWrapper, status: InvoiceRequestStatus = 'APPROVED') {
  mockGet.mockResolvedValue({ data: buildDetail(status) })
  await wrapper.find('tbody tr td:last-child button').trigger('click')
  await flushPromises()
}

describe('AdminInvoiceRequestsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockList.mockResolvedValue({
      data: {
        items: [buildDetail('APPROVED').request],
        page: 1,
        page_size: 10,
        total: 1,
        pages: 1,
      },
    })
    mockUploadAttachment.mockResolvedValue({ data: {} })
  })

  it('审批后展示按明细汇总的应开发票金额', async () => {
    const wrapper = mount(AdminInvoiceRequestsView, { attachTo: document.body })
    await flushPromises()
    await openDetail(wrapper)
    const text = document.body.textContent || ''
    expect(text).toContain('admin.invoices.invoiceAmount')
    expect(text).toContain('¥300.00')
    wrapper.unmount()
  })

  it('支持拖拽上传附件并在上传后刷新详情', async () => {
    const wrapper = mount(AdminInvoiceRequestsView, { attachTo: document.body })
    await flushPromises()
    await openDetail(wrapper)
    mockGet.mockResolvedValue({ data: buildDetail('APPROVED') })
    const callsBefore = mockGet.mock.calls.length
    // 详情弹窗通过 Teleport 挂到 body，拖拽区域要在 document 上定位。
    const dropzone = document.body.querySelector('[class*="border-dashed"]')
    expect(dropzone).not.toBeNull()
    const file = new File(['invoice'], 'invoice.pdf', { type: 'application/pdf' })
    const event = new Event('drop', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'dataTransfer', { value: { files: [file] } })
    dropzone?.dispatchEvent(event)
    await flushPromises()
    expect(mockUploadAttachment).toHaveBeenCalledWith(9, file)
    expect(mockGet.mock.calls.length).toBeGreaterThan(callsBefore)
    wrapper.unmount()
  })
})
