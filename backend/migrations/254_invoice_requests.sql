-- 支付完成后的人工开票申请与受保护附件；支付订单继续作为金额权威来源。
CREATE TABLE IF NOT EXISTS invoice_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_no VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'SUBMITTED',
    currency VARCHAR(16) NOT NULL,
    total_amount DECIMAL(20,2) NOT NULL,
    invoice_title VARCHAR(255) NOT NULL,
    tax_id VARCHAR(128),
    recipient_email VARCHAR(255) NOT NULL,
    account_email VARCHAR(255) NOT NULL,
    remark TEXT,
    rejection_reason TEXT,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    invoice_number VARCHAR(128),
    issued_at TIMESTAMPTZ,
    issue_remark TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invoice_requests_status_check CHECK (status IN ('SUBMITTED', 'REJECTED', 'APPROVED', 'ISSUED', 'SENT')),
    CONSTRAINT invoice_requests_total_amount_check CHECK (total_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_invoice_requests_user_created_at ON invoice_requests (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_status_created_at ON invoice_requests (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoice_requests_invoice_number ON invoice_requests (invoice_number) WHERE invoice_number IS NOT NULL;

CREATE TABLE IF NOT EXISTS invoice_request_items (
    id BIGSERIAL PRIMARY KEY,
    invoice_request_id BIGINT NOT NULL REFERENCES invoice_requests(id) ON DELETE CASCADE,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    order_no VARCHAR(64) NOT NULL,
    order_type VARCHAR(20) NOT NULL,
    currency VARCHAR(16) NOT NULL,
    pay_amount DECIMAL(20,2) NOT NULL,
    credited_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    recharge_code VARCHAR(64),
    product_snapshot JSONB,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoice_request_items_request ON invoice_request_items (invoice_request_id);
CREATE INDEX IF NOT EXISTS idx_invoice_request_items_order ON invoice_request_items (payment_order_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoice_request_items_active_order ON invoice_request_items (payment_order_id) WHERE active;

CREATE TABLE IF NOT EXISTS invoice_attachments (
    id BIGSERIAL PRIMARY KEY,
    invoice_request_id BIGINT NOT NULL REFERENCES invoice_requests(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_key VARCHAR(255) NOT NULL UNIQUE,
    sha256 VARCHAR(64) NOT NULL,
    uploaded_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invoice_attachments_size_check CHECK (size_bytes > 0)
);

CREATE INDEX IF NOT EXISTS idx_invoice_attachments_request_created ON invoice_attachments (invoice_request_id, created_at);

CREATE TABLE IF NOT EXISTS invoice_deliveries (
    id BIGSERIAL PRIMARY KEY,
    invoice_request_id BIGINT NOT NULL REFERENCES invoice_requests(id) ON DELETE CASCADE,
    recipient_email VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    message_id VARCHAR(255),
    attachment_summary TEXT,
    error_message TEXT,
    sent_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invoice_deliveries_status_check CHECK (status IN ('SENT', 'FAILED'))
);

CREATE INDEX IF NOT EXISTS idx_invoice_deliveries_request_created ON invoice_deliveries (invoice_request_id, created_at);
