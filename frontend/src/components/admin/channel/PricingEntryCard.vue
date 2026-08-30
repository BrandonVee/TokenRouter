<template>
  <div class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800">
    <!-- Collapsed summary header (clickable) -->
    <div
      class="flex cursor-pointer select-none items-center gap-2"
      @click="collapsed = !collapsed"
    >
      <Icon
        :name="collapsed ? 'chevronRight' : 'chevronDown'"
        size="sm"
        :stroke-width="2"
        class="flex-shrink-0 text-gray-400 transition-transform duration-200"
      />

      <!-- Summary: model tags + billing badge -->
      <div v-if="collapsed" class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
        <!-- Compact model tags (show first 3) -->
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
          <span
            v-for="(m, i) in entry.models.slice(0, 3)"
            :key="i"
            class="inline-flex shrink-0 rounded px-1.5 py-0.5 text-xs"
            :class="getPlatformTagClass(props.platform || '')"
          >
            {{ m }}
          </span>
          <span
            v-if="entry.models.length > 3"
            class="whitespace-nowrap text-xs text-gray-400"
          >
            +{{ entry.models.length - 3 }}
          </span>
          <span
            v-if="entry.models.length === 0"
            class="text-xs italic text-gray-400"
          >
            {{ t('admin.channels.form.noModels', '未添加模型') }}
          </span>
        </div>

        <!-- Billing mode badge -->
        <span
          class="flex-shrink-0 rounded-full bg-primary-100 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
        >
          {{ billingModeLabel }}
        </span>
        <span
          v-if="entry.price_multiplier !== null && entry.price_multiplier !== undefined && entry.price_multiplier !== ''"
          class="flex-shrink-0 rounded bg-gray-200 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300"
        >
          {{ entry.price_multiplier }}x
        </span>
        <span
          v-if="props.showFastModeMultiplier && entry.billing_mode === 'token' && entry.fast_mode_multiplier !== null && entry.fast_mode_multiplier !== undefined && entry.fast_mode_multiplier !== ''"
          class="flex-shrink-0 rounded bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
        >
          Fast {{ entry.fast_mode_multiplier }}x
        </span>
      </div>

      <!-- Expanded: show the label "Pricing Entry" or similar -->
      <div v-else class="flex-1 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.pricingEntry', '定价配置') }}
      </div>

      <!-- Remove button (always visible, stop propagation) -->
      <button
        type="button"
        @click.stop="emit('remove')"
        class="flex-shrink-0 rounded p-1 text-gray-400 hover:text-red-500"
      >
        <Icon name="trash" size="sm" />
      </button>
    </div>

    <!-- Expandable content with transition -->
    <div
      class="collapsible-content"
      :class="{ 'collapsible-content--collapsed': collapsed }"
    >
      <div class="collapsible-inner">
        <!-- Header: Models + Billing Mode -->
        <div
          class="mt-3 grid grid-cols-1 items-start gap-2"
          :class="props.showFastModeMultiplier && entry.billing_mode === 'token'
            ? 'sm:grid-cols-[minmax(0,1fr)_10rem_8rem_8rem]'
            : 'sm:grid-cols-[minmax(0,1fr)_10rem_8rem]'"
        >
          <div>
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.channels.form.models', '模型列表') }} <span class="text-red-500">*</span>
            </label>
            <ModelTagInput
              :models="entry.models"
              :platform="props.platform"
              @update:models="onModelsUpdate($event)"
              :placeholder="t('admin.channels.form.modelsPlaceholder', '输入模型名后按回车添加，支持通配符 *')"
              class="mt-1"
            />
          </div>
          <div>
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.channels.form.billingMode', '计费模式') }}
            </label>
            <Select
              :modelValue="entry.billing_mode"
              @update:modelValue="onBillingModeUpdate($event as BillingMode)"
              :options="billingModeOptions"
              class="mt-1"
            />
          </div>
          <div>
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.channels.form.priceMultiplier', '定价倍率') }}
            </label>
            <input
              :value="entry.price_multiplier"
              @input="emitField('price_multiplier', ($event.target as HTMLInputElement).value)"
              type="number"
              step="any"
              min="0"
              class="input mt-1 text-sm"
              :placeholder="t('admin.channels.form.priceMultiplierPlaceholder', '不调整')"
            />
          </div>
          <div v-if="props.showFastModeMultiplier && entry.billing_mode === 'token'">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.channels.form.fastModeMultiplier', 'Fast 模式倍率') }}
            </label>
            <input
              :value="entry.fast_mode_multiplier"
              @input="emitField('fast_mode_multiplier', ($event.target as HTMLInputElement).value)"
              type="number"
              step="any"
              min="0"
              class="input mt-1 text-sm"
              data-testid="fast-mode-multiplier"
              :placeholder="t('admin.channels.form.fastModeMultiplierPlaceholder', '例如 2')"
            />
          </div>
        </div>

        <!-- Token mode -->
        <div v-if="entry.billing_mode === 'token'">
          <!-- Default prices (fallback when no interval matches) -->
          <label class="mt-3 block text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.channels.form.defaultPrices', '默认价格（未命中区间时使用）') }}
            <span class="ml-1 font-normal text-gray-400">$/MTok</span>
          </label>
          <div class="mt-1 grid grid-cols-2 gap-2 sm:grid-cols-6">
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.inputPrice', '输入') }}</label>
              <input :value="entry.input_price" @input="emitField('input_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.outputPrice', '输出') }}</label>
              <input :value="entry.output_price" @input="emitField('output_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.cacheWritePrice', '缓存写入') }}</label>
              <input :value="entry.cache_write_price" @input="emitField('cache_write_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.cacheReadPrice', '缓存读取') }}</label>
              <input :value="entry.cache_read_price" @input="emitField('cache_read_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.imageInputPrice', '图片输入') }}</label>
              <input :value="entry.image_input_price" @input="emitField('image_input_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.imageTokenPrice', '图片输出') }}</label>
              <input :value="entry.image_output_price" @input="emitField('image_output_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
            </div>
          </div>

          <div v-if="enableTierMultipliers" class="mt-3 grid max-w-md grid-cols-2 gap-2">
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.fastMultiplier', 'Fast 倍率') }}</label>
              <input :value="entry.fast_multiplier" @input="emitField('fast_multiplier', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0.000001" class="input mt-0.5 text-sm"
                :placeholder="t('admin.channels.form.multiplierPlaceholder', '未配置')" />
            </div>
            <div>
              <label class="text-xs text-gray-400">{{ t('admin.channels.form.flexMultiplier', 'Flex 倍率') }}</label>
              <input :value="entry.flex_multiplier" @input="emitField('flex_multiplier', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0.000001" class="input mt-0.5 text-sm"
                :placeholder="t('admin.channels.form.multiplierPlaceholder', '未配置')" />
            </div>
          </div>

          <!-- 单模型峰谷价格，仅对 token 计费生效。 -->
          <div v-if="showPeakRate" class="mt-3 rounded border border-amber-200 bg-amber-50/60 p-2 dark:border-amber-800 dark:bg-amber-900/10">
            <div class="flex items-center justify-between gap-2">
              <button
                type="button"
                @click="togglePeakRate"
                data-testid="peak-rate-toggle"
                class="inline-flex items-center gap-1.5 rounded border px-2 py-1 text-xs font-medium transition-colors"
                :class="entry.peak_rate_enabled
                  ? 'border-amber-500 bg-amber-100 text-amber-800 dark:border-amber-500 dark:bg-amber-900/40 dark:text-amber-200'
                  : 'border-gray-300 bg-white text-gray-600 hover:border-amber-400 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300'"
                :aria-pressed="!!entry.peak_rate_enabled"
              >
                <Icon :name="entry.peak_rate_enabled ? 'check' : 'clock'" size="xs" />
                {{ t('admin.channels.form.peakRateEnabled', '启用该模型峰谷价格') }}
              </button>
              <button
                v-if="entry.peak_rate_enabled"
                type="button"
                @click="addPeakRateWindow"
                class="inline-flex items-center gap-1 text-xs font-medium text-primary-600 hover:text-primary-700"
              >
                <Icon name="plus" size="xs" />
                {{ t('admin.channels.form.addPeakRateWindow', '添加价格时段') }}
              </button>
            </div>
            <div v-if="entry.peak_rate_enabled" class="mt-2 space-y-2">
              <div
                v-for="(window, windowIndex) in entry.peak_rate_windows || []"
                :key="windowIndex"
                class="rounded border border-amber-200/80 bg-white/70 p-2 dark:border-amber-900/70 dark:bg-dark-800/50"
              >
                <div class="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(13rem,1fr)_7rem_7rem_7rem_auto] sm:items-end">
                  <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channels.form.peakRateWeekdays', '适用日期') }}</label>
                    <div class="mt-0.5 flex flex-wrap gap-1">
                      <button
                        v-for="day in peakRateWeekdays"
                        :key="day.value"
                        type="button"
                        @click="togglePeakRateWeekday(windowIndex, day.value)"
                        class="min-w-7 rounded border px-1.5 py-1 text-xs transition-colors"
                        :class="window.weekdays.includes(day.value)
                          ? 'border-primary-500 bg-primary-100 text-primary-700 dark:border-primary-500 dark:bg-primary-900/40 dark:text-primary-200'
                          : 'border-gray-200 bg-white text-gray-500 hover:border-gray-400 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-400'"
                        :aria-pressed="window.weekdays.includes(day.value)"
                      >
                        {{ day.label }}
                      </button>
                    </div>
                  </div>
                  <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channels.form.peakStart', '高峰开始') }}</label>
                    <input :value="window.start" @input="updatePeakRateWindow(windowIndex, 'start', ($event.target as HTMLInputElement).value)"
                      type="time" step="60" class="input mt-0.5 text-sm" :data-testid="`peak-start-${windowIndex}`" />
                  </div>
                  <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channels.form.peakEnd', '高峰结束') }}</label>
                    <input :value="window.end" @input="updatePeakRateWindow(windowIndex, 'end', ($event.target as HTMLInputElement).value)"
                      type="time" step="60" class="input mt-0.5 text-sm" :data-testid="`peak-end-${windowIndex}`" />
                  </div>
                  <div>
                    <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channels.form.peakRateMultiplier', '高峰价格倍率') }}</label>
                    <input :value="window.multiplier" @input="updatePeakRateWindow(windowIndex, 'multiplier', ($event.target as HTMLInputElement).value)"
                      type="number" step="any" min="0" class="input mt-0.5 text-sm" :data-testid="`peak-rate-multiplier-${windowIndex}`" placeholder="1" />
                  </div>
                  <button
                    type="button"
                    @click="removePeakRateWindow(windowIndex)"
                    class="mb-0.5 rounded p-1 text-gray-400 hover:text-red-500 disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="(entry.peak_rate_windows || []).length <= 1"
                    :title="t('admin.channels.form.removePeakRateWindow', '删除价格时段')"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>
            </div>
            <p v-if="entry.peak_rate_enabled" class="mt-1 text-xs text-gray-400">{{ t('admin.channels.form.peakRateHint', '按服务器时区计算；可添加多个时段，并支持跨天') }}</p>
          </div>

          <!-- token 区间仅用于渠道；分组长上下文价格使用内置模型规则。 -->
          <div v-if="!hideTokenIntervals" class="mt-3">
            <div class="flex items-center justify-between">
              <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.channels.form.intervals', '上下文区间定价（可选）') }}
                <span class="ml-1 font-normal text-gray-400">(min, max]</span>
              </label>
              <button type="button" @click="addInterval" class="text-xs text-primary-600 hover:text-primary-700">
                + {{ t('admin.channels.form.addInterval', '添加区间') }}
              </button>
            </div>
            <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
              <IntervalRow
                v-for="(iv, idx) in entry.intervals"
                :key="idx"
                :interval="iv"
                :mode="entry.billing_mode"
                :enable-multipliers="enableTierMultipliers"
                @update="updateInterval(idx, $event)"
                @remove="removeInterval(idx)"
              />
            </div>
          </div>
        </div>

        <!-- Per-request mode -->
        <div v-else-if="entry.billing_mode === 'per_request'">
          <!-- Default per-request price -->
          <label class="mt-3 block text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.channels.form.defaultPerRequestPrice', '默认单次价格（未命中层级时使用）') }}
            <span class="ml-1 font-normal text-gray-400">$</span>
          </label>
          <div class="mt-1 w-48">
            <input :value="entry.per_request_price" @input="emitField('per_request_price', ($event.target as HTMLInputElement).value)"
              type="number" step="any" min="0" class="input text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
          </div>

          <!-- Tiers -->
          <div class="mt-3 flex items-center justify-between">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.channels.form.requestTiers', '按次计费层级') }}
            </label>
            <button type="button" @click="addInterval" class="text-xs text-primary-600 hover:text-primary-700">
              + {{ t('admin.channels.form.addTier', '添加层级') }}
            </button>
          </div>
          <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
            <IntervalRow
              v-for="(iv, idx) in entry.intervals"
              :key="idx"
              :interval="iv"
              :mode="entry.billing_mode"
              @update="updateInterval(idx, $event)"
              @remove="removeInterval(idx)"
            />
          </div>
          <div v-else class="mt-2 rounded border border-dashed border-gray-300 p-3 text-center text-xs text-gray-400 dark:border-dark-500">
            {{ t('admin.channels.form.noTiersYet', '暂无层级，点击添加配置按次计费价格') }}
          </div>
        </div>

        <!-- 图片或视频计费模式。 -->
        <div v-else-if="entry.billing_mode === 'image' || entry.billing_mode === 'video'">
          <!-- Default image price (per-request, same as per_request mode) -->
          <label class="mt-3 block text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ entry.billing_mode === 'video'
              ? t('admin.channels.form.defaultVideoPrice', '默认视频价格（未命中层级时使用）')
              : t('admin.channels.form.defaultImagePrice', '默认图片价格（未命中层级时使用）') }}
            <span class="ml-1 font-normal text-gray-400">$</span>
          </label>
          <div class="mt-1 w-48">
            <input :value="entry.per_request_price" @input="emitField('per_request_price', ($event.target as HTMLInputElement).value)"
              type="number" step="any" min="0" class="input text-sm" :placeholder="t('admin.channels.form.pricePlaceholder', '默认')" />
          </div>

          <!-- Image tiers -->
          <div class="mt-3 flex items-center justify-between">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ entry.billing_mode === 'video'
                ? t('admin.channels.form.videoTiers', '视频计费层级')
                : t('admin.channels.form.imageTiers', '图片计费层级（按次）') }}
            </label>
            <button type="button" @click="addMediaTier" class="text-xs text-primary-600 hover:text-primary-700">
              + {{ t('admin.channels.form.addTier', '添加层级') }}
            </button>
          </div>
          <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
            <IntervalRow
              v-for="(iv, idx) in entry.intervals"
              :key="idx"
              :interval="iv"
              :mode="entry.billing_mode"
              @update="updateInterval(idx, $event)"
              @remove="removeInterval(idx)"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import IntervalRow from './IntervalRow.vue'
