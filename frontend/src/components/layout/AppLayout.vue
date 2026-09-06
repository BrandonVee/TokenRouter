<template>
  <div class="ba-theme-shell min-h-screen">
    <!-- Background Decoration -->
    <div class="ba-theme-backdrop pointer-events-none fixed inset-0"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area；flex 列布局让内容区按剩余视口自适应，表格页不再依赖像素扣减高度。 -->
    <div
      class="relative z-10 flex min-h-screen flex-col transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-56']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="flex flex-1 flex-col p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour, startTeamTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
  onboardingStore.setTeamGuideCallback(startTeamTour)
})

defineExpose({ replayTour })
</script>
