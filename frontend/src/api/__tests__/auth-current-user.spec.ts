import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get: getMock
  }
}))

import { getCurrentUser } from '@/api/auth'

describe('getCurrentUser', () => {
  beforeEach(() => {
    getMock.mockReset()
    getMock.mockResolvedValue({ data: { user: {} } })
  })

  it('每次读取用户余额时显式绕过浏览器与中间代理缓存', async () => {
    // 固定时间戳，确保请求配置的缓存穿透参数可精确断言。
    vi.spyOn(Date, 'now').mockReturnValue(1_789_412_345_678)

    await getCurrentUser()

    expect(getMock).toHaveBeenCalledWith('/auth/me', {
      params: { _fresh: 1_789_412_345_678 },
      headers: {
        'Cache-Control': 'no-cache',
        Pragma: 'no-cache'
      }
    })
  })
})
