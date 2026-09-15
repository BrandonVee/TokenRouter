/** 按本地日历月份计算账号过期时间，并在月末自动收敛到目标月最后一天。 */
export function getAccountExpiryTimestamp(months: number, now = new Date()): number {
  const expiry = new Date(now.getTime())
  const lastDay = new Date(expiry.getFullYear(), expiry.getMonth() + months + 1, 0).getDate()
  expiry.setMonth(expiry.getMonth() + months, Math.min(expiry.getDate(), lastDay))
  return Math.floor(expiry.getTime() / 1000)
}
