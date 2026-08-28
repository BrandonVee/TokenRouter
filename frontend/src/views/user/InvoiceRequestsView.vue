<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.invoices.userTitle') }}</h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.invoices.userSubtitle') }}</p>
    </div>

    <section class="card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
        <div><p class="text-sm font-semibold text-gray-900 dark:text-white">1. 选择订单</p><p class="mt-1 text-xs text-gray-500">可合并同一币种的充值和订阅订单</p></div>
        <div class="text-right"><p class="text-xs text-gray-500">已选 {{ form.orderIds.length }} 笔</p><p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ formatAmount(selectedTotal, selectedCurrency) }}</p></div>
      </div>
      <div v-if="eligibleLoading" class="p-8 text-center text-sm text-gray-500">加载可开票订单中...</div>
      <div v-else-if="eligibleOrders.length === 0" class="p-8 text-center text-sm text-gray-500">暂无可申请发票的已完成订单</div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="w-12 px-5 py-3"><input type="checkbox" :checked="allCurrentPageSelected" :indeterminate="someCurrentPageSelected && !allCurrentPageSelected" aria-label="选择当前页订单" @change="toggleCurrentPage" /></th><th class="px-5 py-3">订单</th><th class="px-5 py-3">充值时间</th><th class="px-5 py-3">支付方式</th><th class="px-5 py-3">订单类型</th><th class="px-5 py-3 text-right">实付金额</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="order in eligibleOrders" :key="order.id" class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800" :class="isSelected(order.id) ? 'bg-primary-50/60 dark:bg-primary-900/10' : ''"><td class="px-5 py-3"><input type="checkbox" :checked="isSelected(order.id)" :aria-label="`选择订单 ${order.out_trade_no}`" @change="toggleOrder(order, $event)" /></td><td class="px-5 py-3"><span class="block font-mono text-xs text-gray-700 dark:text-gray-200">{{ order.out_trade_no }}</span><span class="mt-1 block text-xs text-gray-500">#{{ order.id }}</span></td><td class="whitespace-nowrap px-5 py-3 text-xs text-gray-600 dark:text-gray-300">{{ date(order.paid_at || order.completed_at || order.created_at) }}</td><td class="px-5 py-3 text-xs text-gray-600 dark:text-gray-300">{{ paymentMethodLabel(order.payment_type) }}</td><td class="px-5 py-3 text-xs text-gray-600 dark:text-gray-300">{{ order.order_type === 'subscription' ? '订阅额度' : '余额充值' }}</td><td class="px-5 py-3 text-right font-medium text-gray-900 dark:text-white">{{ formatAmount(order.pay_amount, order.currency || '') }}</td></tr></tbody></table>
      </div>
      <Pagination v-if="eligiblePagination.total > 0" :page="eligiblePagination.page" :total="eligiblePagination.total" :page-size="eligiblePagination.page_size" @update:page="handleEligiblePageChange" @update:pageSize="handleEligiblePageSizeChange" />
      <div v-if="currencyMismatch" class="border-t border-red-100 bg-red-50 px-5 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">请选择同一币种的订单合并开票。</div>
    </section>

    <section class="card p-5">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3"><div><p class="text-sm font-semibold text-gray-900 dark:text-white">2. 填写开票信息</p><p class="mt-1 text-xs text-gray-500">开票类型和收件邮箱会随本次申请保存。</p></div><span v-if="selectedCurrency" class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ selectedCurrency }}</span></div>
      <div class="mb-5 inline-flex rounded-md border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800" role="group" aria-label="发票类型">
        <button type="button" class="min-w-24 rounded px-4 py-2 text-sm font-medium transition-colors" :class="form.invoiceType === 'PERSONAL' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-600 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'" :aria-pressed="form.invoiceType === 'PERSONAL'" @click="selectInvoiceType('PERSONAL')">个人</button>
        <button type="button" class="min-w-24 rounded px-4 py-2 text-sm font-medium transition-colors" :class="form.invoiceType === 'ENTERPRISE' ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-600 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'" :aria-pressed="form.invoiceType === 'ENTERPRISE'" @click="selectInvoiceType('ENTERPRISE')">企业</button>
      </div>
      <div class="grid gap-4 md:grid-cols-2">
        <div><label class="input-label">{{ form.invoiceType === 'ENTERPRISE' ? '企业名称' : '姓名' }}</label><input v-model="form.invoiceTitle" class="input mt-1 w-full" :placeholder="form.invoiceType === 'ENTERPRISE' ? '填写企业名称' : '填写个人姓名'" /></div>
        <div v-if="form.invoiceType === 'ENTERPRISE'"><label class="input-label">纳税人识别号</label><input v-model="form.taxId" class="input mt-1 w-full" placeholder="填写企业税号" /></div>
        <div v-if="form.invoiceType === 'ENTERPRISE'"><label class="input-label">开户行（可选）</label><input v-model="form.bankName" class="input mt-1 w-full" placeholder="填写开户银行名称" /></div>
        <div v-if="form.invoiceType === 'ENTERPRISE'"><label class="input-label">银行账号（可选）</label><input v-model="form.bankAccount" class="input mt-1 w-full" inputmode="numeric" placeholder="填写企业银行账号" /></div>
        <div><label class="input-label">接收邮箱</label><input v-model="form.recipientEmail" type="email" class="input mt-1 w-full" placeholder="留空使用账户邮箱" /></div>
        <div><label class="input-label">备注</label><input v-model="form.remark" class="input mt-1 w-full" placeholder="可选" /></div>
      </div>
      <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-700"><p v-if="!form.orderIds.length" class="text-sm text-gray-500">请先选择订单</p><p v-else-if="form.invoiceType === 'ENTERPRISE' && !form.taxId.trim()" class="text-sm text-amber-600 dark:text-amber-400">企业发票需要填写企业名称和纳税人识别号</p><span v-else /><button class="btn btn-primary" :disabled="submitting || !canSubmit" @click="submit">{{ submitting ? '提交中...' : `提交 ${form.orderIds.length || ''} 笔订单的申请` }}</button></div>
    </section>

    <section class="card overflow-hidden">
      <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-700"><p class="text-sm font-semibold text-gray-900 dark:text-white">申请记录</p><button class="btn btn-secondary btn-sm" :disabled="loading" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" /></button></div>
      <div v-if="loading" class="p-8 text-center text-sm text-gray-500">加载中...</div>
      <div v-else-if="requests.length === 0" class="p-8 text-center text-sm text-gray-500">暂无发票申请记录</div>
      <div v-else class="overflow-x-auto"><table class="w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">申请编号</th><th class="px-5 py-3">类型</th><th class="px-5 py-3">抬头</th><th class="px-5 py-3">金额</th><th class="px-5 py-3">状态</th><th class="px-5 py-3"></th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="request in requests" :key="request.id"><td class="px-5 py-3 font-mono text-xs">{{ request.request_no }}</td><td class="px-5 py-3">{{ invoiceTypeLabel(request.invoice_type) }}</td><td class="px-5 py-3">{{ request.invoice_title }}</td><td class="px-5 py-3">{{ formatAmount(request.total_amount, request.currency) }}</td><td class="px-5 py-3"><span :class="statusClass(request.status)" class="rounded px-2 py-1 text-xs font-medium">{{ statusLabel(request.status) }}</span></td><td class="px-5 py-3 text-right"><button class="btn btn-secondary btn-sm" @click="openDetail(request.id)"><Icon name="eye" size="sm" /></button></td></tr></tbody></table></div>
      <Pagination v-if="requestPagination.total > 0" :page="requestPagination.page" :total="requestPagination.total" :page-size="requestPagination.page_size" @update:page="handleRequestPageChange" @update:pageSize="handleRequestPageSizeChange" />
    </section>

    <BaseDialog :show="!!detail" title="发票申请详情" width="wide" @close="detail = null">
      <div v-if="detail" class="space-y-5">
        <div class="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3"><div><span class="text-gray-500">申请编号</span><p class="font-mono">{{ detail.request.request_no }}</p></div><div><span class="text-gray-500">发票类型</span><p>{{ invoiceTypeLabel(detail.request.invoice_type) }}</p></div><div><span class="text-gray-500">金额</span><p>{{ formatAmount(detail.request.total_amount, detail.request.currency) }}</p></div><div><span class="text-gray-500">状态</span><p>{{ statusLabel(detail.request.status) }}</p></div><div v-if="detail.request.invoice_number"><span class="text-gray-500">发票号码</span><p>{{ detail.request.invoice_number }}</p></div><div class="sm:col-span-2"><span class="text-gray-500">接收邮箱</span><p>{{ detail.request.recipient_email }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">开户行</span><p>{{ detail.request.bank_name || '-' }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">银行账号</span><p>{{ detail.request.bank_account || '-' }}</p></div></div>
        <p v-if="detail.request.rejection_reason" class="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">驳回原因：{{ detail.request.rejection_reason }}</p>
        <div><h2 class="mb-2 text-sm font-medium">订单明细</h2><div class="divide-y divide-gray-100 rounded border border-gray-200 text-sm dark:divide-dark-700 dark:border-dark-700"><div v-for="item in detail.items" :key="item.id" class="flex justify-between gap-4 px-3 py-2"><span class="font-mono text-xs">{{ item.order_no }}</span><span>{{ formatAmount(item.pay_amount, item.currency) }}</span></div></div></div>
        <div v-if="detail.attachments.length"><h2 class="mb-2 text-sm font-medium">发票附件</h2><div class="space-y-2"><button v-for="file in detail.attachments" :key="file.id" class="flex w-full items-center justify-between rounded border border-gray-200 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800" @click="download(file.id, file.file_name)"><span>{{ file.file_name }}</span><Icon name="download" size="sm" /></button></div></div>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { invoiceAPI } from '@/api/invoice'
import { useAppStore } from '@/stores'
import type { PaymentOrder } from '@/types/payment'
import type { InvoiceRequest, InvoiceRequestDetail, InvoiceRequestStatus, InvoiceType } from '@/types/invoice'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { formatPaymentAmount } from '@/components/payment/currency'

const appStore = useAppStore()
const { t } = useI18n()
const loading = ref(false)
const eligibleLoading = ref(false)
const submitting = ref(false)
const requests = ref<InvoiceRequest[]>([])
const eligibleOrders = ref<PaymentOrder[]>([])
const selectedOrderMap = reactive(new Map<number, PaymentOrder>())
const eligiblePagination = reactive({ page: 1, total: 0, page_size: 10, pages: 0 })
const requestPagination = reactive({ page: 1, total: 0, page_size: 10, pages: 0 })
const detail = ref<InvoiceRequestDetail | null>(null)
// 发票类型变更时保留名称和邮箱，个人发票清理企业专属字段。
const form = reactive({ invoiceType: 'PERSONAL' as InvoiceType, invoiceTitle: '', taxId: '', bankName: '', bankAccount: '', recipientEmail: '', remark: '', orderIds: [] as number[] })

const selectedOrders = computed(() => Array.from(selectedOrderMap.values()))
const selectedCurrency = computed(() => selectedOrders.value[0]?.currency || '')
const selectedTotal = computed(() => selectedOrders.value.reduce((total, order) => total + order.pay_amount, 0))
const currencyMismatch = computed(() => new Set(selectedOrders.value.map(order => order.currency || '')).size > 1)
const canSubmit = computed(() => form.invoiceTitle.trim() !== '' && form.orderIds.length > 0 && !currencyMismatch.value && (form.invoiceType === 'PERSONAL' || form.taxId.trim() !== ''))
const allCurrentPageSelected = computed(() => eligibleOrders.value.length > 0 && eligibleOrders.value.every(order => selectedOrderMap.has(order.id)))
const someCurrentPageSelected = computed(() => eligibleOrders.value.some(order => selectedOrderMap.has(order.id)))

function updatePagination(target: { page: number; total: number; page_size: number; pages: number }, data: { page: number; total: number; page_size: number; pages: number }) { target.page = data.page; target.total = data.total; target.page_size = data.page_size; target.pages = data.pages }
async function load() { loading.value = true; try { const response = await invoiceAPI.list({ page: requestPagination.page, page_size: requestPagination.page_size }); requests.value = response.data.items || []; updatePagination(requestPagination, response.data) } catch { appStore.showError('无法加载发票申请') } finally { loading.value = false } }
async function loadEligible(page = eligiblePagination.page) { eligibleLoading.value = true; try { const response = await invoiceAPI.getEligibleOrders({ page, page_size: eligiblePagination.page_size }); eligibleOrders.value = response.data.items || []; updatePagination(eligiblePagination, response.data) } catch { appStore.showError('无法加载可开票订单') } finally { eligibleLoading.value = false } }
function isSelected(id: number) { return selectedOrderMap.has(id) }
function toggleOrder(order: PaymentOrder, event: Event) { if ((event.target as HTMLInputElement).checked) selectedOrderMap.set(order.id, order); else selectedOrderMap.delete(order.id); form.orderIds = Array.from(selectedOrderMap.keys()) }
function toggleCurrentPage(event: Event) { const checked = (event.target as HTMLInputElement).checked; eligibleOrders.value.forEach(order => checked ? selectedOrderMap.set(order.id, order) : selectedOrderMap.delete(order.id)); form.orderIds = Array.from(selectedOrderMap.keys()) }
function handleEligiblePageChange(page: number) { loadEligible(page) }
function handleEligiblePageSizeChange(size: number) { eligiblePagination.page_size = size; eligiblePagination.page = 1; loadEligible(1) }
function handleRequestPageChange(page: number) { requestPagination.page = page; load() }
function handleRequestPageSizeChange(size: number) { requestPagination.page_size = size; requestPagination.page = 1; load() }
function selectInvoiceType(type: InvoiceType) { form.invoiceType = type; if (type === 'PERSONAL') { form.taxId = ''; form.bankName = ''; form.bankAccount = '' } }
async function submit() { if (!canSubmit.value) return; submitting.value = true; try { await invoiceAPI.create({ order_ids: form.orderIds, invoice_type: form.invoiceType, invoice_title: form.invoiceTitle.trim(), tax_id: form.invoiceType === 'ENTERPRISE' ? form.taxId.trim() : undefined, bank_name: form.invoiceType === 'ENTERPRISE' ? form.bankName.trim() || undefined : undefined, bank_account: form.invoiceType === 'ENTERPRISE' ? form.bankAccount.trim() || undefined : undefined, recipient_email: form.recipientEmail.trim() || undefined, remark: form.remark.trim() || undefined }); appStore.showSuccess('发票申请已提交'); form.invoiceTitle = ''; form.taxId = ''; form.bankName = ''; form.bankAccount = ''; form.recipientEmail = ''; form.remark = ''; selectedOrderMap.clear(); form.orderIds = []; await Promise.all([load(), loadEligible(eligiblePagination.page)]) } catch { appStore.showError('发票申请提交失败') } finally { submitting.value = false } }
async function openDetail(id: number) { try { const response = await invoiceAPI.get(id); detail.value = response.data } catch { appStore.showError('无法加载发票申请详情') } }
async function download(id: number, name: string) { try { const response = await invoiceAPI.downloadAttachment(id); const url = URL.createObjectURL(response.data); const link = document.createElement('a'); link.href = url; link.download = name; link.click(); URL.revokeObjectURL(url) } catch { appStore.showError('附件下载失败') } }
function formatAmount(value: number, currency: string) { return formatPaymentAmount(value, currency) }
function date(value: string) { return new Date(value).toLocaleString() }
function paymentMethodLabel(value: string) { return ({ alipay: '支付宝', alipay_direct: '支付宝', wxpay: '微信支付', wxpay_direct: '微信支付', stripe: 'Stripe', airwallex: 'Airwallex', easypay: '易支付', card: '银行卡', link: 'Link', test: '测试支付' } as Record<string, string>)[value] || value || '-' }
function invoiceTypeLabel(type: InvoiceType) { return type === 'ENTERPRISE' ? '企业' : '个人' }
function statusLabel(status: InvoiceRequestStatus) { return ({ SUBMITTED: '待审批', REJECTED: '已驳回', APPROVED: '已审批', ISSUED: '已开票', SENT: '已发送' } as Record<InvoiceRequestStatus, string>)[status] }
function statusClass(status: InvoiceRequestStatus) { return ({ SUBMITTED: 'bg-amber-100 text-amber-800', REJECTED: 'bg-red-100 text-red-700', APPROVED: 'bg-blue-100 text-blue-700', ISSUED: 'bg-violet-100 text-violet-700', SENT: 'bg-emerald-100 text-emerald-700' } as Record<InvoiceRequestStatus, string>)[status] }
onMounted(() => Promise.all([load(), loadEligible()]))
</script>
