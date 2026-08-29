import { describe, expect, it } from 'vitest'
import { normalizeDashboardAdFitMode } from '../dashboardAd'

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
