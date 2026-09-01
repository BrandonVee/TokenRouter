import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ImageHistoryStorageConfig } from '@/api/admin/fileStorage'
import FileStorageSettings from '../FileStorageSettings.vue'

const {
  getConfig,
  updateConfig,
  testConnection,
  resetConfig,
  showError,
  showSuccess,
  runStepUp,
} = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  testConnection: vi.fn(),
  resetConfig: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  runStepUp: vi.fn((operation: () => Promise<unknown>) => operation()),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    fileStorage: {
      getImageHistoryStorageConfig: getConfig,
      updateImageHistoryStorageConfig: updateConfig,
      testImageHistoryStorageConnection: testConnection,
      resetImageHistoryStorageConfig: resetConfig,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: runStepUp }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

// 页面只接收脱敏后的 Secret 状态，空密码框表示沿用已保存值。
const databaseConfig: ImageHistoryStorageConfig = {
  enabled: true,
  endpoint: 'https://s3.example.test',
  region: 'auto',
  bucket: 'images',
  access_key_id: 'access-key',
  prefix: 'generated/images',
  force_path_style: true,
  secret_configured: true,
  available: true,
  source: 'database',
  encryption_key_ready: true,
}

function mountView() {
  return mount(FileStorageSettings, {
    global: {
      stubs: {
        Icon: true,
        TotpStepUpDialog: true,
      },
    },
  })
}

describe('FileStorageSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getConfig.mockResolvedValue(structuredClone(databaseConfig))
    updateConfig.mockResolvedValue(structuredClone(databaseConfig))
    testConnection.mockResolvedValue({ ok: true, message: 'connection successful' })
    resetConfig.mockResolvedValue({
      ...structuredClone(databaseConfig),
      source: 'deployment',
    })
  })

  it('loads the effective config and keeps the saved Secret when testing and saving', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getConfig).toHaveBeenCalledOnce()
    expect(wrapper.get('input[type="password"]').attributes('placeholder')).toContain('secretConfigured')

    const testButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('fileStorage.images.test'))
    expect(testButton).toBeDefined()
    await testButton!.trigger('click')
    await flushPromises()

    expect(testConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        secret_access_key: '',
        prefix: 'generated/images',
      }),
    )

    const saveButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('common.save'))
    expect(saveButton).toBeDefined()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(runStepUp).toHaveBeenCalledOnce()
    expect(updateConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        secret_access_key: '',
        bucket: 'images',
      }),
    )
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.fileStorage.images.saved')
  })

  it('requires an encryption key to save without blocking a connection test', async () => {
    getConfig.mockResolvedValue({
      ...structuredClone(databaseConfig),
      encryption_key_ready: false,
    })
    const wrapper = mountView()
    await flushPromises()

    const testButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('fileStorage.images.test'))
    const saveButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('common.save'))
    expect(wrapper.get('[data-testid="image-history-encryption-warning"]').text()).toContain('encryptionKeyRequired')
    expect(saveButton!.attributes('disabled')).toBeDefined()

    await testButton!.trigger('click')
    await flushPromises()

    expect(testConnection).toHaveBeenCalledOnce()
    expect(updateConfig).not.toHaveBeenCalled()
  })

  it('restores deployment config after confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountView()
    await flushPromises()

    const restoreButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('restoreDeployment'))
    expect(restoreButton).toBeDefined()
    await restoreButton!.trigger('click')
    await flushPromises()

    expect(resetConfig).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('fileStorage.images.source.deployment')
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.fileStorage.images.restored')
  })

  it('keeps other file stores independent and links to their existing settings', async () => {
    const wrapper = mountView()
    await flushPromises()

    const otherTab = wrapper
      .findAll('[role="tab"]')
      .find((button) => button.text().includes('fileStorage.sections.other'))
    expect(otherTab).toBeDefined()
    await otherTab!.trigger('click')

    expect(wrapper.text()).toContain('DATA_DIR/backups')
    expect(wrapper.text()).toContain('DATA_DIR/invoice-attachments')
    expect(wrapper.text()).toContain('DATA_DIR/data-sharing-exports')

    const backupButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('fileStorage.other.backup.action'))
    await backupButton!.trigger('click')
    expect(wrapper.emitted('open-backup')).toHaveLength(1)
  })
})
