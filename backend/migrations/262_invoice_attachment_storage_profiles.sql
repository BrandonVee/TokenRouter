-- 发票附件记录写入时使用的存储档案，旧附件仍指向默认本地目录。
ALTER TABLE invoice_attachments
    ALTER COLUMN storage_key TYPE VARCHAR(1024),
    ADD COLUMN IF NOT EXISTS storage_type VARCHAR(16) NOT NULL DEFAULT 'local',
    ADD COLUMN IF NOT EXISTS storage_profile_id VARCHAR(64) NOT NULL DEFAULT 'local-default';

CREATE INDEX IF NOT EXISTS idx_invoice_attachments_storage_profile
    ON invoice_attachments (storage_profile_id);
