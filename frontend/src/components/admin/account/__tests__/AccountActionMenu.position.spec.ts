import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import AccountActionMenu from '../AccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const account = {
  id: 1,
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  rate_limit_reset_at: '2099-01-01T00:00:00Z'
} as Account

let menuHeight: number
let resizeMenu: ResizeObserverCallback

const getMenu = () => document.body.querySelector<HTMLElement>('.action-menu-content')!
const getTop = () => Number.parseFloat(getMenu().style.top)
const getLeft = () => Number.parseFloat(getMenu().style.left)

function setViewport(width: number, height: number) {
  vi.stubGlobal('innerWidth', width)
  vi.stubGlobal('innerHeight', height)
  window.dispatchEvent(new Event('resize'))
}

async function mountMenu(anchorRect = new DOMRect(950, 720, 32, 24)) {
  const wrapper = mount(AccountActionMenu, {
    props: { show: true, account, anchorRect },
    global: { stubs: { Icon: true } }
  })
  await flushPromises()
  return wrapper
}

enableAutoUnmount(afterEach)

describe('AccountActionMenu viewport positioning', () => {
  beforeEach(() => {
    menuHeight = 305
    setViewport(1024, 768)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      if (!this.classList.contains('action-menu-content')) return new DOMRect()
      return new DOMRect(0, 0, Math.min(208, window.innerWidth - 16), Math.min(menuHeight, window.innerHeight - 16))
    })
    vi.stubGlobal('ResizeObserver', class {
      constructor(callback: ResizeObserverCallback) { resizeMenu = callback }
      observe = vi.fn()
      disconnect = vi.fn()
      unobserve = vi.fn()
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('measures a long OAuth menu and opens above the last row', async () => {
    await mountMenu()

    expect(getTop()).toBe(720 - 305 - 4)
    expect(getLeft()).toBe(982 - 208)
    expect(getTop() + menuHeight).toBeLessThanOrEqual(window.innerHeight - 8)
    expect(getMenu().textContent).toContain('admin.accounts.recoverState')
  })

  it('opens below the trigger when the complete menu fits', async () => {
    await mountMenu(new DOMRect(500, 100, 32, 24))

    expect(getTop()).toBe(128)
    expect(getLeft()).toBe(324)
  })

  it('constrains an oversized menu and preserves the recover-state action', async () => {
    setViewport(180, 220)
    const wrapper = await mountMenu(new DOMRect(150, 180, 24, 24))

    expect(getTop()).toBe(8)
    expect(getLeft()).toBe(8)
    expect(getMenu().style.maxHeight).toBe('204px')
    expect(getMenu().style.maxWidth).toBe('164px')
    expect(getMenu().classList.contains('overflow-y-auto')).toBe(true)

    const recover = Array.from(getMenu().querySelectorAll('button'))
      .find(button => button.textContent?.includes('admin.accounts.recoverState'))!
    recover.click()
    expect(wrapper.emitted('recover-state')).toEqual([[account]])
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('repositions when menu content grows', async () => {
    await mountMenu()
    menuHeight = 420
    resizeMenu([], {} as ResizeObserver)
    await flushPromises()

    expect(getTop()).toBe(296)
  })

  it('still closes on Escape and backdrop clicks', async () => {
    const wrapper = await mountMenu()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    const backdrop = getMenu().previousElementSibling as HTMLElement
    backdrop.click()

    expect(wrapper.emitted('close')).toHaveLength(2)
  })
})
