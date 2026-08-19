import type { HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, definitionList, grid, page, statGrid, statusLine, table } from '@liveshops/design-tokens'

interface Shop {
  id: number
  merchantId: number
  name: string
  code: string
  status: string
  version: number
}

interface AccountOverview {
  subject: string
  displayName: string
  account: string
  principalType: string
  owner: boolean
  status: string
  merchant: { merchantId: number; name: string; status: string }
  currentShopId: number
  shops: Shop[]
  subscription: {
    merchantId: number
    planId: number
    planCode: string
    planName: string
    expiresAt: string
    version: number
    productLimit: number | null
    quotaConfigured: boolean
    permissionNames: string[]
  }
  permissionNames: string[]
  organization: {
    id: number
    name: string
    status: string
    unitCount: number
    memberCount: number
    shopCount: number
  }
}

const path = '/merch/identity/account'

const principalLabels: Record<string, string> = {
  MERCHANT_OWNER: '商户所有者',
  MERCHANT_STAFF: '员工',
  SHOP_ANCHOR: '主播',
}

function statusBadge(status: string): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  if (status === 'DISABLED') return badge({ label: '停用', tone: 'warning' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function formatExpiry(planId: number, value?: string): string {
  if (!planId) return '—'
  if (!value) return '永久'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function formatLimit(limit: number | null | undefined, configured = true): string {
  if (!configured) return '未配置'
  if (limit == null) return '不限额'
  return String(limit)
}

function currentShopLabel(value: AccountOverview): string {
  const shop = (value.shops || []).find(item => item.id === value.currentShopId)
  if (shop) return `${shop.name}（${shop.id}）`
  if (value.currentShopId) return String(value.currentShopId)
  return '未选择'
}

function inlineValue(...nodes: Array<string | HTMLElement>): HTMLElement {
  const row = create('div', 'identity-account-inline')
  for (const node of nodes) {
    if (typeof node === 'string') row.append(create('span', undefined, node))
    else row.append(node)
  }
  return row
}

function tagList(names: string[], empty: string): HTMLElement {
  const wrap = create('div', 'identity-account-tags')
  if (!names.length) {
    wrap.append(create('p', 'identity-subscription-empty', empty))
    return wrap
  }
  for (const name of names) wrap.append(badge({ label: name, tone: 'neutral' }))
  return wrap
}

function section(title: string, child: HTMLElement): HTMLElement {
  const node = create('div', 'identity-account-section')
  node.append(create('p', 'identity-account-caption', title), child)
  return node
}

export async function renderAccount(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const identityBody = create('div', 'identity-account-body')
  const subscriptionBody = create('div', 'identity-account-body')
  const shopTable = table({
    columns: ['店铺', '短码', '店铺 ID', '状态', '当前上下文'],
    empty: '当前账号没有可访问店铺',
  })

  function render(value: AccountOverview): void {
    const heading = create('div', 'identity-account-heading')
    heading.append(create('h3', 'identity-plan-name', value.displayName || value.account || '当前账号'))
    const marks = create('div', 'identity-plan-meta')
    marks.append(badge({
      label: principalLabels[value.principalType] || value.principalType || '未知身份',
      tone: 'info',
    }))
    marks.append(statusBadge(value.status))
    if (value.owner) marks.append(badge({ label: '所有者', tone: 'success' }))
    heading.append(marks)

    identityBody.replaceChildren(
      heading,
      statGrid([
        { label: '可访问店铺', value: value.shops.length },
        { label: '组织单元', value: value.organization.unitCount },
        { label: '员工与主播', value: value.organization.memberCount },
        { label: '名下店铺', value: value.organization.shopCount },
      ]),
      definitionList([
        { label: '登录账号', value: value.account || '—' },
        { label: '所属商户', value: inlineValue(value.merchant.name || '—', statusBadge(value.merchant.status)) },
        { label: '当前店铺', value: currentShopLabel(value) },
        { label: '组织', value: inlineValue(value.organization.name || '—', statusBadge(value.organization.status)) },
        { label: '主体', value: value.subject || '—' },
      ]),
    )

    const subscription = value.subscription
    const planHeading = create('div', 'identity-account-heading')
    planHeading.append(create('h3', 'identity-plan-name', subscription.planId ? subscription.planName : '尚未指派套餐'))
    const planMarks = create('div', 'identity-plan-meta')
    if (subscription.planId) {
      planMarks.append(badge({ label: '当前套餐', tone: 'success' }))
      if (subscription.planCode) planMarks.append(badge({ label: subscription.planCode, tone: 'neutral' }))
    } else {
      planMarks.append(badge({ label: '未开通', tone: 'warning' }))
    }
    planHeading.append(planMarks)

    subscriptionBody.replaceChildren(
      planHeading,
      definitionList([
        { label: '到期时间', value: formatExpiry(subscription.planId, subscription.expiresAt) },
        { label: '商品额度上限', value: formatLimit(subscription.productLimit, subscription.quotaConfigured) },
      ]),
      section('套餐权益', tagList(subscription.permissionNames || [], '未开通套餐权益')),
      section('本账号有效权限', tagList(value.permissionNames || [], '当前没有可展示的有效权限')),
    )

    shopTable.setRows((value.shops || []).map(shop => [
      shop.name,
      shop.code || '—',
      shop.id,
      statusBadge(shop.status),
      shop.id === value.currentShopId ? badge({ label: '当前', tone: 'info' }) : '—',
    ]))
  }

  async function load(): Promise<void> {
    state.set('正在加载账号总览…')
    try {
      const value = await api.request<AccountOverview>(path)
      value.shops ||= []
      value.permissionNames ||= []
      value.subscription ||= { merchantId: 0, planId: 0, planCode: '', planName: '', expiresAt: '', version: 0, productLimit: null, quotaConfigured: false, permissionNames: [] }
      value.subscription.permissionNames ||= []
      value.merchant ||= { merchantId: 0, name: '', status: '' }
      value.organization ||= { id: 0, name: '', status: '', unitCount: 0, memberCount: 0, shopCount: 0 }
      render(value)
      const role = principalLabels[value.principalType] || value.principalType || '当前账号'
      state.set(`${value.displayName || value.account || '当前账号'} · ${role} · ${value.shops.length} 家可访问店铺`)
    } catch (error) {
      state.set(String(error), 'danger')
    }
  }

  const layout = create('div', 'identity-account-layout')
  layout.append(
    grid([
      dataCard({
        title: '当前身份',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void load() })],
        status: state.element,
        body: identityBody,
      }),
      dataCard({
        title: '套餐权益',
        body: subscriptionBody,
      }),
    ]),
    dataCard({
      title: '可访问店铺',
      body: shopTable.element,
    }),
  )
  root.replaceChildren(page({ showSummary: false, children: [layout] }))
  await load()
}
