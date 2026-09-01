<template>
  <button
    type="button"
    :disabled="props.disabled"
    @click="toggle"
    class="relative inline-flex flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
    :class="[
      props.modelValue ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600',
      props.disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
      props.size === 'sm' ? 'h-5 w-9' : 'h-6 w-11'
    ]"
    role="switch"
    :aria-checked="props.modelValue"
  >
    <span
      class="pointer-events-none inline-block transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
      :class="[
        props.size === 'sm' ? 'h-4 w-4' : 'h-5 w-5',
        props.modelValue ? (props.size === 'sm' ? 'translate-x-4' : 'translate-x-5') : 'translate-x-0'
      ]"
    />
  </button>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  modelValue: boolean
  size?: 'sm' | 'md'
  disabled?: boolean
}>(), {
  size: 'md',
  disabled: false
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>
