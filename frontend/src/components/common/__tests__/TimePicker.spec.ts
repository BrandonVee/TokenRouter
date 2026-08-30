import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import TimePicker from '../TimePicker.vue'

describe('TimePicker', () => {
  it('通过单个时间输入输出规范化的 HH:MM', async () => {
    const wrapper = mount(TimePicker, {
      props: {
        modelValue: '09:30',
        ariaLabel: '峰谷开始',
        testId: 'peak-start',
      },
    })

    const input = wrapper.get('input[type="time"]')
    expect(input.attributes('value')).toBe('09:30')
    await input.setValue('10:05')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['10:05'])
  })

  it('异常或空时间值会清空模型值', async () => {
    const wrapper = mount(TimePicker, {
      props: {
        modelValue: 'invalid',
        ariaLabel: '峰谷结束',
      },
    })

    const input = wrapper.get('input[type="time"]')
    expect(input.attributes('value')).toBe('')

    await input.setValue('')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
  })
})
