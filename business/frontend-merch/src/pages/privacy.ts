import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, statusLine, ui } from '@liveshops/design-tokens'

interface PrivacySetting {
  id?: number
  merchantId: number
  shopId: number
  collectConsent: boolean
  marketingConsent: boolean
  cookieBanner: boolean
  dataRetentionDays: number
  contactEmail: string
  version: number
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
  editable: boolean
}

const prefix = '/merch/identity/privacy'

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
  const row = create('label', 'identity-privacy-option')
  row.append(input, document.createTextNode(label))
  return { row, input }
}

function platformLabel(value: PrivacySetting['platformStatus']): string {
  if (value === 'restricted') return '平台限制'
  if (value === 'suspended') return '平台暂停'
  return '平台正常'
}

export async function renderPrivacy(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const state = statusLine()
  const body = create('div', 'identity-privacy-form')
  let current: PrivacySetting | undefined
  let collect: HTMLInputElement
  let marketing: HTMLInputElement
  let cookie: HTMLInputElement
  let retention: HTMLInputElement
  let email: HTMLInputElement
  const saveButton = button({ label: '保存', onClick: () => void save(), disabled: true })

  function shopLabel(): string {
    const tenant = context.tenant
    if (!tenant?.merchantId || !tenant.shopId) return '当前店铺'
    return `商户 ${tenant.merchantId} · 店铺 ${tenant.shopId}`
  }

  function renderForm(value: PrivacySetting): void {
    const disabled = !value.editable
    const collectOption = option('结账前收集隐私同意', value.collectConsent, disabled)
    const marketingOption = option('允许独立收集营销同意', value.marketingConsent, disabled)
    const cookieOption = option('展示 Cookie 提示', value.cookieBanner, disabled)
    collect = collectOption.input
    marketing = marketingOption.input
    cookie = cookieOption.input
    retention = document.createElement('input')
    retention.className = ui.input
    retention.type = 'number'
    retention.min = '1'
    retention.max = '3650'
    retention.value = String(value.dataRetentionDays)
    retention.disabled = disabled
    email = document.createElement('input')
    email.className = ui.input
    email.type = 'email'
    email.placeholder = 'privacy@example.com'
    email.value = value.contactEmail
    email.disabled = disabled
    const overlay = create('div', 'identity-privacy-overlay')
    overlay.append(badge({
      label: platformLabel(value.platformStatus),
      tone: value.platformStatus === 'active' ? 'success' : 'warning',
    }))
    if (value.platformReasonPublic) overlay.append(create('p', undefined, value.platformReasonPublic))
    body.replaceChildren(
      create('p', undefined, '控制顾客数据收集同意、营销同意、Cookie 提示及数据保留周期。平台叠加层由总后台商户能力治理维护，本页不可改写。'),
      overlay,
      collectOption.row,
      marketingOption.row,
      cookieOption.row,
      field('数据保留天数', retention),
      field('隐私联系人邮箱', email),
    )
  }

  async function load(): Promise<void> {
    state.set('正在加载客户隐私设置…')
    try {
      current = await api.request<PrivacySetting>(prefix)
      renderForm(current)
      saveButton.disabled = !current.editable
      const saved = current.version > 0 ? `版本 ${current.version}` : '尚未保存，显示默认值'
      const overlay = current.editable ? saved : `${saved} · 平台已限制，只读`
      state.set(`${shopLabel()} · ${overlay}`, current.editable ? 'neutral' : 'warning')
    } catch (error) {
      current = undefined
      saveButton.disabled = true
      body.replaceChildren()
      state.set(`客户隐私加载失败：${String(error)}`, 'danger')
    }
  }

  async function save(): Promise<void> {
    if (!current) return
    if (!current.editable) {
      state.set('平台已限制该店铺隐私设置，无法保存。', 'warning')
      return
    }
    const days = Number(retention.value)
    if (!Number.isSafeInteger(days) || days < 1 || days > 3650) {
      state.set('数据保留天数必须是 1–3650 的整数。', 'danger')
      return
    }
    state.set('正在保存…')
    try {
      const result = await api.request<{ setting: PrivacySetting }>(prefix, {
        method: 'PUT',
        body: JSON.stringify({
          commandKey: crypto.randomUUID(),
          expectedVersion: current.version,
          collectConsent: collect.checked,
          marketingConsent: marketing.checked,
          cookieBanner: cookie.checked,
          dataRetentionDays: days,
          contactEmail: email.value.trim(),
        }),
      })
      current = result.setting
      renderForm(current)
      saveButton.disabled = !current.editable
      state.set(`${shopLabel()} · 已保存 · 版本 ${current.version}`, 'success')
    } catch (error) {
      state.set(`保存失败：${String(error)}`, 'danger')
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: dataCard({
      title: '客户隐私',
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
