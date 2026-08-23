import { describe, expect, it } from 'vitest'
import { hasExplicitPricing, validateIntervals, type IntervalFormEntry, type PricingFormEntry } from '../types'

function makeInterval(over: Partial<IntervalFormEntry>): IntervalFormEntry {
  return {
    min_tokens: 0,
    max_tokens: null,
    tier_label: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null,
    sort_order: 0,
    ...over,
  }
}

function makePricingEntry(over: Partial<PricingFormEntry>): PricingFormEntry {
  return {
    models: ['test-model'],
    billing_mode: 'token',
    price_multiplier: null,
    fast_mode_multiplier: null,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    ...over,
  }
}

function t(key: string, params?: Record<string, unknown>): string {
  return `${key}${params ? ` ${JSON.stringify(params)}` : ''}`
}

describe('validateIntervals', () => {
  describe('token mode', () => {
    it('拒绝不在最后的无上限区间', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: null, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: 500000, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('unboundedLast')
    })

    it('接受放在最后的无上限区间', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: 200000, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: null, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toBeNull()
    })

    it('拒绝重叠区间', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: 250000, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: 500000, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('overlap')
    })

    it('在 token 模式下拒绝不在最后的无上限区间', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: null, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 100, max_tokens: 200, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('unboundedLast')
    })
  })

  describe('image / per_request mode', () => {
    it('允许多个按标签识别的无上限层级', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', per_request_price: 0.04 }),
        makeInterval({ tier_label: '2K', per_request_price: 0.06 }),
        makeInterval({ tier_label: '4K', per_request_price: 0.08 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toBeNull()
      expect(validateIntervals(intervals, 'per_request', t)).toBeNull()
    })

    it('仍然拒绝负价格', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', per_request_price: -1 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toContain('negativePrice')
    })

    it('仍然拒绝单个层级中 max <= min 的配置', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', min_tokens: 100, max_tokens: 50, per_request_price: 0.04 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toContain('maxGreaterThanMin')
    })
  })
})

describe('hasExplicitPricing', () => {
  it('拒绝没有任何价格的 token 定价', () => {
    expect(hasExplicitPricing(makePricingEntry({ price_multiplier: 1.5 }))).toBe(false)
    expect(hasExplicitPricing(makePricingEntry({ fast_mode_multiplier: 2 }))).toBe(false)
  })

  it('把显式零价格和图片区间价格视为有效定价', () => {
    expect(hasExplicitPricing(makePricingEntry({ input_price: 0 }))).toBe(true)
    expect(hasExplicitPricing(makePricingEntry({ image_input_price: 0.01 }))).toBe(true)
    expect(hasExplicitPricing(makePricingEntry({
      billing_mode: 'image',
      intervals: [makeInterval({ tier_label: '1K', per_request_price: 0.04 })],
    }))).toBe(true)
  })

  it('不把当前计费模式无关的价格字段视为有效定价', () => {
    expect(hasExplicitPricing(makePricingEntry({
      billing_mode: 'image',
      input_price: 1,
    }))).toBe(false)
    expect(hasExplicitPricing(makePricingEntry({
      billing_mode: 'token',
      per_request_price: 1,
    }))).toBe(false)
  })
})
