-- 为企业发票申请保存开户行和银行账号，个人申请保持为空。
ALTER TABLE invoice_requests
    ADD COLUMN IF NOT EXISTS bank_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS bank_account VARCHAR(128);
