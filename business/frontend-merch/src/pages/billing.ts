import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshops/design-tokens'

interface BillingOrder {
  orderNo: string
  planId: number
  planCode: string
  planName: string
  priceMinor: number
  durationDays: number
  status: 'PENDING' | 'PAID' | 'CANCELLED' | string
  payNo: string
  channelCode: string
  createdAt: string
  paidAt: string
}

interface BillingPage {
  items: BillingOrder[]
  page: number
  pageSize: number
  total: number
  owner: boolean
}

const prefix = '/merch/identity/subscription/orders'

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function formatPrice(minor: number): string {
  return `¥${(minor / 100).toFixed(2)}`
}

function formatDuration(days: number): string {
  return days === 0 ? '永久' : `${days} 天`
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function statusBadge(status: string): HTMLElement {
  if (status === 'PAID') return badge({ label: '已支付', tone: 'success' })
  if (status === 'CANCELLED') return badge({ label: '已关闭', tone: 'neutral' })
  if (status === 'PENDING') return badge({ label: '待支付', tone: 'warning' })
  return badge({ label: status || '未知', tone: 'neutral' })
}

export async function renderBilling(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canPurchase = context.permissions.includes('identity.subscription.purchase')
  const orderTable = table({
    columns: ['订单号', '套餐', '价格', '周期', '状态', '支付单号', '通道', '创建时间', '操作'],
    empty: '暂无账单',
  })
  let rows: BillingOrder[] = []
  let owner = false
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadOrders() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadOrders() },
  })

  const filter = searchForm({
    fields: [
      {
        name: 'status',
        label: '状态',
        kind: 'select',
        options: [
          { value: '', label: '全部状态' },
          { value: 'PENDING', label: '待支付' },
          { value: 'PAID', label: '已支付' },
          { value: 'CANCELLED', label: '已关闭' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadOrders()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ status: '' })
      void loadOrders()
    },
  })

  function canClose(item: BillingOrder): boolean {
    return owner && canPurchase && item.status === 'PENDING'
  }

  function renderRows(): void {
    orderTable.setRows(rows.map(item => {
      const buttons: HTMLElement[] = []
      if (canClose(item)) {
        buttons.push(button({ label: '关闭', size: 'sm', variant: 'secondary', onClick: () => openClose(item) }))
      }
      return [
        item.orderNo,
        item.planName || item.planCode || '—',
        formatPrice(item.priceMinor),
        formatDuration(item.durationDays),
        statusBadge(item.status),
        item.payNo || '—',
        item.channelCode || '—',
        formatTime(item.createdAt),
        buttons.length ? actions(...buttons) : '—',
      ]
    }))
  }

  async function loadOrders(): Promise<void> {
    filter.setBusy(true)
    try {
      const values = filter.values()
      const query = new URLSearchParams({
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      if (values.status) query.set('status', values.status)
      const pageData = await api.request<BillingPage>(`${prefix}?${query.toString()}`)
      rows = pageData.items ?? []
      owner = pageData.owner
      pager.set({ page: pageData.page, pageSize: pageData.pageSize, total: pageData.total })
      renderRows()
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      notify(`账单加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  function openClose(item: BillingOrder): void {
    const modal = hostFormModal({
      title: `关闭 ${item.orderNo}`,
      fields: [{
        name: 'confirm',
        label: '关闭后订单进入终态，不能再支付。同一套餐随后可以重新下单。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: item.orderNo, label: '确认关闭' }],
      }],
      submitLabel: '关闭',
      onSubmit: (values, editor) => {
        if (values.confirm !== item.orderNo) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<BillingOrder>(`${prefix}/${encodeURIComponent(item.orderNo)}/close`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID() }),
        })
          .then(() => {
            editor.close()
            notify(`已关闭 ${item.orderNo}`, 'success')
            return loadOrders()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const toolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadOrders() })]

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '购买订单',
        actions: toolbar,
        body: orderTable.element,
        footer: pager.element,
      }),
    ],
  }))

  await loadOrders()
}
