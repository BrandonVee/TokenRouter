-- 专用图片端点可按实际返回张数计费，存量分组默认保持原计费口径。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS image_count_billing BOOLEAN NOT NULL DEFAULT FALSE;
