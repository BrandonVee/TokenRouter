import { describe, expect, it } from 'vitest'
import { resolveCustomMenuLabel } from '../customMenu'

describe('resolveCustomMenuLabel', () => {
  it('按中文界面优先显示中文名称', () => {
    expect(resolveCustomMenuLabel({ label: 'Legacy', label_zh: '帮助', label_en: 'Help' }, 'zh')).toBe('帮助')
  })

  it('按英文界面优先显示英文名称', () => {
    expect(resolveCustomMenuLabel({ label: 'Legacy', label_zh: '帮助', label_en: 'Help' }, 'en')).toBe('Help')
  })

  it('缺少当前语言名称时回退到另一语言和旧字段', () => {
    expect(resolveCustomMenuLabel({ label: 'Legacy', label_zh: '', label_en: 'Help' }, 'zh')).toBe('Help')
    expect(resolveCustomMenuLabel({ label: 'Legacy' }, 'en')).toBe('Legacy')
  })
})