import ModelTagInput from './ModelTagInput.vue'
import type { PricingFormEntry, IntervalFormEntry, PeakRateWindowFormEntry } from './types'
import { perTokenToMTok, getPlatformTagClass } from './types'
import type { BillingMode } from '@/api/admin/channels'
import channelsAPI from '@/api/admin/channels'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  entry: PricingFormEntry
  platform?: string
  showFastModeMultiplier?: boolean
  enableTierMultipliers?: boolean
  hideTokenIntervals?: boolean
  showPeakRate?: boolean
}>(), {
  showFastModeMultiplier: false,
  enableTierMultipliers: false,
  hideTokenIntervals: false,
  showPeakRate: true,
})

const emit = defineEmits<{
  update: [entry: PricingFormEntry]
  remove: []
}>()

// Collapse state: entries with existing models default to collapsed
const collapsed = ref(props.entry.models.length > 0)

const billingModeOptions = computed(() => [
  { value: 'token', label: 'Token' },
  { value: 'per_request', label: t('admin.channels.billingMode.perRequest', '按次') },
  { value: 'image', label: t('admin.channels.billingMode.image', '图片（按次）') },
  { value: 'video', label: t('admin.channels.billingMode.video', '视频') }
])

const peakRateWeekdays = computed(() => [
  { value: 0, label: t('admin.channels.form.weekdayMon', '一') },
  { value: 1, label: t('admin.channels.form.weekdayTue', '二') },
  { value: 2, label: t('admin.channels.form.weekdayWed', '三') },
  { value: 3, label: t('admin.channels.form.weekdayThu', '四') },
  { value: 4, label: t('admin.channels.form.weekdayFri', '五') },
  { value: 5, label: t('admin.channels.form.weekdaySat', '六') },
  { value: 6, label: t('admin.channels.form.weekdaySun', '日') },
])

