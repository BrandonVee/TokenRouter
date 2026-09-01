import { apiClient } from './client'

export interface ImageHistorySettings {
  available: boolean
  enabled: boolean
}

export interface ImageHistoryRecord {
  id: string
  api_key_id?: number
  request_id?: string
  source: string
  endpoint: string
  model: string
  prompt: string
  revised_prompt?: string
  parameters?: string
  mime_type: string
  size_bytes: number
  width: number
  height: number
  sha256: string
  preview_url?: string
  created_at: string
}

export interface ImageHistoryList {
  items: ImageHistoryRecord[]
  total: number
  page: number
  page_size: number
  pages: number
}

// imageHistoryAPI 只通过已认证用户接口管理私有生图记录。
export const imageHistoryAPI = {
  async getSettings(): Promise<ImageHistorySettings> {
    const { data } = await apiClient.get<ImageHistorySettings>('/user/image-history/settings')
    return data
  },

  async updateSettings(enabled: boolean): Promise<ImageHistorySettings> {
    const { data } = await apiClient.put<ImageHistorySettings>('/user/image-history/settings', { enabled })
    return data
  },

  async list(page: number, pageSize: number, search = ''): Promise<ImageHistoryList> {
    // 搜索词为空时保持原有分页请求形状，便于兼容旧客户端和缓存键。
    const params: { page: number; page_size: number; search?: string } = { page, page_size: pageSize }
    const normalizedSearch = search.trim()
    if (normalizedSearch) params.search = normalizedSearch
    const { data } = await apiClient.get<ImageHistoryList>('/user/image-history', {
      params,
    })
    return data
  },

  async download(id: string): Promise<Blob> {
    const { data } = await apiClient.get<Blob>(`/user/image-history/${id}/content`, {
      responseType: 'blob',
    })
    return data
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/user/image-history/${id}`)
  },
}
