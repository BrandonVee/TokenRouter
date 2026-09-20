<template>
  <ConfigProvider :locale="antLocale" :theme="antTheme">
    <DatePicker
      :value="pickerValue"
      :allow-clear="allowClear"
      :disabled="disabled"
      :format="displayFormat"
      :placeholder="placeholder || defaultPlaceholder"
      :show-time="showTime ? { format: 'HH:mm' } : false"
      :show-now="showTime"
      :disabled-date="disabledDate"
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

// 日期上下限只限制日历日期；具体时间仍由业务保存时校验，避免同一天被整体禁用。
const disabledDate = (current: Dayjs) => {
  const min = props.min ? dayjs(props.min) : null
  const max = props.max ? dayjs(props.max) : null
  if (min?.isValid() && current.endOf('day').isBefore(min.startOf('day'))) return true
  if (max?.isValid() && current.startOf('day').isAfter(max.endOf('day'))) return true
  return false
}

const handleChange = (value: Dayjs | string | null) => {
  const parsed = typeof value === 'string' ? dayjs(value) : value
  const nextValue = parsed?.isValid()
    ? parsed.format(props.showTime ? 'YYYY-MM-DDTHH:mm' : 'YYYY-MM-DD')
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

.dark .tokenrouter-date-time-picker {
  border-color: rgb(71 85 105);
  background: rgb(15 23 42);
}

:global(.tokenrouter-ant-date-popup) {
  z-index: 11000;
}
</style>
