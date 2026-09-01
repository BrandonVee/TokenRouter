-- 保存生图请求的非敏感参数快照；原始图片输入和凭据不会写入该字段。
ALTER TABLE image_histories
    ADD COLUMN IF NOT EXISTS parameters TEXT NOT NULL DEFAULT '';
