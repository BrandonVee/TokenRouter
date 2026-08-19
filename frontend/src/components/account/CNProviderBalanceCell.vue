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
import { cnBalanceCellVisible } from './credentialsBuilder'

const props = defineProps<{ account: Account }>()
const loading = ref(false)
const error = ref('')
const balance = ref<string | null>(null)
const visible = computed(() => cnBalanceCellVisible(props.account.platform, String(props.account.credentials?.account_mode || '')))
const label = computed(() => balance.value || '余额')

// 只在用户主动点击时查询，避免账号列表刷新时集中触发供应商接口。
const probe = async () => {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const result = await adminAPI.cnProviders.queryBalance(props.account.id)
    if (result.success) {
      balance.value = (result.balances || [{ currency: result.currency || '', balance: result.balance }])
        .map(item => `${item.currency} ${item.balance.toFixed(2)}`)
        .join(' · ')
    } else {
      error.value = result.error || '查询失败'
    }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '查询失败'
  } finally {
    loading.value = false
  }
}
</script>
