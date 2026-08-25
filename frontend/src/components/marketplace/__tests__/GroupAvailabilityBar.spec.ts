import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { MarketplaceGroupAvailabilityDay } from '@/types'
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

function bucket(index: number, overrides: Partial<MarketplaceGroupAvailabilityDay> = {}): MarketplaceGroupAvailabilityDay {
  return {
    date: new Date(Date.UTC(2026, 7, 24, index)).toISOString(),
    success_count: 10,
    slow_stream_count: 0,
    total_count: 10,
    availability_rate: 1,
    ...overrides,
  }
}

describe('GroupAvailabilityBar passive time buckets', () => {
  it('does not render the availability percentage as visible text', () => {
    const wrapper = mount(GroupAvailabilityBar, {
      props: {
        availability: {
          mode: 'active',
          window_days: 7,
          bucket_minutes: 1440,
          success_count: 6,
          total_count: 7,
          availability_rate: 6 / 7,
          days: [],
        },
      },
    })

    expect(wrapper.text()).not.toContain('%')
  })

  it('renders sixty time buckets with weighted color thresholds', () => {
    const days = Array.from({ length: 60 }, (_, index) => bucket(index))
    days[0] = bucket(0, { success_count: 0, total_count: 0, availability_rate: null })
    days[1] = bucket(1, { success_count: 13, total_count: 20, availability_rate: 0.65 })
    days[2] = bucket(2, { success_count: 2, total_count: 5, availability_rate: 0.4 })
    days[3] = bucket(3, { slow_stream_count: 10 })
    days[4] = bucket(4, { success_count: 6, slow_stream_count: 1, total_count: 10, availability_rate: 0.6 })

    const wrapper = mount(GroupAvailabilityBar, {
      props: {
        availability: {
          mode: 'passive',
          window_days: 7,
          bucket_minutes: 120,
          success_count: 41,
          pressure_count: 0,
          total_count: 50,
          availability_rate: 0.82,
          days,
        },
      },
    })

    const bars = wrapper.get('[role="img"]').findAll('span')
    expect(bars).toHaveLength(60)
    expect(bars[0].classes()).toContain('bg-emerald-500')
    expect(bars[1].classes()).toContain('bg-amber-400')
    expect(bars[2].classes()).toContain('bg-rose-500')
    expect(bars[3].classes()).toContain('bg-amber-400')
    expect(bars[4].classes()).toContain('bg-amber-400')
  })

  it('renders empty active time buckets as successful bars', () => {
    const wrapper = mount(GroupAvailabilityBar, {
      props: {
        availability: {
          mode: 'active',
          window_days: 1,
          bucket_minutes: 120,
          success_count: 1,
          total_count: 1,
          availability_rate: 1,
          days: [{
            date: new Date(Date.UTC(2026, 7, 24)).toISOString(),
            success_count: 0,
            total_count: 0,
            availability_rate: null,
          }],
        },
      },
    })

    const bars = wrapper.get('[role="img"]').findAll('span')
    expect(bars[0].classes()).toContain('bg-emerald-500')
  })

  it('fills missing time buckets as successful bars', () => {
    const wrapper = mount(GroupAvailabilityBar, {
      props: {
        availability: {
          mode: 'passive',
          window_days: 7,
          bucket_minutes: 120,
          success_count: 0,
          total_count: 0,
          availability_rate: 1,
          days: [],
        },
      },
    })

    const bars = wrapper.get('[role="img"]').findAll('span')
    expect(bars).toHaveLength(60)
    expect(bars.every((bar) => bar.classes().includes('bg-emerald-500'))).toBe(true)
  })
})
