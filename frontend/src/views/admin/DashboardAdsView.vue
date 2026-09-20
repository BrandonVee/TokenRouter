<template>
  <div class="mx-auto max-w-[1600px] space-y-6 pb-8">
    <header
      class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800"
    >
      <div class="relative px-5 py-5 sm:px-6 lg:px-8">
        <div class="pointer-events-none absolute inset-y-0 right-0 hidden w-80 lg:block">
          <div class="dashboard-ads-header-grid absolute inset-0 opacity-60"></div>
          <div
            class="absolute -right-8 -top-20 h-56 w-56 rounded-full bg-primary-100/70 blur-3xl dark:bg-primary-900/20"
          ></div>
        </div>

        <div class="relative flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
          <div class="max-w-2xl">
            <div class="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">
              <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-primary-50 dark:bg-primary-900/30">
                <Icon name="sparkles" size="sm" />
              </span>
              {{ t('admin.dashboardAds.workspace') }}
            </div>
            <h1 class="text-2xl font-semibold tracking-tight text-gray-950 dark:text-white sm:text-3xl">
              {{ t('admin.dashboardAds.title') }}
            </h1>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">
              {{ t('admin.dashboardAds.description') }}
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading || loadFailed"
              @click="add"
            >
              <Icon name="plus" size="sm" class="mr-1.5" :stroke-width="2" />
              {{ t('admin.dashboardAds.add') }}
            </button>
            <button
              type="button"
              class="btn btn-primary min-w-28"
              :disabled="saving || loading || loadFailed"
              @click="save"
            >
              <Icon
                v-if="saving"
                name="refresh"
                size="sm"
                class="mr-1.5 animate-spin"
                :stroke-width="2"
              />
              {{ saving ? t('common.saving') : t('admin.dashboardAds.save') }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="!loading && !loadFailed"
        class="grid grid-cols-3 divide-x divide-gray-100 border-t border-gray-100 bg-gray-50/70 dark:divide-dark-700 dark:border-dark-700 dark:bg-dark-900/30"
      >
        <div class="px-4 py-3 sm:px-6">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.summary.total') }}</p>
          <p class="mt-0.5 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ ads.length }}</p>
        </div>
        <div class="px-4 py-3 sm:px-6">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.summary.enabled') }}</p>
          <p class="mt-0.5 text-lg font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">{{ enabledCount }}</p>
        </div>
        <div class="px-4 py-3 sm:px-6">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.summary.scheduled') }}</p>
          <p class="mt-0.5 text-lg font-semibold tabular-nums text-amber-600 dark:text-amber-400">{{ scheduledCount }}</p>
        </div>
      </div>
    </header>

    <div v-if="loading" class="card flex min-h-72 items-center justify-center">
      <div class="text-center text-gray-500 dark:text-gray-400">
        <Icon name="refresh" size="lg" class="mx-auto animate-spin text-primary-500" />
        <p class="mt-3 text-sm">{{ t('common.loading') }}</p>
      </div>
    </div>

    <div v-else-if="loadFailed" class="card flex min-h-72 items-center justify-center p-8 text-center">
      <div class="max-w-sm">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-red-50 text-red-500 dark:bg-red-900/20 dark:text-red-400">
          <Icon name="exclamationTriangle" size="lg" />
        </div>
        <p class="mt-4 font-medium text-gray-900 dark:text-white">{{ t('admin.dashboardAds.loadFailed') }}</p>
        <button class="btn btn-secondary mt-4" type="button" @click="load">
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ t('admin.dashboardAds.retry') }}
        </button>
      </div>
    </div>

    <div v-else-if="ads.length" class="space-y-5">
      <article
        v-for="(ad, index) in ads"
        :key="ad.id"
        class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm transition-shadow hover:shadow-md dark:border-dark-700 dark:bg-dark-800"
        :data-testid="`dashboard-ad-card-${index}`"
      >
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
          <div class="flex min-w-0 items-center gap-3">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 text-sm font-semibold tabular-nums text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              {{ String(index + 1).padStart(2, '0') }}
            </span>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboardAds.adIndex', { index: index + 1 }) }}
                </h2>
                <span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(ad)">
                  {{ statusLabel(ad) }}
                </span>
              </div>
              <p class="mt-0.5 truncate text-xs text-gray-400 dark:text-gray-500">{{ ad.id }}</p>
            </div>
          </div>

          <div class="flex items-center gap-1">
            <button
              type="button"
              class="dashboard-ad-icon-button"
              :disabled="index === 0"
              :aria-label="t('admin.dashboardAds.moveUp')"
              :title="t('admin.dashboardAds.moveUp')"
              @click="move(index, -1)"
            >
              <Icon name="arrowUp" size="sm" />
            </button>
            <button
              type="button"
              class="dashboard-ad-icon-button"
              :disabled="index === ads.length - 1"
              :aria-label="t('admin.dashboardAds.moveDown')"
              :title="t('admin.dashboardAds.moveDown')"
              @click="move(index, 1)"
            >
              <Icon name="arrowDown" size="sm" />
            </button>
            <span class="mx-1 h-5 w-px bg-gray-200 dark:bg-dark-600"></span>
            <button
              type="button"
              class="dashboard-ad-icon-button text-red-500 hover:bg-red-50 hover:text-red-600 dark:text-red-400 dark:hover:bg-red-900/20"
              :aria-label="t('common.delete')"
              :title="t('common.delete')"
              @click="remove(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>

        <div data-testid="dashboard-ad-editor-grid" class="grid xl:grid-cols-[minmax(0,2fr)_23rem] 2xl:grid-cols-[minmax(0,2.4fr)_24rem]">
          <section class="border-b border-gray-100 p-5 dark:border-dark-700 sm:p-6 xl:border-b-0 xl:border-r">
            <div class="mb-3 flex items-center justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.dashboardAds.preview') }}</h3>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.previewHint') }}</p>
              </div>
              <span class="hidden items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-300 sm:inline-flex">
                <Icon name="grid" size="xs" />
                {{ fitModeLabel(ad.fit_mode) }}
              </span>
            </div>

            <div class="overflow-hidden rounded-xl border border-gray-200 bg-gray-100 shadow-inner dark:border-dark-600 dark:bg-dark-900">
              <div class="flex h-8 items-center gap-1.5 border-b border-gray-200 bg-white/90 px-3 dark:border-dark-600 dark:bg-dark-800/90">
                <span class="h-2 w-2 rounded-full bg-red-300 dark:bg-red-500/70"></span>
                <span class="h-2 w-2 rounded-full bg-amber-300 dark:bg-amber-500/70"></span>
                <span class="h-2 w-2 rounded-full bg-emerald-300 dark:bg-emerald-500/70"></span>
                <span class="ml-2 h-2 flex-1 rounded-full bg-gray-100 dark:bg-dark-700"></span>
              </div>
              <div class="dashboard-ad-preview" :class="previewFrameClass(ad.fit_mode)">
                <img
                  v-if="ad.image_url"
                  :src="ad.image_url"
                  alt=""
                  class="dashboard-ad-preview-image"
                  :class="previewImageClass(ad.fit_mode)"
                />
                <div v-else class="flex h-full min-h-44 flex-col items-center justify-center px-6 text-center text-gray-400 dark:text-gray-500">
                  <Icon name="sparkles" size="xl" :stroke-width="1.2" />
                  <p class="mt-2 text-sm font-medium">{{ t('admin.dashboardAds.previewEmpty') }}</p>
                  <p class="mt-1 text-xs">{{ t('admin.dashboardAds.previewEmptyHint') }}</p>
                </div>
              </div>
            </div>

            <div class="mt-5 grid gap-4 lg:grid-cols-[minmax(16rem,0.85fr)_minmax(0,1.15fr)]">
              <div class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-600 dark:bg-dark-900/30">
                <p class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
                  {{ t('admin.dashboardAds.uploadSection') }}
                </p>
                <ImageUpload
                  v-model="ad.image_url"
                  size="md"
                  :max-size="5 * 1024 * 1024"
                  :upload-label="t('admin.dashboardAds.imageUploadLabel')"
                  :remove-label="t('admin.dashboardAds.imageRemoveLabel')"
                  :hint="t('admin.dashboardAds.imageHint')"
                />
              </div>
              <label class="block rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-600 dark:bg-dark-900/30">
                <span class="input-label">{{ t('admin.dashboardAds.orUseImageUrl') }}</span>
                <input
                  v-model.trim="ad.image_url"
                  class="input mt-1"
                  type="url"
                  :placeholder="t('admin.dashboardAds.imageUrlPlaceholder')"
                />
                <span class="mt-2 block text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboardAds.imageUrlHint') }}
                </span>
              </label>
            </div>
          </section>

          <section class="flex flex-col p-5 sm:p-6">
            <div class="flex-1 space-y-5">
              <div>
                <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.dashboardAds.delivery') }}</h3>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.deliveryHint') }}</p>
              </div>

              <button
                type="button"
                class="flex w-full items-center justify-between gap-4 rounded-xl border border-gray-200 bg-gray-50/70 px-4 py-3 text-left transition hover:border-gray-300 dark:border-dark-600 dark:bg-dark-900/30 dark:hover:border-dark-500"
                :aria-pressed="ad.enabled"
                @click="ad.enabled = !ad.enabled"
              >
                <span>
                  <span class="block text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('admin.dashboardAds.visibility') }}</span>
                  <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                    {{ ad.enabled ? t('admin.dashboardAds.visibilityEnabled') : t('admin.dashboardAds.visibilityDisabled') }}
                  </span>
                </span>
                <span
                  class="relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors"
                  :class="ad.enabled ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'"
                  aria-hidden="true"
                >
                  <span
                    class="absolute top-1 h-4 w-4 rounded-full bg-white shadow-sm transition-all"
                    :class="ad.enabled ? 'left-6' : 'left-1'"
                  ></span>
                </span>
              </button>

              <label class="block">
                <span class="input-label">{{ t('admin.dashboardAds.linkUrl') }}</span>
                <div class="relative mt-1">
                  <Icon name="link" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                  <input
                    v-model.trim="ad.link_url"
                    class="input pl-9"
                    type="url"
                    :placeholder="t('admin.dashboardAds.linkUrlPlaceholder')"
                  />
                </div>
              </label>

              <label class="block">
                <span class="input-label">{{ t('admin.dashboardAds.fitMode') }}</span>
                <Select
                  v-model="ad.fit_mode"
                  class="mt-1"
                  :options="fitModeOptions"
                  :aria-label="t('admin.dashboardAds.fitMode')"
                />
              </label>

              <div>
                <p class="input-label">{{ t('admin.dashboardAds.schedule') }}</p>
                <div class="mt-1 grid gap-3 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                  <label class="block">
                    <span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.startsAt') }}</span>
                    <DateTimePicker v-model="ad.starts_at" show-time />
                  </label>
                  <label class="block">
                    <span class="mb-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.endsAt') }}</span>
                    <DateTimePicker v-model="ad.ends_at" show-time :min="ad.starts_at" />
                  </label>
                </div>
                <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.scheduleHint') }}</p>
              </div>
            </div>
          </section>
        </div>
      </article>

      <button
        type="button"
        class="group flex w-full items-center justify-center gap-2 rounded-2xl border border-dashed border-gray-300 bg-white/60 px-5 py-5 text-sm font-medium text-gray-600 transition hover:border-primary-400 hover:bg-primary-50/50 hover:text-primary-700 dark:border-dark-600 dark:bg-dark-800/50 dark:text-gray-300 dark:hover:border-primary-600 dark:hover:bg-primary-900/10 dark:hover:text-primary-400"
        @click="add"
      >
        <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 transition group-hover:bg-primary-100 dark:bg-dark-700 dark:group-hover:bg-primary-900/30">
          <Icon name="plus" size="sm" :stroke-width="2" />
        </span>
        {{ t('admin.dashboardAds.addAnother') }}
      </button>
    </div>

    <div v-else class="card flex min-h-80 items-center justify-center p-8 text-center">
      <div class="max-w-md">
        <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-primary-50 text-primary-500 dark:bg-primary-900/20 dark:text-primary-400">
          <Icon name="sparkles" size="xl" :stroke-width="1.3" />
        </div>
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.dashboardAds.emptyTitle') }}</h2>
        <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.empty') }}</p>
        <button type="button" class="btn btn-primary mt-5" @click="add">
          <Icon name="plus" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.dashboardAds.add') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import DateTimePicker from '@/components/common/DateTimePicker.vue'
