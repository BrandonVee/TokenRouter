<template>
  <section v-if="visibleAds.length" class="dashboard-panel overflow-hidden">
    <div class="divide-y divide-white/20">
      <div v-for="ad in visibleAds" :key="ad.id" class="relative">
        <a
          :href="ad.link_url || undefined"
          :target="ad.link_url ? '_blank' : undefined"
          :rel="ad.link_url ? 'noopener noreferrer' : undefined"
          class="block"
        >
          <div class="dashboard-ad-frame" :class="frameClass(ad.fit_mode)">
            <img :src="ad.image_url" alt="" :class="imageClass(ad.fit_mode)" />
          </div>
        </a>
        <button
          type="button"
          class="absolute right-2 top-2 rounded-md bg-black/55 p-1.5 text-white transition hover:bg-black/75"
          :aria-label="t('common.close')"
          @click.prevent="requestDismiss(ad)"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>
  </section>

  <BaseDialog
    :show="Boolean(pendingAd)"
    :title="t('dashboard.ads.dismissTitle')"
    width="narrow"
    @close="cancelDismiss"
  >
    <p class="text-sm text-gray-600 dark:text-gray-400">
      {{ t('dashboard.ads.dismissMessage') }}
    </p>

    <template #footer>
      <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button
          type="button"
          class="rounded-md border border-primary-900/10 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:border-black/20 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-black/10 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:border-dark-600 dark:hover:bg-dark-600 dark:focus:ring-primary-500 dark:focus:ring-offset-dark-800"
          data-testid="dashboard-ad-dismiss-cancel"
          @click="cancelDismiss"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
          data-testid="dashboard-ad-dismiss-today"
          @click="dismissForToday"
        >
          {{ t('dashboard.ads.dismissToday') }}
        </button>
        <button
          type="button"
          class="rounded-md bg-gray-700 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 dark:bg-gray-600 dark:hover:bg-gray-500 dark:focus:ring-offset-dark-800"
          data-testid="dashboard-ad-dismiss-permanent"
          @click="dismissPermanently"
        >
          {{ t('dashboard.ads.dismissPermanently') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { DashboardAd } from '@/types/dashboardAd'
import {
  dashboardAdDismissKey,
  dashboardAdDismissPermanentKey,
  dashboardAdDismissTodayKey,
  isDashboardAdActive,
  normalizeDashboardAdFitMode
} from '@/types/dashboardAd'
import { sanitizeUrl } from '@/utils/url'

const props = defineProps<{ ads: DashboardAd[] }>()
const { t } = useI18n()
const dismissed = ref(new Set<string>())
const pendingAd = ref<DashboardAd | null>(null)

const visibleAds = computed(() => props.ads
  .filter((ad) => {
    if (!isDashboardAdActive(ad)) return false
    return !isDismissed(ad)
  })
  .map((ad) => ({
    ...ad,
    fit_mode: normalizeDashboardAdFitMode(ad.fit_mode),
    // 公开展示前统一过滤 URL，避免配置值被解释为脚本协议。
    image_url: sanitizeUrl(ad.image_url, { allowRelative: true, allowDataUrl: true }),
    link_url: sanitizeUrl(ad.link_url, { allowRelative: true }),
  }))
  .filter((ad) => ad.image_url))

function isDismissed(ad: DashboardAd) {
  const keys = [
    // 读取旧版周期键，避免升级后恢复已经关闭的广告。
    dashboardAdDismissKey(ad),
    dashboardAdDismissTodayKey(ad),
    dashboardAdDismissPermanentKey(ad)
  ]
  return keys.some((key) => dismissed.value.has(key) || localStorage.getItem(key) === '1')
}

function requestDismiss(ad: DashboardAd) {
  pendingAd.value = ad
}

function cancelDismiss() {
  pendingAd.value = null
}

function saveDismissal(key: string) {
  localStorage.setItem(key, '1')
  dismissed.value = new Set([...dismissed.value, key])
  pendingAd.value = null
}

function dismissForToday() {
  if (!pendingAd.value) return
  saveDismissal(dashboardAdDismissTodayKey(pendingAd.value))
}

function dismissPermanently() {
  if (!pendingAd.value) return
  saveDismissal(dashboardAdDismissPermanentKey(pendingAd.value))
}

// 填充和拉伸需要固定展示区域，自适应则使用图片自身高度。
function frameClass(mode: DashboardAd['fit_mode']) {
  return normalizeDashboardAdFitMode(mode) === 'adaptive' ? 'dashboard-ad-frame--adaptive' : 'dashboard-ad-frame--fixed'
}

// 根据选择的模式映射到对应的 object-fit 样式。
function imageClass(mode: DashboardAd['fit_mode']) {
  const fitMode = normalizeDashboardAdFitMode(mode)
  if (fitMode === 'cover') return 'dashboard-ad-image dashboard-ad-image--cover'
  if (fitMode === 'fill') return 'dashboard-ad-image dashboard-ad-image--fill'
  return 'dashboard-ad-image dashboard-ad-image--adaptive'
}
</script>

<style scoped>
.dashboard-ad-frame {
  width: 100%;
  overflow: hidden;
  background: rgb(243 244 246 / 0.7);
}

.dashboard-ad-frame--fixed {
  height: clamp(10rem, 26vw, 13rem);
}

.dashboard-ad-frame--adaptive {
  max-height: 13rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.dashboard-ad-image {
  display: block;
  width: 100%;
}

.dashboard-ad-image--adaptive {
  height: auto;
  max-height: 13rem;
  object-fit: contain;
}

.dashboard-ad-image--cover,
.dashboard-ad-image--fill {
  height: 100%;
}

.dashboard-ad-image--cover {
  object-fit: cover;
}

.dashboard-ad-image--fill {
  object-fit: fill;
}
</style>
