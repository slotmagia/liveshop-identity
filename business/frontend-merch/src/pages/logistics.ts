import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshop/design-tokens'

interface Trace {
  occurredAt: string
  node: string
}

interface Shipment {
  id: number
  orderId: number
  carrier: string
  trackingNo: string
  status: 'SHIPPED' | 'DELIVERED' | string
  traces: Trace[]
  version: number
  createdAt: string
  updatedAt: string
}

interface ShipmentPage {
  items: Shipment[]
  page: number
  pageSize: number
  total: number
}

interface ShipmentResult {
  shipment: Shipment
  replayed?: boolean
}

const prefix = '/merch/identity/shipments'

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

function statusBadge(status: Shipment['status']): HTMLElement {
  if (status === 'SHIPPED') return badge({ label: '在途', tone: 'warning' })
  if (status === 'DELIVERED') return badge({ label: '已签收', tone: 'success' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function statusLabel(status: Shipment['status']): string {
  if (status === 'SHIPPED') return '在途'
  if (status === 'DELIVERED') return '已签收'
  return status || '—'
}

function lastTrace(item: Shipment): string {
  const traces = item.traces || []
  if (!traces.length) return '—'
  return traces[traces.length - 1]?.node || '—'
}

function formatTraces(item: Shipment): string {
  const traces = item.traces || []
  if (!traces.length) return '暂无轨迹'
  return traces.map(trace => `${formatTime(trace.occurredAt)}  ${trace.node}`).join('\n')
}

export async function renderLogistics(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.shipment.manage')
  const shipmentTable = table({
    columns: ['订单', '承运商', '运单号', '状态', '轨迹', '时间', '操作'],
    empty: '当前店铺没有发货单',
  })
  let rows: Shipment[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadShipments() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadShipments() },
  })

  const filter = searchForm({
    fields: [
      { name: 'orderId', label: '订单号', placeholder: '订单编号' },
      {
        name: 'status',
        label: '状态',
        kind: 'select',
        options: [
          { value: '', label: '全部状态' },
          { value: 'SHIPPED', label: '在途' },
          { value: 'DELIVERED', label: '已签收' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadShipments()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ orderId: '', status: '' })
      void loadShipments()
    },
  })

  function query(): string {
    const params = new URLSearchParams()
    const values = filter.values()
    const orderId = String(values.orderId || '').trim()
    const status = String(values.status || '')
    if (orderId) params.set('orderId', orderId)
    if (status) params.set('status', status)
    params.set('page', String(currentPage))
    params.set('pageSize', String(currentPageSize))
    return `${prefix}?${params.toString()}`
  }

  function renderRows(): void {
    shipmentTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '查看', size: 'sm', variant: 'secondary', onClick: () => void openDetail(item) }),
      ]
      if (canManage && item.status === 'SHIPPED') {
        buttons.push(button({ label: '追加轨迹', size: 'sm', variant: 'secondary', onClick: () => void openTrace(item) }))
        buttons.push(button({ label: '确认收货', size: 'sm', onClick: () => void openClose(item) }))
      }
      return [
        item.orderId ? `#${item.orderId}` : '—',
        item.carrier || '—',
        item.trackingNo || '—',
        statusBadge(item.status),
        lastTrace(item),
        formatTime(item.updatedAt),
        actions(...buttons),
      ]
    }))
  }

  async function loadShipments(): Promise<void> {
    filter.setBusy(true)
    try {
      const pageValue = await api.request<ShipmentPage>(query())
      rows = pageValue.items || []
      currentPage = pageValue.page || 1
      currentPageSize = pageValue.pageSize || currentPageSize
      pager.set({ page: currentPage, pageSize: currentPageSize, total: pageValue.total || 0 })
      renderRows()
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      notify(`发货单加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadOne(id: number): Promise<Shipment | undefined> {
    try {
      const value = await api.request<{ shipment: Shipment }>(`${prefix}/${id}`)
      return value.shipment
    } catch (error) {
      notify(`发货单详情加载失败：${String(error)}`, 'danger')
      return undefined
    }
  }

  async function openDetail(row: Shipment): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    const modal = hostFormModal({
      title: `发货单 #${item.id}`,
      fields: [
        { name: 'orderId', label: '订单', disabled: true },
        { name: 'carrier', label: '承运商', disabled: true },
        { name: 'trackingNo', label: '运单号', disabled: true },
        { name: 'status', label: '状态', disabled: true },
        { name: 'traces', label: '轨迹', kind: 'textarea', disabled: true },
        { name: 'createdAt', label: '发货时间', disabled: true },
        { name: 'updatedAt', label: '更新时间', disabled: true },
      ],
      submitLabel: '关闭',
      onSubmit: (_values, editor) => editor.close(),
    })
    modal.open({
      orderId: `#${item.orderId}`,
      carrier: item.carrier,
      trackingNo: item.trackingNo,
      status: statusLabel(item.status),
      traces: formatTraces(item),
      createdAt: formatTime(item.createdAt),
      updatedAt: formatTime(item.updatedAt),
    })
  }

  function openCreate(): void {
    const modal = hostFormModal({
      title: '发货',
      fields: [
        { name: 'orderId', label: '订单号', required: true, placeholder: 'Trade 订单编号' },
        { name: 'carrier', label: '承运商', required: true, placeholder: '1–64 个字符' },
        { name: 'trackingNo', label: '运单号', required: true, placeholder: '1–64 个字符' },
      ],
      submitLabel: '确认发货',
      onSubmit: (values, editor) => {
        const orderId = Number(values.orderId.trim())
        const carrier = values.carrier.trim()
        const trackingNo = values.trackingNo.trim()
        if (!Number.isInteger(orderId) || orderId <= 0) {
          editor.setError('订单号必须是正整数。')
          return
        }
        if ([...carrier].length < 1 || [...carrier].length > 64 || [...trackingNo].length < 1 || [...trackingNo].length > 64) {
          editor.setError('承运商和运单号长度为 1–64 个字符。')
          return
        }
        editor.setBusy(true)
        api.request<ShipmentResult>(prefix, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), orderId, carrier, trackingNo }),
        })
          .then(() => { editor.close(); return loadShipments() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ orderId: '', carrier: '', trackingNo: '' })
  }

  async function openTrace(row: Shipment): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    if (item.status !== 'SHIPPED') {
      notify('已签收发货单不能追加轨迹。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `追加轨迹 #${item.id}`,
      fields: [
        { name: 'trackingNo', label: '运单号', disabled: true },
        { name: 'node', label: '轨迹节点', kind: 'textarea', required: true, placeholder: '1–200 个字符' },
      ],
      submitLabel: '追加',
      onSubmit: (values, editor) => {
        const node = values.node.trim()
        if ([...node].length < 1 || [...node].length > 200) {
          editor.setError('轨迹节点长度为 1–200 个字符。')
          return
        }
        editor.setBusy(true)
        api.request<ShipmentResult>(`${prefix}/${item.id}/traces`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: item.version, node }),
        })
          .then(() => { editor.close(); return loadShipments() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ trackingNo: item.trackingNo, node: '' })
  }

  async function openClose(row: Shipment): Promise<void> {
    const item = await loadOne(row.id)
    if (!item) return
    if (item.status !== 'SHIPPED') {
      notify('该发货单已确认收货。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `确认收货 #${item.id}`,
      fields: [
        { name: 'orderId', label: '订单', disabled: true },
        { name: 'trackingNo', label: '运单号', disabled: true },
        { name: 'status', label: '当前状态', disabled: true },
      ],
      submitLabel: '确认收货',
      onSubmit: (_values, editor) => {
        editor.setBusy(true)
        api.request<ShipmentResult>(`${prefix}/${item.id}/close`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: item.version }),
        })
          .then(() => { editor.close(); return loadShipments() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      orderId: `#${item.orderId}`,
      trackingNo: item.trackingNo,
      status: statusLabel(item.status),
    })
  }

  const toolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadShipments() })]
  if (canManage) toolbar.push(button({ label: '发货', onClick: openCreate }))

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '发货单',
        actions: toolbar,
        body: shipmentTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadShipments()
}
