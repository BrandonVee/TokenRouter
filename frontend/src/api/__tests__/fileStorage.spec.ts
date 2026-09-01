import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
  remove: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
    delete: remove,
  },
}))

import fileStorageAPI, { type ImageHistoryStorageConfig } from '@/api/admin/fileStorage'

// 管理端四个操作共享同一份脱敏配置响应。
const config: ImageHistoryStorageConfig = {
  enabled: true,
  endpoint: 'https://s3.example.test',
  region: 'auto',
  bucket: 'images',
  access_key_id: 'access-key',
  prefix: 'image-history',
  force_path_style: true,
  secret_configured: true,
  available: true,
  source: 'database',
  encryption_key_ready: true,
}

describe('file storage api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    get.mockResolvedValue({ data: config })
    put.mockResolvedValue({ data: config })
    post.mockResolvedValue({ data: { ok: true, message: 'connection successful' } })
    remove.mockResolvedValue({ data: { ...config, source: 'deployment' } })
  })

  it('uses the image history storage administration endpoints', async () => {
    await expect(fileStorageAPI.getImageHistoryStorageConfig()).resolves.toEqual(config)
    await expect(fileStorageAPI.updateImageHistoryStorageConfig(config)).resolves.toEqual(config)
    await expect(fileStorageAPI.testImageHistoryStorageConnection(config)).resolves.toEqual({
      ok: true,
      message: 'connection successful',
    })
    await expect(fileStorageAPI.resetImageHistoryStorageConfig()).resolves.toEqual({
      ...config,
      source: 'deployment',
    })

    expect(get).toHaveBeenCalledWith('/admin/settings/image-history-storage')
    expect(put).toHaveBeenCalledWith('/admin/settings/image-history-storage', config)
    expect(post).toHaveBeenCalledWith('/admin/settings/image-history-storage/test', config)
    expect(remove).toHaveBeenCalledWith('/admin/settings/image-history-storage')
  })
})
