package service

import (
	"context"
	"strings"
	"time"
)

const (
	GroupAvailabilityProbeStatusSuccess         = "success"
	GroupAvailabilityProbeStatusFailed          = "failed"
	GroupAvailabilityRequestStatusPressure      = "pressure"
	GroupAvailabilityRequestStatusUpstreamError = "upstream_error"
	GroupAvailabilityRequestStatusSlowStream    = "slow_stream"
	GroupAvailabilityRequestStatusUnknown       = "unknown"
	// PassiveAvailabilityBucketCount 是被动模式固定展示的时间桶数。
	PassiveAvailabilityBucketCount = 60
	// PassiveAvailabilitySlowStreamWeight 是慢首字请求在异常分中的权重。
	PassiveAvailabilitySlowStreamWeight = 0.25
)

// GroupAvailabilityProbeDueGroup 是等待主动探测的分组快照。
type GroupAvailabilityProbeDueGroup struct {
	GroupID int64
	Name    string
	// Platform 用于选择对应平台的账号调度器。
	Platform string
	Config   GroupAvailabilityProbeConfig
}

// GroupAvailabilityProbeResult 是单次主动探测结果。
type GroupAvailabilityProbeResult struct {
	GroupID      int64
	AccountID    *int64
	ModelID      string
	Status       string
	Success      bool
	LatencyMs    int64
	ErrorMessage string
	StartedAt    time.Time
	FinishedAt   time.Time
}

// IsUpstreamAvailabilityProbeFailure 判断主动探测失败是否属于可计入分母的上游故障。
// 账号选择、权限、模型限制等本地失败不应降低模型广场可用率。
func IsUpstreamAvailabilityProbeFailure(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "upstream request failed") {
		return true
	}
	for i := 0; i+2 < len(normalized); i++ {
		if normalized[i] != '5' || normalized[i+1] < '0' || normalized[i+1] > '9' || normalized[i+2] < '0' || normalized[i+2] > '9' {
			continue
		}
		if (i == 0 || normalized[i-1] < '0' || normalized[i-1] > '9') &&
			(i+3 == len(normalized) || normalized[i+3] < '0' || normalized[i+3] > '9') {
			return true
		}
	}
	return false
}

// GroupAvailabilityRequest 是被动模式下最近一次真实请求的状态样本。
type GroupAvailabilityRequest struct {
	Status    string    `json:"status"`
	Success   bool      `json:"success"`
	CreatedAt time.Time `json:"created_at"`
}

// GroupAvailabilityBucket 是模型广场条形组件的时间桶聚合。
type GroupAvailabilityBucket struct {
	Date             string
	SuccessCount     int64
	SlowStreamCount  int64
	TotalCount       int64
	AvailabilityRate *float64
}

// GroupAvailabilitySummary 是模型广场分组可用性摘要。
type GroupAvailabilitySummary struct {
	Mode             string
	WindowDays       int
	BucketMinutes    int
	SuccessCount     int64
	PressureCount    int64
	SlowStreamCount  int64
	TotalCount       int64
	AvailabilityRate *float64
	LastStatus       string
	LastCheckedAt    *time.Time
	Days             []GroupAvailabilityBucket
	Requests         []GroupAvailabilityRequest
}

// GroupAvailabilityProbeRepository 定义分组主动可用性探测的数据访问接口。
type GroupAvailabilityProbeRepository interface {
	ClaimDue(ctx context.Context, now time.Time, lockUntil time.Time, lockedBy string, limit int) ([]GroupAvailabilityProbeDueGroup, error)
	SaveResultAndScheduleNext(ctx context.Context, result *GroupAvailabilityProbeResult, nextRunAt time.Time) error
	GetSummaryByGroupIDs(ctx context.Context, groupIDs []int64, days int, bucketMinutes int, timezone string, now time.Time) (map[int64]*GroupAvailabilitySummary, error)
	GetPassiveSummaryByGroupIDs(ctx context.Context, groupIDs []int64, days int, bucketMinutes int, timezone string, now time.Time) (map[int64]*GroupAvailabilitySummary, error)
	CleanupOldResults(ctx context.Context, before time.Time) error
}
