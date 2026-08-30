import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import TimePicker from '../TimePicker.vue'

const SelectStub = {
  props: ['modelValue', 'ariaLabel'],
  template: `
    <button
      type="button"
      class="select-stub"
      :data-testid="ariaLabel?.endsWith('小时') ? 'hour-option' : 'minute-option'"
      @click="$emit('update:modelValue', ariaLabel?.endsWith('小时') ? 10 : 45)"
    >{{ modelValue }}</button>
  `,
}

describe('TimePicker', () => {
  it('按小时和分钟选择器输出规范化的 HH:MM', async () => {
    const wrapper = mount(TimePicker, {
      props: {
        modelValue: '09:30',
        ariaLabel: '峰谷开始',
      },
      global: {
        stubs: {
          Icon: true,
          Select: SelectStub,
        },
      },
    })

    await wrapper.get('[data-testid="hour-option"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['10:30'])

    await wrapper.get('[data-testid="minute-option"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['09:45'])
  })

  it('异常时间值从零点开始修正', async () => {
    const wrapper = mount(TimePicker, {
      props: {
        modelValue: 'invalid',
        ariaLabel: '峰谷结束',
      },
      global: {
        stubs: {
          Icon: true,
          Select: SelectStub,
        },
      },
    })

    await wrapper.get('[data-testid="minute-option"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['00:45'])
  })
})
