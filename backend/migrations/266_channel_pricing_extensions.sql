-- 推理倍率按最终转发等级显式配置，不按特定模型注入默认倍数。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'channel_model_pricing'::regclass
          AND attname = 'reasoning_effort_multipliers'
          AND NOT attisdropped
    ) THEN
        ALTER TABLE channel_model_pricing
            ADD COLUMN IF NOT EXISTS reasoning_effort_multipliers JSONB NOT NULL DEFAULT '{}'::jsonb;

        -- fork 未曾引入旧 max 列；仅在兼容导入的数据库存在该列时回填。
        IF EXISTS (
            SELECT 1 FROM pg_attribute
            WHERE attrelid = 'channel_model_pricing'::regclass
              AND attname = 'max_reasoning_effort_multiplier' AND NOT attisdropped
        ) THEN
            UPDATE channel_model_pricing
            SET reasoning_effort_multipliers = jsonb_build_object('max', max_reasoning_effort_multiplier)
            WHERE max_reasoning_effort_multiplier IS NOT NULL;
        END IF;
    END IF;
END $$;

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS reasoning_effort_multipliers JSONB NOT NULL DEFAULT '{}'::jsonb;

-- 分组 JSON 中显式配置的通用倍率优先；消费旧字段后删除它，重放不会恢复已清空配置。
UPDATE groups AS g
SET model_pricing = (
    SELECT jsonb_agg(
        CASE
            WHEN jsonb_typeof(entry) = 'object' AND entry ? 'max_reasoning_effort_multiplier' THEN
                (entry - 'max_reasoning_effort_multiplier') ||
                CASE
                    WHEN NOT (entry ? 'reasoning_effort_multipliers')
                         AND jsonb_typeof(entry->'max_reasoning_effort_multiplier') = 'number' THEN
                        jsonb_build_object('reasoning_effort_multipliers',
                            jsonb_build_object('max', entry->'max_reasoning_effort_multiplier'))
                    ELSE '{}'::jsonb
                END
            ELSE entry
        END ORDER BY ordinal
    )
    FROM jsonb_array_elements(g.model_pricing) WITH ORDINALITY AS pricing(entry, ordinal)
)
WHERE jsonb_typeof(g.model_pricing) = 'array'
  AND EXISTS (
      SELECT 1
      FROM jsonb_array_elements(
          CASE WHEN jsonb_typeof(g.model_pricing) = 'array' THEN g.model_pricing ELSE '[]'::jsonb END
      ) AS pricing(entry)
      WHERE jsonb_typeof(entry) = 'object' AND entry ? 'max_reasoning_effort_multiplier'
  );

COMMENT ON COLUMN channel_model_pricing.reasoning_effort_multipliers IS
    '按实际推理等级配置计费倍率，缺省等级为一倍';
COMMENT ON COLUMN channel_account_stats_model_pricing.reasoning_effort_multipliers IS
    '账号统计按实际推理等级配置倍率，缺省等级为一倍';

-- 分时和 1h 缓存写入扩展不改写已有峰谷规则或基础价格。
ALTER TABLE channel_model_pricing ADD COLUMN IF NOT EXISTS time_pricing JSONB NULL;
ALTER TABLE channel_account_stats_model_pricing ADD COLUMN IF NOT EXISTS time_pricing JSONB NULL;
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS cache_write_1h_price NUMERIC(20,12);

ALTER TABLE channel_pricing_intervals
    ADD COLUMN IF NOT EXISTS cache_write_1h_price NUMERIC(20,12);

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS cache_write_1h_price NUMERIC(20,12);

ALTER TABLE channel_account_stats_pricing_intervals
    ADD COLUMN IF NOT EXISTS cache_write_1h_price NUMERIC(20,12);

COMMENT ON COLUMN channel_model_pricing.cache_write_1h_price IS
    '每 token 的 1h 缓存写入价格，NULL 保留原缓存写入价格行为';
COMMENT ON COLUMN channel_pricing_intervals.cache_write_1h_price IS
    '上下文区间内每 token 的 1h 缓存写入价格';
COMMENT ON COLUMN channel_account_stats_model_pricing.cache_write_1h_price IS
    '账号统计每 token 的 1h 缓存写入价格';
COMMENT ON COLUMN channel_account_stats_pricing_intervals.cache_write_1h_price IS
    '上下文区间内每 token 的 1h 缓存写入价格 for account stats pricing';

-- 账号统计规则与管理表单的图片输入字段保持一致，避免保存时静默丢失。
ALTER TABLE channel_account_stats_model_pricing ADD COLUMN IF NOT EXISTS image_input_price NUMERIC(20,12);
