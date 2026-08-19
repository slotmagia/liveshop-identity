import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshops/design-tokens'

interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
interface AppScope { code: string; group: string; label: string }
interface App {
  id: number
  merchantId: number
  shopId: number
  name: string
  clientId: string
  secretHint: string
  scopes: string
  status: 'ACTIVE' | 'DISABLED'
  version: number
  createdAt: string
  updatedAt: string
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
  editable: boolean
}
interface AppPage {
  items: App[]
  page: number
  pageSize: number
  total: number
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
}
interface AppResult {
  app: App
  clientSecret?: string
  replayed: boolean
}

const prefix = '/merch/identity/apps'
const groupLabels: Record<string, string> = {
  orders: '订单',
  products: '商品',
  customers: '客户',
  inventory: '库存',
  live: '直播',
}

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

function statusBadge(status: App['status']): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  return badge({ label: '停用', tone: 'warning' })
}

function platformBadge(status: App['platformStatus']): HTMLElement {
  if (status === 'restricted') return badge({ label: '平台限制', tone: 'warning' })
  if (status === 'suspended') return badge({ label: '平台暂停', tone: 'danger' })
  return badge({ label: '平台正常', tone: 'success' })
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function scopeCodes(value: string): string[] {
  return [...new Set(value.split(/[\s,]+/).map(code => code.trim()).filter(Boolean))]
}

function scopeTree(scopes: AppScope[]): Array<{ id: string; label: string; children: Array<{ id: string; value: string; label: string }> }> {
  const groups = new Map<string, { id: string; label: string; children: Array<{ id: string; value: string; label: string }> }>()
  for (const item of scopes) {
    const current = groups.get(item.group) ?? { id: `scope-group:${item.group}`, label: groupLabels[item.group] ?? item.group, children: [] }
    current.children.push({ id: `scope:${item.code}`, value: item.code, label: item.label })
    groups.set(item.group, current)
  }
  return [...groups.values()]
}

export async function renderApps(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.app.manage')
  const state = statusLine()
  const appTable = table({
    columns: ['名称', 'Client ID', '范围', '密钥尾号', '商户状态', '平台叠加', '更新时间', '操作'],
    empty: '暂无应用',
  })
  let shops: Shop[] = []
  let scopes: AppScope[] = []
  let rows: App[] = []
  let overlayStatus: App['platformStatus'] = 'active'
  let overlayReason = ''
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadApps() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadApps() },
  })

  const filter = searchForm({
    fields: [
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '请选择店铺' }] },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'ACTIVE', label: '启用' },
        { value: 'DISABLED', label: '停用' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadApps()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ shopId: defaultShopId(), status: '' })
      void loadApps()
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
    appTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '复制标识', size: 'sm', variant: 'secondary', onClick: () => copyClientId(item) }),
      ]
      if (canManage && item.editable) {
        buttons.push(button({ label: '轮换密钥', size: 'sm', onClick: () => openReset(item) }))
        if (item.status === 'DISABLED') {
          buttons.push(button({ label: '启用', size: 'sm', onClick: () => openToggle(item, true) }))
        }
      }
      if (canManage && item.status === 'ACTIVE') {
        buttons.push(button({ label: '停用', size: 'sm', variant: 'secondary', onClick: () => openToggle(item, false) }))
      }
      return [
        item.name,
        item.clientId,
        item.scopes || '—',
        `••••${item.secretHint}`,
        statusBadge(item.status),
        platformBadge(item.platformStatus),
        formatTime(item.updatedAt),
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

  async function loadApps(): Promise<void> {
    const shopId = selectedShopId()
    if (!shopId) {
      rows = []
      overlayStatus = 'active'
      overlayReason = ''
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      state.set(shops.length ? '请选择店铺后查看应用。' : '当前商户没有可管理的店铺。', shops.length ? 'warning' : 'danger')
      return
    }
    state.set('正在加载应用…')
    try {
      const query = new URLSearchParams({
        shopId: String(shopId),
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      const values = filter.values()
      if (values.status) query.set('status', values.status)
      const pageResult = await api.request<AppPage>(`${prefix}?${query}`)
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
      state.set(`应用加载失败：${String(error)}`, 'danger')
    }
  }

  function copyClientId(item: App): void {
    void navigator.clipboard.writeText(item.clientId).then(
      () => state.set(`已复制 ${item.clientId}`),
      () => state.set('复制失败，请手动选择 Client ID。', 'warning'),
    )
  }

  function showSecret(title: string, result: AppResult): void {
    const modal = hostFormModal({
      title,
      fields: [
        { name: 'name', label: '名称', disabled: true },
        { name: 'clientId', label: 'Client ID', disabled: true },
        { name: 'clientSecret', label: 'Client Secret（只显示一次）', disabled: true },
        { name: 'hint', label: '密钥尾号', disabled: true },
      ],
      submitLabel: '我已保存',
      onSubmit: (_values, editor) => editor.close(),
    })
    modal.open({
      name: result.app.name,
      clientId: result.app.clientId,
      clientSecret: result.clientSecret || '',
      hint: result.app.secretHint,
    })
  }

  function openCreate(): void {
    const shopId = selectedShopId()
    if (!shopId) {
      state.set('请先选择店铺。', 'warning')
      return
    }
    if (!overlayActive()) {
      state.set('平台已限制该店铺应用，当前不能新建。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: '新建应用',
      fields: [
        { name: 'name', label: '名称', required: true, placeholder: '订单同步' },
        {
          name: 'scopes',
          label: '授权范围',
          kind: 'checkbox-tree',
          tree: scopeTree(scopes),
          wide: true,
          columns: 2,
          empty: '当前没有可勾选的范围',
        },
      ],
      onSubmit: (values, editor) => {
        const name = values.name.trim()
        if ([...name].length < 1 || [...name].length > 120) {
          editor.setError('名称长度为 1–120 个字符。')
          return
        }
        editor.setBusy(true)
        api.request<AppResult>(`${prefix}`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            shopId,
            name,
            scopes: scopeCodes(values.scopes ?? '').join(','),
          }),
        })
          .then(async result => {
            editor.close()
            await loadApps()
            showSecret('请保存 Client Secret', result)
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ name: '', scopes: 'orders:read,products:read' })
  }

  function openReset(item: App): void {
    const modal = hostFormModal({
      title: `轮换密钥 · ${item.name}`,
      fields: [{
        name: 'confirm',
        label: '轮换后旧密钥立即失效，新密钥只显示一次。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认轮换 ${item.clientId}` }],
      }],
      submitLabel: '轮换',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<AppResult>(`${prefix}/${item.id}/reset`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            shopId: item.shopId,
            expectedVersion: item.version,
          }),
        })
          .then(async result => {
            editor.close()
            await loadApps()
            showSecret('请保存新的 Client Secret', result)
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openToggle(item: App, enabled: boolean): void {
    const action = enabled ? '启用' : '停用'
    const modal = hostFormModal({
      title: `${action} ${item.name}`,
      fields: [{
        name: 'confirm',
        label: enabled ? '启用后该应用可再次使用当前密钥访问授权范围。' : '停用后该应用立即失效，密钥仍保留。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认${action} ${item.clientId}` }],
      }],
      submitLabel: action,
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${item.id}/${enabled ? 'enable' : 'disable'}`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            shopId: item.shopId,
            expectedVersion: item.version,
          }),
        })
          .then(() => { editor.close(); return loadApps() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const toolbar = [
    button({ label: '刷新', variant: 'secondary', onClick: () => void loadApps() }),
  ]
  if (canManage) toolbar.push(button({ label: '新建应用', onClick: () => openCreate() }))

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '店铺应用',
        actions: toolbar,
        status: state.element,
        body: appTable.element,
        footer: pager.element,
      }),
    ],
  }))

  try {
    const [loadedScopes] = await Promise.all([
      api.request<AppScope[]>(`${prefix}/scopes`),
      loadShops(),
    ])
    scopes = loadedScopes
    await loadApps()
  } catch (error) {
    shops = []
    rows = []
    renderRows()
    state.set(`应用页面加载失败：${String(error)}`, 'danger')
  }
}
