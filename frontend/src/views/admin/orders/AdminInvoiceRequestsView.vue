<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div><h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.invoices.title') }}</h1><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.invoices.subtitle') }}</p></div>
      <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" /></button>
    </div>
    <div class="card flex flex-wrap gap-3 p-4"><input v-model="keyword" class="input w-64" :placeholder="t('admin.invoices.searchPlaceholder')" @keyup.enter="load" /><Select v-model="status" :options="statusOptions" class="w-44" @change="load" /><button class="btn btn-primary" @click="load">{{ t('admin.invoices.search') }}</button><button class="btn btn-secondary" @click="clearSearch">{{ t('admin.invoices.clear') }}</button></div>
    <div class="card overflow-hidden">
      <div v-if="loading" class="p-8 text-center text-sm text-gray-500">{{ t('admin.invoices.loading') }}</div>
      <div v-else-if="requests.length === 0" class="p-8 text-center text-sm text-gray-500">{{ t('admin.invoices.empty') }}</div>
      <table v-else class="w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ t('admin.invoices.requestNo') }}</th><th class="px-4 py-3">{{ t('admin.invoices.userId') }}</th><th class="px-4 py-3">{{ t('admin.invoices.email') }}</th><th class="px-4 py-3">{{ t('admin.invoices.type') }}</th><th class="px-4 py-3">{{ t('admin.invoices.titleField') }}</th><th class="px-4 py-3">{{ t('admin.invoices.amount') }}</th><th class="px-4 py-3">{{ t('admin.invoices.status') }}</th><th class="px-4 py-3"></th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="request in requests" :key="request.id"><td class="px-4 py-3 font-mono text-xs">{{ request.request_no }}</td><td class="px-4 py-3">#{{ request.user_id }}</td><td class="px-4 py-3">{{ request.user_email || request.account_email || request.recipient_email }}</td><td class="px-4 py-3">{{ invoiceTypeLabel(request.invoice_type) }}</td><td class="px-4 py-3">{{ request.invoice_title }}</td><td class="px-4 py-3">{{ amount(request.total_amount, request.currency) }}</td><td class="px-4 py-3"><span class="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ statusLabel(request.status) }}</span></td><td class="px-4 py-3 text-right"><button class="btn btn-secondary btn-sm" :title="t('admin.invoices.detail')" @click="openDetail(request.id)"><Icon name="eye" size="sm" /></button></td></tr></tbody></table>
    </div>

    <BaseDialog :show="!!detail" title="发票申请处理" width="wide" @close="detail = null">
      <div v-if="detail" class="space-y-5">
        <div class="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3"><div><span class="text-gray-500">申请编号</span><p class="font-mono">{{ detail.request.request_no }}</p></div><div><span class="text-gray-500">用户</span><p>#{{ detail.request.user_id }}</p></div><div><span class="text-gray-500">发票类型</span><p>{{ invoiceTypeLabel(detail.request.invoice_type) }}</p></div><div><span class="text-gray-500">状态</span><p>{{ statusLabel(detail.request.status) }}</p></div><div><span class="text-gray-500">抬头</span><p>{{ detail.request.invoice_title }}</p></div><div><span class="text-gray-500">税号</span><p>{{ detail.request.tax_id || '-' }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">开户行</span><p>{{ detail.request.bank_name || '-' }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">银行账号</span><p>{{ detail.request.bank_account || '-' }}</p></div><div><span class="text-gray-500">收件邮箱</span><p>{{ detail.request.recipient_email }}</p></div></div>
        <p v-if="detail.request.rejection_reason" class="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">驳回原因：{{ detail.request.rejection_reason }}</p>
        <div><h2 class="mb-2 text-sm font-medium">合并订单</h2><div class="divide-y divide-gray-100 rounded border border-gray-200 dark:divide-dark-700 dark:border-dark-700"><div v-for="item in detail.items" :key="item.id" class="flex justify-between gap-3 px-3 py-2 text-sm"><span class="font-mono text-xs">{{ item.order_no }}</span><span>{{ item.order_type === 'subscription' ? '订阅' : '充值' }}</span><span>{{ amount(item.pay_amount, item.currency) }}</span></div></div></div>
        <div v-if="detail.request.status === 'SUBMITTED'" class="flex flex-wrap gap-2"><button class="btn btn-primary" :disabled="actionLoading" @click="approve">审批通过</button><button class="btn btn-danger" :disabled="actionLoading" @click="rejectOpen = true">驳回</button></div>
        <div v-if="detail.request.status === 'APPROVED' || detail.request.status === 'ISSUED'" class="space-y-3 rounded border border-gray-200 p-4 dark:border-dark-700"><div class="flex flex-wrap items-center justify-between gap-3"><span class="text-sm font-medium">发票附件</span><label class="btn btn-secondary btn-sm cursor-pointer"><Icon name="upload" size="sm" />上传附件<input class="sr-only" type="file" accept="application/pdf,image/png,image/jpeg" @change="upload" /></label></div><div v-if="detail.attachments.length" class="space-y-2"><div v-for="file in detail.attachments" :key="file.id" class="flex items-center justify-between gap-2 rounded border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"><span class="min-w-0 truncate">{{ file.file_name }}</span><div class="flex shrink-0 gap-2"><button class="btn btn-secondary btn-sm" @click="preview(file.id, file.file_name, file.content_type)"><Icon name="eye" size="sm" />预览</button><button class="btn btn-secondary btn-sm" @click="download(file.id, file.file_name)"><Icon name="download" size="sm" />下载</button></div></div></div><p v-else class="text-sm text-gray-500">开票前至少上传一个 PDF、PNG 或 JPEG 附件。</p></div>
        <div v-if="detail.request.status === 'APPROVED'" class="grid gap-3 rounded border border-gray-200 p-4 md:grid-cols-3 dark:border-dark-700"><input v-model="issueForm.invoiceNumber" class="input" placeholder="发票号码" /><input v-model="issueForm.issuedAt" type="datetime-local" class="input" /><input v-model="issueForm.remark" class="input" placeholder="开票备注" /><button class="btn btn-primary md:col-span-3" :disabled="actionLoading || !issueForm.invoiceNumber.trim() || detail.attachments.length === 0" @click="issue">确认开票</button></div>
        <div v-if="detail.request.status === 'ISSUED'" class="flex justify-end"><button class="btn btn-primary" :disabled="actionLoading" @click="send">发送发票邮件</button></div>
        <div v-if="detail.deliveries?.length"><h2 class="mb-2 text-sm font-medium">投递记录</h2><div class="space-y-2"><div v-for="delivery in detail.deliveries" :key="delivery.id" class="rounded border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"><span>{{ delivery.status === 'SENT' ? '已发送' : '发送失败' }}</span><span class="ml-2 text-gray-500">{{ delivery.recipient_email }} · {{ date(delivery.created_at) }}</span><p v-if="delivery.error_message" class="mt-1 text-red-600">{{ delivery.error_message }}</p></div></div></div>
      </div>
    </BaseDialog>
    <BaseDialog :show="!!previewState" title="附件预览" width="wide" @close="closePreview"><div v-if="previewState" class="flex min-h-[60vh] items-center justify-center"><img v-if="previewState.contentType.startsWith('image/')" :src="previewState.url" :alt="previewState.name" class="max-h-[70vh] max-w-full object-contain" /><iframe v-else :src="previewState.url" :title="previewState.name" class="h-[70vh] w-full rounded border border-gray-200" /></div></BaseDialog>
    <BaseDialog :show="rejectOpen" title="驳回发票申请" @close="rejectOpen = false"><textarea v-model="rejectReason" rows="4" class="input w-full" placeholder="填写驳回原因" /><template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="rejectOpen = false">取消</button><button class="btn btn-danger" :disabled="actionLoading || !rejectReason.trim()" @click="reject">确认驳回</button></div></template></BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminInvoiceAPI } from '@/api/admin/invoice'