import ImageUpload from '@/components/common/ImageUpload.vue'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import { normalizeDashboardAdFitMode, type DashboardAd, type DashboardAdFitMode } from '@/types/dashboardAd'

const { t } = useI18n()

const ads = ref<DashboardAd[]>([])
const loading = ref(true)
const saving = ref(false)
const loadFailed = ref(false)

const enabledCount = computed(() => ads.value.filter((ad) => ad.enabled).length)
const scheduledCount = computed(() => {
  const now = Date.now()
  return ads.value.filter((ad) => ad.enabled && ad.starts_at && new Date(ad.starts_at).getTime() > now).length
})

// 适应方式选项跟随语言切换重新计算，避免切换语言后仍是旧语言标签。
const fitModeOptions = computed<SelectOption[]>(() => [
  { value: 'adaptive', label: t('admin.dashboardAds.fitModes.adaptive') },
  { value: 'cover', label: t('admin.dashboardAds.fitModes.cover') },
  { value: 'fill', label: t('admin.dashboardAds.fitModes.fill') },
])

function fitModeLabel(mode: DashboardAd['fit_mode']) {
  const normalized = normalizeDashboardAdFitMode(mode)
  return t(`admin.dashboardAds.fitModes.${normalized}`)
}

function statusLabel(ad: DashboardAd) {
  if (!ad.enabled) return t('admin.dashboardAds.status.disabled')
  const now = Date.now()
  const startsAt = ad.starts_at ? new Date(ad.starts_at).getTime() : 0
  const endsAt = ad.ends_at ? new Date(ad.ends_at).getTime() : 0
  if (startsAt > now) return t('admin.dashboardAds.status.scheduled')
  if (endsAt > 0 && endsAt <= now) return t('admin.dashboardAds.status.expired')
  return t('admin.dashboardAds.status.active')
}

