import theme from 'ant-design-vue/es/theme'

// 日期控件弹层挂载在 body，显式提供完整色板，避免只继承触发器所在区域的明暗样式。
export const createDatePickerTheme = (isDark: boolean) => ({
  algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
  token: {
    colorPrimary: '#2563eb',
    borderRadius: 8,
    controlHeight: 40,
    fontSize: 14,
    ...(isDark
      ? {
          colorBgBase: '#252a2f',
          colorBgContainer: '#252a2f',
          colorBgElevated: '#2a2e35',
          colorText: '#f2f3f5',
          colorTextSecondary: '#c5c9d0',
          colorTextDisabled: '#848b97',
          colorBorder: '#4d535d',
          colorBorderSecondary: '#393e46',
          colorSplit: '#393e46',
          colorFillSecondary: 'rgba(77, 83, 93, 0.48)',
        }
      : {}),
  },
})
