-- 为 API Key 增加 30 天滚动月限额；0 表示该维度不限制。
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS rate_limit_30d DECIMAL(20, 8) NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS usage_30d DECIMAL(20, 8) NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS window_30d_start TIMESTAMPTZ;

COMMENT ON COLUMN api_keys.rate_limit_30d IS '30 天滚动消费金额限额（USD，0 表示不限额）';
COMMENT ON COLUMN api_keys.usage_30d IS '当前 30 天滚动窗口内的已消费金额（USD）';
COMMENT ON COLUMN api_keys.window_30d_start IS '当前 30 天消费金额窗口的起始时间';
