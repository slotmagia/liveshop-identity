import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshops/design-tokens'

interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
interface Policy {
  id: number
  merchantId: number
  shopId: number
  policyType: 'privacy' | 'terms' | 'refund' | 'shipping'
  title: string
  content: string
  versionNo: number
  status: 'DRAFT' | 'PUBLISHED' | 'ARCHIVED'
  version: number
  publishedAt?: string
  createdAt: string
  updatedAt: string
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
  editable: boolean
}
interface PolicyPage {
  items: Policy[]
  page: number
  pageSize: number
  total: number
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
}

const prefix = '/merch/identity/policies'
const policyTypes: Array<{ value: Policy['policyType']; label: string }> = [
  { value: 'privacy', label: '隐私政策' },
  { value: 'terms', label: '服务条款' },
  { value: 'refund', label: '退款政策' },
  { value: 'shipping', label: '配送政策' },
]

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function setSelectOptions(
  control: HTMLSelectElement | undefined,
  values: Array<{ value: string; label: string }>,
  emptyLabel: string,
): void {
  if (!control) return
  control.replaceChildren()
  const empty = document.createElement('option')
  empty.value = ''
  empty.textContent = values.length ? emptyLabel : '暂无店铺'
  control.append(empty)
  control.disabled = !values.length
  for (const value of values) {
    const option = document.createElement('option')
    option.value = value.value
    option.textContent = value.label
    control.append(option)
  }
  ;(control as HTMLSelectElement & { refreshSearchSelect?: () => void }).refreshSearchSelect?.()
}

function typeLabel(value: Policy['policyType']): string {
  return policyTypes.find(item => item.value === value)?.label ?? value
}

function statusBadge(status: Policy['status']): HTMLElement {
  if (status === 'PUBLISHED') return badge({ label: '已发布', tone: 'success' })
  if (status === 'ARCHIVED') return badge({ label: '已归档', tone: 'neutral' })
  return badge({ label: '草稿', tone: 'warning' })
}

function platformBadge(status: Policy['platformStatus']): HTMLElement {
  if (status === 'restricted') return badge({ label: '平台限制', tone: 'warning' })
  if (status === 'suspended') return badge({ label: '平台暂停', tone: 'danger' })
  return badge({ label: '平台正常', tone: 'success' })
}