const billingModeLabel = computed(() => {
  const opt = billingModeOptions.value.find(o => o.value === props.entry.billing_mode)
  return opt ? opt.label : props.entry.billing_mode
})

function emitField(field: keyof PricingFormEntry, value: string) {
  emit('update', { ...props.entry, [field]: value === '' ? null : value })
}

function defaultPeakRateWindow(): PeakRateWindowFormEntry {
  return { weekdays: [0, 1, 2, 3, 4, 5, 6], start: '09:00', end: '18:00', multiplier: 1 }
}

function togglePeakRate() {
  const enabled = !props.entry.peak_rate_enabled
  const windows = props.entry.peak_rate_windows || []
  emit('update', {
    ...props.entry,
    peak_rate_enabled: enabled,
    peak_rate_windows: enabled && windows.length === 0 ? [defaultPeakRateWindow()] : windows,
  })
}

function addPeakRateWindow() {
  emit('update', {
    ...props.entry,
    peak_rate_windows: [...(props.entry.peak_rate_windows || []), defaultPeakRateWindow()],
  })
}

function updatePeakRateWindow(
  index: number,
  field: keyof Omit<PeakRateWindowFormEntry, 'weekdays'>,
  value: string,
) {
  const windows = [...(props.entry.peak_rate_windows || [])]
  windows[index] = { ...windows[index], [field]: value === '' ? null : value }
  emit('update', { ...props.entry, peak_rate_windows: windows })
}

