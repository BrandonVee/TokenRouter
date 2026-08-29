-- 为已有仪表盘广告补充图片适应模式，默认保持比例自适应。
ALTER TABLE dashboard_ads
    ADD COLUMN IF NOT EXISTS fit_mode VARCHAR(16) NOT NULL DEFAULT 'adaptive';

UPDATE dashboard_ads
SET fit_mode = 'adaptive'
WHERE fit_mode IS NULL OR LOWER(fit_mode) NOT IN ('adaptive', 'cover', 'fill');
