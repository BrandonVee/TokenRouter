import { apiClient } from '../client'

export type ImageHistoryStorageSource = 'deployment' | 'database'

export interface ImageHistoryStorageConfig {
  enabled: boolean
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  prefix: string
  force_path_style: boolean
  secret_configured: boolean
  available: boolean
  source: ImageHistoryStorageSource
  encryption_key_ready: boolean
}

export interface StorageConnectionTestResult {
  ok: boolean
  message: string
}

export async function getImageHistoryStorageConfig(): Promise<ImageHistoryStorageConfig> {
  const { data } = await apiClient.get<ImageHistoryStorageConfig>('/admin/settings/image-history-storage')
  return data
}

export async function updateImageHistoryStorageConfig(
  config: ImageHistoryStorageConfig,
): Promise<ImageHistoryStorageConfig> {
  const { data } = await apiClient.put<ImageHistoryStorageConfig>('/admin/settings/image-history-storage', config)
  return data
}

export async function testImageHistoryStorageConnection(
  config: ImageHistoryStorageConfig,
): Promise<StorageConnectionTestResult> {
  const { data } = await apiClient.post<StorageConnectionTestResult>('/admin/settings/image-history-storage/test', config)
  return data
}

export async function resetImageHistoryStorageConfig(): Promise<ImageHistoryStorageConfig> {
  const { data } = await apiClient.delete<ImageHistoryStorageConfig>('/admin/settings/image-history-storage')
  return data
}

export const fileStorageAPI = {
  getImageHistoryStorageConfig,
  updateImageHistoryStorageConfig,
  testImageHistoryStorageConnection,
  resetImageHistoryStorageConfig,
}

export default fileStorageAPI
