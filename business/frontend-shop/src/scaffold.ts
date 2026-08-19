import type { RemoteModuleContext } from '@liveshop/host-sdk'
import { create, notify } from '@liveshop/design-tokens'

import { IdentityShopApi, type SMSRegion } from './api'

export function renderIdentityScaffold(container: HTMLElement, context: RemoteModuleContext): void {
  if (context.contributionId === 'identity.shop.profile' || context.contributionId === 'identity.placeholder.shop.profile') {
    void renderProfile(container, context)
    return
  }
  if (context.contributionId === 'identity.shop.login') {
    void renderLogin(container, context)
  }
}

async function renderLogin(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  const shopCode = new URLSearchParams(window.location.search).get('shopCode') || ''
  const root = create('main', 'identity-login')
  const card = create('section', 'identity-login__card')
  card.append(create('p', 'identity-login__brand', 'WOKFOY'))
  card.append(create('h1', '', '登录 / 注册'))
  card.append(create('p', 'identity-login__copy', '输入店铺编码和手机或邮箱，获取验证码。核验通过后由店铺 Host 升级当前游客会话。'))

  const shop = field('店铺编码', 'text', shopCode, 'shop-code')
  const regionWrap = create('label', 'identity-login__field')
  regionWrap.append(create('span', '', '区号'))
  const region = document.createElement('select')
  region.name = 'sms-region'
  const fallback = document.createElement('option')
  fallback.value = '+86'
  fallback.textContent = '+86'
  region.append(fallback)
  regionWrap.append(region)
  const phone = field('手机号', 'tel', '', 'phone')
  const email = field('邮箱', 'email', '', 'email')
  const code = field('验证码', 'text', '', 'otp-code')
  code.input.maxLength = 6
  card.append(shop.wrap, regionWrap, phone.wrap, email.wrap, code.wrap)

  const loadRegions = (): void => {
    const codeValue = shop.input.value.trim()
    if (!codeValue) return
    api.loginSMSRegions(codeValue).then(result => {
      const items = result.items ?? []
      region.replaceChildren()
      if (!items.length) {
        region.append(fallback)
        return
      }
      for (const item of items) {
        const option = document.createElement('option')
        option.value = item.dialCode
        option.textContent = regionLabel(item)
        region.append(option)
      }
    }).catch(() => {
      region.replaceChildren(fallback)
    })
  }
  shop.input.addEventListener('change', loadRegions)
  loadRegions()

  let challengeId = ''
  let verified = false
  const send = create('button', 'identity-login__secondary', '获取验证码')
  send.type = 'button'
  send.addEventListener('click', () => {
    send.disabled = true
    const local = phone.input.value.trim()
    const dial = region.value.trim()
    const fullPhone = local && !local.startsWith('+') && dial ? `${dial}${local.replace(/^0+/, '')}` : local
    context.api.request<{ challengeId: string }>('/shop/identity/login/otp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ shopCode: shop.input.value.trim(), phone: fullPhone, email: email.input.value.trim() }),
    }).then(data => {
      challengeId = data.challengeId
      verified = false
      notify('验证码已发送', 'success')
    }).catch(error => {
      notify(error instanceof Error ? error.message : String(error), 'danger')
    }).finally(() => { send.disabled = false })
  })

  const submit = create('button', 'identity-login__primary', '登录')
  submit.type = 'button'
  submit.addEventListener('click', () => {
    if (!challengeId) {
      notify('请先获取验证码', 'warning')
      return
    }
    submit.disabled = true
    const redeem = async (): Promise<void> => {
      if (!context.login) {
        throw new Error('当前 Host 不支持会话升级')
      }
      context.navigate('/profile')
      await context.login({ challengeId })
    }
    const verify = verified
      ? Promise.resolve()
      : context.api.request('/shop/identity/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ shopCode: shop.input.value.trim(), challengeId, code: code.input.value.trim() }),
        }).then(() => { verified = true })
    verify.then(redeem).catch(error => {
      notify(error instanceof Error ? error.message : String(error), 'danger')
    }).finally(() => { submit.disabled = false })
  })

  const back = create('button', 'identity-login__link', '返回个人中心')
  back.type = 'button'
  back.addEventListener('click', () => context.navigate('/profile'))
  card.append(send, submit, back)
  root.append(card)
  container.replaceChildren(root)
}

function regionLabel(item: SMSRegion): string {
  return [item.emoji, item.dialCode, item.name].filter(Boolean).join(' ')
}

function field(label: string, type: string, value: string, name: string): { wrap: HTMLElement; input: HTMLInputElement } {
  const wrap = create('label', 'identity-login__field')
  wrap.append(create('span', '', label))
  const input = document.createElement('input')
  input.type = type
  input.name = name
  input.value = value
  input.autocomplete = 'off'
  wrap.append(input)
  return { wrap, input }
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
