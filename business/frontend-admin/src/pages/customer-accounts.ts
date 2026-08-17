import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, create, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshop/design-tokens'

interface Merchant { merchantId: number; name: string; status: 'ACTIVE' | 'DISABLED' }
interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
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

const prefix = '/admin/identity/customer-accounts'
const platformPattern = /^[a-z0-9_-]{1,32}$/

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

function setSelectOptions(
  control: HTMLSelectElement | undefined,
  values: Array<{ value: number; label: string }>,
  allLabel: string,
  emptyLabel: string,
): void {
  if (!control) return
  control.replaceChildren()
  const all = document.createElement('option')
  all.value = ''
  all.textContent = values.length ? allLabel : emptyLabel
  control.append(all)
  control.disabled = !values.length
  for (const value of values) {
    const option = document.createElement('option')
    option.value = String(value.value)
    option.textContent = value.label
    control.append(option)
  }
  ;(control as HTMLSelectElement & { refreshSearchSelect?: () => void }).refreshSearchSelect?.()
}

export async function renderCustomerAccounts(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const accountTable = table({
    columns: ['店铺范围', '平台', '客服账号', '昵称', '状态', '备注', '版本', '操作'],
    empty: '暂无客服账号',
  })
  let merchants: Merchant[] = []
  let shops: Shop[] = []
  let lastMerchantId = ''
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
      { name: 'merchantId', label: '商户', kind: 'select', options: [{ value: '', label: '全部商户' }] },
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
      const merchantId = filter.values().merchantId
      if (merchantId !== lastMerchantId) {
        lastMerchantId = merchantId
        filter.set({ shopId: '' })
        await loadShops(Number(merchantId) || 0, false)
      }
      await loadAccounts()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      lastMerchantId = ''
      filter.set({ merchantId: '', shopId: '', platform: '', account: '', status: '' })
      void loadShops(0, true)
    },
  })

  const merchantSelect = filter.control('merchantId') as HTMLSelectElement | undefined
  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined

  function selectedScope(): { merchantId: number; shopId: number; shop?: Shop } {
    const values = filter.values()
    const merchantId = Number(values.merchantId) || 0
    const shopId = Number(values.shopId) || 0
    return { merchantId, shopId, shop: shops.find(item => item.shopId === shopId) }
  }

  function renderRows(): void {
    const shopByID = new Map(shops.map(item => [item.shopId, item]))
    accountTable.setRows(rows.map(item => {
      const shop = shopByID.get(item.shopId)
      const merchant = merchants.find(entry => entry.merchantId === item.merchantId)
      return [
        `${merchant?.name ?? `商户 ${item.merchantId}`} · ${shop?.name ?? `店铺 ${item.shopId}`} · shop_id ${item.shopId}`,
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

  async function loadMerchants(): Promise<void> {
    state.set('正在加载商户和店铺范围…')
    filter.setBusy(true)
    try {
      merchants = await api.request<Merchant[]>(`${prefix}/merchants`)
      lastMerchantId = ''
      setSelectOptions(merchantSelect, merchants.map(item => ({
        value: item.merchantId,
        label: `${item.name} · merchant_id ${item.merchantId}${item.status === 'DISABLED' ? ' · 已停用' : ''}`,
      })), '全部商户', '暂无商户')
      filter.set({ merchantId: '', shopId: '' })
      await loadShops(0, true)
    } catch (error) {
      merchants = []
      shops = []
      lastMerchantId = ''
      rows = []
      setSelectOptions(merchantSelect, [], '全部商户', '暂无商户')
      setSelectOptions(shopSelect, [], '全部店铺', '暂无店铺')
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      state.set(`客服管理范围加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadShops(merchantId: number, loadRows: boolean): Promise<void> {
    const scopedMerchantId = Number.isSafeInteger(merchantId) && merchantId > 0 ? merchantId : 0
    filter.setBusy(true)
    state.set(scopedMerchantId ? '正在加载商户店铺…' : '正在加载全部店铺…')
    try {
      const query = scopedMerchantId ? `?merchantId=${encodeURIComponent(scopedMerchantId)}` : ''
      shops = await api.request<Shop[]>(`${prefix}/shops${query}`)
      setSelectOptions(shopSelect, shops.map(item => {
        const merchant = merchants.find(entry => entry.merchantId === item.merchantId)
        const shopLabel = `${item.name || item.code} · shop_id ${item.shopId}${item.status === 'DISABLED' ? ' · 已停用' : ''}`
        return {
          value: item.shopId,
          label: scopedMerchantId || !merchant ? shopLabel : `${merchant.name} · ${shopLabel}`,
        }
      }), '全部店铺', '暂无店铺')
      const currentShopId = filter.values().shopId
      const shopId = shops.some(item => String(item.shopId) === currentShopId) ? currentShopId : ''
      filter.set({ shopId })
      if (loadRows) await loadAccounts()
    } catch (error) {
      shops = []
      rows = []
      setSelectOptions(shopSelect, [], '全部店铺', '暂无店铺')
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      state.set(`店铺范围加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadAccounts(): Promise<void> {
    const values = filter.values()
    const { merchantId, shopId } = selectedScope()
    const platform = values.platform.trim().toLowerCase()
    if (platform && !platformPattern.test(platform)) {
      state.set('平台标识只能包含小写字母、数字、下划线和连字符，长度为 1–32。', 'danger')
      return
    }
    filter.setBusy(true)
    pager.setBusy(true)
    state.set('正在加载客服账号…')
    try {
      const query = new URLSearchParams({ page: String(currentPage), pageSize: String(currentPageSize) })
      if (merchantId > 0) query.set('merchantId', String(merchantId))
      if (shopId > 0) query.set('shopId', String(shopId))
      if (platform) query.set('platform', platform)
      if (values.account.trim()) query.set('account', values.account.trim())
      if (values.status) query.set('status', values.status)
      const result = await api.request<CustomerAccountPage>(`${prefix}?${query.toString()}`)
      rows = result.items ?? []
      currentPage = result.page
      currentPageSize = result.pageSize
      renderRows()
      pager.set({ page: result.page, pageSize: result.pageSize, total: result.total, itemCount: rows.length })
      state.clear()
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      state.set(`客服账号加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
      pager.setBusy(false)
    }
  }

  function openEditor(current?: CustomerAccount): void {
    const scope = current
      ? {
          merchantId: current.merchantId,
          shopId: current.shopId,
          shop: shops.find(item => item.shopId === current.shopId),
        }
      : selectedScope()
    if (!current && (scope.merchantId <= 0 || scope.shopId <= 0 || !scope.shop)) {
      state.set('新增客服请先选择具体商户和店铺。', 'danger')
      return
    }
    const modal = hostFormModal({
      title: current ? `编辑客服账号 · ${current.nickname || current.account}` : '新增客服账号',
      fields: [
        { name: 'merchantScope', label: '商户范围', disabled: true },
        { name: 'shopScope', label: '店铺范围', disabled: true },
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
        const platform = values.platform.trim().toLowerCase()
        const account = values.account.trim()
        const config = values.config.trim()
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
            merchantId: scope.merchantId, shopId: scope.shopId, platform, account,
            nickname: values.nickname.trim(), status: values.status, config, remark: values.remark.trim(),
          }),
        })
          .then(() => { editor.close(); return loadAccounts() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      merchantScope: `${merchants.find(item => item.merchantId === scope.merchantId)?.name ?? '商户'} · merchant_id ${scope.merchantId}`,
      shopScope: `${scope.shop?.name || scope.shop?.code || '店铺'} · shop_id ${scope.shopId}`,
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
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: current.version, merchantId: current.merchantId, shopId: current.shopId }),
        })
          .then(() => { editor.close(); return loadAccounts() })
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
        status: state.element,
        body: accountTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await loadMerchants()
}

