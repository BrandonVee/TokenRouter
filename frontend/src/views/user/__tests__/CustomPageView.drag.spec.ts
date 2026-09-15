import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CustomPageView from '../CustomPageView.vue'

const { appStore } = vi.hoisted(() => ({
  appStore: {
    publicSettingsLoaded: true,
    cachedPublicSettings: { custom_menu_items: [{ id: 'docs', url: 'https://example.com/docs' }] }
  }
}))

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'docs' } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false, user: { id: 7 }, token: 'test-token' }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}` }))

let notifyResize: () => void
const wrappers: ReturnType<typeof mount>[] = []

function mountEmbed() {
  const wrapper = mount(CustomPageView, { global: { stubs: { Icon: true } } })
  wrappers.push(wrapper)
  const shell = wrapper.get('.custom-embed-shell').element
  const button = wrapper.get<HTMLAnchorElement>('.custom-open-fab').element
  const size = { width: 800, height: 600 }
  let capturedPointer: number | null = null
  Object.defineProperties(shell, {
    clientWidth: { get: () => size.width },
    clientHeight: { get: () => size.height }
  })
  Object.defineProperties(button, {
    offsetWidth: { value: 100 },
    offsetHeight: { value: 32 },
    offsetLeft: { get: () => Number.parseFloat(button.style.left || '688') },
    offsetTop: { get: () => Number.parseFloat(button.style.top || '12') },
    setPointerCapture: { value: vi.fn((id: number) => { capturedPointer = id }) },
    hasPointerCapture: { value: (id: number) => capturedPointer === id },
    releasePointerCapture: { value: vi.fn(() => { capturedPointer = null }) }
  })
  return { wrapper, button, size }
}

async function pointer(button: HTMLElement, type: string, x: number, y: number, extra = {}) {
  const event = new MouseEvent(type, { clientX: x, clientY: y, bubbles: true, cancelable: true, ...extra })
  Object.defineProperties(event, {
    pointerId: { value: 1 },
    isPrimary: { value: true }
  })
  button.dispatchEvent(event)
  await nextTick()
}

describe('custom page open button drag', () => {
  beforeEach(() => {
    vi.stubGlobal('ResizeObserver', class {
      constructor(callback: () => void) { notifyResize = callback }
      observe() {}
      disconnect() {}
    })
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.unstubAllGlobals()
  })

  it('拖动时保持按钮位于容器边界内，并在容器缩小时重新收敛', async () => {
    const { button, size } = mountEmbed()
    await pointer(button, 'pointerdown', 700, 24)
    await pointer(button, 'pointermove', -1000, -1000)
    expect([button.style.left, button.style.top]).toEqual(['0px', '0px'])
    await pointer(button, 'pointermove', 2000, 2000)
    expect([button.style.left, button.style.top]).toEqual(['700px', '568px'])
    await pointer(button, 'pointerup', 2000, 2000)

    size.width = 300
    size.height = 200
    notifyResize()
    await nextTick()
    expect([button.style.left, button.style.top]).toEqual(['200px', '168px'])
  })
})
