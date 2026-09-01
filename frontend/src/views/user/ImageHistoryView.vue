<template>
  <div class="space-y-5">
    <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
      <div class="flex flex-col gap-4 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-5">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 flex-none items-center justify-center rounded-lg bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-200">
            <Icon name="cloud" size="md" />
          </div>
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('imageHistory.settingsTitle') }}
            </h2>
            <p class="mt-0.5 text-xs" :class="settingsStatusClass">
              {{ settingsStatus }}
            </p>
          </div>
        </div>

        <div class="flex items-center justify-between gap-3 sm:justify-end">
          <button
            type="button"
            class="flex h-9 w-9 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :disabled="loading"
            :title="t('imageHistory.refresh')"
            :aria-label="t('imageHistory.refresh')"
            @click="refreshAll"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
          <Toggle
            :model-value="settings.enabled"
            :disabled="!settings.available || savingSettings || loadingSettings"
            @update:model-value="updateSavingSetting"
          />
        </div>
      </div>
    </section>

    <div v-if="loadingHistory" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <div v-for="index in 8" :key="index" class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="aspect-square animate-pulse bg-gray-100 dark:bg-dark-800"></div>
        <div class="space-y-2 p-4">
          <div class="h-4 w-2/3 animate-pulse rounded bg-gray-100 dark:bg-dark-800"></div>
          <div class="h-3 w-full animate-pulse rounded bg-gray-100 dark:bg-dark-800"></div>
        </div>
      </div>
    </div>

    <div
      v-else-if="!settings.available"
      class="flex min-h-64 flex-col items-center justify-center rounded-lg border border-dashed border-gray-300 px-5 text-center dark:border-dark-600"
    >
      <Icon name="cloud" size="xl" class="text-gray-400 dark:text-dark-500" />
      <p class="mt-3 text-sm font-medium text-gray-800 dark:text-gray-200">
        {{ t('imageHistory.unavailable') }}
      </p>
    </div>

    <div
      v-else-if="records.length === 0"
      class="flex min-h-64 flex-col items-center justify-center rounded-lg border border-dashed border-gray-300 px-5 text-center dark:border-dark-600"
    >
      <Icon name="inbox" size="xl" class="text-gray-400 dark:text-dark-500" />
      <p class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('imageHistory.empty') }}
      </p>
      <p class="mt-1 max-w-md text-xs text-gray-500 dark:text-gray-400">
        {{ t('imageHistory.emptyHint') }}
      </p>
    </div>

    <template v-else>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        <article
          v-for="record in records"
          :key="record.id"
          class="group overflow-hidden rounded-lg border border-gray-200 bg-white transition-colors hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600"
        >
          <div class="relative aspect-square overflow-hidden bg-gray-100 dark:bg-dark-950">
            <button
              v-if="record.preview_url && !previewErrors[record.id]"
              type="button"
              class="block h-full w-full focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
              :title="t('imageHistory.preview')"
              @click="openPreview(record)"
            >
              <img
                :src="record.preview_url"
                :alt="record.prompt || record.model || t('imageHistory.preview')"
                class="h-full w-full object-contain transition-transform duration-200 group-hover:scale-[1.015]"
                loading="lazy"
                referrerpolicy="no-referrer"
                @error="markPreviewError(record.id)"
              />
            </button>
            <div v-else class="flex h-full flex-col items-center justify-center text-gray-400 dark:text-dark-500">
              <Icon name="exclamationCircle" size="lg" />
              <span class="mt-2 text-xs">{{ t('imageHistory.imageUnavailable') }}</span>
            </div>

            <div class="absolute right-2 top-2 flex gap-1 opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
              <button
                type="button"
                class="flex h-9 w-9 items-center justify-center rounded-md bg-white/95 text-gray-700 shadow-sm transition-colors hover:bg-white hover:text-gray-950 dark:bg-dark-900/95 dark:text-gray-200 dark:hover:bg-dark-800 dark:hover:text-white"
                :title="t('imageHistory.download')"
                :aria-label="t('imageHistory.download')"
                :disabled="downloadingId === record.id"
                @click="downloadRecord(record)"
              >
                <Icon name="download" size="sm" :class="downloadingId === record.id ? 'animate-pulse' : ''" />
              </button>
              <button
                type="button"
                class="flex h-9 w-9 items-center justify-center rounded-md bg-white/95 text-gray-700 shadow-sm transition-colors hover:bg-red-50 hover:text-red-600 dark:bg-dark-900/95 dark:text-gray-200 dark:hover:bg-red-950/70 dark:hover:text-red-300"
                :title="t('imageHistory.delete')"
                :aria-label="t('imageHistory.delete')"
                @click="pendingDelete = record"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <div class="space-y-3 p-4">
            <div class="flex min-w-0 items-center justify-between gap-3">
              <p class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="record.model">
                {{ record.model || t('imageHistory.unknownModel') }}
              </p>
              <span class="flex-none rounded-md bg-gray-100 px-2 py-1 text-[11px] font-medium uppercase text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                {{ record.source }}
              </span>
            </div>
            <p class="min-h-10 line-clamp-2 text-sm leading-5 text-gray-600 dark:text-gray-400" :title="record.prompt">
              {{ record.prompt || '-' }}
            </p>
            <div class="flex items-center justify-between gap-3 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ dimensions(record) }}</span>
              <time :datetime="record.created_at">{{ formatDate(record.created_at) }}</time>
            </div>
          </div>
        </article>
      </div>

      <Pagination
        v-if="pagination.total > 0"
        class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700"
        :page="pagination.page"
        :page-size="pagination.pageSize"
        :total="pagination.total"
        :page-size-options="[12, 24, 48, 96]"
        @update:page="changePage"
        @update:page-size="changePageSize"
      />
    </template>

    <BaseDialog
      :show="Boolean(selectedRecord)"
      :title="selectedRecord?.model || t('imageHistory.preview')"
      width="extra-wide"
      body-class="p-0"
      @close="selectedRecord = null"
    >
      <div v-if="selectedRecord" class="grid min-w-0 lg:grid-cols-[minmax(0,1fr)_19rem]">
        <div class="flex min-h-64 items-center justify-center bg-gray-100 p-3 dark:bg-dark-950 sm:min-h-[34rem]">
          <img
            v-if="selectedRecord.preview_url"
            :src="selectedRecord.preview_url"
            :alt="selectedRecord.prompt || selectedRecord.model"
            class="max-h-[70vh] max-w-full object-contain"
            referrerpolicy="no-referrer"
          />
        </div>
        <dl class="min-w-0 space-y-4 border-t border-gray-200 p-5 text-sm dark:border-dark-700 lg:border-l lg:border-t-0">
          <div>
            <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.prompt') }}</dt>
            <dd class="mt-1 break-words text-gray-900 dark:text-gray-100">{{ selectedRecord.prompt || '-' }}</dd>
          </div>
          <div v-if="selectedRecord.revised_prompt">
            <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.revisedPrompt') }}</dt>
            <dd class="mt-1 break-words text-gray-900 dark:text-gray-100">{{ selectedRecord.revised_prompt }}</dd>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.source') }}</dt>
              <dd class="mt-1 text-gray-900 dark:text-gray-100">{{ selectedRecord.source }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.dimensions') }}</dt>
              <dd class="mt-1 text-gray-900 dark:text-gray-100">{{ dimensions(selectedRecord) }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.size') }}</dt>
              <dd class="mt-1 text-gray-900 dark:text-gray-100">{{ formatBytes(selectedRecord.size_bytes) }}</dd>
            </div>
            <div>
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('imageHistory.createdAt') }}</dt>
              <dd class="mt-1 text-gray-900 dark:text-gray-100">{{ formatDate(selectedRecord.created_at) }}</dd>
            </div>
          </div>
        </dl>
      </div>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="selectedRecord && downloadRecord(selectedRecord)">
            <Icon name="download" size="sm" />
            <span>{{ t('imageHistory.download') }}</span>
          </button>
          <button type="button" class="btn btn-danger" @click="selectedRecord && (pendingDelete = selectedRecord)">
            <Icon name="trash" size="sm" />
            <span>{{ t('imageHistory.delete') }}</span>
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="Boolean(pendingDelete)"
      :title="t('imageHistory.deleteTitle')"
      :message="t('imageHistory.deleteMessage')"
      :confirm-text="t('common.delete')"
      danger
      @cancel="pendingDelete = null"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { imageHistoryAPI, type ImageHistoryRecord, type ImageHistorySettings } from '@/api/imageHistory'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t, locale } = useI18n()
