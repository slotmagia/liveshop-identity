import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { badge, button, create, dataCard, definitionList, page, statusLine, ui } from '@liveshop/design-tokens'

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
interface CurrentShop { shop: Shop; owner: boolean }
interface ShopResult { shop: Shop; replayed: boolean }

const prefix = '/merch/identity/shops'
const subdomainPattern = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/

function field(label: string, control: HTMLElement): HTMLElement {
  const node = create('label', ui.field)
  node.append(create('span', undefined, label), control)
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

export async function renderShopSettings(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.shop.manage')
  const state = statusLine()
  const body = create('div', 'identity-shop-settings-form')
  let current: CurrentShop | undefined
  let categories: ShopCategory[] = []
  let nameInput: HTMLInputElement
  let subdomainInput: HTMLInputElement
  const saveButton = button({ label: '保存', onClick: () => void save(), disabled: true })

  function editable(): boolean {
    return Boolean(current?.owner && canManage)
  }

  function renderForm(value: CurrentShop): void {
    const disabled = !editable()
    nameInput = document.createElement('input')
    nameInput.className = ui.input
    nameInput.maxLength = 191
    nameInput.value = value.shop.name
    nameInput.disabled = disabled
    subdomainInput = document.createElement('input')
    subdomainInput.className = ui.input
    subdomainInput.maxLength = 63
    subdomainInput.placeholder = 'shop-subdomain'
    subdomainInput.value = value.shop.subdomain
    subdomainInput.disabled = disabled
    body.replaceChildren(
      create('p', undefined, '维护当前会话店铺的名称和子域名。短码、币种、默认语言和经营品类在创建后不可改。新建、启停和关闭请使用店铺管理。'),
      definitionList([
        { label: '店铺 ID', value: String(value.shop.shopId) },
        { label: '商户 ID', value: String(value.shop.merchantId) },
        { label: '短码', value: value.shop.code || '—' },
        { label: '默认语言', value: value.shop.defaultLocale || '—' },
        { label: '结算币种', value: value.shop.currency || '—' },
        { label: '经营品类', value: categoryLabel(value.shop.categoryCode, categories) },
        { label: '状态', value: statusBadge(value.shop.status) },
      ]),
      field('店铺名称', nameInput),
      field('子域名', subdomainInput),
      create('p', undefined, '子域名只保存店铺标识，不是完整公网地址。广告像素不在本页维护。'),
    )
  }

  async function load(): Promise<void> {
    state.set('正在加载当前店铺…')
    try {
      if (!categories.length) categories = await api.request<ShopCategory[]>(`${prefix}/categories`)
      current = await api.request<CurrentShop>(`${prefix}/current`)
      renderForm(current)
      saveButton.disabled = !editable()
      const scope = `商户 ${current.shop.merchantId} · 店铺 ${current.shop.shopId} · 版本 ${current.shop.version}`
      state.set(editable() ? scope : `${scope} · 仅所有者可改`, editable() ? 'neutral' : 'warning')
    } catch (error) {
      current = undefined
      saveButton.disabled = true
      body.replaceChildren()
      state.set(`当前店铺加载失败：${String(error)}`, 'danger')
    }
  }

  async function save(): Promise<void> {
    if (!current) return
    if (!editable()) {
      state.set('仅商户所有者可以保存店铺资料。', 'warning')
      return
    }
    const name = nameInput.value.trim()
    const subdomain = subdomainInput.value.trim().toLowerCase()
    if ([...name].length < 1 || [...name].length > 191) {
      state.set('名称长度为 1–191 个字符。', 'danger')
      return
    }
    if (!subdomainPattern.test(subdomain)) {
      state.set('子域名仅允许小写字母、数字和中间的连字符。', 'danger')
      return
    }
    state.set('正在保存…')
    try {
      const result = await api.request<ShopResult>(`${prefix}/${current.shop.shopId}`, {
        method: 'PUT',
        body: JSON.stringify({
          commandKey: crypto.randomUUID(),
          expectedVersion: current.shop.version,
          name,
          subdomain,
        }),
      })
      current = { shop: result.shop, owner: current.owner }
      renderForm(current)
      saveButton.disabled = !editable()
      state.set(`商户 ${current.shop.merchantId} · 店铺 ${current.shop.shopId} · 已保存 · 版本 ${current.shop.version}`, 'success')
    } catch (error) {
      state.set(`保存失败：${String(error)}`, 'danger')
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: dataCard({
      title: '当前店铺',
      actions: [
        button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
        saveButton,
      ],
      status: state.element,
      body,
    }),
  }))
  await load()
}
