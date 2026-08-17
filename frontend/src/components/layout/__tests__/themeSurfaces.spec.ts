import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('全局主题表面', () => {
  it('为工作区和卡片使用 Atom One Dark 层级', () => {
    // 工作区保持低明度，卡片通过提升面和更清晰的描边建立层级。
    const shellBlock = styleSource.match(/\.ba-theme-shell\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const backdropBlock = styleSource.match(/\.ba-theme-backdrop\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const cardBlock = styleSource.match(/\.card\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''

    expect(shellBlock).toContain('@apply bg-white dark:bg-[#1b1d1ff2];')
    expect(backdropBlock).toContain('@apply bg-white dark:bg-[#1b1d1ff2];')
    expect(cardBlock).toContain('@apply bg-white dark:bg-[#282c34];')
    expect(cardBlock).toContain('@apply rounded-xl;')
    expect(cardBlock).toContain('dark:border-[#3e4451]')
  })
})