function statusClass(ad: DashboardAd) {
  if (!ad.enabled) return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  const now = Date.now()
  const startsAt = ad.starts_at ? new Date(ad.starts_at).getTime() : 0
  const endsAt = ad.ends_at ? new Date(ad.ends_at).getTime() : 0
  if (startsAt > now) return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
  if (endsAt > 0 && endsAt <= now) return 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400'
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
}

// 管理端预览复用用户仪表盘的图片适应语义，但限制高度以保持编辑卡片紧凑。
function previewFrameClass(mode: DashboardAd['fit_mode']) {
  return normalizeDashboardAdFitMode(mode) === 'adaptive'
    ? 'dashboard-ad-preview--adaptive'
    : 'dashboard-ad-preview--fixed'
}

function previewImageClass(mode: DashboardAd['fit_mode']) {
  return `dashboard-ad-preview-image--${normalizeDashboardAdFitMode(mode)}`
}

function localDateTimeNow() {
  const date = new Date()
  const offset = date.getTimezoneOffset()
  return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16)
}

function add() {
  ads.value.push({
    id: crypto.randomUUID(),
    image_url: '',
    link_url: '',
    fit_mode: 'adaptive',
    starts_at: localDateTimeNow(),
    ends_at: null,
    enabled: true,
  })
}

