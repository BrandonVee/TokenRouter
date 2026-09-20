<template>
  <ConfigProvider :locale="antLocale" :theme="antTheme">
    <!-- 直接使用单层范围面板，避免先打开自定义浮层、再打开日历的重复操作。 -->
    <AntRangePicker
      :value="calendarRange"
      :allow-clear="false"
      :disabled-date="disabledCalendarDate"
      :format="rangeDisplayFormat"
      :placeholder="[t('dates.startDate'), t('dates.endDate')]"
      :presets="antPresets"
      placement="bottomLeft"
      popup-class-name="tokenrouter-ant-date-popup tokenrouter-ant-date-range-popup"
      class="tokenrouter-date-range-picker"
      @change="onCalendarRangeChange"
    />
  </ConfigProvider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DatePicker from 'ant-design-vue/es/date-picker'
import ConfigProvider from 'ant-design-vue/es/config-provider'
import zhCN from 'ant-design-vue/es/locale/zh_CN'
import enUS from 'ant-design-vue/es/locale/en_US'
import theme from 'ant-design-vue/es/theme'
import dayjs, { type Dayjs } from 'dayjs'
import { useTheme } from '@/composables/useTheme'

interface DatePreset {
  labelKey: string
  value: string
  durationMs?: number
  getRange: () => { start: string; end: string }
}

interface Props {
  startDate: string
  endDate: string
}

