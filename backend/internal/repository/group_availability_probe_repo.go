package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/lib/pq"
)

type groupAvailabilityProbeRepository struct {
	db *sql.DB
}

func NewGroupAvailabilityProbeRepository(db *sql.DB) service.GroupAvailabilityProbeRepository {
	return &groupAvailabilityProbeRepository{db: db}
}

func (r *groupAvailabilityProbeRepository) ClaimDue(ctx context.Context, now time.Time, lockUntil time.Time, lockedBy string, limit int) ([]service.GroupAvailabilityProbeDueGroup, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// 先将启用探测的分组补进状态表；分组被禁用时再清掉状态，避免长期扫描无效记录。
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_availability_probe_states (group_id, next_run_at, created_at, updated_at)
		SELECT id, $1, NOW(), NOW()
		FROM groups
		WHERE deleted_at IS NULL
		  AND status = 'active'
		  AND availability_probe_config @> '{"enabled": true}'::jsonb
		ON CONFLICT (group_id) DO NOTHING
	`, now); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM group_availability_probe_states s
		WHERE NOT EXISTS (
			SELECT 1
			FROM groups g
			WHERE g.id = s.group_id
			  AND g.deleted_at IS NULL
			  AND g.status = 'active'
			  AND g.availability_probe_config @> '{"enabled": true}'::jsonb
		)
	`); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		WITH due AS (
			SELECT s.group_id
			FROM group_availability_probe_states s
			JOIN groups g ON g.id = s.group_id
			WHERE g.deleted_at IS NULL
			  AND g.status = 'active'
			  AND g.availability_probe_config @> '{"enabled": true}'::jsonb
			  AND (s.next_run_at IS NULL OR s.next_run_at <= $1)
			  AND (s.locked_until IS NULL OR s.locked_until <= $1)
			ORDER BY COALESCE(s.next_run_at, 'epoch'::timestamptz), s.group_id
			LIMIT $4
			FOR UPDATE OF s SKIP LOCKED
		),
		claimed AS (
			UPDATE group_availability_probe_states s
			SET locked_until = $2,
				locked_by = $3,
				updated_at = NOW()
			FROM due
			WHERE s.group_id = due.group_id
			RETURNING s.group_id
		)
		SELECT g.id, g.name, g.platform, g.availability_probe_config
		FROM claimed c
		JOIN groups g ON g.id = c.group_id
		ORDER BY g.id
	`, now, lockUntil, lockedBy, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	dueGroups := make([]service.GroupAvailabilityProbeDueGroup, 0)
	for rows.Next() {
		var item service.GroupAvailabilityProbeDueGroup
		var rawConfig []byte
		if err := rows.Scan(&item.GroupID, &item.Name, &item.Platform, &rawConfig); err != nil {
			return nil, err
		}
		if len(rawConfig) > 0 {
			if err := json.Unmarshal(rawConfig, &item.Config); err != nil {
				return nil, fmt.Errorf("decode availability probe config: %w", err)
			}
		}
		dueGroups = append(dueGroups, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return dueGroups, nil
}

func (r *groupAvailabilityProbeRepository) SaveResultAndScheduleNext(ctx context.Context, result *service.GroupAvailabilityProbeResult, nextRunAt time.Time) error {
	if r == nil || r.db == nil || result == nil {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_availability_probe_results (
			group_id, account_id, model_id, status, success, latency_ms,
			error_message, started_at, finished_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`, result.GroupID, result.AccountID, result.ModelID, result.Status, result.Success, result.LatencyMs, nullableProbeString(result.ErrorMessage), result.StartedAt, result.FinishedAt); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO group_availability_probe_states (
			group_id, next_run_at, locked_until, locked_by, last_status,
			last_success, last_latency_ms, last_error, last_checked_at, created_at, updated_at
		)
		VALUES ($1, $2, NULL, NULL, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (group_id) DO UPDATE SET
			next_run_at = EXCLUDED.next_run_at,
			locked_until = NULL,
			locked_by = NULL,
			last_status = EXCLUDED.last_status,
			last_success = EXCLUDED.last_success,
			last_latency_ms = EXCLUDED.last_latency_ms,
			last_error = EXCLUDED.last_error,
			last_checked_at = EXCLUDED.last_checked_at,
			updated_at = NOW()
	`, result.GroupID, nextRunAt, result.Status, result.Success, result.LatencyMs, nullableProbeString(result.ErrorMessage), result.FinishedAt); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *groupAvailabilityProbeRepository) GetSummaryByGroupIDs(ctx context.Context, groupIDs []int64, days int, bucketMinutes int, timezoneName string, now time.Time) (map[int64]*service.GroupAvailabilitySummary, error) {
	out := make(map[int64]*service.GroupAvailabilitySummary, len(groupIDs))
	if r == nil || r.db == nil || len(groupIDs) == 0 {
		return out, nil
	}
	days, bucketMinutes = service.NormalizeMarketplaceAvailabilityWindow(days, bucketMinutes)

	loc := time.UTC
	if timezoneName != "" {
		if parsed, err := time.LoadLocation(timezoneName); err == nil && parsed != nil {
			loc = parsed
		}
	}
	localNow := now.In(loc)
	bucketDuration := time.Duration(bucketMinutes) * time.Minute
	endLocal := nextLocalBucketBoundary(localNow, bucketDuration)
	totalMinutes := days * 24 * 60
	bucketCount := (totalMinutes + bucketMinutes - 1) / bucketMinutes
	if bucketCount <= 0 {
		bucketCount = 1
	}
	startLocal := endLocal.Add(-time.Duration(bucketCount) * bucketDuration)
	startUTC := startLocal.UTC()
	endUTC := endLocal.UTC()

	for _, groupID := range groupIDs {
		summary := &service.GroupAvailabilitySummary{
			WindowDays:    days,
			BucketMinutes: bucketMinutes,
			Days:          make([]service.GroupAvailabilityBucket, 0, bucketCount),
		}
		for i := 0; i < bucketCount; i++ {
			bucketStart := startLocal.Add(time.Duration(i) * bucketDuration)
			summary.Days = append(summary.Days, service.GroupAvailabilityBucket{
				Date: bucketStart.Format(time.RFC3339),
			})
		}
		out[groupID] = summary
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			group_id,
			FLOOR((EXTRACT(EPOCH FROM started_at) - EXTRACT(EPOCH FROM $2::timestamptz)) / ($4::double precision * 60))::int AS bucket_index,
			COUNT(*) FILTER (WHERE success = true) AS success_count,
			COUNT(*) AS total_count
		FROM group_availability_probe_results
		WHERE group_id = ANY($1)
		  AND started_at >= $2
		  AND started_at < $3
		GROUP BY group_id, bucket_index
	`, pq.Array(groupIDs), startUTC, endUTC, bucketMinutes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var groupID int64
		var bucketIndex int
		var successCount int64
		var totalCount int64
		if err := rows.Scan(&groupID, &bucketIndex, &successCount, &totalCount); err != nil {
			return nil, err
		}
		summary, ok := out[groupID]
		if !ok {
			continue
		}
		if bucketIndex < 0 || bucketIndex >= len(summary.Days) {
			continue
		}
		rate := availabilityRate(successCount, totalCount)
		summary.Days[bucketIndex].SuccessCount = successCount
		summary.Days[bucketIndex].TotalCount = totalCount
		summary.Days[bucketIndex].AvailabilityRate = rate
		summary.SuccessCount += successCount
		summary.TotalCount += totalCount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, summary := range out {
		summary.Mode = service.MarketplaceAvailabilityModeActive
		summary.AvailabilityRate = availabilityRate(summary.SuccessCount, summary.TotalCount)
	}

	lastRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (group_id)
			group_id, status, finished_at
		FROM group_availability_probe_results
		WHERE group_id = ANY($1)
		ORDER BY group_id, started_at DESC, id DESC
	`, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = lastRows.Close() }()

	for lastRows.Next() {
		var groupID int64
		var status string
		var checkedAt time.Time
		if err := lastRows.Scan(&groupID, &status, &checkedAt); err != nil {
			return nil, err
		}
		if summary, ok := out[groupID]; ok {
			summary.LastStatus = status
			t := checkedAt
			summary.LastCheckedAt = &t
		}
	}
	if err := lastRows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// GetPassiveSummaryByGroupIDs 从真实请求日志构建固定 60 个时间桶的可用性摘要。
func (r *groupAvailabilityProbeRepository) GetPassiveSummaryByGroupIDs(ctx context.Context, groupIDs []int64, days int, bucketMinutes int, timezoneName string, now time.Time) (map[int64]*service.GroupAvailabilitySummary, error) {
	out := make(map[int64]*service.GroupAvailabilitySummary, len(groupIDs))
	if r == nil || r.db == nil || len(groupIDs) == 0 {
		return out, nil
	}
	days, bucketMinutes = service.NormalizeMarketplaceAvailabilityWindow(days, bucketMinutes)
	loc := time.UTC
	if timezoneName != "" {
		if parsed, err := time.LoadLocation(timezoneName); err == nil && parsed != nil {
			loc = parsed
		}
	}
	bucketDuration := time.Duration(bucketMinutes) * time.Minute
	endLocal := nextLocalBucketBoundary(now.In(loc), bucketDuration)
	startLocal := endLocal.Add(-service.PassiveAvailabilityBucketCount * bucketDuration)
	startUTC := startLocal.UTC()
	endUTC := endLocal.UTC()
	for _, groupID := range groupIDs {
		summary := &service.GroupAvailabilitySummary{
			Mode:          service.MarketplaceAvailabilityModePassive,
			WindowDays:    days,
			BucketMinutes: bucketMinutes,
			Days:          make([]service.GroupAvailabilityBucket, 0, service.PassiveAvailabilityBucketCount),
		}
		for i := 0; i < service.PassiveAvailabilityBucketCount; i++ {
			summary.Days = append(summary.Days, service.GroupAvailabilityBucket{
				Date: startLocal.Add(time.Duration(i) * bucketDuration).Format(time.RFC3339),
			})
		}
		out[groupID] = summary
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH events AS (
			SELECT
				ul.id,
				ul.group_id,
				ul.request_id,
				ul.created_at,
				CASE
					WHEN COALESCE(ul.stream, false)
						AND COALESCE(ul.first_token_ms, 0) > 30000
						AND LOWER(COALESCE(ul.billing_mode, '')) <> 'image'
						AND COALESCE(ul.image_count, 0) = 0
						THEN 'slow_stream'
					ELSE 'success'
				END::text AS status,
				true AS success,
				1 AS source_priority
			FROM usage_logs ul
			WHERE ul.group_id = ANY($1) AND ul.created_at >= $2 AND ul.created_at < $3
			UNION ALL
			SELECT
				oe.id,
				oe.group_id,
				oe.request_id,
				oe.created_at,
				CASE
					WHEN COALESCE(oe.upstream_status_code, 0) BETWEEN 500 AND 599
						OR (
							COALESCE(oe.status_code, 0) BETWEEN 500 AND 599
							AND (
								LOWER(COALESCE(oe.error_owner, '')) = 'provider'
								OR LOWER(COALESCE(oe.error_phase, '')) = 'upstream'
								OR LOWER(COALESCE(oe.error_source, '')) = 'upstream_http'
							)
						)
						OR LOWER(COALESCE(oe.error_message, '')) ~ 'upstream[[:space:]_-]+request[[:space:]_-]+failed'
						THEN 'upstream_error'
					ELSE 'unknown'
				END AS status,
				false AS success,
				0 AS source_priority
			FROM ops_error_logs oe
			WHERE oe.group_id = ANY($1)
			  AND oe.created_at >= $2 AND oe.created_at < $3
			  AND NOT COALESCE(oe.is_business_limited, false)
			  AND NOT COALESCE(oe.is_count_tokens, false)
			  AND COALESCE(oe.status_code, 0) NOT IN (429, 529)
			  AND COALESCE(oe.upstream_status_code, 0) NOT IN (429, 529)
			  AND NOT (LOWER(COALESCE(oe.error_type, '') || ' ' || COALESCE(oe.error_message, '')) ~ '(rate.?limit|throttl|capacity|compute|overload|concurrency)')
			  AND (
				COALESCE(oe.upstream_status_code, 0) BETWEEN 500 AND 599
				OR (
					COALESCE(oe.status_code, 0) BETWEEN 500 AND 599
					AND (
						LOWER(COALESCE(oe.error_owner, '')) = 'provider'
						OR LOWER(COALESCE(oe.error_phase, '')) = 'upstream'
						OR LOWER(COALESCE(oe.error_source, '')) = 'upstream_http'
					)
				)
				OR LOWER(COALESCE(oe.error_message, '')) ~ 'upstream[[:space:]_-]+request[[:space:]_-]+failed'
			  )
		), deduplicated AS (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY group_id, COALESCE(NULLIF(request_id, ''), source_priority::text || ':' || id::text)
				ORDER BY source_priority DESC, created_at DESC, id DESC
			) AS request_rank
			FROM events
		), bucketed AS (
			SELECT *, FLOOR(
				(EXTRACT(EPOCH FROM created_at) - EXTRACT(EPOCH FROM $2::timestamptz)) /
				($4::double precision * 60)
			)::int AS bucket_index
			FROM deduplicated
			WHERE request_rank = 1
		)
		SELECT
			group_id,
			bucket_index,
			COUNT(*) FILTER (WHERE success = true) AS success_count,
			COUNT(*) FILTER (WHERE status = 'slow_stream') AS slow_stream_count,
			COUNT(*) AS total_count,
			(ARRAY_AGG(status ORDER BY created_at DESC, id DESC))[1] AS last_status,
			MAX(created_at) AS last_checked_at
		FROM bucketed
		WHERE bucket_index >= 0 AND bucket_index < 60
		GROUP BY group_id, bucket_index
		ORDER BY group_id, bucket_index
	`, pq.Array(groupIDs), startUTC, endUTC, bucketMinutes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var groupID int64
		var bucketIndex int
		var successCount, slowStreamCount, totalCount int64
		var lastStatus string
		var lastCheckedAt time.Time
		if err := rows.Scan(&groupID, &bucketIndex, &successCount, &slowStreamCount, &totalCount, &lastStatus, &lastCheckedAt); err != nil {
			return nil, err
		}
		summary, ok := out[groupID]
		if !ok || bucketIndex < 0 || bucketIndex >= len(summary.Days) {
			continue
		}
		bucket := &summary.Days[bucketIndex]
		bucket.SuccessCount = successCount
		bucket.SlowStreamCount = slowStreamCount
		bucket.TotalCount = totalCount
		bucket.AvailabilityRate = passiveWeightedAvailabilityRate(successCount, slowStreamCount, totalCount)
		summary.SuccessCount += successCount
		summary.SlowStreamCount += slowStreamCount
		summary.TotalCount += totalCount
		summary.LastStatus = lastStatus
		checkedAt := lastCheckedAt
		summary.LastCheckedAt = &checkedAt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, summary := range out {
		summary.Mode = service.MarketplaceAvailabilityModePassive
		summary.AvailabilityRate = passiveWeightedAvailabilityRate(summary.SuccessCount, summary.SlowStreamCount, summary.TotalCount)
	}
	return out, nil
}

