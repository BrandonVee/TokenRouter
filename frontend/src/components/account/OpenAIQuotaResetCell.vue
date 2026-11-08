<template>
  <div v-if="visible" class="space-y-1">
    <!-- 查询行共用父组件的本地查询插槽，保留只读次数查询，并展示积分、邀请与自动用卡状态。 -->
    <div class="flex flex-wrap items-center gap-1.5">
      <slot name="pre-actions" />
      <slot />

      <button
        type="button"
        class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-blue-600 transition-colors hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-blue-400 dark:hover:bg-blue-900/30"
        :disabled="loading"
        :title="countButtonTitle"
        @click="handleQuery()"
      >
        <svg
          class="h-2.5 w-2.5"
          :class="{ 'animate-spin': loading }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        {{ t('admin.accounts.openaiQuotaReset.count') }}<span v-if="data"> {{ availableResetCount }}</span>
      </button>



      <button
        type="button"
        data-testid="codex-credits"
        class="inline-flex max-w-full items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-900/30"
        :disabled="loading"
        :title="creditsButtonTitle"
        @click="handleQuery()"
      >
        {{ t('admin.accounts.openaiQuotaReset.points') }}
        <span class="truncate tabular-nums">{{ creditsDisplay }}</span>
      </button>
      <OpenAIReferralCell :account="account" />
    </div>

    <div v-if="creditsCacheWarning" class="text-[10px] text-amber-600 dark:text-amber-400">
      {{ t('admin.accounts.openaiQuotaReset.pointsCachePersistFailed') }}
    </div>

    <div
      v-if="autoResetState"
      class="flex flex-wrap items-center gap-1 text-[10px]"
      data-testid="auto-reset-credit-state"
    >
      <span
        class="inline-flex items-center rounded px-1.5 py-0.5 font-medium"
        :class="autoResetStateClass"
      >
        {{ autoResetStateLabel }}
        <span v-if="autoResetState.trigger_window" class="ml-1 tabular-nums">
          {{ autoResetState.trigger_window }}
        </span>
      </span>
      <span v-if="autoResetState.checked_at" class="text-gray-500 dark:text-gray-400">
        {{ formatResetCreditExpiry(autoResetState.checked_at, 'short') }}
      </span>
      <span
        v-if="autoResetState.error_code"
        class="max-w-full truncate text-red-600 dark:text-red-400"
        :title="autoResetState.error_code"
      >
        {{ autoResetState.error_code }}
      </span>
    </div>

    <div v-if="primaryResetCreditExpiry" class="space-y-1">
      <div class="flex flex-wrap items-center gap-1">
        <span
          class="inline-flex max-w-full items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] leading-4 text-gray-600 tabular-nums dark:bg-dark-800 dark:text-gray-300"
          :title="t('admin.accounts.openaiQuotaReset.expiresAtFull', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'full') })"
        >
          {{ t('admin.accounts.openaiQuotaReset.expiresAt', { time: formatResetCreditExpiry(primaryResetCreditExpiry, 'short') }) }}
        </span>
        <button
          v-if="hiddenResetCreditCount > 0"
          type="button"
          data-testid="reset-credit-expiry-toggle"
          class="inline-flex items-center rounded-full bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium leading-4 text-gray-600 transition-colors hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
          :aria-expanded="showResetCreditDetails"
          :aria-label="resetCreditDetailsToggleLabel"
          :title="resetCreditDetailsTitle"
          @click="toggleResetCreditDetails"
        >
          +{{ hiddenResetCreditCount }}
        </button>
      </div>

      <div
        v-if="showResetCreditDetails && resetCreditExpirations.length > 1"
        data-testid="reset-credit-expiry-details"
        class="inline-grid max-w-full gap-0.5 rounded border border-gray-200 bg-white px-1.5 py-1 text-[10px] leading-4 text-gray-600 shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
      >
        <span class="sr-only">{{ t('admin.accounts.openaiQuotaReset.expirationDetails') }}</span>
        <span
          v-for="(expiresAt, index) in resetCreditExpirations"
          :key="`${expiresAt}-${index}`"
          class="flex min-w-0 items-center gap-1 tabular-nums"
          :title="t('admin.accounts.openaiQuotaReset.expiresAtFull', { time: formatResetCreditExpiry(expiresAt, 'full') })"
        >
          <span class="h-1 w-1 shrink-0 rounded-full bg-gray-400 dark:bg-dark-500" />
          <span class="truncate">{{ formatResetCreditExpiry(expiresAt, 'short') }}</span>
        </span>
      </div>
    </div>

    <!-- 查询结果与错误提示 -->
    <div
      v-if="error"
      class="text-[10px] text-red-600 dark:text-red-400"
      :title="error"
    >
      {{ truncatedError }}
    </div>




  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import {
  refreshOpenAIQuota,
  type OpenAIQuotaUsage
} from '@/api/admin/accounts'
import OpenAIReferralCell from '@/components/account/OpenAIReferralCell.vue'

