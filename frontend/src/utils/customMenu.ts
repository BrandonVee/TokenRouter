import type { CustomMenuItem } from '@/types'

/** 按当前界面语言解析自定义菜单名称，并兼容旧版单名称字段。 */
export function resolveCustomMenuLabel(
  item: Pick<CustomMenuItem, 'label' | 'label_zh' | 'label_en'>,
  locale: string,
): string {
  const legacy = item.label?.trim() || ''
  const zh = item.label_zh?.trim() || ''
  const en = item.label_en?.trim() || ''

  if (locale.toLowerCase().startsWith('zh')) {
    return zh || en || legacy
  }
  return en || zh || legacy
}