func nextLocalBucketBoundary(t time.Time, bucketDuration time.Duration) time.Time {
	if bucketDuration <= 0 {
		bucketDuration = 24 * time.Hour
	}
	// 以本地自然日零点对齐，避免非 UTC 时区下使用 time.Truncate 产生偏移桶。
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	bucketStart := dayStart.Add(t.Sub(dayStart) / bucketDuration * bucketDuration)
	if bucketStart.Before(t) {
		return bucketStart.Add(bucketDuration)
	}
	return bucketStart
}

func (r *groupAvailabilityProbeRepository) CleanupOldResults(ctx context.Context, before time.Time) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM group_availability_probe_results
		WHERE created_at < $1
	`, before)
	return err
}

func availabilityRate(successCount int64, totalCount int64) *float64 {
	if totalCount <= 0 {
		return nil
	}
	value := float64(successCount) / float64(totalCount)
	return &value
}

// passiveWeightedAvailabilityRate 用上游故障全权重和慢首字四分之一权重计算健康分。
func passiveWeightedAvailabilityRate(successCount int64, slowStreamCount int64, totalCount int64) *float64 {
	if totalCount <= 0 {
		value := 1.0
		return &value
	}
	upstreamErrorCount := totalCount - successCount
	if upstreamErrorCount < 0 {
		upstreamErrorCount = 0
	}
	issueScore := (float64(upstreamErrorCount) + float64(slowStreamCount)*service.PassiveAvailabilitySlowStreamWeight) / float64(totalCount)
	value := 1 - issueScore
	if value < 0 {
		value = 0
	}
	return &value
}

func nullableProbeString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
