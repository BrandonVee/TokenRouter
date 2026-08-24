import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { MarketplaceGroupAvailabilityRequest } from '@/types'
import GroupAvailabilityBar from '../GroupAvailabilityBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function request(index: number, status: string): MarketplaceGroupAvailabilityRequest {
  return {
    status,
    success: status === 'success',
    created_at: new Date(Date.UTC(2026, 7, 24, 0, index)).toISOString(),
  }
}

describe('GroupAvailabilityBar passive groups', () => {
  it('renders sixty bars and summarizes each five eligible requests', () => {
    const requests = Array.from({ length: 300 }, (_, index) => request(index, 'success'))
    requests[5] = request(5, 'upstream_error')
    requests[10] = request(10, 'upstream_error')
    requests[11] = request(11, 'upstream_error')

    const wrapper = mount(GroupAvailabilityBar, {
      props: {
        availability: {
          mode: 'passive',
          window_days: 7,
          bucket_minutes: 120,
          success_count: 297,
          pressure_count: 0,
          total_count: 300,
          availability_rate: 0.99,
          days: [],
          requests,
        },
      },
    })

    const bars = wrapper.get('[role="img"]').findAll('span')
    expect(bars).toHaveLength(60)
    expect(bars[0].classes()).toContain('bg-emerald-500')
    expect(bars[1].classes()).toContain('bg-amber-400')
    expect(bars[2].classes()).toContain('bg-rose-500')
  })
})
