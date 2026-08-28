/** 仪表盘广告数据契约。 */
export interface DashboardAd {
  id: string
  image_url: string
  link_url: string
  starts_at?: string | null
  ends_at?: string | null
  enabled: boolean
}

/** 判断广告当前是否处于有效展示周期。 */
export function isDashboardAdActive(ad: DashboardAd, now = Date.now()): boolean {
  if (!ad.enabled || !ad.image_url) return false
  const startsAt = ad.starts_at ? Date.parse(ad.starts_at) : Number.NEGATIVE_INFINITY
  const endsAt = ad.ends_at ? Date.parse(ad.ends_at) : Number.POSITIVE_INFINITY
  return now >= startsAt && now < endsAt
}

/** 关闭状态按广告周期生成，广告更新周期后自动重新展示。 */
export function dashboardAdDismissKey(ad: DashboardAd): string {
  return `dashboard-ad-dismissed:${ad.id}:${ad.starts_at || ''}:${ad.ends_at || ''}`
}
