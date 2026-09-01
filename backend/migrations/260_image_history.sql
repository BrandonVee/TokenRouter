-- 用户主动开启后才保存同步生图结果；图片字节位于私有 S3，数据库只保存元数据。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS save_image_history BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS image_histories (
    id VARCHAR(36) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    request_id VARCHAR(255) NOT NULL DEFAULT '',
    source VARCHAR(32) NOT NULL,
    endpoint VARCHAR(64) NOT NULL,
    model VARCHAR(255) NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    revised_prompt TEXT NOT NULL DEFAULT '',
    object_key TEXT NOT NULL UNIQUE,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    sha256 VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_histories_size_bytes_positive CHECK (size_bytes > 0),
    CONSTRAINT image_histories_width_nonnegative CHECK (width >= 0),
    CONSTRAINT image_histories_height_nonnegative CHECK (height >= 0)
);

CREATE INDEX IF NOT EXISTS idx_image_histories_user_created_at
    ON image_histories (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_image_histories_request_id
    ON image_histories (request_id);
