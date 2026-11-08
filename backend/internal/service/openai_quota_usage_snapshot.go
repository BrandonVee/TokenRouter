package service

import "time"

// buildOpenAIAutoResetUsageUpdates 统一转换原生主/次窗口，保留各自重置时间。
func buildOpenAIAutoResetUsageUpdates(usage *OpenAIQuotaUsage, now time.Time) map[string]any {
	if usage == nil || usage.RateLimit == nil {
		return nil
	}
	rateLimit := usage.RateLimit
	snapshot := &OpenAICodexUsageSnapshot{UpdatedAt: now.UTC().Format(time.RFC3339)}
	applyWindow := func(window *OpenAIRateLimitWindow, primary bool) {
		if window == nil {
			return
		}
		used := window.UsedPercent
		resetAfter := int(window.ResetAfterSeconds)
		windowMinutes := int(window.LimitWindowSeconds / 60)
		if primary {
			snapshot.PrimaryUsedPercent = &used
			snapshot.PrimaryResetAfterSeconds = &resetAfter
			snapshot.PrimaryWindowMinutes = &windowMinutes
		} else {
			snapshot.SecondaryUsedPercent = &used
			snapshot.SecondaryResetAfterSeconds = &resetAfter
			snapshot.SecondaryWindowMinutes = &windowMinutes
		}
	}
	applyWindow(rateLimit.PrimaryWindow, true)
	applyWindow(rateLimit.SecondaryWindow, false)
	return buildCodexUsageExtraUpdates(snapshot, now)
}
