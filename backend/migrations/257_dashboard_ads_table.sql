-- 将仪表盘广告从 settings JSON 拆分为可独立更新的表。
CREATE TABLE IF NOT EXISTS dashboard_ads (
    id VARCHAR(100) PRIMARY KEY,
    image_url TEXT NOT NULL DEFAULT '',
    link_url TEXT NOT NULL DEFAULT '',
    starts_at TIMESTAMPTZ DEFAULT NULL,
    ends_at TIMESTAMPTZ DEFAULT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dashboard_ads_sort_order ON dashboard_ads(sort_order);
CREATE INDEX IF NOT EXISTS idx_dashboard_ads_enabled ON dashboard_ads(enabled);

-- 首次升级时尽力迁移旧 JSON；异常或非数组值不会阻断整次迁移。
DO $$
DECLARE
    legacy_value TEXT;
BEGIN
    SELECT value INTO legacy_value FROM settings WHERE key = 'dashboard_ads';
    IF legacy_value IS NULL OR btrim(legacy_value) = '' THEN
        RETURN;
    END IF;
    BEGIN
        INSERT INTO dashboard_ads (id, image_url, link_url, starts_at, ends_at, enabled, sort_order)
        SELECT
            COALESCE(NULLIF(item->>'id', ''), md5('legacy-dashboard-ad-' || ordinality::TEXT)),
            COALESCE(item->>'image_url', ''),
            COALESCE(item->>'link_url', ''),
            NULLIF(item->>'starts_at', '')::TIMESTAMPTZ,
            NULLIF(item->>'ends_at', '')::TIMESTAMPTZ,
            COALESCE((item->>'enabled')::BOOLEAN, TRUE),
            (ordinality - 1)::INTEGER
        FROM jsonb_array_elements(legacy_value::JSONB) WITH ORDINALITY AS entries(item, ordinality)
        ON CONFLICT (id) DO NOTHING;
        DELETE FROM settings WHERE key = 'dashboard_ads';
    EXCEPTION WHEN OTHERS THEN
        RAISE NOTICE 'skip legacy dashboard_ads migration: %', SQLERRM;
    END;
END $$;
