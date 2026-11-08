-- 利润和倍率变动通过持久 outbox 失效普通及复合 Key 的认证快照。
CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.platform IS NOT DISTINCT FROM NEW.platform
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.peak_rate_enabled IS NOT DISTINCT FROM NEW.peak_rate_enabled
       AND OLD.peak_start IS NOT DISTINCT FROM NEW.peak_start
       AND OLD.peak_end IS NOT DISTINCT FROM NEW.peak_end
       AND OLD.peak_rate_multiplier IS NOT DISTINCT FROM NEW.peak_rate_multiplier
       AND OLD.require_oauth_only IS NOT DISTINCT FROM NEW.require_oauth_only
       AND OLD.require_privacy_set IS NOT DISTINCT FROM NEW.require_privacy_set
       AND OLD.allow_messages_dispatch IS NOT DISTINCT FROM NEW.allow_messages_dispatch
       AND OLD.unavailable_fallback_group_id IS NOT DISTINCT FROM NEW.unavailable_fallback_group_id
       AND OLD.profit_control_enabled IS NOT DISTINCT FROM NEW.profit_control_enabled
       AND OLD.profit_min_margin IS NOT DISTINCT FROM NEW.profit_min_margin
       AND OLD.profit_safety_buffer IS NOT DISTINCT FROM NEW.profit_safety_buffer
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE (k.group_id = target_group_id OR EXISTS (SELECT 1 FROM api_key_composite_groups cg WHERE cg.api_key_id = k.id AND cg.group_id = target_group_id))
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

-- 删除分组前先保存受影响的 Key；外键级联完成后已无法再找到原成员关系。
DROP TRIGGER IF EXISTS trg_groups_auth_cache_invalidation ON groups;
CREATE TRIGGER trg_groups_auth_cache_invalidation
BEFORE UPDATE OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation();
