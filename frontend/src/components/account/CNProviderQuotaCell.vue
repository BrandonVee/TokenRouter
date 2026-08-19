<template>
  <div v-if="visible" class="text-[10px] text-gray-600 dark:text-gray-300">
    <button
      type="button"
      class="text-primary-600 hover:text-primary-700 disabled:opacity-50"
      :disabled="loading"
      @click="probe"
    >
      {{ label }}
    </button>
    <span v-if="error" class="ml-1 text-red-600" :title="error">!</span>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import { cnQuotaCellVisible } from './credentialsBuilder'

const props = defineProps<{ account: Account }>()
const loading = ref(false)
const error = ref('')
const label = ref('额度')
const visible = computed(() => cnQuotaCellVisible(props.account.platform, String(props.account.credentials?.account_mode || '')))

// 额度窗口属于主动探测数据，不在列表初始化时自动请求。
const probe = async () => {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const result = await adminAPI.cnProviders.queryQuota(props.account.id)
    if (result.success && result.tiers?.length) {
      label.value = result.tiers.map(item => `${item.window} ${Math.round(item.used_percent)}%`).join(' · ')
    } else if (!result.success) {
      error.value = result.error || '查询失败'
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '查询失败'
  } finally {
    loading.value = false
  }
}
</script>