interface Emits {
  (event: 'update:startDate', value: string): void
  (event: 'update:endDate', value: string): void
  (event: 'change', range: { startDate: string; endDate: string; preset: string | null }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t, locale } = useI18n()
const { isDark } = useTheme()
const AntRangePicker = DatePicker.RangePicker

const antLocale = computed(() => locale.value.toLowerCase().startsWith('zh') ? zhCN : enUS)
const antTheme = computed(() => ({
  algorithm: isDark.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
  token: {
    colorPrimary: '#2563eb',
    borderRadius: 8,
    controlHeight: 40,
    fontSize: 14,
  },
}))
const rangeDisplayFormat = computed(() => locale.value.toLowerCase().startsWith('zh')
  ? 'YYYY年M月D日'
  : 'MMM D, YYYY')

const formatDateToString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const formatDateTimeToString = (date: Date): string => {
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${formatDateToString(date)}T${hours}:${minutes}:${seconds}`
}

const today = computed(() => formatDateToString(new Date()))

// 服务端与浏览器可能跨时区，允许选择本地明天，保持原有查询边界。
const tomorrow = computed(() => {
  const date = new Date()
  date.setDate(date.getDate() + 1)
  return formatDateToString(date)
})

const presets: DatePreset[] = [
  {
    labelKey: 'dates.last15Minutes',
    value: 'last15Minutes',
    durationMs: 15 * 60 * 1000,
    getRange: () => {
      const end = new Date()
      return {
        start: formatDateTimeToString(new Date(end.getTime() - 15 * 60 * 1000)),
        end: formatDateTimeToString(end),
      }
    },
  },
  {
    labelKey: 'dates.last30Minutes',
    value: 'last30Minutes',
    durationMs: 30 * 60 * 1000,
    getRange: () => {
      const end = new Date()
      return {
        start: formatDateTimeToString(new Date(end.getTime() - 30 * 60 * 1000)),
        end: formatDateTimeToString(end),
      }
    },
  },
  {
    labelKey: 'dates.today',
    value: 'today',
    getRange: () => ({ start: today.value, end: today.value }),
  },
  {
    labelKey: 'dates.yesterday',
    value: 'yesterday',
    getRange: () => {
      const date = new Date()
      date.setDate(date.getDate() - 1)
      const value = formatDateToString(date)
      return { start: value, end: value }
    },
  },
  {
    labelKey: 'dates.last24Hours',
    value: 'last24Hours',
    getRange: () => {
      const end = new Date()
      return {
        start: formatDateToString(new Date(end.getTime() - 24 * 60 * 60 * 1000)),
        end: formatDateToString(end),
      }
    },
  },
  {
    labelKey: 'dates.last7Days',
    value: '7days',
    getRange: () => {
      const date = new Date()
      date.setDate(date.getDate() - 6)
      return { start: formatDateToString(date), end: today.value }
    },
  },
  {
    labelKey: 'dates.last14Days',
    value: '14days',
    getRange: () => {
      const date = new Date()
      date.setDate(date.getDate() - 13)
      return { start: formatDateToString(date), end: today.value }
    },
  },
  {
    labelKey: 'dates.last30Days',
    value: '30days',
    getRange: () => {
      const date = new Date()
      date.setDate(date.getDate() - 29)
      return { start: formatDateToString(date), end: today.value }
    },
  },
  {
    labelKey: 'dates.thisMonth',
    value: 'thisMonth',
    getRange: () => {
      const now = new Date()
      return {
        start: formatDateToString(new Date(now.getFullYear(), now.getMonth(), 1)),
        end: today.value,
      }
    },
  },
  {
    labelKey: 'dates.lastMonth',
    value: 'lastMonth',
    getRange: () => {
      const now = new Date()
      return {
        start: formatDateToString(new Date(now.getFullYear(), now.getMonth() - 1, 1)),
        end: formatDateToString(new Date(now.getFullYear(), now.getMonth(), 0)),
      }
    },
  },
]

const calendarRange = computed<[Dayjs, Dayjs] | undefined>(() => {
  const start = dayjs(props.startDate)
  const end = dayjs(props.endDate)
  return start.isValid() && end.isValid() ? [start, end] : undefined
})

const antPresets = computed(() => presets.map((preset) => ({
  label: t(preset.labelKey),
  value: (() => {
    const range = preset.getRange()
    return [dayjs(range.start), dayjs(range.end)] as [Dayjs, Dayjs]
  })(),
})))

const disabledCalendarDate = (current: Dayjs) => current.startOf('day').isAfter(dayjs(tomorrow.value).endOf('day'))

const parseRangeTime = (value: string): number | null => {
  const timestamp = new Date(value.length === 10 ? `${value}T00:00:00` : value).getTime()
  return Number.isFinite(timestamp) ? timestamp : null
}

const resolvePreset = (startValue: string, endValue: string): DatePreset | null => {
  const startMs = parseRangeTime(startValue)
  const endMs = parseRangeTime(endValue)

  for (const preset of presets) {
    const range = preset.getRange()
    if (!preset.durationMs && range.start === startValue.slice(0, 10) && range.end === endValue.slice(0, 10)) {
      return preset
    }
    if (preset.durationMs && startMs !== null && endMs !== null) {
      const durationDrift = Math.abs(endMs - startMs - preset.durationMs)
      const endDrift = Math.abs(Date.now() - endMs)
      if (durationDrift <= 1000 && endDrift <= 90 * 1000) return preset
    }
  }
  return null
}

const onCalendarRangeChange = (values: [Dayjs, Dayjs] | [string, string] | null) => {
  if (!values) return
  const start = typeof values[0] === 'string' ? dayjs(values[0]) : values[0]
  const end = typeof values[1] === 'string' ? dayjs(values[1]) : values[1]
  if (!start.isValid() || !end.isValid()) return

  const rawStart = start.format('YYYY-MM-DDTHH:mm:ss')
  const rawEnd = end.format('YYYY-MM-DDTHH:mm:ss')
  const preset = resolvePreset(rawStart, rawEnd)
  const preserveTime = Boolean(preset?.durationMs)
  const startDate = start.format(preserveTime ? 'YYYY-MM-DDTHH:mm:ss' : 'YYYY-MM-DD')
  const endDate = end.format(preserveTime ? 'YYYY-MM-DDTHH:mm:ss' : 'YYYY-MM-DD')

  // 完成范围选择或点击快捷项后立即应用，省去第二层“应用”确认。
  emit('update:startDate', startDate)
  emit('update:endDate', endDate)
  emit('change', {
    startDate,
    endDate,
    preset: preset?.value ?? null,
  })
}
</script>

<style scoped>
.tokenrouter-date-range-picker {
  min-width: 17rem;
  border-color: rgb(15 23 42 / 0.1);
  background: rgb(255 255 255);
  box-shadow: none;
}

.tokenrouter-date-range-picker:hover,
.tokenrouter-date-range-picker.ant-picker-focused {
  border-color: rgb(37 99 235 / 0.65);
  box-shadow: 0 0 0 3px rgb(37 99 235 / 0.12);
}

:global(.dark) .tokenrouter-date-range-picker {
  border-color: rgb(71 85 105);
  background: rgb(15 23 42);
}

:global(.tokenrouter-ant-date-popup) {
  z-index: 11000;
}

:global(.tokenrouter-ant-date-range-popup .ant-picker-presets) {
  max-height: 21rem;
  overflow-y: auto;
}

@media (max-width: 640px) {
  .tokenrouter-date-range-picker {
    min-width: min(17rem, calc(100vw - 2rem));
  }

  :global(.tokenrouter-ant-date-range-popup) {
    left: 0.75rem !important;
    right: 0.75rem !important;
  }

  :global(.tokenrouter-ant-date-range-popup .ant-picker-panel-container) {
    max-width: calc(100vw - 1.5rem);
    overflow: auto;
  }

  :global(.tokenrouter-ant-date-range-popup .ant-picker-panels > :last-child) {
    display: none;
  }
}
</style>
