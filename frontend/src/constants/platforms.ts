import { listPlatforms } from './platformCatalog'
import type { AccountPlatform, GroupPlatform } from '@/types'

export interface PlatformOption<T extends string = string> {
  [key: string]: unknown
  value: T
  label: string
}

/** 账号与路由的平台选项同源，新增平台自动进入筛选器。 */
export const CONCRETE_PLATFORM_OPTIONS: readonly PlatformOption<AccountPlatform>[] = listPlatforms().map(spec => ({
  value: spec.id as AccountPlatform,
  label: spec.display_name
}))

/** 分组可选择具体平台或复合路由。 */
export const GROUP_PLATFORM_OPTIONS = [
  ...CONCRETE_PLATFORM_OPTIONS,
  { value: 'composite', label: 'Composite' }
] as const satisfies readonly PlatformOption<GroupPlatform>[]