const props = defineProps<{
  account: Account
}>()

const { t } = useI18n()

// 仅对 OpenAI OAuth 账号展示。
const visible = computed(() => props.account.platform === 'openai' && props.account.type === 'oauth')

const loading = ref(false)
const error = ref<string | null>(null)
const data = ref<OpenAIQuotaUsage | null>(null)
const cachedData = ref<OpenAIQuotaUsage | null>(null)
const showResetCreditDetails = ref(false)
const creditsCacheWarning = ref(false)

const readCachedCredits = (account: Account) => {
  const snapshot = account.extra?.codex_credits_snapshot
  const credits = snapshot?.credits
  if (!credits || typeof credits.has_credits !== 'boolean' || typeof credits.unlimited !== 'boolean') return null
  if (credits.balance != null && typeof credits.balance !== 'string') return null
  return { credits, fetched_at: snapshot.fetched_at }
}
const creditsData = ref(readCachedCredits(props.account))
const creditsDisplay = computed(() => {
  const credits = creditsData.value?.credits
  if (!credits) return '—'
  if (credits.unlimited) return t('admin.accounts.openaiQuotaReset.pointsUnlimited')
  if (!credits.has_credits) return '0'
  const balance = credits.balance?.trim()
  // 保留上游十进制字符串及小数积分的精度。
  if (balance && Number.isFinite(Number(balance)) && Number(balance) >= 0) return balance
  return t('admin.accounts.openaiQuotaReset.pointsAvailable')
})
const creditsButtonTitle = computed(() => {
  const fetchedAt = creditsData.value?.fetched_at
  const refresh = t('admin.accounts.openaiQuotaReset.pointsTooltip')
  if (!fetchedAt || !Number.isFinite(fetchedAt)) return refresh
  return `${refresh}\n${t('admin.accounts.openaiQuotaReset.pointsUpdatedAt', {
    time: new Date(fetchedAt * 1000).toLocaleString()
  })}`
})

const updateCredits = (usage: OpenAIQuotaUsage | null) => {
  creditsData.value = usage?.credits ? { credits: usage.credits, fetched_at: usage.fetched_at } : null
}

type AutoResetCreditState = NonNullable<NonNullable<Account['extra']>['codex_auto_reset_credit_state']>
const validAutoResetStatuses = new Set(['checking', 'available', 'resetting', 'success', 'no_credit', 'failed'])
const autoResetState = computed<AutoResetCreditState | null>(() => {
  if (props.account.extra?.auto_reset_credit_enabled !== true) return null
  const state = props.account.extra?.codex_auto_reset_credit_state
  if (!state || typeof state !== 'object' || !validAutoResetStatuses.has(String(state.status))) return null
  return state
})
const autoResetStateLabel = computed(() => {
  if (!autoResetState.value?.status) return ''
  const keyByStatus: Record<string, string> = {
    checking: 'checking',
    available: 'available',
    resetting: 'resetting',
    success: 'success',
    no_credit: 'noCredit',
    failed: 'failed'
  }
  return t(`admin.accounts.openaiQuotaReset.autoStatus.${keyByStatus[autoResetState.value.status]}`)
})
const autoResetStateClass = computed(() => {
  switch (autoResetState.value?.status) {
    case 'available':
      return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    case 'success':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'no_credit':
    case 'failed':
      return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'resetting':
      return 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300'
  }
})

// 从持久快照恢复次数时剔除已过期机会；全部过期视为未知，避免展示已经失效的次数。
const readCachedResetCredits = (account: Account): OpenAIQuotaUsage | null => {
  const cached = account.extra?.codex_reset_credit_snapshot
  if (!cached || typeof cached !== 'object' || Array.isArray(cached)) return null

  const { available_count: count, credits: rawCredits } = cached as {
    available_count?: unknown
    credits?: unknown
  }
  if (typeof count !== 'number' || !Number.isFinite(count)) return null

  const now = Date.now()
  const credits: { expires_at?: string }[] = []
  if (Array.isArray(rawCredits)) {
    for (const credit of rawCredits) {
      if (!credit || typeof credit !== 'object') continue
      const expiresAt = (credit as { expires_at?: unknown }).expires_at
      if (typeof expiresAt !== 'string' || expiresAt.trim() === '') continue
      const expiryTime = new Date(expiresAt).getTime()
      // 无法解析的时间保持原样展示，避免静默少计次数。
      if (!Number.isNaN(expiryTime) && expiryTime <= now) continue
      credits.push({ expires_at: expiresAt })
    }
  }
  const availableCount = Math.min(Math.max(count, 0), credits.length)
  // 原快照已没有有效机会时恢复未知状态，引导重新查询。
  if (count > 0 && availableCount <= 0) return null
  return {
    fetched_at: 0,
    rate_limit_reset_credits: {
      available_count: availableCount,
      credits
    }
  }
}

