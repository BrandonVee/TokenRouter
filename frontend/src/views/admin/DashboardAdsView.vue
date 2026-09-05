<template>
  <div class="mx-auto max-w-6xl space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.dashboardAds.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.dashboardAds.description') }}</p>
      </div>
      <button class="btn btn-primary" :disabled="saving || loading || loadFailed" @click="save">{{ t('admin.dashboardAds.save') }}</button>
    </div>

    <div v-if="loading" class="py-12 text-center text-gray-500">{{ t('common.loading') }}</div>
    <div v-else-if="loadFailed" class="card p-10 text-center text-gray-500">
      <p>{{ t('admin.dashboardAds.loadFailed') }}</p>
      <button class="btn btn-secondary mt-4" type="button" @click="load">{{ t('admin.dashboardAds.retry') }}</button>
    </div>
    <div v-else class="space-y-4">
      <div v-for="(ad, index) in ads" :key="ad.id" class="card p-5">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.dashboardAds.adIndex', { index: index + 1 }) }}</h2>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="index === 0" :aria-label="t('admin.dashboardAds.moveUp')" :title="t('admin.dashboardAds.moveUp')" @click="move(index, -1)">↑</button>
            <button class="btn btn-secondary btn-sm" :disabled="index === ads.length - 1" :aria-label="t('admin.dashboardAds.moveDown')" :title="t('admin.dashboardAds.moveDown')" @click="move(index, 1)">↓</button>
            <button class="btn btn-secondary btn-sm text-red-600" @click="remove(index)">{{ t('common.delete') }}</button>
          </div>
        </div>
        <div class="grid gap-5 lg:grid-cols-[240px_1fr]">
          <div class="space-y-3">
            <ImageUpload
              v-model="ad.image_url"
              size="md"
              :max-size="5 * 1024 * 1024"
              :upload-label="t('admin.dashboardAds.imageUploadLabel')"
              :remove-label="t('admin.dashboardAds.imageRemoveLabel')"
              :hint="t('admin.dashboardAds.imageHint')"
            />
            <label class="block"><span class="input-label">{{ t('admin.dashboardAds.orUseImageUrl') }}</span><input v-model.trim="ad.image_url" class="input" type="url" :placeholder="t('admin.dashboardAds.imageUrlPlaceholder')" /></label>
          </div>
          <div class="grid content-start gap-4 sm:grid-cols-2">
            <label class="sm:col-span-2"><span class="input-label">{{ t('admin.dashboardAds.linkUrl') }}</span><input v-model.trim="ad.link_url" class="input" type="url" :placeholder="t('admin.dashboardAds.linkUrlPlaceholder')" /></label>
            <label>
              <span class="input-label">{{ t('admin.dashboardAds.fitMode') }}</span>
              <Select v-model="ad.fit_mode" :options="fitModeOptions" :aria-label="t('admin.dashboardAds.fitMode')" />
            </label>
            <label><span class="input-label">{{ t('admin.dashboardAds.startsAt') }}</span><input v-model="ad.starts_at" class="input" type="datetime-local" /></label>
            <label><span class="input-label">{{ t('admin.dashboardAds.endsAt') }}</span><input v-model="ad.ends_at" class="input" type="datetime-local" /></label>
            <button type="button" class="flex items-center gap-3 sm:col-span-2" @click="ad.enabled = !ad.enabled">
              <span class="relative inline-flex h-6 w-11 rounded-full transition" :class="ad.enabled ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'"><span class="absolute top-1 h-4 w-4 rounded-full bg-white transition" :class="ad.enabled ? 'left-6' : 'left-1'"></span></span>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ ad.enabled ? t('common.enabled') : t('common.disabled') }}</span>
            </button>
          </div>
        </div>
      </div>
      <div v-if="!ads.length" class="card p-10 text-center text-gray-500">{{ t('admin.dashboardAds.empty') }}</div>
      <button class="btn btn-secondary w-full" @click="add">＋ {{ t('admin.dashboardAds.add') }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import ImageUpload from '@/components/common/ImageUpload.vue'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import { normalizeDashboardAdFitMode, type DashboardAd, type DashboardAdFitMode } from '@/types/dashboardAd'

const { t } = useI18n()

const ads = ref<DashboardAd[]>([])
const loading = ref(true)
const saving = ref(false)
const loadFailed = ref(false)
// 适应方式选项跟随语言切换重新计算，避免切换语言后仍是旧语言标签。
const fitModeOptions = computed<SelectOption[]>(() => [
  { value: 'adaptive', label: t('admin.dashboardAds.fitModes.adaptive') },
  { value: 'cover', label: t('admin.dashboardAds.fitModes.cover') },
  { value: 'fill', label: t('admin.dashboardAds.fitModes.fill') },
])

function localDateTimeNow() {
  const date = new Date()
  const offset = date.getTimezoneOffset()
  return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16)
}
function add() {
  ads.value.push({ id: crypto.randomUUID(), image_url: '', link_url: '', fit_mode: 'adaptive', starts_at: localDateTimeNow(), ends_at: null, enabled: true })
}
function remove(index: number) { ads.value.splice(index, 1) }
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
  } finally { loading.value = false }
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
  } finally { saving.value = false }
}
onMounted(load)
</script>