function togglePeakRateWeekday(index: number, weekday: number) {
  const windows = [...(props.entry.peak_rate_windows || [])]
  const weekdays = [...windows[index].weekdays]
  const weekdayIndex = weekdays.indexOf(weekday)
  if (weekdayIndex === -1) {
    weekdays.push(weekday)
    weekdays.sort((a, b) => a - b)
  } else {
    weekdays.splice(weekdayIndex, 1)
  }
  windows[index] = { ...windows[index], weekdays }
  emit('update', { ...props.entry, peak_rate_windows: windows })
}

function removePeakRateWindow(index: number) {
  const windows = [...(props.entry.peak_rate_windows || [])]
  if (windows.length <= 1) return
  windows.splice(index, 1)
  emit('update', { ...props.entry, peak_rate_windows: windows })
}

// Fast 倍率只适用于 token 计费，切换模式时清除隐藏字段，避免提交无效配置。
function onBillingModeUpdate(billingMode: BillingMode) {
  emit('update', {
    ...props.entry,
    billing_mode: billingMode,
    fast_mode_multiplier: billingMode === 'token' ? props.entry.fast_mode_multiplier : null,
    fast_multiplier: billingMode === 'token' ? props.entry.fast_multiplier : null,
    flex_multiplier: billingMode === 'token' ? props.entry.flex_multiplier : null,
    peak_rate_enabled: billingMode === 'token' ? props.entry.peak_rate_enabled : false,
    peak_start: billingMode === 'token' ? props.entry.peak_start : '',
    peak_end: billingMode === 'token' ? props.entry.peak_end : '',
    peak_rate_multiplier: billingMode === 'token' ? props.entry.peak_rate_multiplier : 1,
    peak_rate_windows: billingMode === 'token' ? props.entry.peak_rate_windows : [],
    intervals: [],
  })
}