const appStore = useAppStore()

const settings = ref<ImageHistorySettings>({ available: false, enabled: false })
const records = ref<ImageHistoryRecord[]>([])
const loadingSettings = ref(true)
const loadingHistory = ref(false)
const savingSettings = ref(false)
const downloadingId = ref('')
const deletingId = ref('')
const selectedRecord = ref<ImageHistoryRecord | null>(null)
const pendingDelete = ref<ImageHistoryRecord | null>(null)
const previewErrors = ref<Record<string, boolean>>({})
const pagination = ref({ page: 1, pageSize: 12, total: 0, pages: 0 })

const loading = computed(() => loadingSettings.value || loadingHistory.value)
const settingsStatus = computed(() => {
  if (!settings.value.available) return t('imageHistory.unavailable')
  return settings.value.enabled ? t('imageHistory.enabled') : t('imageHistory.disabled')
})
const settingsStatusClass = computed(() => {
  if (!settings.value.available) return 'text-amber-600 dark:text-amber-400'
  return settings.value.enabled
    ? 'text-emerald-600 dark:text-emerald-400'
    : 'text-gray-500 dark:text-gray-400'
})

onMounted(() => {
  void refreshAll()
})

// 设置与列表分两步加载，部署未配置存储时不发起无效列表请求。
async function refreshAll() {
  loadingSettings.value = true
  try {
    settings.value = await imageHistoryAPI.getSettings()
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageHistory.loadError')))
    settings.value = { available: false, enabled: false }
  } finally {
    loadingSettings.value = false
  }
  if (settings.value.available) {
    await loadRecords()
  } else {
    records.value = []
    pagination.value.total = 0
  }
}

