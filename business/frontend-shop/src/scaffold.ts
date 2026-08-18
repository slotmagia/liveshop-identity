import type { RemoteModuleContext } from '@liveshop/host-sdk'
import { create, notify } from '@liveshop/design-tokens'
import { cta, featureScaffold } from '@liveshop/design-tokens/storefront'

const pages: Record<string, { title: string; description: string; metrics: string[]; sections: Array<[string, string, string[]]>; action?: [string, string] }> = {
  'identity.placeholder.shop.profile': {
    title: '我的 LiveShop', description: '个人中心组合身份信息、订单摘要、钱包、优惠券、地址和售后入口，摘要区域独立失败。',
    metrics: ['待付款', '待发货', '待收货', '售后中'],
    sections: [['账', '账户与安全', ['个人资料', '登录与设备', '隐私设置']], ['址', '收货地址', ['默认地址', '新增与编辑', '结算回跳']], ['服', '客户服务', ['售后进度', '投诉与协商', '物流协作']]],
  },
  'identity.placeholder.shop.aftersales': {
    title: '我的售后', description: '按申请类型和处理状态展示售后工单，退款资金状态由 Trade 提供摘要。',
    metrics: ['处理中', '待退货', '退款中', '已完成'],
    sections: [['单', '售后工单', ['退款/退货类型', '申请原因与凭证', '商户处理意见']], ['流', '退货物流', ['运单信息', '签收状态', '物流异常']], ['款', '退款协作', ['Identity 裁决售后流程', 'Trade 裁决退款结果']]],
  },
  'identity.placeholder.shop.aftersale-detail': {
    title: '售后详情', description: '展示售后状态机、处理记录、退货物流和退款摘要，不允许浏览器覆盖服务端终态。',
    metrics: ['当前状态', '申请金额', '退货状态', '退款状态'],
    sections: [['进', '处理进度', ['申请记录', '商户审核', '完成或关闭']], ['证', '申请凭证', ['原因说明', '图片附件', '协商记录']], ['联', '关联信息', ['原订单', '退货物流', '退款结果']]],
    action: ['返回售后列表', '/aftersales'],
  },
  'identity.placeholder.shop.address': {
    title: '地址簿', description: '管理当前顾客的收货地址、默认地址和结算选择，不从浏览器接收其他主体标识。',
    metrics: ['全部地址', '默认地址', '可配送', '需补充'],
    sections: [['址', '地址列表', ['默认地址标记', '国家/地区层级', '联系人与电话']], ['配', '配送校验', ['结算时重新校验', '不可配送原因明确显示']]],
    action: ['新增收货地址', '/address/edit'],
  },
  'identity.placeholder.shop.address-edit': {
    title: '编辑收货地址', description: '填写联系人、电话、国家地区和详细地址；创建与更新都使用稳定命令标识。',
    metrics: ['联系人', '手机号', '国家地区', '默认地址'],
    sections: [['人', '收件信息', ['姓名', '国家区号', '联系电话']], ['地', '地址信息', ['国家/省市区', '详细地址', '邮政编码']], ['存', '保存规则', ['服务端字段校验', '并发更新版本保护']]],
    action: ['返回地址簿', '/address'],
  },
  'identity.placeholder.shop.favorites': {
    title: '我的收藏', description: '展示顾客收藏的商品引用；商品标题、价格和可售状态实时读取 Catalog。',
    metrics: ['收藏商品', '有货', '已下架', '降价提醒'],
    sections: [['藏', '收藏列表', ['取消收藏', '进入商品详情', '无效商品状态']], ['协', 'Catalog 协作', ['Identity 保存收藏关系', 'Catalog 提供商品实时摘要']]],
    action: ['继续逛商品', '/products'],
  },
}

