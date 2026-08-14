import { mount } from '@vue/test-utils'
import ModelIcon from '../ModelIcon.vue'
import ProviderIcon from '../ProviderIcon.vue'

describe('ModelIcon', () => {
  it.each([
    ['OpenAI', 'gpt-5.6'],
    ['OpenAI Codex', 'codex-auto-review'],
    ['Grok', 'grok-4.5'],
    ['Kimi', 'kimi-k2.6'],
  ])('%s 模型图标继承主题文字色', (_provider, model) => {
    const wrapper = mount(ModelIcon, {
      props: {
        model,
        size: '28px',
      },
    })

    expect(wrapper.get('svg').attributes('width')).toBe('28px')
    expect(wrapper.get('path').attributes('fill')).toBe('currentColor')
    expect(wrapper.find('.model-icon-fallback').exists()).toBe(false)
  })
})

describe('ProviderIcon', () => {
  it.each([
    ['xAI', 'xAI'],
    ['Moonshot', 'Kimi'],
  ])('%s 徽标继承主题文字色', (_provider, brand) => {
    const wrapper = mount(ProviderIcon, {
      props: {
        brand,
      },
    })

    expect(wrapper.get('path').attributes('fill')).toBe('currentColor')
  })
})
