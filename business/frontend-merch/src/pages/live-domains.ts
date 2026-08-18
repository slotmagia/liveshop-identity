import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshop/design-tokens'

interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
interface Domain {
  id: number
  merchantId: number
  shopId: number
  host: string
  scene: 'LIVE' | 'SHOP' | string
  status: 'PENDING' | 'VERIFIED' | 'FAILED' | 'DELETED' | string
  isPrimary: boolean
  txtName: string
  txtValue: string
  cnameTarget?: string
  version: number
  createdAt: string
  updatedAt: string
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
  editable: boolean
}
interface DomainPage {
  items: Domain[]
  page: number
  pageSize: number
  total: number
  cnameTarget?: string
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
}
interface DomainResult { domain: Domain; replayed: boolean }

type DomainScene = 'LIVE' | 'SHOP'

interface DomainPageCopy {
  scene: DomainScene
  title: string
  empty: string
  createTitle: string
  hostPlaceholder: string
  selectShop: string
  loadFailed: string
  pageFailed: string
}

const prefix = '/merch/identity/domains'

const liveDomainPage: DomainPageCopy = {
  scene: 'LIVE',
  title: '直播域名',
  empty: '暂无直播域名',
  createTitle: '添加直播域名',
  hostPlaceholder: 'live.example.com',
  selectShop: '请选择店铺后查看直播域名。',
  loadFailed: '直播域名加载失败',
  pageFailed: '直播域名页面加载失败',
}

