import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import UsageRankingView from '../UsageRankingView.vue'
import { useAppStore } from '@/stores/app'

const mocks = vi.hoisted(() => ({
  getRanking: vi.fn(),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getRanking: (...args: unknown[]) => mocks.getRanking(...args),
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        key === 'usageRanking.reasoningCost' ? `${params?.unit} Used` : key,
    }),
  }
})

vi.mock('@/composables/useBalanceDisplay', () => ({
  useBalanceDisplay: () => ({
    balanceUnitName: { value: 'USD' },
    formatBalanceAmount: (value: number, options?: { fractionDigits?: number }) =>
      `$${value.toFixed(options?.fractionDigits ?? 2)}`,
  }),
}))

describe('UsageRankingView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.getRanking.mockReset()
    mocks.getRanking.mockResolvedValue({
      ranking: [
        {
          rank: 1,
          user_id: 7,
          display_name: 'spender',
          avatar_url: '',
          requests: 9,
          input_tokens: 1_000_000_000,
          output_tokens: 500_000_000,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 1_500_000_000,
          actual_cost: 12.5,
        },
      ],
      total_requests: 9,
      total_tokens: 1_500_000_000,
      total_actual_cost: 12.5,
      start_date: '2026-08-14',
      end_date: '2026-08-14',
      limit: 20,
      sort_by: 'actual_cost',
      data_visible: true,
      show_tokens: true,
      show_requests: true,
      show_amount: true,
    })
  })

  it('以金额作为主指标并使用英文数量级显示 Token', async () => {
    const wrapper = mount(UsageRankingView, {
      global: {
        stubs: {
          DateRangePicker: true,
          Icon: true,
          LoadingSpinner: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('$12.5000')
    expect(wrapper.text()).toContain('1.5B')
    expect(wrapper.text()).not.toContain('亿')
  })

  it('按系统设置使用 Token 排序主指标并隐藏消费金额', async () => {
    const appStore = useAppStore()
    appStore.cachedPublicSettings = {
      usage_ranking_data_visible: true,
      usage_ranking_sort_by: 'total_tokens',
      usage_ranking_show_tokens: true,
      usage_ranking_show_requests: true,
      usage_ranking_show_amount: false,
    } as any
    mocks.getRanking.mockResolvedValueOnce({
      ranking: [
        {
          rank: 1,
          user_id: 7,
          display_name: 'token-user',
          avatar_url: '',
          requests: 9,
          input_tokens: 1_000_000_000,
          output_tokens: 500_000_000,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 1_500_000_000,
          actual_cost: 0,
        },
      ],
      total_requests: 9,
      total_tokens: 1_500_000_000,
      total_actual_cost: 0,
      start_date: '2026-08-14',
      end_date: '2026-08-14',
      limit: 20,
      sort_by: 'total_tokens',
      data_visible: true,
      show_tokens: true,
      show_requests: true,
      show_amount: false,
    })

    const wrapper = mount(UsageRankingView, {
      global: {
        stubs: {
          DateRangePicker: true,
          Icon: true,
          LoadingSpinner: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('1.5B')
    expect(wrapper.text()).not.toContain('$')
    expect(wrapper.text()).toContain('usageRanking.requests')
  })
})
