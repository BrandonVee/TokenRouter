import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import Select from '../Select.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const originalInnerWidth = window.innerWidth

afterEach(() => {
  document.body.innerHTML = ''
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: originalInnerWidth
  })
  vi.restoreAllMocks()
})

describe('Select dropdown viewport constraints', () => {
  it('repositions the teleported dropdown within the viewport', async () => {
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 320
    })

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 220,
      y: 20,
      top: 20,
      right: 300,
      bottom: 60,
      left: 220,
      width: 80,
      height: 40,
      toJSON: () => ({})
    })

    const wrapper = mount(Select, {
      props: {
        modelValue: null,
        options: [
          {
            value: 'example',
            label: 'very-long-unbroken-option-value-that-must-not-overflow'
          }
        ]
      }
    })

    await wrapper.get('button').trigger('click')
    await nextTick()
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.select-dropdown-portal')

    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.left).toBe('104px')
    expect(dropdown?.style.minWidth).toBe('80px')
    expect(dropdown?.style.maxWidth).toBe('288px')

    wrapper.unmount()
  })

  it('打开另一个选择器时关闭之前的下拉菜单', async () => {
    const first = mount(Select, {
      props: {
        modelValue: null,
        options: [{ value: 'first', label: 'First' }]
      }
    })
    const second = mount(Select, {
      props: {
        modelValue: null,
        options: [{ value: 'second', label: 'Second' }]
      }
    })

    await first.get('button').trigger('click')
    await nextTick()
    expect(first.get('button').attributes('aria-expanded')).toBe('true')

    await second.get('button').trigger('click')
    await nextTick()

    expect(first.get('button').attributes('aria-expanded')).toBe('false')
    expect(second.get('button').attributes('aria-expanded')).toBe('true')
    await new Promise(resolve => setTimeout(resolve, 250))
    expect(document.body.querySelectorAll('.select-dropdown-portal')).toHaveLength(1)

    first.unmount()
    second.unmount()
  })
})
