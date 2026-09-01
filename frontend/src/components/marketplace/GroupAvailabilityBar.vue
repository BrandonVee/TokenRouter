<template>
  <div class="flex w-full min-w-0 items-center gap-2" :title="tooltip">
    <div
      v-if="isPassive"
      class="grid h-8 min-w-0 flex-1 items-center overflow-hidden"
      :style="passiveGridStyle"
      role="img"
      :aria-label="passiveAriaLabel"
    >
      <span
        v-for="(bucket, index) in passiveBuckets"
        :key="`${bucket.date || 'empty'}-${index}`"
        :class="[
          'h-6 max-w-full justify-self-center rounded-[2px]',
          passiveBucketClass(bucket),
        ]"
        :title="passiveBucketTitle(bucket)"
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
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  MarketplaceGroupAvailability,
  MarketplaceGroupAvailabilityDay,
} from '@/types'

const props = defineProps<{
  availability?: MarketplaceGroupAvailability | null
}>()

const { t } = useI18n()
const isPassive = computed(() => props.availability?.mode === 'passive')
const passiveBarCount = 60

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

const passiveBuckets = computed<MarketplaceGroupAvailabilityDay[]>(() => {
  const buckets = props.availability?.days ?? []
  return [
    ...Array.from({ length: Math.max(passiveBarCount - buckets.length, 0) }, () => ({
      date: '',
      success_count: 0,
      slow_stream_count: 0,
      total_count: 0,
      availability_rate: null,
    })),
    ...buckets.slice(-passiveBarCount),
  ]
})

const passiveGridStyle = computed(() => ({
  gap: '2px',
  gridTemplateColumns: `repeat(${passiveBarCount}, minmax(0, 1fr))`,
}))

const passiveBarWidth = computed(() => '100%')
const passiveAriaLabel = computed(() => t('marketplace.passiveAvailabilityAriaLabel', {
  minutes: bucketMinutes.value,
}))

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
      total: availability?.total_count ?? 0,
      minutes: bucketMinutes.value,
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

function passiveBucketStatus(bucket: MarketplaceGroupAvailabilityDay): 'success' | 'slow_stream' | 'upstream_error' | 'unknown' {
	if (bucket.total_count <= 0) return 'success'
	const upstreamErrors = Math.max(bucket.total_count - bucket.success_count, 0)
	const issueScore = (upstreamErrors + (bucket.slow_stream_count ?? 0) * 0.25) / bucket.total_count
	if (issueScore >= 0.6 && bucket.total_count >= 5 && upstreamErrors >= 3) return 'upstream_error'
	if (issueScore >= 0.25) return 'slow_stream'
	return 'success'
}

function passiveBucketClass(bucket: MarketplaceGroupAvailabilityDay): string {
	const status = passiveBucketStatus(bucket)
	// 市场页只表达“降级”提示，不把上游瞬时故障渲染成阻断性的红色错误。
	if (status === 'upstream_error') return 'bg-amber-400'
	if (status === 'slow_stream') return 'bg-amber-400'
	return 'bg-emerald-500'
}

function passiveBucketTitle(bucket: MarketplaceGroupAvailabilityDay): string {
  const status = passiveBucketStatus(bucket)
  return t(`marketplace.passiveAvailabilityStatus.${status}`, {}, status)
}

function bucketClass(rate?: number | null, totalCount?: number): string {
  if (!totalCount || typeof rate !== 'number') {
    return 'bg-emerald-500'
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
	return 'bg-amber-400'
}
</script>
