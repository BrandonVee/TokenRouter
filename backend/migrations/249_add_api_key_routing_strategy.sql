ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS routing_strategy varchar(32) NOT NULL DEFAULT 'manual';

COMMENT ON COLUMN api_keys.routing_strategy IS
    'API Key routing strategy: manual, auto, speed, price, or success_rate';
