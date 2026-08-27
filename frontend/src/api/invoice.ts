import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type { PaymentOrder } from '@/types/payment'
import type { InvoiceRequest, InvoiceRequestDetail, InvoiceType } from '@/types/invoice'

export const invoiceAPI = {
	getEligibleOrders(params?: { page?: number; page_size?: number }) {
		return apiClient.get<BasePaginationResponse<PaymentOrder>>('/payment/invoices/eligible-orders', { params })
  },
  list(params?: { page?: number; page_size?: number }) {
    return apiClient.get<BasePaginationResponse<InvoiceRequest>>('/payment/invoices', { params })
  },
	create(data: { order_ids: number[]; invoice_type: InvoiceType; invoice_title: string; tax_id?: string; bank_name?: string; bank_account?: string; recipient_email?: string; remark?: string }) {
    return apiClient.post<InvoiceRequest>('/payment/invoices', data)
  },
  get(id: number) {
    return apiClient.get<InvoiceRequestDetail>(`/payment/invoices/${id}`)
  },
  downloadAttachment(id: number) {
    return apiClient.get(`/payment/invoices/attachments/${id}/download`, { responseType: 'blob' })
  }
}
