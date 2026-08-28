import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type { InvoiceAttachment, InvoiceRequest, InvoiceRequestDetail } from '@/types/invoice'

export const adminInvoiceAPI = {
  list(params?: { page?: number; page_size?: number; status?: string; keyword?: string }) {
    return apiClient.get<BasePaginationResponse<InvoiceRequest>>('/admin/payment/invoices', { params })
  },
  get(id: number) {
    return apiClient.get<InvoiceRequestDetail>(`/admin/payment/invoices/${id}`)
  },
  approve(id: number) {
    return apiClient.post<InvoiceRequest>(`/admin/payment/invoices/${id}/approve`)
  },
  reject(id: number, reason: string) {
    return apiClient.post<InvoiceRequest>(`/admin/payment/invoices/${id}/reject`, { reason })
  },
  uploadAttachment(id: number, attachment: File) {
    const data = new FormData()
    data.append('attachment', attachment)
    return apiClient.post<InvoiceAttachment>(`/admin/payment/invoices/${id}/attachments`, data)
  },
  issue(id: number, data: { invoice_number?: string; issued_at?: string; remark?: string }) {
    return apiClient.post<InvoiceRequest>(`/admin/payment/invoices/${id}/issue`, data)
  },
  deleteAttachment(id: number) {
    return apiClient.delete<{ message: string }>(`/admin/payment/invoices/attachments/${id}`)
  },
  send(id: number) {
    return apiClient.post<InvoiceRequest>(`/admin/payment/invoices/${id}/send`)
  },
  downloadAttachment(id: number, preview = false) {
    return apiClient.get(`/admin/payment/invoices/attachments/${id}/download`, { responseType: 'blob', params: preview ? { preview: '1' } : undefined })
  }
}
