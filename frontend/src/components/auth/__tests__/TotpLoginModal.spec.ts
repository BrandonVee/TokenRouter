import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'

const { showErrorMock } = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: (...args: any[]) => showErrorMock(...args),
  }),
}))

describe('TotpLoginModal', () => {
  beforeEach(() => {
    showErrorMock.mockReset()
  })

  it('sends verification errors to toast and does not render inline red text', async () => {
    const wrapper = mount(TotpLoginModal, {
      props: {
        tempToken: 'temp-token',
        userEmailMasked: 'u***@example.com',
      },
    })

    ;(wrapper.vm as unknown as { setError: (message: string) => void }).setError('Invalid code')
    await wrapper.vm.$nextTick()

    expect(showErrorMock).toHaveBeenCalledWith('Invalid code')
    expect(wrapper.text()).not.toContain('Invalid code')
    expect(wrapper.find('.bg-red-50').exists()).toBe(false)
  })

  it('fills visible code inputs from hidden one-time-code autofill input', async () => {
    const wrapper = mount(TotpLoginModal, {
      props: {
        tempToken: 'temp-token',
      },
    })

    const hiddenInput = wrapper.get('input[autocomplete="one-time-code"]')
    await hiddenInput.setValue('12a3456')
    await wrapper.vm.$nextTick()

    const visibleInputs = wrapper.findAll('input[autocomplete="off"]')
    expect(visibleInputs.map(input => (input.element as HTMLInputElement).value)).toEqual([
      '1',
      '2',
      '3',
      '4',
      '5',
      '6',
    ])
    expect(wrapper.emitted('verify')).toEqual([['123456']])
  })
})
