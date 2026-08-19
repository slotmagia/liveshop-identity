import type { HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, definitionList, emptyState, grid, notify, page, ui } from '@liveshops/design-tokens'

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

interface SaveResult {
  profile: MerchantProfile
  replayed: boolean
}

const prefix = '/merch/identity/profile'

function field(label: string, control: HTMLElement): HTMLElement {
  const node = create('label', ui.field)
  node.append(create('span', undefined, label), control)
  return node
}

function textInput(value: string, placeholder: string, maxLength: number, disabled: boolean): HTMLInputElement {
  const input = document.createElement('input')
  input.className = ui.input
  input.maxLength = maxLength
  input.placeholder = placeholder
  input.value = value
  input.disabled = disabled
  return input
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
    const emailOption = option('同意接收营销电子邮件', value.marketingEmailOptIn, disabled)
    const smsOption = option('同意接收营销短信', value.marketingSmsOptIn, disabled)
    marketingEmail = emailOption.input
    marketingSms = smsOption.input
    externalId = textInput(value.externalId, '外部对接编号（可选）', 64, disabled)
    contactName = textInput(value.contactName, '主要联系人', 128, disabled)
    contactPhone = textInput(value.contactPhone, '电话', 32, disabled)
    const marketing = create('div', 'identity-profile-marketing')
    marketing.append(
      create('p', 'identity-profile-caption', '营销状态'),
      emailOption.row,
      smsOption.row,
      create('p', 'identity-profile-hint', '在为该商户发送营销邮件或短信之前，应征得其许可。这是商户收平台触达的偏好，不是店铺顾客营销同意。'),
    )
    body.replaceChildren(
      definitionList([
        { label: '商户名称', value: value.name || '—' },
        { label: '登录账号', value: value.account || '—' },
        { label: '商户状态', value: statusBadge(value.status) },
      ]),
      field('公司 ID', externalId),
      grid([field('联系人', contactName), field('电话', contactPhone)]),
      marketing,
    )
  }

  async function load(): Promise<void> {
    saveButton.disabled = true
    try {
      current = await api.request<MerchantProfile>(prefix)
      renderForm(current)
      saveButton.disabled = !current.owner
      if (!current.owner) notify('仅商户所有者可以保存商户信息。', 'warning')
    } catch (error) {
      current = undefined
      body.replaceChildren(emptyState('当前无法加载商户信息'))
      notify(`商户信息加载失败：${String(error)}`, 'danger')
    }
  }

  async function save(): Promise<void> {
    if (!current) return
    if (!current.owner) {
      notify('仅商户所有者可以保存商户信息。', 'warning')
      return
    }
    const nextExternalId = externalId.value.trim()
    const nextContactName = contactName.value.trim()
    const nextContactPhone = contactPhone.value.trim()
    if ([...nextExternalId].length > 64) {
      notify('公司 ID 最长 64 个字符。', 'danger')
      return
    }
    if ([...nextContactName].length > 128) {
      notify('联系人最长 128 个字符。', 'danger')
      return
    }
    if ([...nextContactPhone].length > 32) {
      notify('电话最长 32 个字符。', 'danger')
      return
    }
    saveButton.disabled = true
    try {
      const result = await api.request<SaveResult>(prefix, {
        method: 'PUT',
        body: JSON.stringify({
          commandKey: crypto.randomUUID(),
          expectedVersion: current.version,
          externalId: nextExternalId,
          contactName: nextContactName,
          contactPhone: nextContactPhone,
          marketingEmailOptIn: marketingEmail.checked,
          marketingSmsOptIn: marketingSms.checked,
        }),
      })
      current = result.profile
      renderForm(current)
      saveButton.disabled = !current.owner
      notify(result.replayed ? '该保存命令已存在，已返回原记录' : '已保存', 'success')
    } catch (error) {
      saveButton.disabled = !current.owner
      notify(`保存失败：${String(error)}`, 'danger')
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: dataCard({
      title: '联系与对接',
      actions: [
        button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
        saveButton,
      ],
      body,
    }),
  }))
  await load()
}
