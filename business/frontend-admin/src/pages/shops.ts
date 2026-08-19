import type { HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, searchCard, searchForm, statusLine, table, ui } from '@liveshops/design-tokens'

interface Merchant {
  merchantId: number
  name: string
  externalId: string
  status: 'ACTIVE' | 'DISABLED'
  version: number
}

interface Shop {
  shopId: number
  merchantId: number
  code: string
  subdomain: string
  name: string
  defaultLocale: string
  currency: string
  status: 'ACTIVE' | 'DISABLED'
  version: number
}

const prefix = '/admin/identity/shops'

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

export async function renderShops(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const shopTable = table({
    columns: ['店铺 ID', '商户 ID', '店铺名称', '短码', '子域名', '语言 / 币种', '状态', '版本', '访问标识'],
    empty: '暂无店铺',
  })
  let merchants: Merchant[] = []
  let shops: Shop[] = []
  let selectedMerchantId = 0

  const filter = searchForm({
    fields: [{ name: 'merchantId', label: '商户', kind: 'select', placeholder: '全部商户', options: [{ value: '', label: '全部商户' }] }],
    searchLabel: '查询',
    onSearch: values => {
      const raw = values.merchantId.trim()
      const merchantId = raw ? Number(raw) : 0
      selectedMerchantId = Number.isSafeInteger(merchantId) && merchantId > 0 ? merchantId : 0
      void loadShops()
    },
    onReset: () => {
      selectedMerchantId = 0
      filter.set({ merchantId: '' })
      void loadShops()
    },
  })

  function populateMerchantOptions(): void {
    const select = filter.control('merchantId')
    if (!(select instanceof HTMLSelectElement)) return
    select.replaceChildren()
    const all = document.createElement('option')
    all.value = ''
    all.textContent = merchants.length ? '全部商户' : '暂无商户'
    select.append(all)
    select.disabled = !merchants.length
    for (const merchant of merchants) {
      const option = document.createElement('option')
      option.value = String(merchant.merchantId)
      option.textContent = `${merchant.name} · merchant_id ${merchant.merchantId}${merchant.status === 'DISABLED' ? ' · 已停用' : ''}`
      select.append(option)
    }
    filter.set({ merchantId: selectedMerchantId || '' })
    ;(select as HTMLSelectElement & { refreshSearchSelect?: () => void }).refreshSearchSelect?.()
  }

  async function copy(value: string, label: string): Promise<void> {
    if (!value) {
      state.set(`${label}未配置。`, 'danger')
      return
    }
    try {
      await navigator.clipboard.writeText(value)
      state.set(`已复制${label}：${value}`)
    } catch (error) {
      state.set(`复制失败：${String(error)}`, 'danger')
    }
  }

  function renderRows(): void {
    shopTable.setRows(shops.map(shop => [
      shop.shopId,
      shop.merchantId,
      shop.name,
      shop.code,
      shop.subdomain || '—',
      `${shop.defaultLocale || '—'} / ${shop.currency}`,
      badge({ label: shop.status === 'ACTIVE' ? '启用' : '停用', tone: shop.status === 'ACTIVE' ? 'success' : 'warning' }),
      shop.version,
      actions(
        button({ label: '复制参数标识', size: 'sm', variant: 'secondary', onClick: () => void copy(`?code=${encodeURIComponent(shop.code)}`, '参数标识') }),
        button({ label: '复制子域名', size: 'sm', variant: 'secondary', disabled: !shop.subdomain, onClick: () => void copy(shop.subdomain, '子域名') }),
      ),
    ]))
  }

  async function loadShops(): Promise<void> {
    filter.setBusy(true)
    state.set(selectedMerchantId ? `正在加载 merchant_id ${selectedMerchantId} 的店铺…` : '正在加载全部店铺…')
    try {
      const query = selectedMerchantId ? `?merchantId=${encodeURIComponent(selectedMerchantId)}` : ''
      shops = await api.request<Shop[]>(`${prefix}${query}`)
      renderRows()
      const merchant = merchants.find(item => item.merchantId === selectedMerchantId)
      state.set(selectedMerchantId
        ? `${merchant?.name ?? '商户'} · merchant_id ${selectedMerchantId} · 店铺 ${shops.length} 个`
        : `全部商户 · 店铺 ${shops.length} 个`)
    } catch (error) {
      shops = []
      renderRows()
      state.set(`店铺加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function load(): Promise<void> {
    filter.setBusy(true)
    state.set('正在加载商户目录…')
    try {
      merchants = await api.request<Merchant[]>(`${prefix}/merchants`)
      if (selectedMerchantId && !merchants.some(item => item.merchantId === selectedMerchantId)) selectedMerchantId = 0
      populateMerchantOptions()
      await loadShops()
    } catch (error) {
      merchants = []
      shops = []
      selectedMerchantId = 0
      populateMerchantOptions()
      renderRows()
      state.set(`商户目录加载失败：${String(error)}`, 'danger')
      filter.setBusy(false)
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '商户店铺目录',
        actions: button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
        status: state.element,
        body: shopTable.element,
      }),
    ],
  }))
  await load()
}

