import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import type { DashboardAd } from '@/types/dashboardAd'
import { dashboardAdDismissPermanentKey, dashboardAdDismissTodayKey } from '@/types/dashboardAd'
import UserDashboardAds from '../UserDashboardAds.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => ({
      'common.cancel': '取消',
      'common.close': '关闭',
      'dashboard.ads.dismissTitle': '关闭广告',
      'dashboard.ads.dismissMessage': '要隐藏此广告多久？',
      'dashboard.ads.dismissToday': '关闭今天',
      'dashboard.ads.dismissPermanently': '永久关闭'
    }[key] ?? key)
  })
}))

const ad: DashboardAd = {
  id: 'ad-1',
  image_url: 'data:image/png;base64,AAAA',
  link_url: '',
  enabled: true
}

describe('UserDashboardAds', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  it('点击关闭后提供取消、关闭今天和永久关闭选项', async () => {
    const wrapper = mount(UserDashboardAds, {
      props: { ads: [ad] },
      attachTo: document.body,
      global: { stubs: { Icon: { template: '<span />' } } }
    })

    await wrapper.find('section button').trigger('click')
    await nextTick()

    expect(document.body.textContent).toContain('要隐藏此广告多久？')
    expect(document.body.querySelector('[data-testid="dashboard-ad-dismiss-cancel"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="dashboard-ad-dismiss-today"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="dashboard-ad-dismiss-permanent"]')).not.toBeNull()
    expect(localStorage.length).toBe(0)

    document.body.querySelector<HTMLButtonElement>('[data-testid="dashboard-ad-dismiss-cancel"]')?.click()
    await nextTick()
    expect(wrapper.find('section').exists()).toBe(true)
    expect(localStorage.length).toBe(0)

    wrapper.unmount()
  })

  it('关闭今天只写入当天键', async () => {
    const wrapper = mount(UserDashboardAds, {
      props: { ads: [ad] },
      attachTo: document.body,
      global: { stubs: { Icon: { template: '<span />' } } }
    })

    await wrapper.find('section button').trigger('click')
    document.body.querySelector<HTMLButtonElement>('[data-testid="dashboard-ad-dismiss-today"]')?.click()
    await flushPromises()
    expect(localStorage.getItem(dashboardAdDismissTodayKey(ad))).toBe('1')
    expect(wrapper.find('section').exists()).toBe(false)

    wrapper.unmount()
  })

  it('永久关闭使用广告 ID 键', async () => {
    const wrapper = mount(UserDashboardAds, {
      props: { ads: [ad] },
      attachTo: document.body,
      global: { stubs: { Icon: { template: '<span />' } } }
    })

    await wrapper.find('section button').trigger('click')
    document.body.querySelector<HTMLButtonElement>('[data-testid="dashboard-ad-dismiss-permanent"]')?.click()
    await flushPromises()
    expect(localStorage.getItem(dashboardAdDismissPermanentKey(ad))).toBe('1')
    expect(wrapper.find('section').exists()).toBe(false)

    wrapper.unmount()
  })
})
