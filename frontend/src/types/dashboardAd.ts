/** 广告图片在展示区域内的适应方式。 */
export type DashboardAdFitMode = 'adaptive' | 'cover' | 'fill'

/** 仪表盘广告数据契约。 */
export interface DashboardAd {
  id: string
  image_url: string
  link_url: string
  fit_mode?: DashboardAdFitMode | string | null
  starts_at?: string | null
  ends_at?: string | null
  enabled: boolean
}

/** 将历史或非法配置统一映射为可用的图片适应方式。 */
export function normalizeDashboardAdFitMode(mode: unknown): DashboardAdFitMode {
  if (mode === 'cover' || mode === 'fill') return mode
  return 'adaptive'
}

/** 判断广告当前是否处于有效展示周期。 */
export function isDashboardAdActive(ad: DashboardAd, now = Date.now()): boolean {
  if (!ad.enabled || !ad.image_url) return false
  const startsAt = ad.starts_at ? Date.parse(ad.starts_at) : Number.NEGATIVE_INFINITY
  const endsAt = ad.ends_at ? Date.parse(ad.ends_at) : Number.POSITIVE_INFINITY
  return now >= startsAt && now < endsAt
}

/** 旧版关闭状态按广告周期生成，广告更新周期后自动重新展示。 */
export function dashboardAdDismissKey(ad: DashboardAd): string {
  return `dashboard-ad-dismissed:${ad.id}:${ad.starts_at || ''}:${ad.ends_at || ''}`
}

/** 生成当天关闭键，日期使用浏览器本地日历，广告周期变化后自动重新展示。 */
export function dashboardAdDismissTodayKey(ad: DashboardAd, date = new Date()): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `dashboard-ad-dismissed-today:${ad.id}:${ad.starts_at || ''}:${ad.ends_at || ''}:${year}-${month}-${day}`
}

/** 生成永久关闭键，广告 ID 不变时持续隐藏广告。 */
export function dashboardAdDismissPermanentKey(ad: DashboardAd): string {
  return `dashboard-ad-dismissed-permanent:${ad.id}`
}
