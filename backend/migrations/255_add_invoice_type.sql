-- 为人工发票申请冻结个人或企业开票类型，存量申请按个人开票兼容。
ALTER TABLE invoice_requests
    ADD COLUMN IF NOT EXISTS invoice_type VARCHAR(20) NOT NULL DEFAULT 'PERSONAL';

ALTER TABLE invoice_requests
    DROP CONSTRAINT IF EXISTS invoice_requests_invoice_type_check;

ALTER TABLE invoice_requests
    ADD CONSTRAINT invoice_requests_invoice_type_check
    CHECK (invoice_type IN ('PERSONAL', 'ENTERPRISE'));
