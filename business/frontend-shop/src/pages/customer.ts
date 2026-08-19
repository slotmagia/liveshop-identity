import type { RemoteModuleContext } from '@liveshops/host-sdk'
import { create, notify } from '@liveshops/design-tokens'
import { cta, emptyState } from '@liveshops/design-tokens/storefront'

import { IdentityShopApi, type Address, type AddressInput } from '../api'

function requireLogin(context: RemoteModuleContext, error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error)
  if (/401|403|unauthorized|forbidden|customer/i.test(message)) {
    notify('请先登录后再管理地址', 'warning')
    context.navigate('/login')
    return true
  }
  notify(message, 'danger')
  return false
}

function region(item: Address): string {
  return [item.country, item.province, item.city, item.district].filter(Boolean).join(' ')
}

export async function renderAddressBook(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  let items: Address[] = []
  const root = create('main', 'identity-address')
  const layout = create('div', 'identity-address__layout')
  root.append(layout)
  container.replaceChildren(root)

  const render = (): void => {
    layout.replaceChildren()
    const heading = create('header', 'identity-address__heading')
    heading.append(create('h1', undefined, '地址簿'))
    heading.append(cta({ label: '新增地址', onClick: () => context.navigate('/address/edit') }))
    layout.append(heading)
    if (!items.length) {
      layout.append(emptyState('还没有收货地址'))
      return
    }
    const list = create('div', 'identity-address__list')
    for (const item of items) {
      const card = create('article', item.isDefault ? 'identity-address__card is-default' : 'identity-address__card')
      const title = create('div', 'identity-address__who')
      title.append(create('b', undefined, item.recipient), create('span', undefined, item.phone))
      if (item.isDefault) title.append(create('em', undefined, '默认'))
      card.append(title)
      card.append(create('p', undefined, `${region(item)} ${item.detail}`.trim()))
      const actions = create('div', 'identity-address__actions')
      const edit = create('button', undefined, '编辑')
      edit.type = 'button'
      edit.addEventListener('click', () => context.navigate(`/address/edit?addressId=${item.id}`))
      actions.append(edit)
      if (!item.isDefault) {
        const makeDefault = create('button', undefined, '设为默认')
        makeDefault.type = 'button'
        makeDefault.addEventListener('click', () => {
          api.replaceDefault(item.id, crypto.randomUUID(), item.version).then(load).catch(error => requireLogin(context, error))
        })
        actions.append(makeDefault)
      }
      const remove = create('button', undefined, '删除')
      remove.type = 'button'
      remove.addEventListener('click', () => {
        api.deleteAddress(item.id, crypto.randomUUID(), item.version).then(load).catch(error => requireLogin(context, error))
      })
      actions.append(remove)
      card.append(actions)
      list.append(card)
    }
    layout.append(list)
  }

  const load = async (): Promise<void> => {
    try {
      const result = await api.addresses()
      items = result.items ?? []
    } catch (error) {
      items = []
      if (requireLogin(context, error)) return
    }
    render()
  }

  render()
  await load()
}

function readForm(form: HTMLFormElement): AddressInput {
  const value = (name: string) => (form.elements.namedItem(name) as HTMLInputElement | null)?.value.trim() ?? ''
  return {
    recipient: value('recipient'),
    phone: value('phone'),
    country: value('country'),
    province: value('province'),
    city: value('city'),
    district: value('district'),
    detail: value('detail'),
    postalCode: value('postalCode'),
    isDefault: (form.elements.namedItem('isDefault') as HTMLInputElement | null)?.checked ?? false,
  }
}

function field(label: string, name: string, value: string, required = false): HTMLLabelElement {
  const wrap = create('label', 'identity-login__field')
  wrap.append(create('span', undefined, label))
  const input = document.createElement('input')
  input.name = name
  input.value = value
  input.required = required
  wrap.append(input)
  return wrap
}

