import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  remove: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    delete: remove,
  },
}))

import { imageHistoryAPI } from '@/api/imageHistory'

describe('image history api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    remove.mockReset()
  })

  it('uses authenticated settings and pagination endpoints', async () => {
    get
      .mockResolvedValueOnce({ data: { available: true, enabled: false } })
      .mockResolvedValueOnce({ data: { items: [], total: 0, page: 2, page_size: 24, pages: 0 } })
    put.mockResolvedValue({ data: { available: true, enabled: true } })

    await expect(imageHistoryAPI.getSettings()).resolves.toEqual({ available: true, enabled: false })
    await expect(imageHistoryAPI.updateSettings(true)).resolves.toEqual({ available: true, enabled: true })
    await expect(imageHistoryAPI.list(2, 24)).resolves.toEqual(
      expect.objectContaining({ page: 2, page_size: 24 }),
    )

    expect(get).toHaveBeenNthCalledWith(1, '/user/image-history/settings')
    expect(put).toHaveBeenCalledWith('/user/image-history/settings', { enabled: true })
    expect(get).toHaveBeenNthCalledWith(2, '/user/image-history', {
      params: { page: 2, page_size: 24 },
    })
  })

  it('includes a non-empty search query in the history list request', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 12, pages: 0 } })

    await imageHistoryAPI.list(1, 12, '  lighthouse  ')

    expect(get).toHaveBeenCalledWith('/user/image-history', {
      params: { page: 1, page_size: 12, search: 'lighthouse' },
    })
  })

  it('downloads private content as a blob and deletes through the backend', async () => {
    const blob = new Blob(['image'], { type: 'image/png' })
    get.mockResolvedValue({ data: blob })
    remove.mockResolvedValue({ data: { success: true } })

    await expect(imageHistoryAPI.download('record-id')).resolves.toBe(blob)
    await imageHistoryAPI.delete('record-id')

    expect(get).toHaveBeenCalledWith('/user/image-history/record-id/content', {
      responseType: 'blob',
    })
    expect(remove).toHaveBeenCalledWith('/user/image-history/record-id')
  })
})
