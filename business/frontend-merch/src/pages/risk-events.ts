import type { HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table } from '@liveshops/design-tokens'

interface RiskEvent {
  id: number
  visitorId: string
  nickname: string
  roomId: number
  reason: string
  scoreBefore: number
  scoreAfterDecay: number
  scoreDelta: number
  scoreAfter: number
  currentScore: number
  currentLevel: 'NONE' | 'LOW' | 'MEDIUM' | 'HIGH' | string
  visitorStatus: 'NORMAL' | 'WATCH' | 'RESTRICTED' | 'BLOCKED' | string
  createdAt: string
}

interface RiskEventPage {
  items: RiskEvent[]
  page: number
  pageSize: number
  total: number
}

const prefix = '/merch/identity/risk-events'

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatDelta(value: number): string {
  if (value > 0) return `+${value}`
  return String(value)
}

function levelBadge(level: RiskEvent['currentLevel']): HTMLElement {
  if (level === 'HIGH') return badge({ label: '高', tone: 'danger' })
  if (level === 'MEDIUM') return badge({ label: '中', tone: 'warning' })
  if (level === 'LOW') return badge({ label: '低', tone: 'info' })
  if (level === 'NONE') return badge({ label: '无', tone: 'neutral' })
  return badge({ label: level || '—', tone: 'neutral' })
}

function statusBadge(status: RiskEvent['visitorStatus']): HTMLElement {
  if (status === 'BLOCKED') return badge({ label: '拦截', tone: 'danger' })
  if (status === 'RESTRICTED') return badge({ label: '限制', tone: 'warning' })
  if (status === 'WATCH') return badge({ label: '关注', tone: 'info' })
  if (status === 'NORMAL') return badge({ label: '正常', tone: 'success' })
  return badge({ label: status || '—', tone: 'neutral' })
}

export async function renderRiskEvents(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const eventTable = table({
    columns: ['游客', '昵称', '房间', '原因', '分数变化', '当前分', '等级', '状态', '时间'],
    empty: '当前店铺没有风控记录',
  })
  let rows: RiskEvent[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadEvents() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadEvents() },
  })

  const filter = searchForm({
    fields: [
      { name: 'visitorId', label: '游客', placeholder: '游客标识' },
      { name: 'roomId', label: '房间', placeholder: '房间编号' },
      { name: 'reason', label: '原因', placeholder: '触发原因' },
      {
        name: 'visitorStatus',
        label: '状态',
        kind: 'select',
        options: [
          { value: '', label: '全部状态' },
          { value: 'NORMAL', label: '正常' },
          { value: 'WATCH', label: '关注' },
          { value: 'RESTRICTED', label: '限制' },
          { value: 'BLOCKED', label: '拦截' },
        ],
      },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadEvents()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ visitorId: '', roomId: '', reason: '', visitorStatus: '' })
      void loadEvents()
    },
  })

  function query(): string {
    const params = new URLSearchParams()
    const values = filter.values()
    const visitorId = String(values.visitorId || '').trim()
    const roomId = String(values.roomId || '').trim()
    const reason = String(values.reason || '').trim()
    const visitorStatus = String(values.visitorStatus || '')
    if (visitorId) params.set('visitorId', visitorId)
    if (roomId) params.set('roomId', roomId)
    if (reason) params.set('reason', reason)
    if (visitorStatus) params.set('visitorStatus', visitorStatus)
    params.set('page', String(currentPage))
    params.set('pageSize', String(currentPageSize))
    return `${prefix}?${params.toString()}`
  }

  function renderRows(): void {
    eventTable.setRows(rows.map(item => [
      item.visitorId || '—',
      item.nickname || '—',
      item.roomId ? String(item.roomId) : '—',
      item.reason || '—',
      `${item.scoreBefore} → ${item.scoreAfter} (${formatDelta(item.scoreDelta)})`,
      String(item.currentScore),
      levelBadge(item.currentLevel),
      statusBadge(item.visitorStatus),
      formatTime(item.createdAt),
    ]))
  }

  async function loadEvents(): Promise<void> {
    filter.setBusy(true)
    try {
      const pageValue = await api.request<RiskEventPage>(query())
      rows = pageValue.items || []
      currentPage = pageValue.page || 1
      currentPageSize = pageValue.pageSize || currentPageSize
      pager.set({ page: currentPage, pageSize: currentPageSize, total: pageValue.total || 0 })
      renderRows()
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      notify(`风控记录加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '风控事件',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void loadEvents() })],
        body: eventTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadEvents()
}
