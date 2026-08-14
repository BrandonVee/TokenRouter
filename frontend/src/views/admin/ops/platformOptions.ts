import type { SelectOption } from '@/components/common/Select.vue'

interface OpsPlatformGroup {
  platform: string
}

const BUILTIN_PLATFORM_OPTIONS: SelectOption[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'qoder', label: 'Qoder' },
  { value: 'grok', label: 'Grok' }
]

// 内置平台保持稳定顺序，同时从分组数据补充后续新增的平台。
export function buildOpsPlatformOptions(groups: OpsPlatformGroup[], allLabel: string): SelectOption[] {
  const options: SelectOption[] = [{ value: '', label: allLabel }, ...BUILTIN_PLATFORM_OPTIONS]
  const knownPlatforms = new Set(options.map(option => option.value))

  for (const group of groups) {
    const platform = group.platform.trim().toLowerCase()
    if (!platform || knownPlatforms.has(platform)) continue

    options.push({ value: platform, label: platform.toUpperCase() })
    knownPlatforms.add(platform)
  }

  return options
}
