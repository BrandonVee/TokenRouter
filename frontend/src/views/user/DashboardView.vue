<template>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardAds :ads="dashboardAds" />
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" />
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="onDateRangeChange" @granularityChange="loadCharts" @refresh="refreshAll" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardAnnouncements /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
        </div>
      </template>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores/announcements'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardAnnouncements from '@/components/user/dashboard/UserDashboardAnnouncements.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserDashboardAds from '@/components/user/dashboard/UserDashboardAds.vue'
import type { DashboardAd } from '@/types/dashboardAd'
import type { ModelStat, TrendDataPoint } from '@/types'
import { formatDateLocalInput } from '@/utils/format'
import { useAppStore } from '@/stores/app'

const authStore = useAuthStore()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
// 广告由公开设置注入，未配置时保持空数组。
const dashboardAds = computed<DashboardAd[]>(() => appStore.cachedPublicSettings?.dashboard_ads || [])

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref('day')

// 短时间范围使用小时粒度，避免趋势图只剩一个数据点。
const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const parsePoint = (value: string) => new Date(value.length === 10 ? `${value}T00:00:00` : value).getTime()
  const startTime = parsePoint(start)
  const endTime = parsePoint(end)
  if (!Number.isFinite(startTime) || !Number.isFinite(endTime)) return 'day'
  return Math.ceil((endTime - startTime) / 86400000) <= 1 ? 'hour' : 'day'
}
const onDateRangeChange = (range: { startDate: string; endDate: string }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  loadCharts()
}

const loadStats = async () => {
  loading.value = true
  try {
    const [, nextStats] = await Promise.all([
      authStore.refreshUser(),
      usageAPI.getDashboardStats(),
    ])
    stats.value = nextStats
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const res = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value as any,
      }),
      usageAPI.getDashboardModels({
        start_date: startDate.value,
        end_date: endDate.value,
      }),
    ])
    trendData.value = res[0].trend || []
    modelStats.value = res[1].models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}

// App 负责首次预加载；用户主动刷新时同时绕过公告节流获取最新内容。
const refreshAll = () => {
  void loadStats()
  void loadCharts()
  void announcementStore.fetchAnnouncements(true)
}

onMounted(() => {
  // 仪表盘进入时确保获取最新广告配置，避免登录后缓存尚未初始化。
  void appStore.fetchPublicSettings(true)
  void loadStats()
  void loadCharts()
})
</script>