const settingsDomainPage: DomainPageCopy = {
  scene: 'SHOP',
  title: '域名',
  empty: '暂无域名',
  createTitle: '添加域名',
  hostPlaceholder: 'shop.example.com',
  selectShop: '请选择店铺后查看域名。',
  loadFailed: '域名加载失败',
  pageFailed: '域名页面加载失败',
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

function statusBadge(status: Domain['status']): HTMLElement {
  if (status === 'VERIFIED') return badge({ label: '已验证', tone: 'success' })
  if (status === 'FAILED') return badge({ label: '校验失败', tone: 'danger' })
  if (status === 'PENDING') return badge({ label: '待验证', tone: 'warning' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function platformBadge(status: Domain['platformStatus']): HTMLElement {
  if (status === 'restricted') return badge({ label: '平台限制', tone: 'warning' })
  if (status === 'suspended') return badge({ label: '平台暂停', tone: 'danger' })
  return badge({ label: '平台正常', tone: 'success' })
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

export async function renderLiveDomains(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  return renderDomainPage(root, api, context, liveDomainPage)
}

export async function renderSettingsDomains(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  return renderDomainPage(root, api, context, settingsDomainPage)
}

async function renderDomainPage(root: HTMLElement, api: HostHttpClient, context: HostContext, copy: DomainPageCopy): Promise<void> {
  const canManage = context.permissions.includes('identity.domain.manage')
  const domainTable = table({
    columns: ['主机名', '验证状态', '主域名', 'DNS 说明', '平台叠加', '更新时间', '操作'],
    empty: copy.empty,
  })
  let shops: Shop[] = []
  let rows: Domain[] = []
  let overlayStatus: Domain['platformStatus'] = 'active'
  let overlayReason = ''
  let cnameTarget = ''
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadDomains() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadDomains() },
  })

  const filter = searchForm({
    fields: [
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '请选择店铺' }] },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'PENDING', label: '待验证' },
        { value: 'VERIFIED', label: '已验证' },
        { value: 'FAILED', label: '校验失败' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadDomains()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ shopId: defaultShopId(), status: '' })
      void loadDomains()
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

  function writeBody(item: Pick<Domain, 'shopId' | 'version'>): string {
    return JSON.stringify({
      commandKey: crypto.randomUUID(),
      shopId: item.shopId,
      expectedVersion: item.version,
      scene: copy.scene,
    })
  }

  function dnsHint(item: Domain): string {
    const target = item.cnameTarget || cnameTarget
    const cname = target ? `CNAME → ${target}` : 'CNAME 目标暂不可用'
    return `${item.txtName} = ${item.txtValue} · ${cname}`
  }

  function renderRows(): void {
    domainTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '复制 TXT', size: 'sm', variant: 'secondary', onClick: () => copyTxt(item) }),
      ]
      if (canManage && item.editable) {
        buttons.push(button({ label: '校验 DNS', size: 'sm', onClick: () => openTest(item) }))
        if (item.status === 'VERIFIED' && !item.isPrimary) {
          buttons.push(button({ label: '设为主域名', size: 'sm', onClick: () => openActivate(item) }))
        }
      }
      if (canManage) {
        buttons.push(button({ label: '删除', size: 'sm', variant: 'secondary', onClick: () => openDelete(item) }))
      }
      return [
        item.host,
        statusBadge(item.status),
        item.isPrimary ? badge({ label: '主域名', tone: 'success' }) : '—',
        dnsHint(item),
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

  async function loadDomains(): Promise<void> {
    const shopId = selectedShopId()
    if (!shopId) {
      rows = []
      overlayStatus = 'active'
      overlayReason = ''
      cnameTarget = ''
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      notify(shops.length ? copy.selectShop : '当前商户没有可管理的店铺。', shops.length ? 'warning' : 'danger')
      return
    }
    try {
      const query = new URLSearchParams({
        shopId: String(shopId),
        scene: copy.scene,
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      const values = filter.values()
      if (values.status) query.set('status', values.status)
      const pageResult = await api.request<DomainPage>(`${prefix}?${query}`)
      rows = pageResult.items
      overlayStatus = pageResult.platformStatus || 'active'
      overlayReason = pageResult.platformReasonPublic || ''
      cnameTarget = pageResult.cnameTarget || ''
      currentPage = pageResult.page
      currentPageSize = pageResult.pageSize
      renderRows()
      pager.set({ page: pageResult.page, pageSize: pageResult.pageSize, total: pageResult.total })
      if (!overlayActive() && overlayReason) notify(overlayReason, 'warning')
      if (overlayActive() && !cnameTarget) notify('平台尚未公布 CNAME 目标，页面不会编造域名。', 'warning')
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      notify(`${copy.loadFailed}：${String(error)}`, 'danger')
    }
  }

  function copyTxt(item: Domain): void {
    void navigator.clipboard.writeText(`${item.txtName} ${item.txtValue}`).then(
      () => notify(`已复制 ${item.txtName}`),
      () => notify('复制失败，请手动选择 TXT 记录。', 'warning'),
    )
  }

  function openCreate(): void {
    const shopId = selectedShopId()
    if (!shopId) {
      notify('请先选择店铺。', 'warning')
      return
    }
    if (!overlayActive()) {
      notify('平台已限制该店铺域名，当前不能绑定。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: copy.createTitle,
      fields: [
        { name: 'host', label: '主机名', required: true, placeholder: copy.hostPlaceholder },
      ],
      onSubmit: (values, editor) => {
        const host = values.host.trim().toLowerCase()
        if (!host || host.includes('://') || host.includes('/')) {
          editor.setError('请填写不含协议和路径的主机名。')
          return
        }
        editor.setBusy(true)
        api.request<DomainResult>(`${prefix}`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), shopId, host, scene: copy.scene }),
        })
          .then(async () => {
            editor.close()
            notify('已添加域名，请按 DNS 说明配置 TXT 后校验。', 'success')
            await loadDomains()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ host: '' })
  }

  function openTest(item: Domain): void {
    const modal = hostFormModal({
      title: `校验 DNS · ${item.host}`,
      fields: [{
        name: 'confirm',
        label: `将查询 ${item.txtName} 是否包含挑战值。`,
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认校验 ${item.host}` }],
      }],
      submitLabel: '校验',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<DomainResult>(`${prefix}/${item.id}/test`, {
          method: 'POST',
          body: writeBody(item),
        })
          .then(async result => {
            editor.close()
            notify(result.domain.status === 'VERIFIED' ? `已验证 ${item.host}` : `${item.host} 尚未通过 TXT 校验`, result.domain.status === 'VERIFIED' ? 'success' : 'warning')
            await loadDomains()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openActivate(item: Domain): void {
    const modal = hostFormModal({
      title: `设为主域名 · ${item.host}`,
      fields: [{
        name: 'confirm',
        label: '该场景下其它主域名标记会被清除。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认设为主域名 ${item.host}` }],
      }],
      submitLabel: '设为主域名',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<DomainResult>(`${prefix}/${item.id}/activate`, {
          method: 'POST',
          body: writeBody(item),
        })
          .then(async () => {
            editor.close()
            notify(`已将 ${item.host} 设为主域名`, 'success')
            await loadDomains()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openDelete(item: Domain): void {
    const modal = hostFormModal({
      title: `删除 ${item.host}`,
      fields: [{
        name: 'confirm',
        label: '删除后释放该主机名，可再次绑定。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认删除 ${item.host}` }],
      }],
      submitLabel: '删除',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<DomainResult>(`${prefix}/${item.id}`, {
          method: 'DELETE',
          body: writeBody(item),
        })
          .then(async () => {
            editor.close()
            notify(`已删除 ${item.host}`, 'success')
            await loadDomains()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const toolbar = [
    button({ label: '刷新', variant: 'secondary', onClick: () => void loadDomains() }),
  ]
  if (canManage) toolbar.push(button({ label: '添加域名', onClick: () => openCreate() }))

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: copy.title,
        actions: toolbar,
        body: domainTable.element,
        footer: pager.element,
      }),
    ],
  }))

  try {
    await loadShops()
    await loadDomains()
  } catch (error) {
    shops = []
    rows = []
    renderRows()
    notify(`${copy.pageFailed}：${String(error)}`, 'danger')
  }
}