cachedData.value = readCachedResetCredits(props.account)
data.value = cachedData.value

const availableResetCount = computed(() => data.value?.rate_limit_reset_credits?.available_count ?? 0)
// 优先实时结果，仅在未知时使用持久快照，避免次数与到期时间来自不同版本。
const resetCreditExpirations = computed(() =>
  ((data.value ?? cachedData.value)?.rate_limit_reset_credits?.credits ?? [])
    .map((credit) => credit.expires_at?.trim() ?? '')
    .filter((expiresAt) => expiresAt.length > 0)
    .sort(compareResetCreditExpiry)
)
const primaryResetCreditExpiry = computed(() => resetCreditExpirations.value[0] ?? '')
const hiddenResetCreditCount = computed(() => Math.max(resetCreditExpirations.value.length - 1, 0))

const resetCreditDetailsTitle = computed(() =>
  resetCreditExpirations.value
    .map((expiresAt) => formatResetCreditExpiry(expiresAt, 'full'))
    .join('\n')
)

const resetCreditDetailsToggleLabel = computed(() => {
  if (showResetCreditDetails.value) {
    return t('admin.accounts.openaiQuotaReset.collapseExpirations')
  }
  return t('admin.accounts.openaiQuotaReset.expandExpirations', { count: hiddenResetCreditCount.value })
})

// 次数按钮同时触发查询和展示数量，按是否已有数据切换提示。
const countButtonTitle = computed(() => {
  if (!data.value) return t('admin.accounts.openaiQuotaReset.countTooltipLoad')
  return t('admin.accounts.openaiQuotaReset.countTooltipRefresh')
})

const truncatedError = computed(() => {
  if (!error.value) return ''
  return error.value.length > 80 ? `${error.value.slice(0, 80)}…` : error.value
})

const getResetCreditExpiryTime = (value: string): number => {
  const time = new Date(value).getTime()
  return Number.isNaN(time) ? Number.POSITIVE_INFINITY : time
}

const compareResetCreditExpiry = (a: string, b: string): number => {
  const diff = getResetCreditExpiryTime(a) - getResetCreditExpiryTime(b)
  if (diff !== 0) return diff
  return a.localeCompare(b)
}

const formatResetCreditExpiry = (value: string, style: 'short' | 'full'): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const options: Intl.DateTimeFormatOptions = {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }
  if (style === 'full') {
    options.year = 'numeric'
  }

  return new Intl.DateTimeFormat(undefined, options).format(date)
}

const extractErrorMessage = (e: unknown): string => {
  // 兼容项目拦截器展开后的错误、原始网络错误和通用本地化提示。
  const err = e as {
    message?: string
    reason?: string
    response?: { data?: { message?: string; error?: string } }
  }
  return (
    err?.message ||
    err?.reason ||
    err?.response?.data?.message ||
    err?.response?.data?.error ||
    t('common.error')
  )
}

const toggleResetCreditDetails = () => {
  if (hiddenResetCreditCount.value <= 0) return
  showResetCreditDetails.value = !showResetCreditDetails.value
}

const handleQuery = async () => {
  if (loading.value) return
  const accountID = props.account.id
  loading.value = true
  creditsCacheWarning.value = false
  error.value = null
  showResetCreditDetails.value = false
  try {
    const result = await refreshOpenAIQuota(accountID)
    if (props.account.id !== accountID) return
    updateCredits(result)
    creditsCacheWarning.value = result.credits_cache_persisted === false
    // 快照保存失败仍采用实时次数，保留原持久视图并单独提示错误。
    data.value = result
    if (result.cache_persisted) {
      cachedData.value = result
    } else {
      error.value=t('admin.accounts.openaiQuotaReset.refreshCachePersistFailed')
    }
  } catch (e) {
    if (props.account.id !== accountID) return
    error.value = extractErrorMessage(e)
  } finally {
    if (props.account.id === accountID) loading.value = false
  }
}

watch(
  () => props.account.id,
  () => {
    // 分页可能复用账号行，切换账号时清除局部状态。
    cachedData.value = readCachedResetCredits(props.account)
    data.value = cachedData.value
    creditsData.value = readCachedCredits(props.account)
    creditsCacheWarning.value = false
    error.value = null
    loading.value = false
    showResetCreditDetails.value = false
  }
)

watch(
  resetCreditExpirations,
  () => {
    if (hiddenResetCreditCount.value <= 0) {
      showResetCreditDetails.value = false
    }
  }
)
</script>
