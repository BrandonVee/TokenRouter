import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import theme from 'ant-design-vue/es/theme'

import { createDatePickerTheme } from '../datePickerTheme'

describe('datePickerTheme', () => {
  it('暗色模式使用 Ant Design 暗色算法和石墨色弹层色板', () => {
    const config = createDatePickerTheme(true)

    expect(config.algorithm).toBe(theme.darkAlgorithm)
    expect(config.token.colorBgElevated).toBe('#2a2e35')
    expect(config.token.colorText).toBe('#f2f3f5')
    expect(config.token.colorBorder).toBe('#4d535d')
  })

  it('全局样式覆盖 Teleport 到 body 的日期和时间面板', () => {
    const styleSource = readFileSync('src/style.css', 'utf8')

    expect(styleSource).toContain('.dark .tokenrouter-ant-date-popup .ant-picker-panel-container')
    expect(styleSource).toContain('.dark .tokenrouter-ant-date-popup :is(.ant-picker-time-panel-column, .ant-picker-time-panel-cell-inner)')
    expect(styleSource).toContain('.dark .tokenrouter-ant-date-popup .ant-picker-ok button')
  })
})