async function loadRecords() {
  loadingHistory.value = true
  try {
    const result = await imageHistoryAPI.list(pagination.value.page, pagination.value.pageSize)
    records.value = result.items || []
    pagination.value.total = result.total
    pagination.value.pages = result.pages
    previewErrors.value = {}
  } catch (error) {
    records.value = []
    appStore.showError(errorMessage(error, t('imageHistory.loadError')))
  } finally {
    loadingHistory.value = false
  }
}

async function updateSavingSetting(enabled: boolean) {
  if (!settings.value.available || savingSettings.value) return
  savingSettings.value = true
  try {
    settings.value = await imageHistoryAPI.updateSettings(enabled)
    appStore.showSuccess(t('imageHistory.settingsSaved'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageHistory.settingsError')))
  } finally {
    savingSettings.value = false
  }
}

function openPreview(record: ImageHistoryRecord) {
  selectedRecord.value = record
}

function markPreviewError(id: string) {
  previewErrors.value = { ...previewErrors.value, [id]: true }
}

async function downloadRecord(record: ImageHistoryRecord) {
  if (downloadingId.value) return
  downloadingId.value = record.id
  try {
    const blob = await imageHistoryAPI.download(record.id)
    saveAs(blob, `${record.id}.${fileExtension(record.mime_type)}`)
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageHistory.downloadError')))
  } finally {
    downloadingId.value = ''
  }
}

async function confirmDelete() {
  const record = pendingDelete.value
  if (!record || deletingId.value) return
  deletingId.value = record.id
  try {
    await imageHistoryAPI.delete(record.id)
    pendingDelete.value = null
    if (selectedRecord.value?.id === record.id) selectedRecord.value = null
    if (records.value.length === 1 && pagination.value.page > 1) pagination.value.page -= 1
    await loadRecords()
    appStore.showSuccess(t('imageHistory.deleteSuccess'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('imageHistory.deleteError')))
  } finally {
    deletingId.value = ''
  }
}

async function changePage(page: number) {
  pagination.value.page = page
  await loadRecords()
}

async function changePageSize(pageSize: number) {
  pagination.value.pageSize = pageSize
  pagination.value.page = 1
  await loadRecords()
}

function dimensions(record: ImageHistoryRecord): string {
  return record.width > 0 && record.height > 0 ? `${record.width} x ${record.height}` : '-'
}

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  const amount = value / 1024 ** index
  return `${amount >= 10 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`
}

function fileExtension(mimeType: string): string {
  if (mimeType === 'image/jpeg') return 'jpg'
  if (mimeType === 'image/webp') return 'webp'
  if (mimeType === 'image/gif') return 'gif'
  return 'png'
}

function errorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = String((error as { message?: unknown }).message || '').trim()
    if (message) return message
  }
  return fallback
}
</script>
