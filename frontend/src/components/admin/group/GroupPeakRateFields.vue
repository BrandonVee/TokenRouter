<template>
  <section class="group-peak-rate mt-4">
    <div class="group-peak-rate__header">
      <div class="flex min-w-0 items-start gap-3">
        <span class="group-peak-rate__icon">
          <Icon name="clock" size="sm" :stroke-width="2" />
        </span>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
              {{ t('admin.groups.peakRate.title') }}
            </h3>
            <span class="group-peak-rate__badge">
              {{ enabled ? t('admin.groups.peakRate.statusEnabled') : t('admin.groups.peakRate.statusDisabled') }}
            </span>
          </div>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.peakRate.description') }}
          </p>
        </div>
      </div>
      <button
        type="button"
        class="group-peak-rate__switch"
        :class="enabled && 'group-peak-rate__switch--active'"
        :aria-pressed="enabled"
        @click="$emit('update:enabled', !enabled)"
      >
        <span class="group-peak-rate__track"><span class="group-peak-rate__thumb" /></span>
        <span class="sr-only">{{ t('admin.groups.peakRate.enable') }}</span>
      </button>
    </div>

    <div v-if="enabled" class="group-peak-rate__body">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div>
          <label class="group-peak-rate__label">{{ t('admin.groups.peakRate.peakStart') }}</label>
          <TimePicker
            :model-value="start"
            :aria-label="t('admin.groups.peakRate.peakStart')"
            class="mt-1.5"
            @update:model-value="$emit('update:start', $event)"
          />
        </div>
        <div>
          <label class="group-peak-rate__label">{{ t('admin.groups.peakRate.peakEnd') }}</label>
          <TimePicker
            :model-value="end"
            :aria-label="t('admin.groups.peakRate.peakEnd')"
            class="mt-1.5"
            @update:model-value="$emit('update:end', $event)"
          />
        </div>
        <div>
          <label class="group-peak-rate__label">{{ t('admin.groups.peakRate.peakMultiplier') }}</label>
          <div class="relative mt-1.5">
            <input
              :value="multiplier"
              type="number"
              step="0.001"
              min="0"
              class="input group-peak-rate__multiplier pr-8"
              placeholder="1"
              :title="t('admin.groups.peakRate.multiplierHint')"
              @input="onMultiplierInput"
            />
            <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm font-semibold text-gray-400">×</span>
          </div>
        </div>
      </div>
      <p class="mt-3 flex items-start gap-1.5 text-xs leading-5 text-gray-500 dark:text-gray-400">
        <Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0 text-amber-500" />
        <span>{{ t('admin.groups.peakRate.hint') }}</span>
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import TimePicker from '@/components/common/TimePicker.vue'

interface Props {
  enabled: boolean
  start: string
  end: string
  multiplier: number | string | null
}

defineProps<Props>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:start': [value: string]
  'update:end': [value: string]
  'update:multiplier': [value: number | string]
}>()

const { t } = useI18n()

// 保持分组表单原有的数字字段语义，清空时保留空值而不是隐式写入 0。
const onMultiplierInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:multiplier', value === '' ? '' : Number(value))
}

</script>

<style scoped>
.group-peak-rate {
  @apply overflow-hidden rounded-xl border border-amber-200/80 bg-amber-50/50 dark:border-amber-900/70 dark:bg-amber-950/10;
}

.group-peak-rate__header {
  @apply flex items-start justify-between gap-4 px-3 py-3 sm:px-4;
}

.group-peak-rate__icon {
  @apply flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300;
}

.group-peak-rate__badge {
  @apply rounded-full border border-amber-200 bg-white/80 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:border-amber-800 dark:bg-dark-800/70 dark:text-amber-300;
}

.group-peak-rate__switch {
  @apply shrink-0 rounded-full p-1 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-amber-500/40;
}

.group-peak-rate__track {
  @apply relative block h-5 w-9 rounded-full bg-gray-300 transition-colors dark:bg-dark-600;
}

.group-peak-rate__switch--active .group-peak-rate__track {
  @apply bg-amber-500;
}

.group-peak-rate__thumb {
  @apply absolute left-0.5 top-0.5 block h-4 w-4 rounded-full bg-white shadow-sm transition-transform;
}

.group-peak-rate__switch--active .group-peak-rate__thumb {
  @apply translate-x-4;
}

.group-peak-rate__body {
  @apply border-t border-amber-200/80 px-3 py-3 dark:border-amber-900/70 sm:px-4;
}

.group-peak-rate__label {
  @apply text-xs font-medium text-gray-500 dark:text-gray-400;
}

.group-peak-rate__multiplier {
  @apply rounded-lg py-2;
}
</style>
