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

    <BaseDialog :show="!!detail" :title="t('admin.invoices.detail')" width="wide" @close="detail = null">
      <div v-if="detail" class="space-y-5">
        <div class="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3"><div><span class="text-gray-500">申请编号</span><p class="font-mono">{{ detail.request.request_no }}</p></div><div><span class="text-gray-500">用户</span><p>#{{ detail.request.user_id }}</p></div><div><span class="text-gray-500">发票类型</span><p>{{ invoiceTypeLabel(detail.request.invoice_type) }}</p></div><div><span class="text-gray-500">状态</span><p>{{ statusLabel(detail.request.status) }}</p></div><div><span class="text-gray-500">抬头</span><p>{{ detail.request.invoice_title }}</p></div><div><span class="text-gray-500">税号</span><p>{{ detail.request.tax_id || '-' }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">开户行</span><p>{{ detail.request.bank_name || '-' }}</p></div><div v-if="detail.request.invoice_type === 'ENTERPRISE'"><span class="text-gray-500">银行账号</span><p>{{ detail.request.bank_account || '-' }}</p></div><div><span class="text-gray-500">收件邮箱</span><p>{{ detail.request.recipient_email }}</p></div></div>
        <p v-if="detail.request.rejection_reason" class="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">驳回原因：{{ detail.request.rejection_reason }}</p>
        <div><h2 class="mb-2 text-sm font-medium">合并订单</h2><div class="divide-y divide-gray-100 rounded border border-gray-200 dark:divide-dark-700 dark:border-dark-700"><div v-for="item in detail.items" :key="item.id" class="flex justify-between gap-3 px-3 py-2 text-sm"><span class="font-mono text-xs">{{ item.order_no }}</span><span>{{ item.order_type === 'subscription' ? '订阅' : '充值' }}</span><span>{{ amount(item.pay_amount, item.currency) }}</span></div><div v-if="detail.items.length" class="flex items-center justify-between gap-3 bg-gray-50 px-3 py-2 text-sm font-semibold text-gray-900 dark:bg-dark-800 dark:text-white"><span>{{ t('admin.invoices.invoiceAmount') }}</span><span>{{ amount(invoiceAmount, detail.request.currency) }}</span></div></div></div>
        <div v-if="detail.request.status === 'SUBMITTED'" class="flex flex-wrap gap-2"><button class="btn btn-primary" :disabled="actionLoading" @click="approve">审批通过</button><button class="btn btn-danger" :disabled="actionLoading" @click="rejectOpen = true">驳回</button></div>
        <div v-if="isAfterApproval" class="space-y-3 rounded border border-gray-200 p-4 dark:border-dark-700"><div class="flex flex-wrap items-center justify-between gap-3"><span class="text-sm font-medium">{{ t('admin.invoices.attachments') }}</span></div><div v-if="canManageAttachments" class="rounded-lg border border-dashed p-3 transition-colors" :class="dragActive ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/10' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'" @dragenter.prevent="dragActive = true" @dragover.prevent="dragActive = true" @dragleave.prevent="dragActive = false" @drop.prevent="handleDrop"><div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"><div class="flex items-start gap-2"><Icon name="upload" size="md" class="mt-0.5 text-gray-400" /><div><p class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('admin.invoices.uploadAttachment') }}</p><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.invoices.uploadHint') }}</p></div></div><label class="btn btn-secondary inline-flex shrink-0 cursor-pointer items-center gap-2"><Icon name="plus" size="sm" />{{ t('admin.invoices.selectFiles') }}<input class="sr-only" type="file" multiple accept="application/pdf,image/png,image/jpeg" @change="handleFileSelect" /></label></div><p v-if="uploading" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.invoices.uploading') }}</p></div><div v-if="detail.attachments.length" class="space-y-2"><div v-for="file in detail.attachments" :key="file.id" class="flex items-center justify-between gap-2 rounded border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"><span class="min-w-0 truncate">{{ file.file_name }}</span><div class="flex shrink-0 gap-2"><button class="btn btn-secondary btn-sm" @click="preview(file.id, file.file_name, file.content_type)"><Icon name="eye" size="sm" />{{ t('admin.invoices.preview') }}</button><button class="btn btn-secondary btn-sm" @click="download(file.id, file.file_name)"><Icon name="download" size="sm" />{{ t('admin.invoices.download') }}</button><button v-if="canManageAttachments" class="btn btn-danger btn-sm" :disabled="actionLoading" @click="removeAttachment(file.id)"><Icon name="trash" size="sm" />{{ t('admin.invoices.deleteAttachment') }}</button></div></div></div><p v-else class="text-sm text-gray-500">{{ t('admin.invoices.attachmentRequired') }}</p></div>
        <div v-if="detail.request.status === 'APPROVED'" class="grid gap-3 rounded border border-gray-200 p-4 md:grid-cols-3 dark:border-dark-700"><input v-model="issueForm.invoiceNumber" class="input" :placeholder="t('admin.invoices.invoiceNumberOptional')" /><input v-model="issueForm.issuedAt" type="datetime-local" class="input" :placeholder="t('admin.invoices.issuedAt')" /><input v-model="issueForm.remark" class="input" :placeholder="t('admin.invoices.issueRemark')" /><button class="btn btn-primary md:col-span-3" :disabled="actionLoading || detail.attachments.length === 0" @click="issue">{{ t('admin.invoices.issueAction') }}</button></div>
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
const dragActive = ref(false)
const uploading = ref(false)
// 后端只支持单文件字段，多文件按队列逐个上传。
const ACCEPTED_ATTACHMENT_TYPES = ['application/pdf', 'image/png', 'image/jpeg']
const isAfterApproval = computed(() => !!detail.value && ['APPROVED', 'ISSUED', 'SENT'].includes(detail.value.request.status))
const canManageAttachments = computed(() => !!detail.value && ['APPROVED', 'ISSUED'].includes(detail.value.request.status))
// 应开发票金额以明细实付金额为准，避免与申请总额快照出现偏差。
const invoiceAmount = computed(() => (detail.value?.items || []).reduce((total, item) => total + (item.pay_amount || 0), 0))
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
function isAcceptedFile(file: File) { return ACCEPTED_ATTACHMENT_TYPES.includes(file.type) }
async function uploadFiles(files: File[]) { if (!detail.value || files.length === 0) return; const rejected = files.filter(file => !isAcceptedFile(file)); if (rejected.length > 0) appStore.showError(t('admin.invoices.invalidAttachmentType')); const accepted = files.filter(isAcceptedFile); if (accepted.length === 0) return; actionLoading.value = true; uploading.value = true; let failed = 0; try { for (const file of accepted) { try { await adminInvoiceAPI.uploadAttachment(detail.value.request.id, file) } catch { failed++ } } if (failed > 0) appStore.showError(t('admin.invoices.attachmentUploadFailed')); else appStore.showSuccess(t('admin.invoices.attachmentUploaded')); await refreshDetail() } finally { uploading.value = false; actionLoading.value = false } }
function handleFileSelect(event: Event) { const input = event.target as HTMLInputElement; uploadFiles(Array.from(input.files || [])); input.value = '' }
function handleDrop(event: DragEvent) { dragActive.value = false; uploadFiles(Array.from(event.dataTransfer?.files || [])) }
async function removeAttachment(id: number) { if (!detail.value || !window.confirm(t('admin.invoices.deleteAttachmentConfirm'))) return; actionLoading.value = true; try { await adminInvoiceAPI.deleteAttachment(id); appStore.showSuccess(t('admin.invoices.attachmentDeleted')); await refreshDetail() } catch { appStore.showError(t('admin.invoices.attachmentDeleteFailed')) } finally { actionLoading.value = false } }
async function issue() { if (!detail.value) return; actionLoading.value = true; try { await adminInvoiceAPI.issue(detail.value.request.id, { invoice_number: issueForm.invoiceNumber.trim() || undefined, issued_at: issueForm.issuedAt ? new Date(issueForm.issuedAt).toISOString() : undefined, remark: issueForm.remark.trim() || undefined }); appStore.showSuccess('发票已开具'); await refreshDetail(); await load() } catch { appStore.showError('开票失败') } finally { actionLoading.value = false } }
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
