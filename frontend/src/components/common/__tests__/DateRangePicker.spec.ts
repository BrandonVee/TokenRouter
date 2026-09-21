import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import dayjs, { type Dayjs } from 'dayjs'
import DatePicker from 'ant-design-vue/es/date-picker'

import DateRangePicker from '../DateRangePicker.vue'

const messages: Record<string, string> = {
  'dates.today': 'Today',
  'dates.yesterday': 'Yesterday',
  'dates.last15Minutes': 'Last 15 Minutes',
  'dates.last30Minutes': 'Last 30 Minutes',
  'dates.last24Hours': 'Last 24 Hours',
  'dates.last7Days': 'Last 7 Days',
  'dates.last14Days': 'Last 14 Days',
  'dates.last30Days': 'Last 30 Days',
  'dates.thisMonth': 'This Month',
  'dates.lastMonth': 'Last Month',
  'dates.startDate': 'Start Date',
  'dates.endDate': 'End Date',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
    locale: ref('en'),
  }),
}))

type RangePreset = {
  label: string
  value: [Dayjs, Dayjs]
}

const mountPicker = () => mount(DateRangePicker, {
  props: {
    startDate: '2026-09-01',
    endDate: '2026-09-20',
  },
})

describe('DateRangePicker', () => {
  it('直接渲染带时分选择的单层范围面板和快捷项', () => {
    const wrapper = mountPicker()
    const picker = wrapper.findComponent(DatePicker.RangePicker)

    expect(picker.exists()).toBe(true)
    expect(wrapper.find('.date-picker-trigger').exists()).toBe(false)
    expect(picker.props('showTime')).toMatchObject({ format: 'HH:mm', minuteStep: 5 })
    expect((picker.props('presets') as RangePreset[]).map((preset) => preset.label)).toContain('Last 7 Days')
  })

  it('完成手动范围选择后立即应用日期和时分', async () => {
    const wrapper = mountPicker()
    wrapper.findComponent(DatePicker.RangePicker).vm.$emit('change', [
      dayjs('2026-09-05T08:35:00'),
      dayjs('2026-09-18T19:45:00'),
    ])
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:startDate')?.[0]).toEqual(['2026-09-05T08:35:00'])
    expect(wrapper.emitted('update:endDate')?.[0]).toEqual(['2026-09-18T19:45:00'])
    expect(wrapper.emitted('change')?.[0]).toEqual([{
      startDate: '2026-09-05T08:35:00',
      endDate: '2026-09-18T19:45:00',
      preset: null,
    }])
  })

  it('快捷日期由同一个面板直接应用并保留预设标识', async () => {
    const wrapper = mountPicker()
    const picker = wrapper.findComponent(DatePicker.RangePicker)
    const preset = (picker.props('presets') as RangePreset[])
      .find((item) => item.label === 'Last Month')

    expect(preset).toBeDefined()
    picker.vm.$emit('change', preset!.value)
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('change')?.[0]?.[0]).toMatchObject({
      preset: 'lastMonth',
    })
  })

  it('分钟级快捷范围保留具体时间', async () => {
    const wrapper = mountPicker()
    const picker = wrapper.findComponent(DatePicker.RangePicker)
    const preset = (picker.props('presets') as RangePreset[])
      .find((item) => item.label === 'Last 15 Minutes')

    picker.vm.$emit('change', preset!.value)
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('change')?.[0]?.[0] as { startDate: string; endDate: string; preset: string }
    expect(emitted.preset).toBe('last15Minutes')
    expect(emitted.startDate).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/)
    expect(emitted.endDate).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/)
  })

  it('显式关闭时间选择时保持仅日期输出', async () => {
    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: '2026-09-01',
        endDate: '2026-09-20',
        showTime: false,
      },
    })
    const picker = wrapper.findComponent(DatePicker.RangePicker)
    picker.vm.$emit('change', [dayjs('2026-09-05T08:35:00'), dayjs('2026-09-18T19:45:00')])
    await wrapper.vm.$nextTick()

    expect(picker.props('showTime')).toBe(false)
    expect(wrapper.emitted('update:startDate')?.[0]).toEqual(['2026-09-05'])
    expect(wrapper.emitted('update:endDate')?.[0]).toEqual(['2026-09-18'])
  })
})
