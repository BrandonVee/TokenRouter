<template>
  <div class="flex w-full min-w-0 items-center gap-2" :title="tooltip">
    <div
      v-if="isPassive"
      class="grid h-8 min-w-0 flex-1 items-center overflow-hidden"
      :style="passiveGridStyle"
      role="img"
      :aria-label="t('marketplace.passiveAvailabilityAriaLabel')"
    >
      <span
        v-for="(request, index) in passiveRequestGroups"
        :key="`${request.created_at || 'empty'}-${index}`"
        :class="[
          'h-6 max-w-full justify-self-center rounded-[2px]',
          requestClass(request.status, request.success),
        ]"
        :title="requestTitle(request.status)"
        :style="{ width: passiveBarWidth }"
      />
    </div>
    <div
      v-else
      class="grid h-8 min-w-0 flex-1 items-center overflow-hidden"
      :style="barGridStyle"
      role="img"
      :aria-label="ariaLabel"
    >
      <span
        v-for="(bucket, index) in normalizedBuckets"
        :key="`${bucket.date || 'empty'}-${index}`"
        :class="[
          'h-6 max-w-full justify-self-center rounded-[2px]',
          bucketClass(bucket.availability_rate, bucket.total_count),
        ]"
        :style="{ width: bucketWidth }"
      />
    </div>
    <div class="w-[96px] shrink-0 text-left">
      <div class="text-base font-semibold leading-5 text-gray-900 dark:text-white">
        {{ rateLabel }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  MarketplaceGroupAvailability,
  MarketplaceGroupAvailabilityDay,
  MarketplaceGroupAvailabilityRequest,
} from '@/types'

const props = defineProps<{
  availability?: MarketplaceGroupAvailability | null
}>()

const { t } = useI18n()
const isPassive = computed(() => props.availability?.mode === 'passive')
const passiveRequestLimit = 300
const passiveRequestGroupSize = 5
const passiveBarCount = passiveRequestLimit / passiveRequestGroupSize

const windowDays = computed(() => Math.max(props.availability?.window_days ?? 7, 1))
const bucketMinutes = computed(() => Math.max(props.availability?.bucket_minutes ?? 24 * 60, 1))
const targetBucketCount = computed(() =>
  Math.max(Math.ceil((windowDays.value * 24 * 60) / bucketMinutes.value), 1),
)

const normalizedBuckets = computed<MarketplaceGroupAvailabilityDay[]>(() => {
  const buckets = props.availability?.days ?? []
  const target = targetBucketCount.value
  if (buckets.length >= target) {
    return buckets.slice(buckets.length - target)
  }
  return [
    ...Array.from({ length: target - buckets.length }, () => ({
      date: '',
      success_count: 0,
      total_count: 0,
      availability_rate: null,
    })),
    ...buckets,
  ]
})

const normalizedRequests = computed<MarketplaceGroupAvailabilityRequest[]>(() => {
  const requests = props.availability?.requests ?? []
  return [
    ...Array.from({ length: Math.max(passiveRequestLimit - requests.length, 0) }, () => ({
      status: 'unknown' as const,
      success: false,
      created_at: '',
    })),
    ...requests.slice(-passiveRequestLimit),
  ]
})

// 每根柱子汇总连续 5 次有效请求，孤立故障降级为黄色，同组至少 2 次明确上游故障才标红。
const passiveRequestGroups = computed<MarketplaceGroupAvailabilityRequest[]>(() => {
  const groups: MarketplaceGroupAvailabilityRequest[] = []
  for (let start = 0; start < normalizedRequests.value.length; start += passiveRequestGroupSize) {
    const requests = normalizedRequests.value.slice(start, start + passiveRequestGroupSize)
    const actualRequests = requests.filter((request) => Boolean(request.created_at))
    const upstreamErrors = actualRequests.filter((request) => request.status === 'upstream_error').length
    const hasPressure = actualRequests.some((request) => request.status === 'pressure')
    const allSuccess = actualRequests.length > 0 && actualRequests.every(
      (request) => request.success || request.status === 'success',
    )

    let status = 'unknown'
    if (upstreamErrors >= 2) {
      status = 'upstream_error'
    } else if (upstreamErrors > 0 || hasPressure) {
      status = 'pressure'
    } else if (allSuccess) {
      status = 'success'
    }
    groups.push({
      status,
      success: status === 'success',
      created_at: actualRequests.at(-1)?.created_at ?? '',
    })
  }
  return groups
})

const passiveGridStyle = computed(() => ({
  gap: '2px',
  gridTemplateColumns: `repeat(${passiveBarCount}, minmax(0, 1fr))`,
}))

const passiveBarWidth = computed(() => '100%')

const barGridStyle = computed(() => ({
  gap:
    normalizedBuckets.value.length > 360
      ? '0'
      : normalizedBuckets.value.length > 180
        ? '1px'
        : '2px',
  gridTemplateColumns: `repeat(${normalizedBuckets.value.length}, minmax(0, 1fr))`,
}))

const bucketWidth = computed(() => {
  const count = normalizedBuckets.value.length
  if (count <= 30) {
    return '8px'
  }
  if (count <= 90) {
    return '5px'
  }
  if (count <= 180) {
    return '4px'
  }
  return '100%'
})

const rateLabel = computed(() => {
  const rate = props.availability?.availability_rate
  if (typeof rate !== 'number') {
    return t('marketplace.availabilityNoData')
  }
  return `${(rate * 100).toFixed(2)}%`
})

const tooltip = computed(() => {
  const availability = props.availability
  if (isPassive.value) {
    return t('marketplace.passiveAvailabilityHint', {
      rate: rateLabel.value,
      success: availability?.success_count ?? 0,
      pressure: availability?.pressure_count ?? 0,
      total: availability?.total_count ?? 0,
    })
  }
  if (!availability || typeof availability.availability_rate !== 'number') {
    return t('marketplace.availabilityHintNoData', {
      days: windowDays.value,
    })
  }
  return t('marketplace.availabilityHint', {
    days: windowDays.value,
    rate: rateLabel.value,
    success: availability.success_count,
    total: availability.total_count,
  })
})

const ariaLabel = computed(
  () => `${t('marketplace.availabilityWindow', { days: windowDays.value })}: ${rateLabel.value}`,
)

function requestClass(status: string, success: boolean): string {
  if (success || status === 'success') return 'bg-emerald-500'
  if (status === 'pressure') return 'bg-amber-400'
  if (status === 'upstream_error') return 'bg-rose-500'
  return 'bg-gray-200 dark:bg-dark-700'
}

function requestTitle(status: string): string {
  return t(`marketplace.passiveAvailabilityStatus.${status}`, {}, status)
}

function bucketClass(rate?: number | null, totalCount?: number): string {
  if (!totalCount || typeof rate !== 'number') {
    return 'bg-gray-200 dark:bg-dark-700'
  }
  if (rate >= 0.995) {
    return 'bg-emerald-500'
  }
  if (rate >= 0.98) {
    return 'bg-lime-500'
  }
  if (rate >= 0.9) {
    return 'bg-amber-400'
  }
  return 'bg-rose-400'
}
</script>
