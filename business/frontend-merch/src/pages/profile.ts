import type { HostHttpClient } from '@liveshop/host-sdk'
import { badge, button, create, dataCard, definitionList, page, statusLine, ui } from '@liveshop/design-tokens'

interface MerchantProfile {
  merchantId: number
  name: string
  account: string
  externalId: string
  contactName: string
  contactPhone: string
  marketingEmailOptIn: boolean
  marketingSmsOptIn: boolean
  status: string
  version: number
  owner: boolean
}

const prefix = '/merch/identity/profile'

function field(label: string, control: HTMLElement): HTMLElement {
  const node = create('label', ui.field)
  node.append(create('span', undefined, label), control)
  return node
}

function option(label: string, checked: boolean, disabled: boolean): { row: HTMLElement; input: HTMLInputElement } {
  const input = document.createElement('input')
  input.type = 'checkbox'
  input.checked = checked
  input.disabled = disabled
  const row = create('label', 'identity-profile-option')
  row.append(input, document.createTextNode(label))
  return { row, input }
}

function statusBadge(status: string): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  if (status === 'DISABLED') return badge({ label: '停用', tone: 'warning' })
  if (status === 'CLOSED') return badge({ label: '已关闭', tone: 'danger' })
  return badge({ label: status || '—', tone: 'neutral' })
}

export async function renderProfile(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const body = create('div', 'identity-profile-form')
  let current: MerchantProfile | undefined
  let externalId: HTMLInputElement
  let contactName: HTMLInputElement
  let contactPhone: HTMLInputElement
  let marketingEmail: HTMLInputElement
  let marketingSms: HTMLInputElement
  const saveButton = button({ label: '保存', onClick: () => void save(), disabled: true })

  function renderForm(value: MerchantProfile): void {
    const disabled = !value.owner
    const emailOption = option('接收平台邮件营销', value.marketingEmailOptIn, disabled)
    const smsOption = option('接收平台短信营销', value.marketingSmsOptIn, disabled)
    marketingEmail = emailOption.input
    marketingSms = smsOption.input
    externalId = document.createElement('input')
    externalId.className = ui.input
    externalId.maxLength = 64
    externalId.placeholder = '外部编号'
    externalId.value = value.externalId
    externalId.disabled = disabled
    contactName = document.createElement('input')
    contactName.className = ui.input
    contactName.maxLength = 128
    contactName.placeholder = '联系人'
    contactName.value = value.contactName
    contactName.disabled = disabled
    contactPhone = document.createElement('input')
    contactPhone.className = ui.input
    contactPhone.maxLength = 32
    contactPhone.placeholder = '联系电话'
    contactPhone.value = value.contactPhone
    contactPhone.disabled = disabled
    body.replaceChildren(
      create('p', undefined, '维护当前商户对外编号、联系人和平台营销触达偏好。公司名、登录账号和状态由总后台商户管理维护，本页只读。营销开关是商户收平台触达的偏好，不是店铺顾客营销同意。'),
      definitionList([
        { label: '商户名称', value: value.name || '—' },
        { label: '登录账号', value: value.account || '—' },
        { label: '商户状态', value: statusBadge(value.status) },
        { label: '商户 ID', value: String(value.merchantId || '—') },
      ]),
      field('外部编号', externalId),
      field('联系人', contactName),
      field('联系电话', contactPhone),
      emailOption.row,
      smsOption.row,
    )
  }

  async function load(): Promise<void> {
    state.set('正在加载商户信息…')
    try {
      current = await api.request<MerchantProfile>(prefix)
      renderForm(current)
      saveButton.disabled = !current.owner
      state.set(current.owner ? `版本 ${current.version}` : `版本 ${current.version} · 仅所有者可改`, current.owner ? 'neutral' : 'warning')
    } catch (error) {
      current = undefined
      saveButton.disabled = true
      body.replaceChildren()
      state.set(`商户信息加载失败：${String(error)}`, 'danger')
    }
  }

  async function save(): Promise<void> {
    if (!current) return
    if (!current.owner) {
      state.set('仅商户所有者可以保存商户信息。', 'warning')
      return
    }
    state.set('正在保存…')
    try {
      const result = await api.request<{ profile: MerchantProfile }>(prefix, {
        method: 'PUT',
        body: JSON.stringify({
          commandKey: crypto.randomUUID(),
          expectedVersion: current.version,
          externalId: externalId.value.trim(),
          contactName: contactName.value.trim(),
          contactPhone: contactPhone.value.trim(),
          marketingEmailOptIn: marketingEmail.checked,
          marketingSmsOptIn: marketingSms.checked,
        }),
      })
      current = result.profile
      renderForm(current)
      saveButton.disabled = !current.owner
      state.set(`已保存 · 版本 ${current.version}`, 'success')
    } catch (error) {
      state.set(`保存失败：${String(error)}`, 'danger')
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: dataCard({
      title: '商户信息',
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
