<template>
  <ConfigProvider :locale="antLocale" :theme="antTheme">
    <DatePicker
      :value="pickerValue"
      :allow-clear="allowClear"
      :disabled="disabled"
      :format="displayFormat"
      :placeholder="placeholder || defaultPlaceholder"
      :show-time="timeOptions"
      :show-now="showTime && isNowAllowed"
      :disabled-date="disabledDate"
      :disabled-time="disabledTime"
      :status="isOutOfRange ? 'error' : undefined"
      placement="bottomLeft"
      popup-class-name="tokenrouter-ant-date-popup"
      class="tokenrouter-date-time-picker"
      @change="handleChange"
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

const props = withDefaults(defineProps<{
  modelValue?: string | null
  showTime?: boolean
  allowClear?: boolean
  disabled?: boolean
  placeholder?: string
  min?: string | null
  max?: string | null
}>(), {
  modelValue: null,
  showTime: false,
  allowClear: true,
  disabled: false,
  placeholder: '',
  min: null,
  max: null,
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: string | null): void
  (event: 'change', value: string | null): void
}>()

const { t, locale } = useI18n()
const { isDark } = useTheme()

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

const pickerValue = computed(() => {
  const value = props.modelValue?.trim()
  if (!value) return undefined
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed : undefined
})

const displayFormat = computed(() => {
  if (props.showTime) {
    return locale.value.toLowerCase().startsWith('zh') ? 'YYYY年M月D日 HH:mm' : 'MMM D, YYYY HH:mm'
  }
  return locale.value.toLowerCase().startsWith('zh') ? 'YYYY年M月D日' : 'MMM D, YYYY'
})

const defaultPlaceholder = computed(() => props.showTime
  ? t('dates.selectDateTime')
  : t('dates.selectDate'))

const timeOptions = computed(() => props.showTime ? {
  format: 'HH:mm',
  minuteStep: 5,
  hideDisabledOptions: true,
  defaultValue: dayjs().second(0),
} : false)

const minValue = computed(() => {
  const value = props.min ? dayjs(props.min) : null
  return value?.isValid() ? value : null
})

const maxValue = computed(() => {
  const value = props.max ? dayjs(props.max) : null
  return value?.isValid() ? value : null
})

const isOutOfRange = computed(() => {
  const value = pickerValue.value
  if (!value) return false
  if (minValue.value && value.isBefore(minValue.value)) return true
  if (maxValue.value && value.isAfter(maxValue.value)) return true
  return false
})

const isNowAllowed = computed(() => {
  const now = dayjs()
  if (minValue.value && now.isBefore(minValue.value)) return false
  if (maxValue.value && now.isAfter(maxValue.value)) return false
  return true
})

// 日期上下限只限制日历日期；具体时间仍由业务保存时校验，避免同一天被整体禁用。
const disabledDate = (current: Dayjs) => {
  if (minValue.value && current.endOf('day').isBefore(minValue.value.startOf('day'))) return true
  if (maxValue.value && current.startOf('day').isAfter(maxValue.value.endOf('day'))) return true
  return false
}

const numberRange = (start: number, end: number) => Array.from(
  { length: Math.max(0, end - start) },
  (_, index) => start + index,
)

// 同一天内同时限制小时和分钟，避免结束时间仍可选到开始时间之前。
const disabledTime = (current: Dayjs | null) => {
  if (!props.showTime || !current) return {}

  return {
    disabledHours: () => {
      const disabled = new Set<number>()
      if (minValue.value && current.isSame(minValue.value, 'day')) {
        numberRange(0, minValue.value.hour()).forEach((hour) => disabled.add(hour))
      }
      if (maxValue.value && current.isSame(maxValue.value, 'day')) {
        numberRange(maxValue.value.hour() + 1, 24).forEach((hour) => disabled.add(hour))
      }
      return [...disabled]
    },
    disabledMinutes: (hour: number) => {
      const disabled = new Set<number>()
      if (minValue.value && current.isSame(minValue.value, 'day') && hour === minValue.value.hour()) {
        numberRange(0, minValue.value.minute()).forEach((minute) => disabled.add(minute))
      }
      if (maxValue.value && current.isSame(maxValue.value, 'day') && hour === maxValue.value.hour()) {
        numberRange(maxValue.value.minute() + 1, 60).forEach((minute) => disabled.add(minute))
      }
      return [...disabled]
    },
  }
}

const handleChange = (value: Dayjs | string | null) => {
  const parsed = typeof value === 'string' ? dayjs(value) : value
  let normalized = parsed?.isValid() ? parsed : null
  if (normalized && minValue.value && normalized.isBefore(minValue.value)) normalized = minValue.value
  if (normalized && maxValue.value && normalized.isAfter(maxValue.value)) normalized = maxValue.value
  const nextValue = normalized
    ? normalized.format(props.showTime ? 'YYYY-MM-DDTHH:mm' : 'YYYY-MM-DD')
    : null
  emit('update:modelValue', nextValue)
  emit('change', nextValue)
}
</script>

<style scoped>
.tokenrouter-date-time-picker {
  width: 100%;
  border-color: rgb(15 23 42 / 0.1);
  background: rgb(255 255 255);
  box-shadow: none;
}

.tokenrouter-date-time-picker:hover,
.tokenrouter-date-time-picker.ant-picker-focused {
  border-color: rgb(37 99 235 / 0.65);
  box-shadow: 0 0 0 3px rgb(37 99 235 / 0.12);
}

:global(.dark) .tokenrouter-date-time-picker {
  border-color: rgb(71 85 105);
  background: rgb(15 23 42);
}

:global(.tokenrouter-ant-date-popup) {
  z-index: 11000;
}

@media (max-width: 640px) {
  :global(.tokenrouter-ant-date-popup) {
    max-width: calc(100vw - 1.5rem);
  }

  :global(.tokenrouter-ant-date-popup .ant-picker-panel-container) {
    max-width: calc(100vw - 1.5rem);
    overflow: auto;
  }
}
</style>
