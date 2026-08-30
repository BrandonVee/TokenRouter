<template>
  <div class="time-picker" :class="disabled && 'time-picker-disabled'" :data-testid="testId">
    <Select
      :model-value="hourValue"
      :options="hourOptions"
      :disabled="disabled"
      :searchable="false"
      :aria-label="hourAriaLabel"
      class="time-picker-part"
      @update:model-value="onHourChange"
    />
    <span class="time-picker-separator" aria-hidden="true">:</span>
    <Select
      :model-value="minuteValue"
      :options="minuteOptions"
      :disabled="disabled"
      :searchable="false"
      :aria-label="minuteAriaLabel"
      class="time-picker-part"
      @update:model-value="onMinuteChange"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Select from './Select.vue'

interface Props {
  modelValue: string
  disabled?: boolean
  ariaLabel?: string
  testId?: string
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  ariaLabel: '时间',
  testId: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const pad = (value: number): string => String(value).padStart(2, '0')

// 仅接受后端约定的 HH:MM，异常值保持为空并等待用户重新选择。
const parseTime = (value: string): { hour: number; minute: number } | null => {
  const match = /^(\d{2}):(\d{2})$/.exec(value || '')
  if (!match) return null
  const hour = Number(match[1])
  const minute = Number(match[2])
  if (hour > 23 || minute > 59) return null
  return { hour, minute }
}

const parsedTime = computed(() => parseTime(props.modelValue))
const hourValue = computed(() => parsedTime.value?.hour ?? null)
const minuteValue = computed(() => parsedTime.value?.minute ?? null)

const hourOptions = computed(() => Array.from({ length: 24 }, (_, hour) => ({
  value: hour,
  label: pad(hour),
})))

const minuteOptions = computed(() => Array.from({ length: 60 }, (_, minute) => ({
  value: minute,
  label: pad(minute),
})))

const hourAriaLabel = computed(() => `${props.ariaLabel}小时`)
const minuteAriaLabel = computed(() => `${props.ariaLabel}分钟`)

const emitTime = (hour: number | null, minute: number | null) => {
  if (hour === null || minute === null) {
    emit('update:modelValue', '')
    return
  }
  emit('update:modelValue', `${pad(hour)}:${pad(minute)}`)
}

const onHourChange = (value: string | number | boolean | null) => {
  const hour = value === null ? Number.NaN : Number(value)
  const validHour = Number.isInteger(hour) && hour >= 0 && hour <= 23 ? hour : null
  emitTime(validHour, parsedTime.value?.minute ?? 0)
}

const onMinuteChange = (value: string | number | boolean | null) => {
  const minute = value === null ? Number.NaN : Number(value)
  const validMinute = Number.isInteger(minute) && minute >= 0 && minute <= 59 ? minute : null
  emitTime(parsedTime.value?.hour ?? 0, validMinute)
}
</script>

<style scoped>
.time-picker {
  @apply flex items-center gap-1;
}

.time-picker-part {
  min-width: 0;
  flex: 1 1 0;
}

.time-picker-part :deep(.select-trigger) {
  @apply rounded-lg px-2.5 py-2 font-mono text-sm tabular-nums;
  min-height: 2.5rem;
}

.time-picker-part :deep(.select-value) {
  @apply text-center;
}

.time-picker-part :deep(.select-icon) {
  @apply ml-1;
}

.time-picker-separator {
  @apply shrink-0 text-sm font-semibold text-gray-400 dark:text-dark-400;
}

.time-picker-disabled {
  @apply cursor-not-allowed opacity-60;
}
</style>
