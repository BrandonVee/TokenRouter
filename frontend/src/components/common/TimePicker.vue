<template>
  <div class="time-picker" :class="disabled && 'time-picker-disabled'" :data-testid="testId">
    <input
      :value="inputValue"
      type="time"
      step="60"
      :disabled="disabled"
      :aria-label="ariaLabel"
      class="input time-picker-input"
      @input="onInput"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

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

const inputValue = computed(() => {
  const parsed = parseTime(props.modelValue)
  return parsed ? `${pad(parsed.hour)}:${pad(parsed.minute)}` : ''
})

// 原生时间输入一次完成小时和分钟选择，避免两个下拉分别打开造成状态残留。
const onInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  const parsed = parseTime(value)
  emit('update:modelValue', parsed ? `${pad(parsed.hour)}:${pad(parsed.minute)}` : '')
}
</script>

<style scoped>
.time-picker {
  @apply flex w-full;
}

.time-picker-input {
  @apply h-10 w-full min-w-0 rounded-lg px-3 font-mono text-base tabular-nums;
  color-scheme: light dark;
}

.time-picker-input::-webkit-calendar-picker-indicator {
  @apply cursor-pointer opacity-60;
}

.time-picker-disabled {
  @apply cursor-not-allowed opacity-60;
}
</style>
