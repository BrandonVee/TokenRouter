import { describe, expect, it } from 'vitest'
import {
  dashboardAdDismissKey,
  dashboardAdDismissPermanentKey,
  dashboardAdDismissTodayKey,
  normalizeDashboardAdFitMode
} from '../dashboardAd'

const ad = {
  id: 'ad-1',
  image_url: 'https://example.com/ad.png',
  link_url: '',
  enabled: true
}

describe('normalizeDashboardAdFitMode', () => {
  it('keeps supported modes', () => {
    expect(normalizeDashboardAdFitMode('cover')).toBe('cover')
    expect(normalizeDashboardAdFitMode('fill')).toBe('fill')
    expect(normalizeDashboardAdFitMode('adaptive')).toBe('adaptive')
  })

  it('falls back to adaptive for history and invalid values', () => {
    // 缺失或旧版本值必须保持首页广告可展示。
    expect(normalizeDashboardAdFitMode(undefined)).toBe('adaptive')
    expect(normalizeDashboardAdFitMode('legacy')).toBe('adaptive')
  })
})

describe('dashboard ad dismissal keys', () => {
  it('当天键使用本地日期并区分广告周期', () => {
    const date = new Date(2026, 7, 9, 23, 59, 59)
    const current = dashboardAdDismissTodayKey({ ...ad, starts_at: '2026-08-01' }, date)
    const nextDay = dashboardAdDismissTodayKey({ ...ad, starts_at: '2026-08-01' }, new Date(2026, 7, 10))
    const nextPeriod = dashboardAdDismissTodayKey({ ...ad, starts_at: '2026-08-02' }, date)

    expect(current).toContain(':2026-08-09')
    expect(nextDay).not.toBe(current)
    expect(nextPeriod).not.toBe(current)
  })

  it('永久键只绑定广告 ID，周期更新后仍保持关闭', () => {
    const firstPeriod = { ...ad, starts_at: '2026-08-01', ends_at: '2026-08-10' }
    const nextPeriod = { ...ad, starts_at: '2026-08-11', ends_at: '2026-08-20' }

    expect(dashboardAdDismissPermanentKey(firstPeriod)).toBe(dashboardAdDismissPermanentKey(nextPeriod))
    expect(dashboardAdDismissKey(firstPeriod)).not.toBe(dashboardAdDismissKey(nextPeriod))
  })
})