import { useAppStore } from '@/stores'
import type { InvoiceRequest, InvoiceRequestDetail, InvoiceRequestStatus, InvoiceType } from '@/types/invoice'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatPaymentAmount } from '@/components/payment/currency'

const appStore = useAppStore()
const { t } = useI18n()
const loading = ref(false)
const actionLoading = ref(false)
const status = ref('')
const keyword = ref('')
const requests = ref<InvoiceRequest[]>([])
const detail = ref<InvoiceRequestDetail | null>(null)
const rejectOpen = ref(false)
const rejectReason = ref('')
const issueForm = reactive({ invoiceNumber: '', issuedAt: '', remark: '' })
const previewState = ref<{ url: string; name: string; contentType: string } | null>(null)
const statusOptions = computed(() => [{ value: '', label: '全部状态' }, { value: 'SUBMITTED', label: '待审批' }, { value: 'APPROVED', label: '已审批' }, { value: 'ISSUED', label: '已开票' }, { value: 'SENT', label: '已发送' }, { value: 'REJECTED', label: '已驳回' }])

async function load() { loading.value = true; try { const response = await adminInvoiceAPI.list({ status: status.value || undefined, keyword: keyword.value.trim() || undefined }); requests.value = response.data.items || [] } catch { appStore.showError('无法加载发票申请') } finally { loading.value = false } }
function clearSearch() { keyword.value = ''; status.value = ''; load() }
// datetime-local 不带时区，按浏览器本地时间初始化和回显。
function currentDateTimeLocal() { const now = new Date(); const offset = now.getTimezoneOffset(); return new Date(now.getTime() - offset * 60000).toISOString().slice(0, 16) }
function toDateTimeLocal(value: string) { const date = new Date(value); const offset = date.getTimezoneOffset(); return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 16) }
async function openDetail(id: number) { try { const response = await adminInvoiceAPI.get(id); detail.value = response.data; issueForm.invoiceNumber = response.data.request.invoice_number || ''; issueForm.issuedAt = response.data.request.issued_at ? toDateTimeLocal(response.data.request.issued_at) : currentDateTimeLocal(); issueForm.remark = response.data.request.issue_remark || '' } catch { appStore.showError('无法加载发票申请详情') } }
async function refreshDetail() { if (detail.value) await openDetail(detail.value.request.id) }
async function approve() { if (!detail.value) return; actionLoading.value = true; try { await adminInvoiceAPI.approve(detail.value.request.id); appStore.showSuccess('已审批'); await refreshDetail(); await load() } catch { appStore.showError('审批失败') } finally { actionLoading.value = false } }
async function reject() { if (!detail.value || !rejectReason.value.trim()) return; actionLoading.value = true; try { await adminInvoiceAPI.reject(detail.value.request.id, rejectReason.value.trim()); rejectOpen.value = false; rejectReason.value = ''; appStore.showSuccess('已驳回'); await refreshDetail(); await load() } catch { appStore.showError('驳回失败') } finally { actionLoading.value = false } }
async function upload(event: Event) { const file = (event.target as HTMLInputElement).files?.[0]; if (!file || !detail.value) return; actionLoading.value = true; try { await adminInvoiceAPI.uploadAttachment(detail.value.request.id, file); appStore.showSuccess('附件已上传'); await refreshDetail() } catch { appStore.showError('附件上传失败') } finally { actionLoading.value = false; (event.target as HTMLInputElement).value = '' } }
async function issue() { if (!detail.value) return; actionLoading.value = true; try { await adminInvoiceAPI.issue(detail.value.request.id, { invoice_number: issueForm.invoiceNumber.trim(), issued_at: issueForm.issuedAt ? new Date(issueForm.issuedAt).toISOString() : undefined, remark: issueForm.remark.trim() || undefined }); appStore.showSuccess('发票已开具'); await refreshDetail(); await load() } catch { appStore.showError('开票失败') } finally { actionLoading.value = false } }
async function send() { if (!detail.value) return; actionLoading.value = true; try { await adminInvoiceAPI.send(detail.value.request.id); appStore.showSuccess('发票邮件已发送'); await refreshDetail(); await load() } catch { appStore.showError('邮件发送失败') } finally { actionLoading.value = false } }
async function download(id: number, name: string) { try { const response = await adminInvoiceAPI.downloadAttachment(id); const url = URL.createObjectURL(response.data); const link = document.createElement('a'); link.href = url; link.download = name; link.click(); URL.revokeObjectURL(url) } catch { appStore.showError('附件下载失败') } }
async function preview(id: number, name: string, contentType: string) { try { const response = await adminInvoiceAPI.downloadAttachment(id, true); if (previewState.value) URL.revokeObjectURL(previewState.value.url); previewState.value = { url: URL.createObjectURL(response.data), name, contentType } } catch { appStore.showError('附件预览失败') } }
function closePreview() { if (previewState.value) URL.revokeObjectURL(previewState.value.url); previewState.value = null }
function amount(value: number, currency: string) { return formatPaymentAmount(value, currency) }
function date(value: string) { return new Date(value).toLocaleString() }
function invoiceTypeLabel(type: InvoiceType) { return type === 'ENTERPRISE' ? t('admin.invoices.enterprise') : t('admin.invoices.personal') }
function statusLabel(status: InvoiceRequestStatus) { return ({ SUBMITTED: t('admin.invoices.submitted'), REJECTED: t('admin.invoices.rejected'), APPROVED: t('admin.invoices.approved'), ISSUED: t('admin.invoices.issued'), SENT: t('admin.invoices.sent') } as Record<InvoiceRequestStatus, string>)[status] }
onMounted(load)
</script>