function addInterval() {
  const intervals = [...(props.entry.intervals || [])]
  intervals.push({
    min_tokens: 0, max_tokens: null, tier_label: '',
    input_price: null, output_price: null, cache_write_price: null,
    cache_read_price: null, per_request_price: null,
    input_multiplier: null, output_multiplier: null,
    cache_write_multiplier: null, cache_read_multiplier: null,
    sort_order: intervals.length
  })
  emit('update', { ...props.entry, intervals })
}

function addMediaTier() {
  const intervals = [...(props.entry.intervals || [])]
  const labels = props.entry.billing_mode === 'video'
    ? ['480p', '720p', '1080p']
    : ['1K', '2K', '4K', 'HD']
  intervals.push({
    min_tokens: 0, max_tokens: null, tier_label: labels[intervals.length] || '',
    input_price: null, output_price: null, cache_write_price: null,
    cache_read_price: null, per_request_price: null,
    input_multiplier: null, output_multiplier: null,
    cache_write_multiplier: null, cache_read_multiplier: null,
    sort_order: intervals.length
  })
  emit('update', { ...props.entry, intervals })
}

function updateInterval(idx: number, updated: IntervalFormEntry) {
  const intervals = [...(props.entry.intervals || [])]
  intervals[idx] = updated
  emit('update', { ...props.entry, intervals })
}

