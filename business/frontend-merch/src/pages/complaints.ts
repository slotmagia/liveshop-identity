import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshop/design-tokens'

interface Complaint {
  id: number
  customerSubject: string
  targetType: 'ORDER' | 'AFTERSALE' | 'LIVE' | 'PRODUCT' | 'OTHER' | string
  targetId: number
  reasonCode: string
  content: string
  status: 'OPEN' | 'ACCEPTED' | 'REJECTED' | string
  handleNote: string
  version: number
  createdAt: string
  updatedAt: string
  handledAt?: string
}

interface ComplaintPage {
  items: Complaint[]
  page: number
  pageSize: number
  total: number
}

interface ComplaintResult {
  complaint: Complaint
  replayed?: boolean
}

const prefix = '/merch/identity/complaints'
const targetLabels: Record<string, string> = {
  ORDER: '订单',
  AFTERSALE: '售后',
  LIVE: '直播',
  PRODUCT: '商品',
  OTHER: '其他',
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

function targetLabel(type: string, id: number): string {
  const name = targetLabels[type] || type || '—'
  return id > 0 ? `${name} #${id}` : name
}

function statusBadge(status: Complaint['status']): HTMLElement {
  if (status === 'OPEN') return badge({ label: '待处理', tone: 'warning' })
  if (status === 'ACCEPTED') return badge({ label: '已受理', tone: 'success' })
  if (status === 'REJECTED') return badge({ label: '已驳回', tone: 'danger' })
  return badge({ label: status || '—', tone: 'neutral' })
}

export async function renderComplaints(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const complaintTable = table({
    columns: ['客户', '对象', '原因', '内容', '状态', '时间', '操作'],
    empty: '当前店铺没有投诉',
  })
  let rows: Complaint[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadComplaints() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadComplaints() },
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
          { value: 'OPEN', label: '待处理' },
          { value: 'ACCEPTED', label: '已受理' },
          { value: 'REJECTED', label: '已驳回' },
        ],
      },
      {
        name: 'targetType',
        label: '对象',
        kind: 'select',
        options: [
          { value: '', label: '全部对象' },
          { value: 'ORDER', label: '订单' },
          { value: 'AFTERSALE', label: '售后' },
          { value: 'LIVE', label: '直播' },
          { value: 'PRODUCT', label: '商品' },
          { value: 'OTHER', label: '其他' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadComplaints()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ customerSubject: '', status: '', targetType: '' })
      void loadComplaints()
    },
  })

  function query(): string {
    const params = new URLSearchParams()
    const values = filter.values()
    const customerSubject = String(values.customerSubject || '').trim()
    const status = String(values.status || '')
    const targetType = String(values.targetType || '')
    if (customerSubject) params.set('customerSubject', customerSubject)
    if (status) params.set('status', status)
    if (targetType) params.set('targetType', targetType)
    params.set('page', String(currentPage))
    params.set('pageSize', String(currentPageSize))
    return `${prefix}?${params.toString()}`
  }

  function renderRows(): void {
    complaintTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '查看', size: 'sm', variant: 'secondary', onClick: () => void openDetail(item) }),
      ]
      if (item.status === 'OPEN') {
        buttons.push(button({ label: '处理', size: 'sm', onClick: () => void openReview(item) }))
      }
      return [
        item.customerSubject || '—',
        targetLabel(item.targetType, item.targetId),
        item.reasonCode || '—',
        item.content || '—',
        statusBadge(item.status),
        formatTime(item.createdAt),
        actions(...buttons),
      ]
    }))
  }

  async function loadComplaints(): Promise<void> {
    filter.setBusy(true)
    try {
      const pageValue = await api.request<ComplaintPage>(query())
      rows = pageValue.items || []
      currentPage = pageValue.page || 1
      currentPageSize = pageValue.pageSize || currentPageSize
      pager.set({ page: currentPage, pageSize: currentPageSize, total: pageValue.total || 0 })
      renderRows()
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      notify(`投诉加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadOne(id: number): Promise<Complaint | undefined> {
    try {
      const value = await api.request<{ complaint: Complaint }>(`${prefix}/${id}`)
      return value.complaint
    } catch (error) {
      notify(`投诉详情加载失败：${String(error)}`, 'danger')
      return undefined
    }
  }

  async function openDetail(row: Complaint): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    const modal = hostFormModal({
      title: `投诉 #${item.id}`,
      fields: [
        { name: 'customerSubject', label: '客户', disabled: true },
        { name: 'target', label: '对象', disabled: true },
        { name: 'reasonCode', label: '原因', disabled: true },
        { name: 'status', label: '状态', disabled: true },
        { name: 'content', label: '内容', kind: 'textarea', disabled: true },
        { name: 'handleNote', label: '处理说明', kind: 'textarea', disabled: true },
        { name: 'createdAt', label: '创建时间', disabled: true },
        { name: 'handledAt', label: '处理时间', disabled: true },
      ],
      submitLabel: '关闭',
      onSubmit: (_values, editor) => editor.close(),
    })
    modal.open({
      customerSubject: item.customerSubject,
      target: targetLabel(item.targetType, item.targetId),
      reasonCode: item.reasonCode,
      status: item.status === 'OPEN' ? '待处理' : item.status === 'ACCEPTED' ? '已受理' : item.status === 'REJECTED' ? '已驳回' : item.status,
      content: item.content,
      handleNote: item.handleNote || '—',
      createdAt: formatTime(item.createdAt),
      handledAt: formatTime(item.handledAt),
    })
  }

  async function openReview(row: Complaint): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    if (item.status !== 'OPEN') {
      notify('该投诉已处理，不能再次审核。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `处理投诉 #${item.id}`,
      fields: [
        { name: 'customerSubject', label: '客户', disabled: true },
        { name: 'content', label: '内容', kind: 'textarea', disabled: true },
        {
          name: 'status',
          label: '处理结果',
          kind: 'select',
          required: true,
          options: [
            { value: 'ACCEPTED', label: '受理' },
            { value: 'REJECTED', label: '驳回' },
          ],
        },
        { name: 'handleNote', label: '处理说明', kind: 'textarea', required: true, placeholder: '1–2000 个字符' },
      ],
      submitLabel: '提交处理',
      onSubmit: (values, editor) => {
        const handleNote = values.handleNote.trim()
        if ([...handleNote].length < 1 || [...handleNote].length > 2000) {
          editor.setError('处理说明长度为 1–2000 个字符。')
          return
        }
        editor.setBusy(true)
        api.request<ComplaintResult>(`${prefix}/${item.id}/review`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: item.version,
            status: values.status,
            handleNote,
          }),
        })
          .then(() => { editor.close(); return loadComplaints() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      customerSubject: item.customerSubject,
      content: item.content,
      status: 'ACCEPTED',
      handleNote: '',
    })
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '投诉工单',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void loadComplaints() })],
        body: complaintTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadComplaints()
}
