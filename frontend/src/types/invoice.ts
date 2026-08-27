export type InvoiceRequestStatus = 'SUBMITTED' | 'REJECTED' | 'APPROVED' | 'ISSUED' | 'SENT'
export type InvoiceType = 'PERSONAL' | 'ENTERPRISE'

export interface InvoiceRequest {
  id: number
  user_id?: number
	user_email?: string
  request_no: string
  status: InvoiceRequestStatus
  currency: string
  total_amount: number
  invoice_type: InvoiceType
	invoice_title: string
	tax_id?: string
	bank_name?: string
	bank_account?: string
  recipient_email: string
  account_email?: string
  remark?: string
  rejection_reason?: string
  reviewed_by?: number
  reviewed_at?: string
  invoice_number?: string
  issued_at?: string
  issue_remark?: string
  sent_at?: string
  created_at: string
  updated_at: string
}

export interface InvoiceRequestItem {
  id: number
  payment_order_id: number
  order_no: string
  order_type: 'balance' | 'subscription'
  currency: string
  pay_amount: number
  credited_amount: number
  recharge_code?: string
  product_snapshot?: Record<string, unknown>
  paid_at?: string
}

export interface InvoiceAttachment {
  id: number
  file_name: string
  content_type: string
  size_bytes: number
  uploaded_by: number
  created_at: string
}

export interface InvoiceDelivery {
  id: number
  recipient_email: string
  status: 'SENT' | 'FAILED'
  error_message?: string
  sent_by?: number
  created_at: string
}

export interface InvoiceRequestDetail {
  request: InvoiceRequest
  items: InvoiceRequestItem[]
  attachments: InvoiceAttachment[]
  deliveries?: InvoiceDelivery[]
}
