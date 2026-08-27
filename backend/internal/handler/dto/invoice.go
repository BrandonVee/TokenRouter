package dto

import "time"

// InvoiceRequestResponse 是面向面板的发票申请安全响应。
type InvoiceRequestResponse struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id,omitempty"`
	UserEmail       string     `json:"user_email,omitempty"`
	RequestNo       string     `json:"request_no"`
	Status          string     `json:"status"`
	Currency        string     `json:"currency"`
	TotalAmount     float64    `json:"total_amount"`
	InvoiceType     string     `json:"invoice_type"`
	InvoiceTitle    string     `json:"invoice_title"`
	TaxID           *string    `json:"tax_id,omitempty"`
	BankName        *string    `json:"bank_name,omitempty"`
	BankAccount     *string    `json:"bank_account,omitempty"`
	RecipientEmail  string     `json:"recipient_email"`
	AccountEmail    string     `json:"account_email,omitempty"`
	Remark          *string    `json:"remark,omitempty"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	ReviewedBy      *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	InvoiceNumber   *string    `json:"invoice_number,omitempty"`
	IssuedAt        *time.Time `json:"issued_at,omitempty"`
	IssueRemark     *string    `json:"issue_remark,omitempty"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// InvoiceRequestItemResponse 是冻结订单的开票快照。
type InvoiceRequestItemResponse struct {
	ID              int64          `json:"id"`
	PaymentOrderID  int64          `json:"payment_order_id"`
	OrderNo         string         `json:"order_no"`
	OrderType       string         `json:"order_type"`
	Currency        string         `json:"currency"`
	PayAmount       float64        `json:"pay_amount"`
	CreditedAmount  float64        `json:"credited_amount"`
	RechargeCode    *string        `json:"recharge_code,omitempty"`
	ProductSnapshot map[string]any `json:"product_snapshot,omitempty"`
	PaidAt          *time.Time     `json:"paid_at,omitempty"`
}

// InvoiceAttachmentResponse 不包含内部存储路径。
type InvoiceAttachmentResponse struct {
	ID          int64     `json:"id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadedBy  int64     `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// InvoiceDeliveryResponse 是邮件投递审计摘要。
type InvoiceDeliveryResponse struct {
	ID             int64     `json:"id"`
	RecipientEmail string    `json:"recipient_email"`
	Status         string    `json:"status"`
	MessageID      *string   `json:"message_id,omitempty"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	SentBy         *int64    `json:"sent_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