function truncate(value: string, max = 80): string {
  const text = value.replace(/\s+/g, ' ').trim()
  return [...text].length > max ? `${[...text].slice(0, max).join('')}…` : text || '—'
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

export async function renderPolicies(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.policy.manage')
  const state = statusLine()
  const policyTable = table({
    columns: ['类型', '标题', '版本', '商户状态', '平台叠加', '更新时间', '摘要', '操作'],
    empty: '暂无政策版本',
  })
  let shops: Shop[] = []
  let rows: Policy[] = []
  let overlayStatus: Policy['platformStatus'] = 'active'
  let overlayReason = ''
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadPolicies() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadPolicies() },
  })

  const filter = searchForm({
    fields: [
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '请选择店铺' }] },
      { name: 'policyType', label: '政策类型', kind: 'select', options: [
        { value: '', label: '全部类型' },
        ...policyTypes,
      ] },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'DRAFT', label: '草稿' },
        { value: 'PUBLISHED', label: '已发布' },
        { value: 'ARCHIVED', label: '已归档' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadPolicies()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ shopId: defaultShopId(), policyType: '', status: '' })
      void loadPolicies()
    },
  })
  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined

  function defaultShopId(): string {
    const tenantShop = context.tenant?.shopId
    if (tenantShop && shops.some(item => item.shopId === tenantShop)) return String(tenantShop)
    return shops.length === 1 ? String(shops[0].shopId) : ''
  }

  function selectedShopId(): number {
    return Number(filter.values().shopId) || 0
  }

  function overlayActive(): boolean {
    return overlayStatus === 'active'
  }

  function renderRows(): void {
    policyTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '详情', size: 'sm', variant: 'secondary', onClick: () => openDetail(item) }),
      ]
      if (canManage && item.editable) {
        buttons.push(button({ label: '发布', size: 'sm', onClick: () => openPublish(item) }))
      }
      return [
        typeLabel(item.policyType),
        item.title,
        `v${item.versionNo}`,
        statusBadge(item.status),
        platformBadge(item.platformStatus),
        formatTime(item.updatedAt),
        truncate(item.content),
        actions(...buttons),
      ]
    }))
  }

  async function loadShops(): Promise<void> {
    shops = await api.request<Shop[]>(`${prefix}/shops`)
    setSelectOptions(shopSelect, shops.map(item => ({
      value: String(item.shopId),
      label: `${item.name} · ${item.code} · shop_id ${item.shopId}`,
    })), '请选择店铺')
    const shopId = defaultShopId()
    if (shopId) filter.set({ shopId })
  }

  async function loadPolicies(): Promise<void> {
    const shopId = selectedShopId()
    if (!shopId) {
      rows = []
      overlayStatus = 'active'
      overlayReason = ''
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      state.set(shops.length ? '请选择店铺后查看政策版本。' : '当前商户没有可管理的店铺。', shops.length ? 'warning' : 'danger')
      return
    }
    state.set('正在加载政策版本…')
    try {
      const query = new URLSearchParams({
        shopId: String(shopId),
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      const values = filter.values()
      if (values.policyType) query.set('policyType', values.policyType)
      if (values.status) query.set('status', values.status)
      const pageResult = await api.request<PolicyPage>(`${prefix}?${query}`)
      rows = pageResult.items
      overlayStatus = pageResult.platformStatus || 'active'
      overlayReason = pageResult.platformReasonPublic || ''
      currentPage = pageResult.page
      currentPageSize = pageResult.pageSize
      renderRows()
      pager.set({ page: pageResult.page, pageSize: pageResult.pageSize, total: pageResult.total })
      const shop = shops.find(item => item.shopId === shopId)
      const overlay = overlayActive() ? '平台正常' : overlayStatus === 'restricted' ? '平台限制' : '平台暂停'
      state.set(`${shop?.name ?? `店铺 ${shopId}`} · ${overlay}${overlayReason ? ` · ${overlayReason}` : ''}`)
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      state.set(`政策加载失败：${String(error)}`, 'danger')
    }
  }

  function openEditor(): void {
    const shopId = selectedShopId()
    if (!shopId) {
      state.set('请先选择店铺。', 'warning')
      return
    }
    const publishable = overlayActive()
    const modal = hostFormModal({
      title: '新建政策版本',
      fields: [
        {
          name: 'policyType', label: '政策类型', kind: 'select', required: true,
          options: policyTypes,
        },
        { name: 'title', label: '标题', required: true, placeholder: '店铺服务条款' },
        { name: 'content', label: '正文（不少于 10 字）', kind: 'textarea', required: true },
        {
          name: 'saveMode', label: '保存方式', kind: 'select', required: true,
          options: publishable
            ? [{ value: 'draft', label: '保存草稿' }, { value: 'publish', label: '保存并发布' }]
            : [{ value: 'draft', label: '保存草稿（平台已限制发布）' }],
        },
      ],
      onSubmit: (values, editor) => {
        const title = values.title.trim()
        const content = values.content.trim()
        if ([...title].length < 1 || [...title].length > 255) {
          editor.setError('标题长度为 1–255 个字符。')
          return
        }
        if ([...content].length < 10 || [...content].length > 20000) {
          editor.setError('正文长度为 10–20000 个字符。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            shopId,
            policyType: values.policyType,
            title,
            content,
            publish: values.saveMode === 'publish',
          }),
        })
          .then(() => { editor.close(); return loadPolicies() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ policyType: 'privacy', title: '', content: '', saveMode: 'draft' })
  }

  function openDetail(item: Policy): void {
    const modal = hostFormModal({
      title: `${typeLabel(item.policyType)} · v${item.versionNo}`,
      fields: [
        { name: 'policyType', label: '政策类型', disabled: true },
        { name: 'title', label: '标题', disabled: true },
        { name: 'status', label: '商户状态', disabled: true },
        { name: 'platform', label: '平台叠加', disabled: true },
        { name: 'content', label: '正文', kind: 'textarea', disabled: true },
      ],
      submitLabel: '关闭',
      onSubmit: (_values, editor) => editor.close(),
    })
    modal.open({
      policyType: typeLabel(item.policyType),
      title: item.title,
      status: item.status === 'PUBLISHED' ? '已发布' : item.status === 'ARCHIVED' ? '已归档' : '草稿',
      platform: item.platformStatus === 'restricted' ? '平台限制' : item.platformStatus === 'suspended' ? '平台暂停' : '平台正常',
      content: item.content,
    })
  }

  function openPublish(item: Policy): void {
    const modal = hostFormModal({
      title: `发布 ${typeLabel(item.policyType)} · v${item.versionNo}`,
      fields: [{
        name: 'confirm',
        label: '发布后同类型当前已发布版本将归档，本版本成为店铺有效政策。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认发布 ${item.title}` }],
      }],
      submitLabel: '发布',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${item.id}/publish`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            shopId: item.shopId,
            expectedVersion: item.version,
          }),
        })
          .then(() => { editor.close(); return loadPolicies() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const toolbar = [
    button({ label: '刷新', variant: 'secondary', onClick: () => void loadPolicies() }),
  ]
  if (canManage) toolbar.push(button({ label: '新建版本', onClick: () => openEditor() }))

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '店铺政策版本',
        actions: toolbar,
        status: state.element,
        body: policyTable.element,
        footer: pager.element,
      }),
    ],
  }))

  try {
    await loadShops()
    await loadPolicies()
  } catch (error) {
    shops = []
    rows = []
    renderRows()
    state.set(`政策页面加载失败：${String(error)}`, 'danger')
  }
}
