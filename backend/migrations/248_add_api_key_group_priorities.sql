-- 为普通 API Key 增加有序候选分组；空数组保持原有默认分组语义。
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS group_ids jsonb NOT NULL DEFAULT '[]'::jsonb;

-- 现有普通 Key 回填当前绑定分组，确保升级后行为不变。
UPDATE api_keys
SET group_ids = jsonb_build_array(group_id)
WHERE is_composite = FALSE
  AND group_id IS NOT NULL
  AND group_ids = '[]'::jsonb;
