-- 为渠道模型定价增加独立的 token 峰谷时段配置。
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS peak_rate_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS peak_start VARCHAR(5) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS peak_end VARCHAR(5) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS peak_rate_multiplier NUMERIC(12,6) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS peak_rate_windows JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN channel_model_pricing.peak_rate_enabled IS
    '是否对该模型 token 价格启用峰谷时段';
COMMENT ON COLUMN channel_model_pricing.peak_start IS
    '高峰开始时间，HH:MM，按服务器时区解释';
COMMENT ON COLUMN channel_model_pricing.peak_end IS
    '高峰结束时间，HH:MM，按服务器时区解释';
COMMENT ON COLUMN channel_model_pricing.peak_rate_multiplier IS
    '高峰窗口内应用到模型 token 价格的倍率';
COMMENT ON COLUMN channel_model_pricing.peak_rate_windows IS
    '每周重复的峰谷定价区间，元素含 weekdays(周一=0至周日=6)、start、end、multiplier；支持跨天';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_model_pricing_peak_rate_multiplier_nonnegative'
          AND conrelid = 'channel_model_pricing'::regclass
    ) THEN
        ALTER TABLE channel_model_pricing
            ADD CONSTRAINT channel_model_pricing_peak_rate_multiplier_nonnegative
            CHECK (peak_rate_multiplier >= 0);
    END IF;
END $$;
