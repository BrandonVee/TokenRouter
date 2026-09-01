import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ImageHistoryView from '../ImageHistoryView.vue'

const {
  getSettings,
  updateSettings,
  list,
  download,
  remove,
  showError,
  showSuccess,
  saveAs,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  list: vi.fn(),
  download: vi.fn(),
  remove: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  saveAs: vi.fn(),
}))

vi.mock('@/api/imageHistory', () => ({
  imageHistoryAPI: {
    getSettings,
    updateSettings,
    list,
    download,
    delete: remove,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('file-saver', () => ({ saveAs }))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en' },
    }),
  }
})

const record = {
  id: 'record-1',
  source: 'openai',
  endpoint: '/v1/images/generations',
  model: 'gpt-image-1',
  prompt: 'draw a lighthouse',
  mime_type: 'image/png',
  size_bytes: 128,
  width: 1024,
  height: 1024,
  sha256: 'digest',
  preview_url: 'https://s3.example.com/record-1.png',
  created_at: '2026-08-31T00:00:00Z',
}

const ToggleStub = {
  props: ['modelValue', 'disabled'],
  emits: ['update:modelValue'],
  template:
    '<button data-testid="history-toggle" :disabled="disabled" @click="$emit(\'update:modelValue\', !modelValue)" />',
}

function mountView() {
  return mount(ImageHistoryView, {
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        Pagination: true,
        Toggle: ToggleStub,
        Icon: true,
      },
    },
  })
}

describe('ImageHistoryView', () => {
  beforeEach(() => {
    getSettings.mockReset()
    updateSettings.mockReset()
    list.mockReset()
    download.mockReset()
    remove.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    saveAs.mockReset()

    getSettings.mockResolvedValue({ available: true, enabled: true })
    updateSettings.mockResolvedValue({ available: true, enabled: false })
    list.mockResolvedValue({ items: [record], total: 1, page: 1, page_size: 12, pages: 1 })
  })

  it('loads available settings and renders saved image metadata', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(1)
    expect(list).toHaveBeenCalledWith(1, 12)
    expect(wrapper.text()).toContain('gpt-image-1')
    expect(wrapper.text()).toContain('draw a lighthouse')
    expect(wrapper.get('img').attributes('src')).toBe(record.preview_url)
  })

  it('updates the explicit user opt-in from the toggle', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="history-toggle"]').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith(false)
    expect(showSuccess).toHaveBeenCalledWith('imageHistory.settingsSaved')
  })

  it('does not request history when storage is unavailable', async () => {
    getSettings.mockResolvedValue({ available: false, enabled: false })
    const wrapper = mountView()
    await flushPromises()

    expect(list).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('imageHistory.unavailable')
    expect(wrapper.get('[data-testid="history-toggle"]').attributes()).toHaveProperty('disabled')
  })
})
