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
          <img :src="ad.image_url" alt="" class="max-h-52 w-full object-cover" />
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
import { dashboardAdDismissKey, isDashboardAdActive } from '@/types/dashboardAd'
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
</script>
