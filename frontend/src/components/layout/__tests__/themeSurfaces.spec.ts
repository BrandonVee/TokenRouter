import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')
const dataTableSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../common/DataTable.vue'), 'utf8')

describe('全局主题表面', () => {
  it('清除根节点间距和暗色边框，避免画布出现额外边缘空隙', () => {
    expect(styleSource).toMatch(/html \{[\s\S]*?margin: 0;\n {4}padding: 0;/)
    expect(styleSource).toMatch(/body \{[\s\S]*?margin: 0;\n {4}padding: 0;/)
    expect(styleSource).toContain('html.dark {\n    /* 暗色画布由背景铺满视口，根节点不应携带额外边框。 */\n    border: 0 !important;')
    expect(styleSource).toContain('  #app {\n    margin: 0;\n    padding: 0;\n  }')
  })

  it('为明暗工作区使用液态玻璃层级', () => {
    // 两种主题共享玻璃材质，但各自保留不同的明度和环境光色阶。
    const shellBlock = styleSource.match(/\.ba-theme-shell\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const backdropBlock = styleSource.match(/\.ba-theme-backdrop\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const cardBlock = styleSource.match(/\.card\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const lightSurfaceBlock = styleSource.match(/html:not\(\.dark\) :is\(\n\s{4}\.glass,[\s\S]*?\n\s{2}\}/)?.[0] ?? ''
    const darkSurfaceBlock = styleSource.match(/\.dark :is\(\n\s{4}\.glass,[\s\S]*?\n\s{2}\}/)?.[0] ?? ''

    expect(shellBlock).toContain('@apply bg-transparent dark:bg-[#1b1d1ff2];')
    expect(backdropBlock).toContain('linear-gradient')
    expect(backdropBlock).toContain('@apply dark:bg-[#1b1d1ff2];')
    expect(styleSource).toContain('.dark .ba-theme-backdrop {\n    background:\n      radial-gradient')
    expect(cardBlock).toContain('@apply bg-white dark:bg-[#282c34];')
    expect(cardBlock).toContain('@apply rounded-xl;')
    expect(cardBlock).toContain('dark:border-[#3e4451]')
    expect(lightSurfaceBlock).toContain('background-color: rgba(250, 253, 255, 0.68);')
    expect(lightSurfaceBlock).toContain('backdrop-filter: blur(28px) saturate(155%);')
    expect(lightSurfaceBlock).toContain('inset 0 1px 0 rgba(255, 255, 255, 0.92)')
    expect(darkSurfaceBlock).toContain('background-color: rgba(29, 35, 41, 0.7);')
    expect(darkSurfaceBlock).toContain('backdrop-filter: blur(28px) saturate(135%);')
    expect(darkSurfaceBlock).toContain('inset 0 1px 0 rgba(255, 255, 255, 0.08)')
    expect(dataTableSource).toContain('--sticky-col-bg: rgba(250, 253, 255, 0.72);')
    expect(dataTableSource).toContain('--sticky-col-bg: rgba(29, 35, 41, 0.7);')
    expect(dataTableSource).not.toContain('tbody .sticky-col {\n  background-color: white;')
    expect(styleSource).toContain('html:not(.dark) .marketplace-model-card {')
    expect(styleSource).toContain('background-color: rgba(255, 255, 255, 0.78) !important;')
    expect(styleSource).toContain('border-color: rgba(94, 128, 143, 0.34) !important;')
  })
})
