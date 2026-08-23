import { apiClient } from '../client'

export interface CNQuotaTier {
  window: '5h' | 'weekly'
  used_percent: number
  reset_at?: string
}

export interface CNProviderQuotaProbeResult {
  provider: string
  success: boolean
  tiers?: CNQuotaTier[]
  error?: string
}

export interface CNProviderBalanceEntry {
  currency: string
  balance: number
}

export interface CNProviderBalanceResult {
  provider: string
  success: boolean
  balance: number
  currency?: string
  balances?: CNProviderBalanceEntry[]
  error?: string
}

// 查询 Coding Plan 的窗口额度。
export async function queryQuota(id: number) {
  const { data } = await apiClient.get<CNProviderQuotaProbeResult>(`/admin/cn-providers/accounts/${id}/quota`)
  return data
}

// 查询按量账号的供应商余额。
export async function queryBalance(id: number) {
  const { data } = await apiClient.get<CNProviderBalanceResult>(`/admin/cn-providers/accounts/${id}/balance`)
  return data
}

export default { queryQuota, queryBalance }
