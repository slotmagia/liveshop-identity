import type { RemoteModuleContext } from '@liveshops/host-sdk'
import { create } from '@liveshops/design-tokens'

import { IdentityShopApi } from './api'

export function renderIdentityScaffold(container: HTMLElement, context: RemoteModuleContext): void {
  if (context.contributionId === 'identity.shop.profile' || context.contributionId === 'identity.placeholder.shop.profile') {
    void renderProfile(container, context)
  }
}

async function renderProfile(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  let signedIn = false
  let displayName = '登录 / 注册'
  let copy = '登录后查看订单、钱包、优惠券与售后进度'
  try {
    const profile = await api.profile()
    signedIn = profile.signedIn
    displayName = profile.displayName || (signedIn ? '已登录' : '登录 / 注册')
    copy = signedIn ? '订单、钱包和优惠券摘要由各模块页面提供' : copy
  } catch {
    signedIn = false
  }

  const root = create('main', 'identity-profile')
  const layout = create('div', 'identity-profile__layout')

  const hero = create('section', 'identity-profile__hero')
  hero.append(create('p', 'identity-profile__brand', 'WOKFOY'))
  const identity = create('button', 'identity-profile__identity')
  identity.type = 'button'
  identity.append(create('b', undefined, displayName), create('span', undefined, copy))
  identity.addEventListener('click', () => context.navigate(signedIn ? '/address' : '/login'))
  hero.append(identity)
  const stats = create('div', 'identity-profile__stats')
  for (const [icon, value, label, route] of [
    ['人', '', '账户', signedIn ? '/address' : '/login'], ['♥', '—', '收藏', '/favorites'], ['单', '—', '足迹', '/orders'], ['券', '—', '优惠券', '/coupons'],
  ]) {
    const button = create('button')
    button.type = 'button'
    button.append(create('strong', undefined, value || icon), create('span', undefined, label))
    button.addEventListener('click', () => context.navigate(route))
    stats.append(button)
  }
  if (!signedIn) {
    const signIn = create('button', 'identity-profile__signin', '立即登录')
    signIn.type = 'button'
    signIn.addEventListener('click', () => context.navigate('/login'))
    hero.append(stats, signIn)
  } else {
    hero.append(stats)
  }
  layout.append(hero)

  const wallet = create('section', 'identity-profile__card identity-profile__wallet')
  const walletHeader = create('div', 'identity-profile__section-head')
  walletHeader.append(create('h2', undefined, '我的钱包'))
  const topUp = create('button', undefined, '去充值')
  topUp.type = 'button'
  topUp.addEventListener('click', () => context.navigate('/balance'))
  walletHeader.append(topUp)
  wallet.append(walletHeader, create('strong', 'identity-profile__balance', '—'), create('p', undefined, '登录后显示可用余额和冻结金额'))
  layout.append(wallet)

  const orders = create('section', 'identity-profile__card')
  const orderHeader = create('div', 'identity-profile__section-head')
  orderHeader.append(create('h2', undefined, '我的订单'))
  const viewAll = create('button', undefined, '查看全部')
  viewAll.type = 'button'
  viewAll.addEventListener('click', () => context.navigate('/orders'))
  orderHeader.append(viewAll)
  const shortcuts = create('div', 'identity-profile__shortcuts')
  for (const [icon, label, route] of [['时', '待付款', '/orders?status=unpaid'], ['付', '待发货', '/orders?status=paid'], ['运', '待收货', '/orders?status=shipped'], ['完', '已完成', '/orders?status=finished']]) {
    const button = create('button')
    button.type = 'button'
    button.append(create('span', undefined, icon), create('b', undefined, label))
    button.addEventListener('click', () => context.navigate(route))
    shortcuts.append(button)
  }
  orders.append(orderHeader, shortcuts)
  layout.append(orders)

  layout.append(profileRows('账户服务', [
    ['藏', '我的收藏', '/favorites'], ['址', '收货地址', '/address'], ['拍', '拍卖订单', '/auctions'], ['售', '我的售后', '/aftersales'],
  ], context))
  layout.append(profileRows('服务与政策', [
    ['盾', '隐私政策', ''], ['退', '退货政策', '/aftersales'], ['运', '配送政策', '/address'], ['介', '关于我们', ''],
  ], context))
  root.append(layout)
  container.replaceChildren(root)
}

function profileRows(title: string, rows: string[][], context: RemoteModuleContext): HTMLElement {
  const card = create('section', 'identity-profile__card identity-profile__rows')
  card.append(create('h2', undefined, title))
  for (const [icon, label, route] of rows) {
    const button = create('button')
    button.type = 'button'
    button.append(create('span', undefined, icon), create('b', undefined, label), create('i', undefined, '›'))
    if (route) button.addEventListener('click', () => context.navigate(route))
    card.append(button)
  }
  return card
}
