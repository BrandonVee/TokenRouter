-- 分组按账号成本倍率执行利润准入，存量默认关闭。
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS profit_control_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS profit_min_margin DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS profit_safety_buffer DECIMAL(10,4) NOT NULL DEFAULT 0;
