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
          @click.prevent="dismiss(ad)"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { DashboardAd } from '@/types/dashboardAd'
import { dashboardAdDismissKey, isDashboardAdActive, normalizeDashboardAdFitMode } from '@/types/dashboardAd'
import { sanitizeUrl } from '@/utils/url'

const props = defineProps<{ ads: DashboardAd[] }>()
const { t } = useI18n()
const dismissed = ref(new Set<string>())

const visibleAds = computed(() => props.ads
  .filter((ad) => {
    if (!isDashboardAdActive(ad)) return false
    return !dismissed.value.has(dashboardAdDismissKey(ad)) && localStorage.getItem(dashboardAdDismissKey(ad)) !== '1'
  })
  .map((ad) => ({
    ...ad,
    fit_mode: normalizeDashboardAdFitMode(ad.fit_mode),
    // 公开展示前统一过滤 URL，避免配置值被解释为脚本协议。
    image_url: sanitizeUrl(ad.image_url, { allowRelative: true, allowDataUrl: true }),
    link_url: sanitizeUrl(ad.link_url, { allowRelative: true }),
  }))
  .filter((ad) => ad.image_url))

function dismiss(ad: DashboardAd) {
  const key = dashboardAdDismissKey(ad)
  localStorage.setItem(key, '1')
  dismissed.value = new Set([...dismissed.value, key])
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