function remove(index: number) {
  ads.value.splice(index, 1)
}

function move(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= ads.value.length) return
  const [item] = ads.value.splice(index, 1)
  ads.value.splice(target, 0, item)
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const storedAds = await adminAPI.settings.getDashboardAds()
    ads.value = (storedAds || []).map((ad) => ({
      ...ad,
      fit_mode: normalizeDashboardAdFitMode(ad.fit_mode),
      starts_at: ad.starts_at ? ad.starts_at.slice(0, 16) : null,
      ends_at: ad.ends_at ? ad.ends_at.slice(0, 16) : null,
    }))
  } catch {
    // 加载失败时保留空编辑态不可保存，避免误清空数据库中的广告。
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const saved = await adminAPI.settings.updateDashboardAds(ads.value.map((ad) => ({
      ...ad,
      fit_mode: normalizeDashboardAdFitMode(ad.fit_mode) as DashboardAdFitMode,
      starts_at: ad.starts_at ? new Date(ad.starts_at).toISOString() : null,
      ends_at: ad.ends_at ? new Date(ad.ends_at).toISOString() : null,
    })))
    ads.value = saved.map((ad) => ({
      ...ad,
      fit_mode: normalizeDashboardAdFitMode(ad.fit_mode),
      starts_at: ad.starts_at ? ad.starts_at.slice(0, 16) : null,
      ends_at: ad.ends_at ? ad.ends_at.slice(0, 16) : null,
    }))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.dashboard-ads-header-grid {
  background-image:
    linear-gradient(to right, rgb(229 231 235 / 0.7) 1px, transparent 1px),
    linear-gradient(to bottom, rgb(229 231 235 / 0.7) 1px, transparent 1px);
  background-size: 24px 24px;
  mask-image: linear-gradient(to left, black, transparent);
}

:global(.dark) .dashboard-ads-header-grid {
  background-image:
    linear-gradient(to right, rgb(55 65 81 / 0.55) 1px, transparent 1px),
    linear-gradient(to bottom, rgb(55 65 81 / 0.55) 1px, transparent 1px);
}

.dashboard-ad-icon-button {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  color: rgb(107 114 128);
  transition: color 150ms ease, background-color 150ms ease;
}

.dashboard-ad-icon-button:hover:not(:disabled) {
  background: rgb(243 244 246);
  color: rgb(31 41 55);
}

.dashboard-ad-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.3;
}

:global(.dark) .dashboard-ad-icon-button:hover:not(:disabled) {
  background: rgb(55 65 81);
  color: rgb(229 231 235);
}

.dashboard-ad-preview {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.dashboard-ad-preview--fixed {
  aspect-ratio: 4 / 1;
  min-height: 10rem;
  max-height: 18rem;
}

.dashboard-ad-preview--adaptive {
  min-height: 10rem;
  max-height: 18rem;
}

.dashboard-ad-preview-image {
  display: block;
  width: 100%;
}

.dashboard-ad-preview-image--adaptive {
  height: auto;
  max-height: 18rem;
  object-fit: contain;
}

.dashboard-ad-preview-image--cover,
.dashboard-ad-preview-image--fill {
  height: 100%;
}

.dashboard-ad-preview-image--cover {
  object-fit: cover;
}

.dashboard-ad-preview-image--fill {
  object-fit: fill;
}
</style>
