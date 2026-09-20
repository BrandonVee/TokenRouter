import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DashboardAdsView from '../DashboardAdsView.vue'

const { getDashboardAds, updateDashboardAds } = vi.hoisted(() => ({
  getDashboardAds: vi.fn(),
  updateDashboardAds: vi.fn()
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getDashboardAds,
      updateDashboardAds
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.index ? `${key}:${params.index}` : key
    })
  }
})

const storedAds = [
  {
    id: 'ad-live',
    image_url: 'https://cdn.example.com/live.jpg',
    link_url: 'https://example.com/live',
    fit_mode: 'cover',
    starts_at: '2026-01-01T00:00:00Z',
    ends_at: null,
    enabled: true
  },
  {
    id: 'ad-scheduled',
    image_url: '',
    link_url: '',
    fit_mode: 'adaptive',
    starts_at: '2099-01-01T00:00:00Z',
    ends_at: null,
    enabled: true
  }
]

function mountView() {
  return mount(DashboardAdsView, {
    global: {
      stubs: {
        Icon: true,
        ImageUpload: {
          props: ['modelValue'],
          template: '<div data-testid="image-upload"></div>'
        },
        Select: {
          props: ['modelValue', 'options'],
          template: '<div data-testid="fit-mode-select"></div>'
        }
      }
    }
  })
}

describe('DashboardAdsView', () => {
  beforeEach(() => {
    getDashboardAds.mockReset()
    updateDashboardAds.mockReset()
    getDashboardAds.mockResolvedValue(storedAds)
    updateDashboardAds.mockImplementation(async (ads) => ads)
  })

  it('以预览和展示设置双栏呈现每条广告', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.findAll('[data-testid^="dashboard-ad-card-"]')).toHaveLength(2)
    expect(wrapper.findAll('.dashboard-ad-preview')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="image-upload"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="fit-mode-select"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('admin.dashboardAds.status.scheduled')
  })

  it('页头可直接新增广告并保持卡片式编辑布局', async () => {
    const wrapper = mountView()
    await flushPromises()

    const addButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.dashboardAds.add')
    )
    expect(addButton).toBeTruthy()
    await addButton!.trigger('click')

    expect(wrapper.findAll('[data-testid^="dashboard-ad-card-"]')).toHaveLength(3)
  })
})
