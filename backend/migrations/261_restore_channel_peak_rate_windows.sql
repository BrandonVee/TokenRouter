-- 历史版本的 259 迁移可能只创建单时段字段，此迁移前向补齐多时段配置列。
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS peak_rate_windows JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN channel_model_pricing.peak_rate_windows IS
    '每周重复的峰谷定价区间，元素含 weekdays(周一=0至周日=6)、start、end、multiplier；支持跨天';
