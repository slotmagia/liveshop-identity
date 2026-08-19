import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshops/design-tokens'

interface Shop { shopId: number; merchantId: number; name: string; code: string; status: string }
interface CustomerAccount {
  id: number
  merchantId: number
  shopId: number
  platform: string
  account: string
  nickname: string
  status: 'ACTIVE' | 'DISABLED'
  config: string
  remark: string
  version: number
  createdAt: string
  updatedAt: string
}
interface CustomerAccountPage { items: CustomerAccount[]; page: number; pageSize: number; total: number }

const prefix = '/merch/identity/customer-accounts'
const platformPattern = /^[a-z0-9_-]{1,32}$/

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
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

function shopLabel(shop: Shop): string {
  return `${shop.name || shop.code} · shop_id ${shop.shopId}${shop.status === 'DISABLED' ? ' · 已停用' : ''}`
}

export async function renderCustomerAccounts(root: HTMLElement, api: HostHttpClient, context?: HostContext): Promise<void> {
  const accountTable = table({
    columns: ['店铺', '平台', '客服账号', '昵称', '状态', '备注', '版本', '操作'],
    empty: '暂无客服账号',
  })
  let shops: Shop[] = []
  let rows: CustomerAccount[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadAccounts() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadAccounts() },
  })

  const filter = searchForm({
    fields: [
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '全部店铺' }] },
      { name: 'platform', label: '平台标识', placeholder: 'whatsapp / telegram' },
      { name: 'account', label: '客服账号', placeholder: '账号、手机号或外部平台 ID' },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' }, { value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadAccounts()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ shopId: defaultShopId(), platform: '', account: '', status: '' })
      void loadAccounts()
    },
  })

  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined

  function defaultShopId(): string {
    const tenantShop = context?.tenant?.shopId
    if (tenantShop && shops.some(item => item.shopId === tenantShop)) return String(tenantShop)
    return shops.length === 1 ? String(shops[0].shopId) : ''
  }

  function renderRows(): void {
    const shopByID = new Map(shops.map(item => [item.shopId, item]))
    accountTable.setRows(rows.map(item => {
      const shop = shopByID.get(item.shopId)
      return [
        shop ? shopLabel(shop) : `shop_id ${item.shopId}`,
        item.platform,
        item.account,
        item.nickname || '—',
        badge({ label: item.status === 'ACTIVE' ? '启用' : '停用', tone: item.status === 'ACTIVE' ? 'success' : 'warning' }),
        item.remark || '—',
        item.version,
        actions(
          button({ label: '编辑', size: 'sm', variant: 'secondary', onClick: () => openEditor(item) }),
          button({ label: '删除', size: 'sm', variant: 'danger', onClick: () => openDelete(item) }),
        ),
      ]
    }))
  }

  async function loadShops(): Promise<void> {
    filter.setBusy(true)
    try {
      shops = await api.request<Shop[]>(`${prefix}/shops`)
      setSelectOptions(shopSelect, shops.map(item => ({ value: String(item.shopId), label: shopLabel(item) })), '全部店铺')
      const currentShopId = filter.values().shopId
      const shopId = shops.some(item => String(item.shopId) === currentShopId) ? currentShopId : defaultShopId()
      filter.set({ shopId })
      await loadAccounts()
    } catch (error) {
      shops = []
      rows = []
      setSelectOptions(shopSelect, [], '全部店铺')
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      notify(`店铺范围加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadAccounts(): Promise<void> {
    const values = filter.values()
    const platform = values.platform.trim().toLowerCase()
    if (platform && !platformPattern.test(platform)) {
      notify('平台标识只能包含小写字母、数字、下划线和连字符，长度为 1–32。', 'danger')
      return
    }
    filter.setBusy(true)
    pager.setBusy(true)
    try {
      const query = new URLSearchParams({ page: String(currentPage), pageSize: String(currentPageSize) })
      if (values.shopId) query.set('shopId', values.shopId)
      if (platform) query.set('platform', platform)
      if (values.account.trim()) query.set('account', values.account.trim())
      if (values.status) query.set('status', values.status)
      const result = await api.request<CustomerAccountPage>(`${prefix}?${query.toString()}`)
      rows = result.items ?? []
      currentPage = result.page
      currentPageSize = result.pageSize
      renderRows()
      pager.set({ page: result.page, pageSize: result.pageSize, total: result.total, itemCount: rows.length })
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      notify(`客服账号加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
      pager.setBusy(false)
    }
  }

  function openEditor(current?: CustomerAccount): void {
    const selectedShopId = current?.shopId || Number(filter.values().shopId) || (shops.length === 1 ? shops[0].shopId : 0)
    const shopOptions = shops.map(item => ({ value: String(item.shopId), label: shopLabel(item) }))
    const modal = hostFormModal({
      title: current ? `编辑客服账号 · ${current.nickname || current.account}` : '新增客服账号',
      fields: [
        {
          name: 'shopId', label: '店铺范围', kind: 'select', required: true, disabled: Boolean(current),
          options: current
            ? [{ value: String(current.shopId), label: shopLabel(shops.find(item => item.shopId === current.shopId) ?? { shopId: current.shopId, merchantId: current.merchantId, name: '', code: '', status: 'ACTIVE' }) }]
            : [{ value: '', label: '请选择店铺' }, ...shopOptions],
        },
        { name: 'platform', label: '平台标识', required: true, mono: true, maxLength: 32, placeholder: 'whatsapp / telegram' },
        { name: 'account', label: '客服账号', required: true, maxLength: 128, placeholder: '账号、手机号或外部平台 ID' },
        { name: 'nickname', label: '客服昵称', maxLength: 64, placeholder: '展示给用户的名称' },
        { name: 'status', label: '状态', kind: 'select', required: true, options: [
          { value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' },
        ] },
        { name: 'config', label: '扩展配置（JSON）', kind: 'textarea', wide: true, mono: true, rows: 5, maxLength: 4096, placeholder: '{"country_code":"US"}' },
        { name: 'remark', label: '备注', kind: 'textarea', wide: true, rows: 4, maxLength: 500 },
      ],
      onSubmit: (values, editor) => {
        const shopId = Number(values.shopId) || 0
        const platform = values.platform.trim().toLowerCase()
        const account = values.account.trim()
        const config = values.config.trim()
        if (shopId <= 0 || !shops.some(item => item.shopId === shopId)) {
          editor.setError('请选择本商户下的店铺。')
          return
        }
        if (!platformPattern.test(platform)) {
          editor.setError('平台标识只能包含小写字母、数字、下划线和连字符，长度为 1–32。')
          return
        }
        if (!account || account.length > 128) {
          editor.setError('客服账号必须包含 1–128 个字符。')
          return
        }
        if (config) {
          try { JSON.parse(config) } catch { editor.setError('扩展配置必须是合法 JSON。'); return }
        }
        editor.setBusy(true)
        api.request(current ? `${prefix}/${current.id}` : prefix, {
          method: current ? 'PUT' : 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(), expectedVersion: current?.version ?? 0,
            shopId, platform, account, nickname: values.nickname.trim(), status: values.status, config, remark: values.remark.trim(),
          }),
        })
          .then(() => { editor.close(); notify(current ? '客服账号已更新。' : '客服账号已创建。', 'success'); return loadAccounts() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      shopId: selectedShopId > 0 ? String(selectedShopId) : '',
      platform: current?.platform ?? '', account: current?.account ?? '', nickname: current?.nickname ?? '',
      status: current?.status ?? 'ACTIVE', config: current?.config ?? '', remark: current?.remark ?? '',
    })
  }

  function openDelete(current: CustomerAccount): void {
    const modal = hostFormModal({
      title: `删除客服账号 · ${current.nickname || current.account}`,
      submitLabel: '删除',
      fields: [{
        name: 'confirm',
        label: `删除后该账号会立即从店铺客服目录移除。当前版本：${current.version}`,
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(current.id), label: `确认删除 ${current.platform} / ${current.account}` }],
      }],
      onSubmit: (values, editor) => {
        if (values.confirm !== String(current.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${current.id}`, {
          method: 'DELETE',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: current.version, shopId: current.shopId }),
        })
          .then(() => { editor.close(); notify('客服账号已删除。', 'success'); return loadAccounts() })
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
        title: '客服账号目录',
        actions: [
          button({ label: '刷新', variant: 'secondary', onClick: () => void loadAccounts() }),
          button({ label: '新增客服', onClick: () => openEditor() }),
        ],
        body: accountTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadShops()
}
