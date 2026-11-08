<template>
  <section v-if="account.opencode_go_usage?.eligible" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600">
    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.opencodeGo.title') }}</h3>
    <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.opencodeGo.autoRefreshHint') }}</p>
    <p v-if="loading" class="text-xs text-gray-500">{{ t('common.loading') }}</p>
    <p v-else-if="error" role="alert" class="text-xs text-red-600 dark:text-red-400">{{ error }}</p>
    <div v-else class="space-y-3">
      <Toggle v-model="settings.enabled" :label="t('admin.accounts.opencodeGo.globalEnabled')" />
      <Toggle v-model="autoRefresh" :label="t('admin.accounts.opencodeGo.autoRefresh')" />
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="text-xs text-gray-600 dark:text-gray-300">
          {{ t('admin.accounts.opencodeGo.maxWaitMinutes') }}
          <input v-model.number="settings.interval_minutes" type="number" min="5" max="1440" class="input mt-1" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">
          {{ t('admin.accounts.opencodeGo.quietMinutes') }}
          <input v-model.number="settings.debounce_minutes" type="number" min="1" max="60" class="input mt-1" />
        </label>
      </div>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="save">{{ t('common.save') }}</button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { Account, OpenCodeGoUsageSettings, OpenCodeGoUsageState } from '@/types'
import Toggle from '@/components/common/Toggle.vue'

const props = defineProps<{ account: Account }>()
const emit = defineEmits<{ updated: [state: OpenCodeGoUsageState] }>()
const { t } = useI18n()
const app = useAppStore()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const autoRefresh = ref(props.account.opencode_go_usage?.auto_refresh_enabled ?? false)
const settings = ref<OpenCodeGoUsageSettings>({ enabled: false, interval_minutes: 15, debounce_minutes: 1 })

// 设置区独立保存；普通账号编辑不会把脱敏快照或未保存的开关写回受管状态。
watch(() => props.account.id, async (_id, _previous, onCleanup) => {
  let active = true
  onCleanup(() => { active = false })
  autoRefresh.value = props.account.opencode_go_usage?.auto_refresh_enabled ?? false
  error.value = ''
  loading.value = true
  if (!props.account.opencode_go_usage?.eligible) { loading.value = false; return }
  try {
    const loaded = await adminAPI.accounts.getOpenCodeGoUsageSettings()
    if (active) settings.value = loaded
  } catch {
    if (active) error.value = t('admin.accounts.opencodeGo.loadFailed')
  } finally {
    if (active) loading.value = false
  }
}, { immediate: true })

async function save() {
  const accountID = props.account.id
  const desiredAutoRefresh = autoRefresh.value
  let globalSaved = false
  saving.value = true
  try {
    settings.value = await adminAPI.accounts.updateOpenCodeGoUsageSettings({ ...settings.value })
    globalSaved = true
    const state = await adminAPI.accounts.setOpenCodeGoUsageAutoRefresh(accountID, desiredAutoRefresh)
    if (props.account.id === accountID) emit('updated', state)
    app.showSuccess(t('common.saved'))
  } catch {
    app.showError(t(globalSaved ? 'admin.accounts.opencodeGo.partialSave' : 'admin.accounts.opencodeGo.saveFailed'))
  }
  finally { saving.value = false }
}
</script>