function removeInterval(idx: number) {
  const intervals = [...(props.entry.intervals || [])]
  intervals.splice(idx, 1)
  emit('update', { ...props.entry, intervals })
}

async function onModelsUpdate(newModels: string[]) {
  const oldModels = props.entry.models
  emit('update', { ...props.entry, models: newModels })

  // 只在新增模型且当前无价格时自动填充
  const addedModels = newModels.filter(m => !oldModels.includes(m))
  if (addedModels.length === 0) return

  // 检查是否所有价格字段都为空
  const e = props.entry
  const hasPrice = e.input_price != null || e.output_price != null ||
                   e.cache_write_price != null || e.cache_read_price != null
  if (hasPrice) return

  // 查询第一个新增模型的默认价格
  try {
    const result = await channelsAPI.getModelDefaultPricing(addedModels[0], props.platform)
    if (result.found) {
      emit('update', {
        ...props.entry,
        models: newModels,
        input_price: perTokenToMTok(result.input_price ?? null),
        output_price: perTokenToMTok(result.output_price ?? null),
        cache_write_price: perTokenToMTok(result.cache_write_price ?? null),
        cache_read_price: perTokenToMTok(result.cache_read_price ?? null),
        image_output_price: perTokenToMTok(result.image_output_price ?? null),
      })
    }
  } catch {
    // 查询失败不影响用户操作
  }
}
</script>

<style scoped>
.collapsible-content {
  display: grid;
  grid-template-rows: 1fr;
  transition: grid-template-rows 0.25s ease;
}

.collapsible-content--collapsed {
  grid-template-rows: 0fr;
}

.collapsible-inner {
  overflow: hidden;
}
</style>
