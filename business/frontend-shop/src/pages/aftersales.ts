import type { RemoteModuleContext } from '@liveshops/host-sdk'
import { create, notify } from '@liveshops/design-tokens'
import { cta, emptyState } from '@liveshops/design-tokens/storefront'

import { IdentityShopApi, type Aftersale } from '../api'

function requireLogin(context: RemoteModuleContext, error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error)
  if (/401|403|unauthorized|forbidden|customer/i.test(message)) {
    notify('请先登录后查看售后', 'warning')
    context.navigate('/login')
    return true
  }
  notify(message, 'danger')
  return false
}

function statusLabel(status: string): string {
  if (status === 'PENDING') return '处理中'
  if (status === 'APPROVED') return '已同意'
  if (status === 'REJECTED') return '已拒绝'
  if (status === 'REFUNDED') return '已退款'
  if (status === 'CLOSED') return '已关闭'
  return status || '未知'
}

function typeLabel(type: string): string {
  if (type === 'REFUND_ONLY') return '仅退款'
  if (type === 'RETURN_REFUND') return '退货退款'
  return type || '售后'
}

export async function renderAftersales(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  const status = new URLSearchParams(window.location.search).get('status') || ''
  let items: Aftersale[] = []
  try {
    items = (await api.aftersales(status)).items ?? []
  } catch (error) {
    if (requireLogin(context, error)) return
  }
  const root = create('main', 'identity-address')
  const layout = create('div', 'identity-address__layout')
  layout.append(create('h1', undefined, '我的售后'))
  if (!items.length) {
    layout.append(emptyState('还没有售后工单'))
    root.append(layout)
    container.replaceChildren(root)
    return
  }
  const list = create('div', 'identity-address__list')
  for (const item of items) {
    const card = create('button', 'identity-address__card')
    card.type = 'button'
    const title = create('div', 'identity-address__who')
    title.append(create('b', undefined, typeLabel(item.type)), create('em', undefined, statusLabel(item.status)))
    card.append(title)
    card.append(create('p', undefined, item.reason || `订单 #${item.orderId}`))
    card.addEventListener('click', () => context.navigate(`/aftersales/detail?aftersaleId=${item.id}`))
    list.append(card)
  }
  layout.append(list)
  root.append(layout)
  container.replaceChildren(root)
}

export async function renderAftersaleDetail(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const id = Number(new URLSearchParams(window.location.search).get('aftersaleId') || '0')
  const api = new IdentityShopApi(context.api)
  let item: Aftersale | undefined
  if (id > 0) {
    try {
      item = (await api.aftersale(id)).aftersale
    } catch (error) {
      if (requireLogin(context, error)) return
    }
  }
  const root = create('main', 'identity-address')
  const layout = create('div', 'identity-address__layout')
  layout.append(create('h1', undefined, '售后详情'))
  if (!item) {
    layout.append(emptyState('未找到该售后工单'))
    layout.append(cta({ label: '返回售后列表', variant: 'secondary', onClick: () => context.navigate('/aftersales') }))
    root.append(layout)
    container.replaceChildren(root)
    return
  }
  const card = create('article', 'identity-address__card')
  const who = create('div', 'identity-address__who')
  who.append(create('b', undefined, typeLabel(item.type)), create('em', undefined, statusLabel(item.status)))
  card.append(who)
  card.append(create('p', undefined, `订单 #${item.orderId}`))
  card.append(create('p', undefined, item.reason))
  if (item.handleNote) card.append(create('p', undefined, `处理说明：${item.handleNote}`))
  for (const line of item.items ?? []) {
    card.append(create('p', undefined, `${line.title} × ${line.quantity}`))
  }
  layout.append(card)
  layout.append(cta({ label: '返回售后列表', variant: 'secondary', onClick: () => context.navigate('/aftersales') }))
  root.append(layout)
  container.replaceChildren(root)
}
