import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshop/design-tokens'

interface Session {
  id: string
  deviceName: string
  ipAddress: string
  userAgent: string
  status: 'ACTIVE' | 'REVOKED' | string
  createdAt: string
  lastRefreshedAt: string
  expiresAt: string
  current: boolean
}

interface SessionPage {
  items: Session[]
  page: number
  pageSize: number
  total: number
}

const prefix = '/merch/identity/account/sessions'

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function statusBadge(status: Session['status']): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '活动', tone: 'success' })
  if (status === 'REVOKED') return badge({ label: '已撤销', tone: 'warning' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

export async function renderDevices(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const sessionTable = table({
    columns: ['设备', 'IP', '状态', '最近活动', '到期', '操作'],
    empty: '当前账号没有未过期的登录设备',
  })
  let rows: Session[] = []
  let currentPage = 1
  let currentPageSize = 20
  let signedOut = false

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadSessions() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadSessions() },
  })

  const filter = searchForm({
    fields: [
      {
        name: 'status',
        label: '状态',
        kind: 'select',
        options: [
          { value: '', label: '全部状态' },
          { value: 'ACTIVE', label: '活动' },
          { value: 'REVOKED', label: '已撤销' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadSessions()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ status: '' })
      void loadSessions()
    },
  })

  function query(): string {
    const params = new URLSearchParams()
    const status = String(filter.values().status || '')
    if (status) params.set('status', status)
    params.set('page', String(currentPage))
    params.set('pageSize', String(currentPageSize))
    return `${prefix}?${params.toString()}`
  }

  function renderRows(): void {
    sessionTable.setRows(rows.map(item => [
      item.current
        ? actions(document.createTextNode(item.deviceName || '未知设备'), badge({ label: '当前设备', tone: 'info' }))
        : (item.deviceName || '未知设备'),
      item.ipAddress || '—',
      statusBadge(item.status),
      formatTime(item.lastRefreshedAt),
      formatTime(item.expiresAt),
      item.status === 'ACTIVE'
        ? button({
            label: item.current ? '退出当前设备' : '强制下线',
            size: 'sm',
            variant: 'danger',
            onClick: () => revoke(item),
          })
        : '—',
    ]))
  }

  async function loadSessions(): Promise<void> {
    if (signedOut) return
    state.set('正在加载登录设备…')
    try {
      const pageValue = await api.request<SessionPage>(query())
      rows = pageValue.items || []
      currentPage = pageValue.page || 1
      currentPageSize = pageValue.pageSize || currentPageSize
      pager.set({ page: currentPage, pageSize: currentPageSize, total: pageValue.total || 0 })
      renderRows()
      const current = rows.find(item => item.current)
      state.set(current ? `当前设备 ${current.deviceName || '未知设备'}` : '')
    } catch (error) {
      state.set(String(error), 'danger')
    }
  }

  function revoke(item: Session): void {
    const modal = hostFormModal({
      title: item.current ? '退出当前设备' : '强制下线',
      fields: [{
        name: 'confirm',
        label: item.current ? '退出后需要重新登录。确认退出当前设备？' : `确认强制下线「${item.deviceName || '未知设备'}」？`,
        kind: 'select',
        options: ['确认'],
        required: true,
      }],
      submitLabel: item.current ? '退出' : '下线',
      onSubmit: (_values, editor) => {
        const id = crypto.randomUUID()
        editor.setBusy(true)
        api.request<{ currentRevoked: boolean }>(`${prefix}/${encodeURIComponent(item.id)}/revoke`, {
          method: 'POST',
          body: JSON.stringify({ idempotencyKey: id, operationId: id }),
        })
          .then(result => {
            editor.close()
            if (result.currentRevoked) {
              signedOut = true
              rows = rows.map(session => session.id === item.id ? { ...session, current: false, status: 'REVOKED' } : session)
              renderRows()
              state.set('当前设备已退出，请重新登录。', 'warning')
              return
            }
            return loadSessions()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '登录设备',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void loadSessions() })],
        status: state.element,
        body: sessionTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadSessions()
}
