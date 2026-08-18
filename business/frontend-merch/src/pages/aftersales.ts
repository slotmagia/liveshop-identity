import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshop/design-tokens'

interface AftersaleItem {
  id: number
  skuId: number
  title: string
  quantity: number
  refundAmount: number
  receivedQuantity: number
}

interface Aftersale {
  id: number
  customerSubject: string
  orderId: number
  paymentNo: string
  type: 'REFUND_ONLY' | 'RETURN_REFUND' | string
  requestedAmount: number
  amount: number
  reason: string
  status: 'PENDING' | 'APPROVED' | 'REJECTED' | 'REFUNDED' | 'CLOSED' | string
  returnStatus: 'NOT_REQUIRED' | 'PENDING' | 'RECEIVED' | string
  handleNote: string
  items: AftersaleItem[]
  version: number
  createdAt: string
  updatedAt: string
  reviewedAt?: string
  receivedAt?: string
}

interface AftersalePage {
  items: Aftersale[]
  page: number
  pageSize: number
  total: number
}

interface AftersaleResult {
  aftersale: Aftersale
  replayed?: boolean
}

const prefix = '/merch/identity/aftersales'
const typeLabels: Record<string, string> = {
  REFUND_ONLY: '仅退款',
  RETURN_REFUND: '退货退款',
}
const statusLabels: Record<string, string> = {
  PENDING: '待审核',
  APPROVED: '已通过',
  REJECTED: '已驳回',
  REFUNDED: '已退款',
  CLOSED: '已关闭',
}
const returnLabels: Record<string, string> = {
  NOT_REQUIRED: '无需退货',
  PENDING: '待收货',
  RECEIVED: '已收货',
}

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function statusBadge(status: Aftersale['status']): HTMLElement {
  if (status === 'PENDING') return badge({ label: '待审核', tone: 'warning' })
  if (status === 'APPROVED') return badge({ label: '已通过', tone: 'success' })
  if (status === 'REJECTED') return badge({ label: '已驳回', tone: 'danger' })
  if (status === 'REFUNDED') return badge({ label: '已退款', tone: 'success' })
  return badge({ label: statusLabels[status] || status || '—', tone: 'neutral' })
}

function returnBadge(status: Aftersale['returnStatus']): HTMLElement {
  if (status === 'RECEIVED') return badge({ label: '已收货', tone: 'success' })
  if (status === 'PENDING') return badge({ label: '待收货', tone: 'warning' })
  return badge({ label: returnLabels[status] || status || '—', tone: 'neutral' })
}

function canReceive(item: Aftersale): boolean {
  return item.type === 'RETURN_REFUND' && item.returnStatus === 'PENDING' &&
    (item.status === 'APPROVED' || item.status === 'REFUNDED')
}

