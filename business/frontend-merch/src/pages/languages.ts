import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { randomUUID } from '@liveshop/host-sdk'
import { button, dataCard, notify, page } from '@liveshop/design-tokens'

interface LanguageItem {
  locale: string
  label: string
  published: boolean
  isDefault: boolean
  sortOrder: number
  completionPercent: number
  platformStatus: string
}

interface Languages {
  defaultLocale: string
  version: number
  items: LanguageItem[]
}

const prefix = '/merch/identity/languages'

export async function renderLanguages(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.language.manage')
  const body = document.createElement('div')
  body.className = 'identity-languages'
  const saveButton = button({ label: '保存', onClick: () => void save(), disabled: !canManage })
  let current: Languages | undefined
  const published = new Set<string>()
  let defaultLocale = 'zh-CN'

  root.replaceChildren(page({
    showSummary: false,
    children: [
      dataCard({ title: '已发布语言', actions: canManage ? [saveButton] : [], body }),
    ],
  }))

  async function load(): Promise<void> {
    try {
      current = await api.request<Languages>(prefix)
      published.clear()
      defaultLocale = current.defaultLocale || 'zh-CN'
      for (const item of current.items || []) {
        if (item.published) published.add(item.locale)
      }
      if (!published.size) published.add(defaultLocale)
      render()
    } catch (error) {
      notify(`无法加载店铺语言：${String(error)}`, 'danger')
    }
  }

  function render(): void {
    const items = current?.items || []
    body.replaceChildren()
    if (!items.length) {
      body.textContent = '没有可选语言。'
      return
    }
    for (const item of items) {
      const row = document.createElement('label')
      row.style.display = 'flex'
      row.style.alignItems = 'center'
      row.style.gap = '12px'
      row.style.padding = '8px 0'
      const box = document.createElement('input')
      box.type = 'checkbox'
      box.checked = published.has(item.locale)
      box.disabled = !canManage
      box.addEventListener('change', () => {
        if (box.checked) published.add(item.locale)
        else {
          if (item.locale === defaultLocale) {
            box.checked = true
            notify('默认语言必须保持已发布。', 'warning')
            return
          }
          published.delete(item.locale)
        }
      })
      const radio = document.createElement('input')
      radio.type = 'radio'
      radio.name = 'default-locale'
      radio.checked = item.locale === defaultLocale
      radio.disabled = !canManage
      radio.addEventListener('change', () => {
        if (!radio.checked) return
        defaultLocale = item.locale
        published.add(item.locale)
        render()
      })
      const copy = document.createElement('span')
      const complete = item.locale === 'zh-CN' ? '源语言' : `${item.completionPercent}%`
      const status = item.platformStatus === 'available' ? complete : '平台目录暂不可用'
      copy.textContent = `${item.label}（${item.locale}）· ${status}`
      const defaultLabel = document.createElement('span')
      defaultLabel.textContent = '默认'
      row.append(box, copy, radio, defaultLabel)
      body.append(row)
    }
  }

  async function save(): Promise<void> {
    if (!current || !canManage) return
    const publishedLocales = [...published]
    if (!publishedLocales.includes(defaultLocale)) {
      notify('默认语言必须在已发布列表中。', 'warning')
      return
    }
    try {
      current = await api.request<Languages & { replayed?: boolean }>(prefix, {
        method: 'PUT',
        body: JSON.stringify({
          commandKey: randomUUID(),
          expectedVersion: current.version,
          defaultLocale,
          publishedLocales,
        }),
      })
      published.clear()
      defaultLocale = current.defaultLocale
      for (const item of current.items || []) {
        if (item.published) published.add(item.locale)
      }
      render()
      notify('店铺语言已保存。', 'success')
    } catch (error) {
      notify(`保存失败：${String(error)}`, 'danger')
    }
  }

  await load()
}
