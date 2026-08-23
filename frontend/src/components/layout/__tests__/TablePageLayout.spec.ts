import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const globalStylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const globalStyleSource = readFileSync(globalStylePath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('does not disable the table horizontal scroll container in mobile mode', () => {
    // 单元测试环境不会编译 scoped Tailwind 样式，因此直接校验组件源码中的覆盖规则。
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toContain('overflow-x-auto')
    expect(mobileBlocks.every(([, , declarations]) => !declarations.includes('overflow-visible'))).toBe(
      true
    )
  })

  it('allows vertical scrolling to chain from tables to their outer page', () => {
    // 表格仍禁止横向边界回弹，但纵向到达边界后必须允许页面继续滚动。
    const overscrollRule = globalStyleSource.match(
      /table,\s*[\s\S]*?:has\(table\)\s*\{([^}]+)\}/
    )

    expect(overscrollRule).not.toBeNull()
    expect(overscrollRule?.[1]).toContain('overscroll-behavior-x: none;')
    expect(overscrollRule?.[1]).toContain('overscroll-behavior-y: auto;')
    expect(overscrollRule?.[1]).not.toMatch(/overscroll-behavior:\s*none/)
  })
})