export async function renderAftersales(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const aftersaleTable = table({
    columns: ['客户', '类型', '订单', '金额', '状态', '退货', '时间', '操作'],
    empty: '当前店铺没有售后工单',
  })
  let rows: Aftersale[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadAftersales() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadAftersales() },
  })

  const filter = searchForm({
    fields: [
      { name: 'customerSubject', label: '客户', placeholder: '客户主体' },
      {
        name: 'status',
        label: '状态',
        kind: 'select',
        options: [
          { value: '', label: '全部状态' },
          { value: 'PENDING', label: '待审核' },
          { value: 'APPROVED', label: '已通过' },
          { value: 'REJECTED', label: '已驳回' },
          { value: 'REFUNDED', label: '已退款' },
          { value: 'CLOSED', label: '已关闭' },
        ],
      },
      {
        name: 'type',
        label: '类型',
        kind: 'select',
        options: [
          { value: '', label: '全部类型' },
          { value: 'REFUND_ONLY', label: '仅退款' },
          { value: 'RETURN_REFUND', label: '退货退款' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadAftersales()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ customerSubject: '', status: '', type: '' })
      void loadAftersales()
    },
  })

  function query(): string {
    const params = new URLSearchParams()
    const values = filter.values()
    const customerSubject = String(values.customerSubject || '').trim()
    const status = String(values.status || '')
    const type = String(values.type || '')
    if (customerSubject) params.set('customerSubject', customerSubject)
    if (status) params.set('status', status)
    if (type) params.set('type', type)
    params.set('page', String(currentPage))
    params.set('pageSize', String(currentPageSize))
    return `${prefix}?${params.toString()}`
  }

  function renderRows(): void {
    aftersaleTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '查看', size: 'sm', variant: 'secondary', onClick: () => void openDetail(item) }),
      ]
      if (item.status === 'PENDING') {
        buttons.push(button({ label: '审核', size: 'sm', onClick: () => void openReview(item) }))
      }
      if (canReceive(item)) {
        buttons.push(button({ label: '确认收货', size: 'sm', onClick: () => void openReceive(item) }))
      }
      return [
        item.customerSubject || '—',
        typeLabels[item.type] || item.type || '—',
        item.orderId > 0 ? `#${item.orderId}` : '—',
        String(item.amount ?? 0),
        statusBadge(item.status),
        returnBadge(item.returnStatus),
        formatTime(item.createdAt),
        actions(...buttons),
      ]
    }))
  }

  async function loadAftersales(): Promise<void> {
    filter.setBusy(true)
    try {
      const pageValue = await api.request<AftersalePage>(query())
      rows = pageValue.items || []
      currentPage = pageValue.page || 1
      currentPageSize = pageValue.pageSize || currentPageSize
      pager.set({ page: currentPage, pageSize: currentPageSize, total: pageValue.total || 0 })
      renderRows()
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      notify(`售后工单加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadOne(id: number): Promise<Aftersale | undefined> {
    try {
      const value = await api.request<{ aftersale: Aftersale }>(`${prefix}/${id}`)
      return value.aftersale
    } catch (error) {
      notify(`售后详情加载失败：${String(error)}`, 'danger')
      return undefined
    }
  }

  async function openDetail(row: Aftersale): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    const lines = (item.items || []).map(line => `${line.title || `SKU ${line.skuId}`} ×${line.quantity} / ${line.refundAmount}`).join('\n')
    const modal = hostFormModal({
      title: `售后 #${item.id}`,
      fields: [
        { name: 'customerSubject', label: '客户', disabled: true },
        { name: 'type', label: '类型', disabled: true },
        { name: 'orderId', label: '订单', disabled: true },
        { name: 'paymentNo', label: '支付单号', disabled: true },
        { name: 'amount', label: '金额', disabled: true },
        { name: 'status', label: '状态', disabled: true },
        { name: 'returnStatus', label: '退货', disabled: true },
        { name: 'reason', label: '原因', kind: 'textarea', disabled: true },
        { name: 'items', label: '行项目', kind: 'textarea', disabled: true },
        { name: 'handleNote', label: '审核说明', kind: 'textarea', disabled: true },
        { name: 'createdAt', label: '创建时间', disabled: true },
        { name: 'reviewedAt', label: '审核时间', disabled: true },
        { name: 'receivedAt', label: '收货时间', disabled: true },
      ],
      submitLabel: '关闭',
      onSubmit: (_values, editor) => editor.close(),
    })
    modal.open({
      customerSubject: item.customerSubject,
      type: typeLabels[item.type] || item.type,
      orderId: item.orderId > 0 ? String(item.orderId) : '—',
      paymentNo: item.paymentNo || '—',
      amount: `${item.amount} / 申请 ${item.requestedAmount}`,
      status: statusLabels[item.status] || item.status,
      returnStatus: returnLabels[item.returnStatus] || item.returnStatus,
      reason: item.reason,
      items: lines || '—',
      handleNote: item.handleNote || '—',
      createdAt: formatTime(item.createdAt),
      reviewedAt: formatTime(item.reviewedAt),
      receivedAt: formatTime(item.receivedAt),
    })
  }

  async function openReview(row: Aftersale): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    if (item.status !== 'PENDING') {
      notify('该售后已审核，不能再次处理。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `审核售后 #${item.id}`,
      fields: [
        { name: 'customerSubject', label: '客户', disabled: true },
        { name: 'reason', label: '原因', kind: 'textarea', disabled: true },
        {
          name: 'status',
          label: '审核结果',
          kind: 'select',
          required: true,
          options: [
            { value: 'APPROVED', label: '通过' },
            { value: 'REJECTED', label: '驳回' },
          ],
        },
        { name: 'amount', label: '批准金额', placeholder: `不超过 ${item.requestedAmount}` },
        { name: 'handleNote', label: '审核说明', kind: 'textarea', required: true, placeholder: '1–2000 个字符' },
      ],
      submitLabel: '提交审核',
      onSubmit: (values, editor) => {
        const handleNote = values.handleNote.trim()
        if ([...handleNote].length < 1 || [...handleNote].length > 2000) {
          editor.setError('审核说明长度为 1–2000 个字符。')
          return
        }
        const amountText = values.amount.trim()
        const amount = amountText ? Number(amountText) : 0
        if (amountText && (!Number.isInteger(amount) || amount < 1 || amount > item.requestedAmount)) {
          editor.setError(`批准金额须为 1–${item.requestedAmount} 的整数，留空则使用申请额。`)
          return
        }
        editor.setBusy(true)
        api.request<AftersaleResult>(`${prefix}/${item.id}/review`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: item.version,
            status: values.status,
            amount: values.status === 'APPROVED' ? amount : 0,
            handleNote,
          }),
        })
          .then(() => { editor.close(); return loadAftersales() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      customerSubject: item.customerSubject,
      reason: item.reason,
      status: 'APPROVED',
      amount: String(item.requestedAmount),
      handleNote: '',
    })
  }

  async function openReceive(row: Aftersale): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    if (!canReceive(item)) {
      notify('当前状态不能确认收货。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `确认收货 #${item.id}`,
      fields: [
        { name: 'customerSubject', label: '客户', disabled: true },
        { name: 'orderId', label: '订单', disabled: true },
        { name: 'hint', label: '说明', kind: 'textarea', disabled: true },
      ],
      submitLabel: '确认收货',
      onSubmit: (_values, editor) => {
        editor.setBusy(true)
        api.request<AftersaleResult>(`${prefix}/${item.id}/returns`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: item.version,
          }),
        })
          .then(() => { editor.close(); return loadAftersales() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      customerSubject: item.customerSubject,
      orderId: item.orderId > 0 ? String(item.orderId) : '—',
      hint: '本操作只记录退货已签收，不会回补库存或执行退款。',
    })
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '售后工单',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void loadAftersales() })],
        body: aftersaleTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadAftersales()
}
