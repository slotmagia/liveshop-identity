import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshop/design-tokens'

interface Shop {
  shopId: number
  merchantId: number
  code: string
  subdomain: string
  name: string
  defaultLocale: string
  currency: string
  categoryCode: string
  status: 'ACTIVE' | 'DISABLED'
  version: number
}

interface ShopCategory { code: string; name: string; icon: string }
interface ShopPage { items: Shop[]; page: number; pageSize: number; total: number; owner: boolean }
interface ShopResult { shop: Shop; replayed: boolean }

const prefix = '/merch/identity/shops'
const currencies = [
  { value: 'CNY', label: 'CNY 人民币' },
  { value: 'USD', label: 'USD 美元' },
  { value: 'EUR', label: 'EUR 欧元' },
  { value: 'GBP', label: 'GBP 英镑' },
  { value: 'JPY', label: 'JPY 日元' },
]

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function statusBadge(status: Shop['status']): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  return badge({ label: '停用', tone: 'warning' })
}

function categoryLabel(code: string, categories: ShopCategory[]): string {
  if (!code) return '—'
  const current = categories.find(item => item.code === code)
  return current ? `${current.icon ? `${current.icon} ` : ''}${current.name}` : code
}

export async function renderShops(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canReadManage = context.permissions.includes('identity.shop.manage')
  const state = statusLine()
  const shopTable = table({
    columns: ['店铺 ID', '名称', '短码', '子域名', '品类', '语言 / 币种', '状态', '版本', '操作'],
    empty: '暂无店铺',
  })
  let categories: ShopCategory[] = []
  let rows: Shop[] = []
  let owner = false
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadShops() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadShops() },
  })

  const filter = searchForm({
    fields: [
      { name: 'keyword', label: '关键词', placeholder: '名称 / 短码 / 子域名' },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'ACTIVE', label: '启用' },
        { value: 'DISABLED', label: '停用' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadShops()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ keyword: '', status: '' })
      void loadShops()
    },
  })

  function canManage(): boolean {
    return owner && canReadManage
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
    shopTable.setRows(rows.map(item => {
      const buttons = [
        button({ label: '复制短码', size: 'sm', variant: 'secondary', onClick: () => void copy(item.code, '短码') }),
        button({ label: '复制子域名', size: 'sm', variant: 'secondary', disabled: !item.subdomain, onClick: () => void copy(item.subdomain, '子域名') }),
      ]
      if (canManage()) {
        buttons.push(button({ label: '编辑', size: 'sm', onClick: () => openEdit(item) }))
        if (item.status === 'DISABLED') {
          buttons.push(button({ label: '启用', size: 'sm', onClick: () => openToggle(item, true) }))
        } else {
          buttons.push(button({ label: '停用', size: 'sm', variant: 'secondary', onClick: () => openToggle(item, false) }))
        }
        buttons.push(button({ label: '关闭', size: 'sm', variant: 'secondary', onClick: () => openClose(item) }))
      }
      return [
        item.shopId,
        item.name,
        item.code,
        item.subdomain || '—',
        categoryLabel(item.categoryCode, categories),
        `${item.defaultLocale || '—'} / ${item.currency}`,
        statusBadge(item.status),
        item.version,
        actions(...buttons),
      ]
    }))
  }

  async function loadCategories(): Promise<void> {
    categories = await api.request<ShopCategory[]>(`${prefix}/categories`)
  }

  async function loadShops(): Promise<void> {
    filter.setBusy(true)
    state.set('正在加载店铺…')
    try {
      const values = filter.values()
      const query = new URLSearchParams({
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      if (values.keyword.trim()) query.set('keyword', values.keyword.trim())
      if (values.status) query.set('status', values.status)
      const pageData = await api.request<ShopPage>(`${prefix}?${query.toString()}`)
      rows = pageData.items ?? []
      owner = pageData.owner
      pager.set({ page: pageData.page, pageSize: pageData.pageSize, total: pageData.total })
      renderToolbar()
      renderRows()
      state.set(`店铺 ${pageData.total} 个`)
    } catch (error) {
      rows = []
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      renderRows()
      state.set(`店铺加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  function openCreate(): void {
    const modal = hostFormModal({
      title: '新建店铺',
      fields: [
        { name: 'name', label: '店铺名称', required: true, maxLength: 191 },
        { name: 'subdomain', label: '子域名', required: true, placeholder: 'second-shop', maxLength: 63 },
        { name: 'currency', label: '结算币种', kind: 'select', required: true, options: currencies },
        {
          name: 'categoryCode',
          label: '经营品类',
          kind: 'select',
          options: [{ value: '', label: '不选择' }, ...categories.map(item => ({ value: item.code, label: `${item.icon ? `${item.icon} ` : ''}${item.name}` }))],
        },
        {
          name: 'status',
          label: '状态',
          kind: 'select',
          required: true,
          options: [{ value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' }],
        },
      ],
      onSubmit: (values, editor) => {
        const name = values.name.trim()
        const subdomain = values.subdomain.trim().toLowerCase()
        if ([...name].length < 1 || [...name].length > 191) {
          editor.setError('名称长度为 1–191 个字符。')
          return
        }
        if (!/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(subdomain)) {
          editor.setError('子域名仅允许小写字母、数字和中间的连字符。')
          return
        }
        editor.setBusy(true)
        api.request<ShopResult>(prefix, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            name,
            subdomain,
            currency: values.currency || 'CNY',
            categoryCode: values.categoryCode || '',
            status: values.status || 'ACTIVE',
          }),
        })
          .then(() => { editor.close(); return loadShops() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ currency: 'CNY', status: 'ACTIVE', categoryCode: '' })
  }

  function openEdit(item: Shop): void {
    const modal = hostFormModal({
      title: `编辑 ${item.name}`,
      fields: [
        { name: 'name', label: '店铺名称', required: true, maxLength: 191 },
        { name: 'subdomain', label: '子域名', required: true, maxLength: 63 },
        { name: 'code', label: '短码', disabled: true },
        { name: 'currency', label: '结算币种', disabled: true },
        { name: 'category', label: '经营品类', disabled: true },
      ],
      onSubmit: (values, editor) => {
        const name = values.name.trim()
        const subdomain = values.subdomain.trim().toLowerCase()
        if ([...name].length < 1 || [...name].length > 191) {
          editor.setError('名称长度为 1–191 个字符。')
          return
        }
        if (!/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(subdomain)) {
          editor.setError('子域名仅允许小写字母、数字和中间的连字符。')
          return
        }
        editor.setBusy(true)
        api.request<ShopResult>(`${prefix}/${item.shopId}`, {
          method: 'PUT',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: item.version,
            name,
            subdomain,
          }),
        })
          .then(() => { editor.close(); return loadShops() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      name: item.name,
      subdomain: item.subdomain,
      code: item.code,
      currency: item.currency,
      category: categoryLabel(item.categoryCode, categories),
    })
  }

  function openToggle(item: Shop, enabled: boolean): void {
    const modal = hostFormModal({
      title: `${enabled ? '启用' : '停用'} ${item.name}`,
      fields: [{
        name: 'confirm',
        label: enabled ? '启用后该店铺可重新对外提供服务。' : '停用后该店铺不再对外提供服务，但仍保留在目录中。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.shopId), label: `确认${enabled ? '启用' : '停用'}` }],
      }],
      submitLabel: enabled ? '启用' : '停用',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.shopId)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<ShopResult>(`${prefix}/${item.shopId}/${enabled ? 'enable' : 'disable'}`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: item.version }),
        })
          .then(() => { editor.close(); return loadShops() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openClose(item: Shop): void {
    const modal = hostFormModal({
      title: `关闭 ${item.name}`,
      fields: [{
        name: 'confirm',
        label: '关闭后店铺进入终态，不能再编辑或启用。每个商户必须至少保留一家非关闭店铺。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.shopId), label: '确认关闭' }],
      }],
      submitLabel: '关闭',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.shopId)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<ShopResult>(`${prefix}/${item.shopId}/close`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: item.version }),
        })
          .then(() => { editor.close(); return loadShops() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const toolbarHolder = document.createElement('div')
  toolbarHolder.className = ui.actions
  function renderToolbar(): void {
    const toolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadShops() })]
    if (canManage()) toolbar.push(button({ label: '新增', onClick: () => openCreate() }))
    toolbarHolder.replaceChildren(...toolbar)
  }

  renderToolbar()
  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '店铺目录',
        actions: toolbarHolder,
        status: state.element,
        body: shopTable.element,
        footer: pager.element,
      }),
    ],
  }))

  filter.setBusy(true)
  Promise.all([loadCategories(), loadShops()]).catch(error => {
    state.set(`店铺目录加载失败：${String(error)}`, 'danger')
    filter.setBusy(false)
  })
}
