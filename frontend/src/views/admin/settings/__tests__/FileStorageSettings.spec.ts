import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ImageHistoryStorageConfig } from '@/api/admin/fileStorage'
import FileStorageSettings from '../FileStorageSettings.vue'

const {
  getConfig,
  updateConfig,
  testConnection,
  getInvoiceConfig,
  updateInvoiceConfig,
  testInvoiceConnection,
  showError,
  showSuccess,
  runStepUp,
} = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  testConnection: vi.fn(),
  getInvoiceConfig: vi.fn(),
  updateInvoiceConfig: vi.fn(),
  testInvoiceConnection: vi.fn(),
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
      getInvoiceAttachmentStorageConfig: getInvoiceConfig,
      updateInvoiceAttachmentStorageConfig: updateInvoiceConfig,
      testInvoiceAttachmentStorageConnection: testInvoiceConnection,
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

// 发票目录默认返回本地档案，S3 字段为空。
const localInvoiceConfig = {
  directory: 'invoice_attachments' as const,
  profile: {
    id: 'local-default',
    type: 'local' as const,
    local_path: '/data/invoice-attachments',
    s3: { endpoint: '', region: 'auto', bucket: '', access_key_id: '', prefix: 'invoice-attachments', force_path_style: false },
    secret_configured: false,
    encryption_key_ready: true,
  },
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
    getInvoiceConfig.mockResolvedValue(structuredClone(localInvoiceConfig))
    updateInvoiceConfig.mockResolvedValue(structuredClone(localInvoiceConfig))
    testInvoiceConnection.mockResolvedValue({ ok: true, message: 'connection successful' })
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

  it('blocks testing and saving an incomplete invoice S3 form before hitting the API', async () => {
    const wrapper = mountView()
    await flushPromises()

    const attachmentsTab = wrapper
      .findAll('[role="tab"]')
      .find((button) => button.text().includes('fileStorage.sections.attachments'))
    await attachmentsTab!.trigger('click')

    // 切到 S3 后表单为空，本地校验应直接拦截，不发请求。
    const s3Toggle = wrapper.findAll('button').find((button) => button.text() === 'S3')
    await s3Toggle!.trigger('click')
    await flushPromises()

    // 默认 prefix 合法，首个错误是必填项缺失；清空 prefix 后应切换为路径校验错误。
    expect(wrapper.get('[data-testid="invoice-storage-validation-error"]').text()).toContain('s3Incomplete')
    const prefixInput = wrapper.findAll('input').find((input) => input.element.placeholder === 'invoice-attachments')!
    await prefixInput.setValue('')
    await flushPromises()
    expect(wrapper.get('[data-testid="invoice-storage-validation-error"]').text()).toContain('prefixInvalid')
    const disabledButtons = wrapper.findAll('button').filter((button) => button.attributes('disabled') !== undefined)
    expect(disabledButtons.length).toBeGreaterThan(0)

    await prefixInput.setValue('invoice-attachments')
    const textInputs = wrapper.findAll('input[type="text"], input:not([type])')
    // 依次填写 bucket 与 access key（端点/区域之后的前两个必填输入框）。
    await textInputs[2].setValue('invoices')
    await textInputs[3].setValue('access-key')
    await wrapper.get('input[type="password"]').setValue('secret-key')
    await flushPromises()

    expect(wrapper.find('[data-testid="invoice-storage-validation-error"]').exists()).toBe(false)

    const invoiceTestButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('fileStorage.images.test'))
    await invoiceTestButton!.trigger('click')
    await flushPromises()

    expect(testInvoiceConnection).toHaveBeenCalledOnce()
    expect(testConnection).not.toHaveBeenCalled()
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
