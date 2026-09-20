import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import dayjs from 'dayjs'
import DatePicker from 'ant-design-vue/es/date-picker'
import DateTimePicker from '../DateTimePicker.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: ref('zh')
  })
}))

describe('DateTimePicker', () => {
  it('将 Ant Design Vue 选择结果转换为 datetime-local 值', async () => {
    const wrapper = mount(DateTimePicker, {
      props: {
        modelValue: '2026-09-20T08:30',
        showTime: true
      }
    })

    const picker = wrapper.findComponent(DatePicker)
    expect(picker.exists()).toBe(true)
    picker.vm.$emit('change', dayjs('2026-10-01T19:45'))
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['2026-10-01T19:45'])
    expect(wrapper.emitted('change')?.[0]).toEqual(['2026-10-01T19:45'])
  })

  it('清空日期时发出 null', async () => {
    const wrapper = mount(DateTimePicker)
    wrapper.findComponent(DatePicker).vm.$emit('change', null)
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([null])
  })

  it('同一天内禁用早于最小值的小时和分钟', () => {
    const wrapper = mount(DateTimePicker, {
      props: {
        modelValue: '2026-09-20T09:45',
        min: '2026-09-20T09:35',
        showTime: true,
      },
    })
    const picker = wrapper.findComponent(DatePicker)
    const disabledTime = picker.props('disabledTime') as (value: dayjs.Dayjs) => {
      disabledHours: () => number[]
      disabledMinutes: (hour: number) => number[]
    }
    const disabled = disabledTime(dayjs('2026-09-20T09:45'))

    expect(disabled.disabledHours()).toContain(8)
    expect(disabled.disabledHours()).not.toContain(9)
    expect(disabled.disabledMinutes(9)).toContain(34)
    expect(disabled.disabledMinutes(9)).not.toContain(35)
  })

  it('输入越界时间时收敛到最小值', async () => {
    const wrapper = mount(DateTimePicker, {
      props: {
        min: '2026-09-20T09:35',
        showTime: true,
      },
    })

    wrapper.findComponent(DatePicker).vm.$emit('change', dayjs('2026-09-20T08:00'))
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['2026-09-20T09:35'])
  })
})