export function renderIdentityScaffold(container: HTMLElement, context: RemoteModuleContext): void {
  if (context.contributionId === 'identity.placeholder.shop.profile') {
    renderProfile(container, context)
    return
  }
  if (context.contributionId === 'identity.shop.login') {
    renderLogin(container, context)
    return
  }
  const page = pages[context.contributionId]
  if (!page) return
  const actions = page.action ? cta({ label: page.action[0], onClick: () => context.navigate(page.action![1]) }) : undefined
  container.replaceChildren(featureScaffold({
    eyebrow: 'Identity · Customer', title: page.title, description: page.description, actions,
    metrics: page.metrics.map(label => ({ label, value: '—' })),
    sections: page.sections.map(([icon, title, items]) => ({ icon, title, description: '等待对应顾客领域契约与真实数据接入。', items })),
  }))
}

function renderLogin(container: HTMLElement, context: RemoteModuleContext): void {
  const shopCode = new URLSearchParams(window.location.search).get('shopCode') || ''
  const root = create('main', 'identity-login')
  const card = create('section', 'identity-login__card')
  card.append(create('p', 'identity-login__brand', 'WOKFOY'))
  card.append(create('h1', '', '登录 / 注册'))
  card.append(create('p', 'identity-login__copy', '输入店铺编码和手机或邮箱，获取验证码。本页核验挑战，不升级游客会话。'))

  const shop = field('店铺编码', 'text', shopCode, 'shop-code')
  const phone = field('手机号', 'tel', '', 'phone')
  const email = field('邮箱', 'email', '', 'email')
  const code = field('验证码', 'text', '', 'otp-code')
  code.input.maxLength = 6
  card.append(shop.wrap, phone.wrap, email.wrap, code.wrap)

  let challengeId = ''
  const send = create('button', 'identity-login__secondary', '获取验证码')
  send.type = 'button'
  send.addEventListener('click', () => {
    send.disabled = true
    context.api.request<{ challengeId: string }>('/shop/identity/login/otp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ shopCode: shop.input.value.trim(), phone: phone.input.value.trim(), email: email.input.value.trim() }),
    }).then(data => {
      challengeId = data.challengeId
      notify('验证码已发送', 'success')
    }).catch(error => {
      notify(error instanceof Error ? error.message : String(error), 'danger')
    }).finally(() => { send.disabled = false })
  })

  const submit = create('button', 'identity-login__primary', '核验验证码')
  submit.type = 'button'
  submit.addEventListener('click', () => {
    if (!challengeId) {
      notify('请先获取验证码', 'warning')
      return
    }
    submit.disabled = true
    context.api.request('/shop/identity/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ shopCode: shop.input.value.trim(), challengeId, code: code.input.value.trim() }),
    }).then(() => {
      notify('验证码正确。会话升级将在后续切片接入。', 'success')
    }).catch(error => {
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

function renderProfile(container: HTMLElement, context: RemoteModuleContext): void {
  const root = create('main', 'identity-profile')
  const layout = create('div', 'identity-profile__layout')

  const hero = create('section', 'identity-profile__hero')
  hero.append(create('p', 'identity-profile__brand', 'WOKFOY'))
  const identity = create('button', 'identity-profile__identity')
  identity.type = 'button'
  identity.append(create('b', undefined, '登录 / 注册'), create('span', undefined, '登录后查看订单、钱包、优惠券与售后进度'))
  identity.addEventListener('click', () => context.navigate('/login'))
  hero.append(identity)
  const stats = create('div', 'identity-profile__stats')
  for (const [icon, value, label, route] of [
    ['人', '', '账户', '/login'], ['♥', '—', '收藏', '/favorites'], ['单', '—', '足迹', '/orders'], ['券', '—', '优惠券', '/coupons'],
  ]) {
    const button = create('button')
    button.type = 'button'
    button.append(create('strong', undefined, value || icon), create('span', undefined, label))
    button.addEventListener('click', () => context.navigate(route))
    stats.append(button)
  }
  const signIn = create('button', 'identity-profile__signin', '立即登录')
  signIn.type = 'button'
  signIn.addEventListener('click', () => context.navigate('/login'))
  hero.append(stats, signIn)
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
