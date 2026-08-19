<template>
  <div class="flex flex-wrap gap-2">
    <button
      v-for="preset in presets"
      :key="`${preset.mode}:${preset.protocol}:${preset.url}`"
      type="button"
      class="rounded-lg bg-gray-100 px-3 py-1 text-xs text-gray-700 transition-colors hover:bg-primary-50 hover:text-primary-700 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-primary-900/30 dark:hover:text-primary-400"
      @click="emit('select', preset)"
    >
      {{ preset.label }} ({{ preset.url.replace(/^https?:\/\//i, '') }})
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { CN_BASE_URL_PRESETS, type CnBaseUrlPreset } from './credentialsBuilder'

const props = defineProps<{
  platform: 'kimi' | 'zhipu' | 'deepseek'
  protocol?: 'chat_completions' | 'anthropic' | 'responses'
}>()

const emit = defineEmits<{ (event: 'select', preset: CnBaseUrlPreset): void }>()

const presets = computed(() => {
  const all = CN_BASE_URL_PRESETS[props.platform]
  return props.protocol ? all.filter(item => item.protocol === props.protocol) : all
})
</script>