export async function renderAddressEdit(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  const addressId = Number(new URLSearchParams(window.location.search).get('addressId') || '0')
  let current: Address | undefined
  if (addressId > 0) {
    try {
      const result = await api.addresses()
      current = (result.items ?? []).find(item => item.id === addressId)
    } catch (error) {
      requireLogin(context, error)
      return
    }
  }

  const root = create('main', 'identity-address')
  const layout = create('div', 'identity-address__layout')
  layout.append(create('h1', undefined, current ? '编辑收货地址' : '新增收货地址'))
  const form = document.createElement('form')
  form.className = 'identity-address__form'
  form.append(
    field('收件人', 'recipient', current?.recipient ?? '', true),
    field('手机号', 'phone', current?.phone ?? '', true),
    field('国家/地区', 'country', current?.country ?? ''),
    field('省', 'province', current?.province ?? ''),
    field('市', 'city', current?.city ?? ''),
    field('区', 'district', current?.district ?? ''),
    field('详细地址', 'detail', current?.detail ?? '', true),
    field('邮编', 'postalCode', current?.postalCode ?? ''),
  )
  const defaultWrap = create('label', 'identity-address__check')
  const checkbox = document.createElement('input')
  checkbox.type = 'checkbox'
  checkbox.name = 'isDefault'
  checkbox.checked = current?.isDefault ?? false
  defaultWrap.append(checkbox, document.createTextNode('设为默认地址'))
  form.append(defaultWrap)
  const commandKey = crypto.randomUUID()
  form.addEventListener('submit', event => {
    event.preventDefault()
    const input = readForm(form)
    const request = current
      ? api.updateAddress(current.id, input, commandKey, current.version)
      : api.createAddress(input, commandKey)
    request.then(() => {
      notify('地址已保存', 'success')
      context.navigate('/address')
    }).catch(error => requireLogin(context, error))
  })
  const actions = create('div', 'identity-address__actions')
  const submit = create('button', 'identity-login__primary', '保存')
  submit.type = 'submit'
  const back = create('button', 'identity-login__secondary', '返回地址簿')
  back.type = 'button'
  back.addEventListener('click', () => context.navigate('/address'))
  actions.append(submit, back)
  form.append(actions)
  layout.append(form)
  root.append(layout)
  container.replaceChildren(root)
}

export async function renderFavorites(container: HTMLElement, context: RemoteModuleContext): Promise<void> {
  const api = new IdentityShopApi(context.api)
  let items: Array<{ productId: number; createdAt: number }> = []
  const root = create('main', 'identity-address')
  const layout = create('div', 'identity-address__layout')
  root.append(layout)
  container.replaceChildren(root)

  const render = (): void => {
    layout.replaceChildren()
    const heading = create('header', 'identity-address__heading')
    heading.append(create('h1', undefined, '我的收藏'))
    heading.append(cta({ label: '继续逛商品', variant: 'secondary', onClick: () => context.navigate('/products') }))
    layout.append(heading)
    if (!items.length) {
      layout.append(emptyState('还没有收藏商品'))
      return
    }
    const list = create('div', 'identity-address__list')
    for (const item of items) {
      const card = create('article', 'identity-address__card')
      const title = create('div', 'identity-address__who')
      title.append(create('b', undefined, `商品 #${item.productId}`))
      card.append(title)
      card.append(create('p', undefined, '商品标题与价格请进入 Catalog 详情查看'))
      const actions = create('div', 'identity-address__actions')
      const open = create('button', undefined, '查看商品')
      open.type = 'button'
      open.addEventListener('click', () => context.navigate(`/product/detail?productId=${item.productId}`))
      const remove = create('button', undefined, '取消收藏')
      remove.type = 'button'
      remove.addEventListener('click', () => {
        api.removeWishlist(item.productId).then(load).catch(error => requireLogin(context, error))
      })
      actions.append(open, remove)
      card.append(actions)
      list.append(card)
    }
    layout.append(list)
  }

  const load = async (): Promise<void> => {
    try {
      const result = await api.wishlist()
      items = result.items ?? []
    } catch (error) {
      items = []
      if (requireLogin(context, error)) return
    }
    render()
  }

  render()
  await load()
}
