<template>
  <div class="space-y-5" data-testid="file-storage-settings">
    <div
      class="inline-flex w-full border-b border-gray-200 dark:border-dark-700 sm:w-auto"
      role="tablist"
      :aria-label="t('admin.settings.fileStorage.sections.label')"
    >
      <button
        v-for="section in sections"
        :key="section"
        type="button"
        role="tab"
        :aria-selected="activeSection === section"
        :class="[
          'border-b-2 px-4 py-2.5 text-sm font-medium transition',
          activeSection === section
            ? 'border-primary-600 text-primary-700 dark:text-primary-300'
            : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white',
        ]"
        @click="activeSection = section"
      >
        {{ t(`admin.settings.fileStorage.sections.${section}`) }}
      </button>
    </div>

    <template v-if="activeSection === 'images'">
      <div v-if="loading" class="flex min-h-48 items-center justify-center text-gray-500">
        <Icon name="refresh" size="sm" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <div class="card">
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.settings.fileStorage.images.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.fileStorage.images.description') }}
              </p>
            </div>
            <div class="flex items-center gap-2 text-xs">
              <span
                class="rounded border px-2 py-1"
                :class="form.available
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
                  : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-400'"
              >
                {{ form.available ? t('common.enabled') : t('common.disabled') }}
              </span>
              <span class="text-gray-500 dark:text-gray-400">
                {{ t(`admin.settings.fileStorage.images.source.${form.source}`) }}
              </span>
            </div>
          </div>

          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between gap-4">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.settings.fileStorage.images.enabled') }}
                </label>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.fileStorage.images.enabledHint') }}
                </p>
              </div>
              <Toggle v-model="form.enabled" data-testid="image-history-storage-enabled" />
            </div>

            <div
              v-if="form.enabled && !form.encryption_key_ready"
              class="border-l-4 border-amber-400 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
              data-testid="image-history-encryption-warning"
            >
              {{ t('admin.settings.fileStorage.images.encryptionKeyRequired') }}
            </div>

            <div class="grid grid-cols-1 gap-4 border-t border-gray-100 pt-5 dark:border-dark-700 md:grid-cols-2">
              <div class="md:col-span-2">
                <label class="input-label">{{ t('admin.settings.fileStorage.images.endpoint') }}</label>
                <input
                  v-model.trim="form.endpoint"
                  class="input w-full font-mono text-sm"
                  placeholder="https://ACCOUNT_ID.r2.cloudflarestorage.com"
                  autocomplete="off"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.settings.fileStorage.images.region') }}</label>
                <input v-model.trim="form.region" class="input w-full font-mono text-sm" placeholder="auto" autocomplete="off" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.settings.fileStorage.images.bucket') }}</label>
                <input v-model.trim="form.bucket" class="input w-full font-mono text-sm" autocomplete="off" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.settings.fileStorage.images.accessKeyId') }}</label>
                <input v-model.trim="form.access_key_id" class="input w-full font-mono text-sm" autocomplete="off" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.settings.fileStorage.images.secretAccessKey') }}</label>
                <input
                  v-model="form.secret_access_key"
                  type="password"
                  class="input w-full font-mono text-sm"
                  :placeholder="form.secret_configured ? t('admin.settings.fileStorage.images.secretConfigured') : ''"
                  autocomplete="new-password"
                />
              </div>
              <div class="md:col-span-2">
                <label class="input-label">{{ t('admin.settings.fileStorage.images.prefix') }}</label>
                <div class="relative">
                  <Icon name="folder" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                  <input
                    v-model.trim="form.prefix"
                    class="input w-full pl-10 font-mono text-sm"
                    placeholder="image-history"
                    autocomplete="off"
                  />
                </div>
              </div>
              <div class="flex items-center justify-between gap-4 md:col-span-2">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">
                    {{ t('admin.settings.fileStorage.images.forcePathStyle') }}
                  </label>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.fileStorage.images.forcePathStyleHint') }}
                  </p>
                </div>
                <Toggle v-model="form.force_path_style" />
              </div>
            </div>

            <div class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-700">
              <button
                v-if="form.source === 'database'"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="resetting"
                @click="resetConfig"
              >
                <Icon name="refresh" size="sm" />
                {{ t('admin.settings.fileStorage.images.restoreDeployment') }}
              </button>
              <span v-else></span>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="testing || !hasConnectionFields" @click="testConnection">
                  <Icon name="cloud" size="sm" />
                  {{ testing ? t('common.loading') : t('admin.settings.fileStorage.images.test') }}
                </button>
                <button type="button" class="btn btn-primary btn-sm" :disabled="saving || !canSave" @click="saveConfig">
                  <Icon name="check" size="sm" />
                  {{ saving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </template>

    <template v-else-if="activeSection === 'attachments'">
      <div class="card">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.fileStorage.invoice.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.fileStorage.invoice.description') }}</p>
        </div>
        <div class="space-y-5 p-6">
          <div class="grid grid-cols-2 gap-2 rounded border border-gray-200 p-1 dark:border-dark-700">
            <button type="button" class="btn btn-sm" :class="invoiceForm.profile.type === 'local' ? 'btn-primary' : 'btn-secondary'" @click="invoiceForm.profile.type = 'local'">{{ t('admin.settings.fileStorage.invoice.local') }}</button>
            <button type="button" class="btn btn-sm" :class="invoiceForm.profile.type === 's3' ? 'btn-primary' : 'btn-secondary'" @click="invoiceForm.profile.type = 's3'">S3</button>
          </div>
          <div v-if="invoiceForm.profile.type === 'local'" class="rounded border border-gray-200 bg-gray-50 px-4 py-3 font-mono text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300">{{ invoiceForm.profile.local_path }}</div>
          <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="md:col-span-2"><label class="input-label">{{ t('admin.settings.fileStorage.images.endpoint') }}</label><input v-model.trim="invoiceForm.profile.s3.endpoint" class="input w-full font-mono text-sm" placeholder="https://ACCOUNT_ID.r2.cloudflarestorage.com" /></div>
            <div><label class="input-label">{{ t('admin.settings.fileStorage.images.region') }}</label><input v-model.trim="invoiceForm.profile.s3.region" class="input w-full font-mono text-sm" placeholder="auto" /></div>
            <div><label class="input-label">{{ t('admin.settings.fileStorage.images.bucket') }}</label><input v-model.trim="invoiceForm.profile.s3.bucket" class="input w-full font-mono text-sm" /></div>
            <div><label class="input-label">{{ t('admin.settings.fileStorage.images.accessKeyId') }}</label><input v-model.trim="invoiceForm.profile.s3.access_key_id" class="input w-full font-mono text-sm" /></div>
            <div><label class="input-label">{{ t('admin.settings.fileStorage.images.secretAccessKey') }}</label><input v-model="invoiceForm.profile.s3.secret_access_key" type="password" class="input w-full font-mono text-sm" :placeholder="invoiceForm.profile.secret_configured ? t('admin.settings.fileStorage.images.secretConfigured') : ''" /></div>
            <div class="md:col-span-2"><label class="input-label">{{ t('admin.settings.fileStorage.images.prefix') }}</label><input v-model.trim="invoiceForm.profile.s3.prefix" class="input w-full font-mono text-sm" placeholder="invoice-attachments" /></div>
            <div class="flex items-center justify-between gap-4 md:col-span-2"><span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.settings.fileStorage.images.forcePathStyle') }}</span><Toggle v-model="invoiceForm.profile.s3.force_path_style" /></div>
          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.fileStorage.invoice.versionHint') }}</p>
          <div class="flex justify-end gap-2 border-t border-gray-100 pt-5 dark:border-dark-700">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="invoiceTesting" @click="testInvoiceStorage"><Icon name="cloud" size="sm" />{{ invoiceTesting ? t('common.loading') : t('admin.settings.fileStorage.images.test') }}</button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="invoiceSaving" @click="saveInvoiceStorage"><Icon name="check" size="sm" />{{ invoiceSaving ? t('common.saving') : t('common.save') }}</button>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="divide-y divide-gray-100 border-y border-gray-200 dark:divide-dark-700 dark:border-dark-700">
      <div v-for="item in otherStorageItems" :key="item.key" class="flex flex-col gap-3 py-5 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-start gap-3">
          <span class="mt-0.5 flex h-9 w-9 flex-none items-center justify-center rounded-md bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
            <Icon :name="item.icon" size="sm" />
          </span>
          <div class="min-w-0">
            <h3 class="font-medium text-gray-900 dark:text-white">{{ item.title }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ item.description }}</p>
            <code class="mt-2 block break-all text-xs text-gray-600 dark:text-gray-300">{{ item.path }}</code>
          </div>
        </div>
        <button v-if="item.action" type="button" class="btn btn-secondary btn-sm flex-none" @click="item.action">
          <Icon name="arrowRight" size="sm" />
          {{ item.actionLabel }}
        </button>
      </div>
    </div>

    <TotpStepUpDialog :controller="storageStepUp" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { FileStorageDirectoryConfig, ImageHistoryStorageConfig } from '@/api/admin/fileStorage'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'

const emit = defineEmits<{
  'open-backup': []
  'open-data-sharing': []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const storageStepUp = useStepUp()
const sections = ['images', 'attachments', 'other'] as const
const activeSection = ref<(typeof sections)[number]>('images')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const resetting = ref(false)
const invoiceSaving = ref(false)
const invoiceTesting = ref(false)

const emptyConfig = (): ImageHistoryStorageConfig => ({
  enabled: false,
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  prefix: 'image-history',
  force_path_style: false,
  secret_configured: false,
  available: false,
  source: 'deployment',
  encryption_key_ready: false,
})

const form = reactive<ImageHistoryStorageConfig>(emptyConfig())
const invoiceForm = reactive<FileStorageDirectoryConfig>({
  directory: 'invoice_attachments',
  profile: { id: '', type: 'local', local_path: '', s3: { endpoint: '', region: 'auto', bucket: '', access_key_id: '', secret_access_key: '', prefix: 'invoice-attachments', force_path_style: false }, secret_configured: false, encryption_key_ready: false },
})

const hasConnectionFields = computed(() =>
  Boolean(form.bucket.trim() && form.access_key_id.trim() && (form.secret_access_key?.trim() || form.secret_configured)),
)
const canSave = computed(() =>
  !form.enabled || (hasConnectionFields.value && Boolean(form.prefix.trim()) && form.encryption_key_ready),
)

function assignConfig(config: ImageHistoryStorageConfig): void {
  Object.assign(form, emptyConfig(), config, { secret_access_key: '' })
}

function buildPayload(): ImageHistoryStorageConfig {
  return {
    ...form,
    endpoint: form.endpoint.trim(),
    region: form.region.trim() || 'auto',
    bucket: form.bucket.trim(),
    access_key_id: form.access_key_id.trim(),
    secret_access_key: form.secret_access_key || '',
    prefix: form.prefix.trim(),
  }
}

function reportStepUpBlocked(error: unknown): boolean {
  if (!isStepUpBlocked(error)) return false
  appStore.showError(
    stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
      ? t('stepUp.adminApiKeyForbidden')
      : t('stepUp.notEnabled'),
  )
  return true
}

async function loadConfig(): Promise<void> {
  loading.value = true
  try {
    assignConfig(await adminAPI.fileStorage.getImageHistoryStorageConfig())
	// 发票目录加载失败不应阻断既有生图历史配置，管理员仍可修复独立接口。
	try {
		const invoiceConfig = await adminAPI.fileStorage.getInvoiceAttachmentStorageConfig()
		Object.assign(invoiceForm, invoiceConfig, { profile: { ...invoiceConfig.profile, s3: { ...invoiceConfig.profile.s3, secret_access_key: '' } } })
	} catch {
		// 保留默认本地档案，避免某个目录不可用导致整个页面不可用。
	}
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.images.loadFailed')))
  } finally {
    loading.value = false
  }
}

function invoicePayload(): FileStorageDirectoryConfig {
  return { ...invoiceForm, profile: { ...invoiceForm.profile, s3: { ...invoiceForm.profile.s3, region: invoiceForm.profile.s3.region || 'auto', secret_access_key: invoiceForm.profile.s3.secret_access_key || '' } } }
}

async function saveInvoiceStorage(): Promise<void> {
  invoiceSaving.value = true
  try {
    const updated = await storageStepUp.run(() => adminAPI.fileStorage.updateInvoiceAttachmentStorageConfig(invoicePayload()))
    Object.assign(invoiceForm, updated, { profile: { ...updated.profile, s3: { ...updated.profile.s3, secret_access_key: '' } } })
    appStore.showSuccess(t('admin.settings.fileStorage.invoice.saved'))
  } catch (error) {
    if (isStepUpCancelled(error) || reportStepUpBlocked(error)) return
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.invoice.saveFailed')))
  } finally { invoiceSaving.value = false }
}

async function testInvoiceStorage(): Promise<void> {
  invoiceTesting.value = true
  try {
    const result = await adminAPI.fileStorage.testInvoiceAttachmentStorageConnection(invoicePayload())
    if (result.ok) appStore.showSuccess(t('admin.settings.fileStorage.images.testSucceeded'))
    else appStore.showError(result.message || t('admin.settings.fileStorage.images.testFailed'))
  } catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.images.testFailed'))) } finally { invoiceTesting.value = false }
}

async function saveConfig(): Promise<void> {
  saving.value = true
  try {
    const updated = await storageStepUp.run(() =>
      adminAPI.fileStorage.updateImageHistoryStorageConfig(buildPayload()),
    )
    assignConfig(updated)
    appStore.showSuccess(t('admin.settings.fileStorage.images.saved'))
  } catch (error) {
    if (isStepUpCancelled(error) || reportStepUpBlocked(error)) return
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.images.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function testConnection(): Promise<void> {
  testing.value = true
  try {
    const result = await adminAPI.fileStorage.testImageHistoryStorageConnection(buildPayload())
    if (result.ok) {
      appStore.showSuccess(t('admin.settings.fileStorage.images.testSucceeded'))
    } else {
      appStore.showError(result.message || t('admin.settings.fileStorage.images.testFailed'))
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.images.testFailed')))
  } finally {
    testing.value = false
  }
}

async function resetConfig(): Promise<void> {
  if (!window.confirm(t('admin.settings.fileStorage.images.restoreConfirm'))) return
  resetting.value = true
  try {
    const restored = await storageStepUp.run(() => adminAPI.fileStorage.resetImageHistoryStorageConfig())
    assignConfig(restored)
    appStore.showSuccess(t('admin.settings.fileStorage.images.restored'))
  } catch (error) {
    if (isStepUpCancelled(error) || reportStepUpBlocked(error)) return
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.fileStorage.images.restoreFailed')))
  } finally {
    resetting.value = false
  }
}

const otherStorageItems = computed(() => [
  {
    key: 'backup',
    icon: 'database' as const,
    title: t('admin.settings.fileStorage.other.backup.title'),
    description: t('admin.settings.fileStorage.other.backup.description'),
    path: 'DATA_DIR/backups',
    actionLabel: t('admin.settings.fileStorage.other.backup.action'),
    action: () => emit('open-backup'),
  },
  {
    key: 'invoice',
    icon: 'document' as const,
    title: t('admin.settings.fileStorage.other.invoice.title'),
    description: t('admin.settings.fileStorage.other.invoice.description'),
    path: 'DATA_DIR/invoice-attachments',
  },
  {
    key: 'dataSharing',
    icon: 'upload' as const,
    title: t('admin.settings.fileStorage.other.dataSharing.title'),
    description: t('admin.settings.fileStorage.other.dataSharing.description'),
    path: 'DATA_DIR/data-sharing-exports',
    actionLabel: t('admin.settings.fileStorage.other.dataSharing.action'),
    action: () => emit('open-data-sharing'),
  },
])

onMounted(loadConfig)
</script>
