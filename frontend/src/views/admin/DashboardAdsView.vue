<template>
  <div class="mx-auto max-w-6xl space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">仪表盘广告</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">管理用户仪表盘展示的广告内容与有效期。</p>
      </div>
      <button class="btn btn-primary" :disabled="saving || loading || loadFailed" @click="save">保存广告</button>
    </div>

    <div v-if="loading" class="py-12 text-center text-gray-500">加载中…</div>
    <div v-else-if="loadFailed" class="card p-10 text-center text-gray-500">
      <p>广告加载失败，请重试。</p>
      <button class="btn btn-secondary mt-4" type="button" @click="load">重试</button>
    </div>
    <div v-else class="space-y-4">
      <div v-for="(ad, index) in ads" :key="ad.id" class="card p-5">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="font-medium text-gray-900 dark:text-white">广告 {{ index + 1 }}</h2>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary btn-sm" :disabled="index === 0" title="上移" @click="move(index, -1)">↑</button>
            <button class="btn btn-secondary btn-sm" :disabled="index === ads.length - 1" title="下移" @click="move(index, 1)">↓</button>
            <button class="btn btn-secondary btn-sm text-red-600" title="删除" @click="remove(index)">删除</button>
          </div>
        </div>
        <div class="grid gap-5 lg:grid-cols-[240px_1fr]">
          <div class="space-y-3">
            <ImageUpload v-model="ad.image_url" size="md" :max-size="5 * 1024 * 1024" upload-label="选择图片" remove-label="移除" hint="最大 5 MB，上传后自动压缩" />
            <label class="block"><span class="input-label">或使用图片地址</span><input v-model.trim="ad.image_url" class="input" type="url" placeholder="https://cdn.example.com/banner.jpg" /></label>
          </div>
          <div class="grid content-start gap-4 sm:grid-cols-2">
            <label class="sm:col-span-2"><span class="input-label">跳转链接</span><input v-model.trim="ad.link_url" class="input" type="url" placeholder="https://example.com" /></label>
            <label>
              <span class="input-label">图片适应方式</span>
              <Select v-model="ad.fit_mode" :options="fitModeOptions" aria-label="图片适应方式" />
            </label>
            <label><span class="input-label">开始时间</span><input v-model="ad.starts_at" class="input" type="datetime-local" /></label>
            <label><span class="input-label">过期时间</span><input v-model="ad.ends_at" class="input" type="datetime-local" /></label>
            <button type="button" class="flex items-center gap-3 sm:col-span-2" @click="ad.enabled = !ad.enabled">
              <span class="relative inline-flex h-6 w-11 rounded-full transition" :class="ad.enabled ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'"><span class="absolute top-1 h-4 w-4 rounded-full bg-white transition" :class="ad.enabled ? 'left-6' : 'left-1'"></span></span>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ ad.enabled ? '已启用' : '已停用' }}</span>
            </button>
          </div>
        </div>
      </div>
      <div v-if="!ads.length" class="card p-10 text-center text-gray-500">暂无广告，点击下方按钮添加。</div>
      <button class="btn btn-secondary w-full" @click="add">＋ 添加广告</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminAPI } from '@/api'
import ImageUpload from '@/components/common/ImageUpload.vue'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import { normalizeDashboardAdFitMode, type DashboardAd, type DashboardAdFitMode } from '@/types/dashboardAd'

const ads = ref<DashboardAd[]>([])
const loading = ref(true)
const saving = ref(false)
const loadFailed = ref(false)
const fitModeOptions: SelectOption[] = [
  { value: 'adaptive', label: '自适应（保持比例）' },
  { value: 'cover', label: '填充（裁剪超出部分）' },
  { value: 'fill', label: '拉伸（铺满区域）' },
]

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
