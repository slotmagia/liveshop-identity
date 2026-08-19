import type { HostContext, HostFormModalSubmitApi, HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, statusLine, ui } from '@liveshops/design-tokens'

interface Plan {
  id: number
  code: string
  name: string
  level: number
  priceMinor: number
  durationDays: number
  description: string
  default: boolean
  current: boolean
  buyable: boolean
  productLimit: number | null
  permissionNames: string[]
}

interface PlanCatalog {
  items: Plan[]
  currentPlanId: number
}

interface SubscriptionCurrent {
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

interface PayMethod {
  channelCode: string
  displayName: string
  typeCode: string
  driverKey: string
}

interface PurchaseOrder {
  orderNo: string
  planId: number
  planCode: string
  planName: string
  priceMinor: number
  durationDays: number
  status: string
  payNo: string
  channelCode: string
  driverKey: string
  payStatus: string
  payload: Record<string, string>
  activated: boolean
  expiresAt: string
  replayed: boolean
}

const prefix = '/merch/identity/subscription'

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

function formatPrice(minor: number): string {
  return `¥${(minor / 100).toFixed(2)}`
}

function formatDuration(days: number): string {
  return days === 0 ? '永久' : `${days} 天`
}

function formatLimit(limit: number | null | undefined, configured = true): string {
  if (!configured) return '未配置'
  if (limit == null) return '不限额'
  return String(limit)
}

function formatExpiry(value?: string): string {
  if (!value) return '永久'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function isWallet(method: PayMethod): boolean {
  const type = method.typeCode.trim().toUpperCase()
  return type === 'WALLET' || type === 'BALANCE'
}

function payloadText(payload: Record<string, string> | undefined): string {
  const entries = Object.entries(payload ?? {}).filter(([, value]) => value.trim())
  if (!entries.length) return '等待 Trade 收款结果'
  return entries.map(([key, value]) => `${key}: ${value}`).join('\n')
}

export async function renderPlans(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canPurchase = context.permissions.includes('identity.subscription.purchase')
  const catalogState = statusLine()
  const currentState = statusLine()
  const catalogBody = create('div', 'identity-plan-grid')
  const currentBody = create('div', 'identity-subscription-current')
  let plans: Plan[] = []
  let current: SubscriptionCurrent | undefined

  function renderCatalog(): void {
    catalogBody.replaceChildren()
    if (!plans.length) {
      catalogBody.append(create('p', 'identity-subscription-empty', '暂无上架套餐'))
      return
    }
    for (const plan of plans) {
      const item = create('article', 'identity-plan-item')
      const title = create('div', 'identity-plan-heading')
      title.append(create('h3', 'identity-plan-name', plan.name))
      const marks = create('div', 'identity-plan-meta')
      if (plan.current) marks.append(badge({ label: '当前', tone: 'success' }))
      if (plan.default) marks.append(badge({ label: '默认', tone: 'info' }))
      title.append(marks)
      const description = create('p', 'identity-plan-description')
      description.textContent = plan.description || '暂无说明'
      const permissions = create('ul', 'identity-plan-permissions')
      if (!plan.permissionNames.length) {
        permissions.append(create('li', undefined, '未开通额外权限'))
      } else {
        for (const name of plan.permissionNames) permissions.append(create('li', undefined, name))
      }
      const buy = canPurchase && plan.buyable
        ? actions(button({
          label: plan.current ? '续费' : '购买',
          onClick: () => void openPurchase(plan),
        }))
        : null
      item.append(
        title,
        create('p', 'identity-plan-price', formatPrice(plan.priceMinor)),
        create('p', 'identity-plan-cycle', formatDuration(plan.durationDays)),
        description,
        create('p', 'identity-plan-quota', `商品额度上限：${formatLimit(plan.productLimit)}`),
        permissions,
      )
      if (buy) item.append(buy)
      catalogBody.append(item)
    }
  }

  function renderCurrent(): void {
    currentBody.replaceChildren()
    if (!current || !current.planId) {
      currentBody.append(create('p', 'identity-subscription-empty', '尚未指派套餐'))
      return
    }
    const permissions = create('ul', 'identity-plan-permissions')
    if (!current.permissionNames.length) {
      permissions.append(create('li', undefined, '未开通额外权限'))
    } else {
      for (const name of current.permissionNames) permissions.append(create('li', undefined, name))
    }
    currentBody.append(
      create('p', undefined, `${current.planName} · ${current.planCode}`),
      create('p', undefined, `到期：${formatExpiry(current.expiresAt)}`),
      create('p', undefined, `商品额度上限：${formatLimit(current.productLimit, current.quotaConfigured)}`),
      create('p', undefined, '已开通权限'),
      permissions,
    )
  }

  async function load(): Promise<void> {
    catalogState.set('正在加载套餐目录…')
    currentState.set('正在加载当前订阅…')
    try {
      const [catalog, subscription] = await Promise.all([
        api.request<PlanCatalog>(`${prefix}/plans`),
        api.request<SubscriptionCurrent>(prefix).catch((error: unknown) => {
          if (String(error).includes('404') || String(error).includes('not found')) return undefined
          throw error
        }),
      ])
      plans = catalog.items ?? []
      current = subscription
      renderCatalog()
      renderCurrent()
      catalogState.set(plans.length ? `共 ${plans.length} 个上架套餐` : '暂无上架套餐')
      currentState.set(current?.planId ? `当前套餐 ${current.planName}` : '尚未指派套餐')
    } catch (error) {
      plans = []
      current = undefined
      renderCatalog()
      renderCurrent()
      catalogState.set(`套餐加载失败：${String(error)}`, 'danger')
      currentState.set(`当前订阅加载失败：${String(error)}`, 'danger')
    }
  }

  async function openPurchase(plan: Plan): Promise<void> {
    if (!canPurchase || !plan.buyable) {
      catalogState.set('当前账号不能购买该套餐。', 'warning')
      return
    }
    catalogState.set('正在读取收款通道…')
    let methods: PayMethod[] = []
    try {
      methods = await api.request<PayMethod[]>(`${prefix}/pay-methods?planId=${plan.id}`)
    } catch (error) {
      catalogState.set(`收款通道读取失败：${String(error)}`, 'danger')
      return
    }
    if (!methods.length) {
      catalogState.set('暂无可用支付方式。收款由 Trade 提供，未接入前不能下单。', 'warning')
      return
    }
    const methodByCode = new Map(methods.map(method => [method.channelCode, method]))
    let pending: PurchaseOrder | undefined
    let selected = methods[0]
    const modal = hostFormModal({
      title: plan.current ? `续费 ${plan.name}` : `购买 ${plan.name}`,
      submitLabel: '去支付',
      fields: [
        { name: 'planName', label: '套餐', disabled: true },
        { name: 'price', label: '价格', disabled: true },
        { name: 'duration', label: '周期', disabled: true },
        {
          name: 'channelCode',
          label: '支付方式',
          kind: 'select',
          required: true,
          options: methods.map(method => ({ value: method.channelCode, label: method.displayName || method.channelCode })),
        },
      ],
      onSubmit: (values, editor) => {
        const channelCode = values.channelCode?.trim() || pending?.channelCode || selected?.channelCode || ''
        const method = methodByCode.get(channelCode) ?? selected
        if (!method) {
          editor.setError('请选择支付方式。')
          return
        }
        selected = method
        editor.setBusy(true)
        const work = pending
          ? Promise.resolve(pending)
          : api.request<PurchaseOrder>(`${prefix}/orders`, {
            method: 'POST',
            body: JSON.stringify({ commandKey: crypto.randomUUID(), planId: plan.id, channelCode: method.channelCode }),
          })
        work
          .then(order => {
            const first = pending == null
            pending = order
            return settlePurchase(editor, order, method, first)
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      planName: plan.name,
      price: formatPrice(plan.priceMinor),
      duration: formatDuration(plan.durationDays),
      channelCode: methods[0]?.channelCode,
    })
  }

  async function settlePurchase(editor: HostFormModalSubmitApi, order: PurchaseOrder, method: PayMethod, first: boolean): Promise<void> {
    if (order.activated || order.status === 'PAID') {
      editor.close()
      catalogState.set(`已开通 ${order.planName}`, 'success')
      await load()
      return
    }
    const showPending = (value: PurchaseOrder, message?: string) => {
      editor.setTitle(`支付 ${value.planName}`)
      editor.setFields(
        [
          { name: 'orderNo', label: '订单号', disabled: true, mono: true },
          { name: 'payNo', label: '支付单号', disabled: true, mono: true },
          { name: 'payStatus', label: '支付状态', disabled: true },
          { name: 'payload', label: '收款信息', kind: 'textarea', disabled: true, rows: 4 },
        ],
        {
          orderNo: value.orderNo,
          payNo: value.payNo,
          payStatus: value.payStatus || value.status,
          payload: payloadText(value.payload),
        },
        `支付 ${value.planName}`,
      )
      if (message) editor.setError(message)
    }
    if (first && !isWallet(method)) {
      showPending(order)
      return
    }
    const settled = isWallet(method)
      ? await api.request<PurchaseOrder>(`${prefix}/orders/${encodeURIComponent(order.orderNo)}/confirm`, {
        method: 'POST',
        body: JSON.stringify({ commandKey: crypto.randomUUID() }),
      })
      : await api.request<PurchaseOrder>(`${prefix}/orders/${encodeURIComponent(order.orderNo)}`)
    if (settled.activated || settled.status === 'PAID') {
      editor.close()
      catalogState.set(`已开通 ${settled.planName}`, 'success')
      await load()
      return
    }
    showPending(settled, isWallet(method) ? '钱包扣款尚未完成。' : '尚未收到 Trade 支付结果，确认支付后可再次提交查询。')
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      dataCard({
        title: '套餐目录',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void load() })],
        status: catalogState.element,
        body: catalogBody,
      }),
      dataCard({
        title: '我的订阅',
        status: currentState.element,
        body: currentBody,
      }),
    ],
  }))
  await load()
}
