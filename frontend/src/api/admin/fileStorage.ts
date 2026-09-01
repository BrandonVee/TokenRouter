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

export type FileStorageType = 'local' | 's3'

export interface FileStorageS3Config {
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  prefix: string
  force_path_style: boolean
}

export interface FileStorageProfile {
  id: string
  type: FileStorageType
  local_path: string
  s3: FileStorageS3Config
  secret_configured: boolean
  encryption_key_ready: boolean
}

export interface FileStorageDirectoryConfig {
  directory: 'invoice_attachments'
  profile: FileStorageProfile
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

export async function getInvoiceAttachmentStorageConfig(): Promise<FileStorageDirectoryConfig> {
  const { data } = await apiClient.get<FileStorageDirectoryConfig>('/admin/settings/file-storage/invoice-attachments')
  return data
}

export async function updateInvoiceAttachmentStorageConfig(config: FileStorageDirectoryConfig): Promise<FileStorageDirectoryConfig> {
  const { data } = await apiClient.put<FileStorageDirectoryConfig>('/admin/settings/file-storage/invoice-attachments', config)
  return data
}

export async function testInvoiceAttachmentStorageConnection(config: FileStorageDirectoryConfig): Promise<StorageConnectionTestResult> {
  const { data } = await apiClient.post<StorageConnectionTestResult>('/admin/settings/file-storage/invoice-attachments/test', config)
  return data
}

export const fileStorageAPI = {
  getImageHistoryStorageConfig,
  updateImageHistoryStorageConfig,
  testImageHistoryStorageConnection,
  resetImageHistoryStorageConfig,
  getInvoiceAttachmentStorageConfig,
  updateInvoiceAttachmentStorageConfig,
  testInvoiceAttachmentStorageConnection,
}

export default fileStorageAPI
