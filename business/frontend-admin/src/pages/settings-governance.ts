import type { HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, searchCard, searchForm, statusLine, table, ui } from '@liveshops/design-tokens'

interface Merchant { merchantId: number; name: string; status: 'ACTIVE' | 'DISABLED' }
interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
interface Module { key: string; label: string }
interface Capability {
  id: number
  merchantId: number
  shopId: number
  module: string
  moduleLabel: string
  name: string
  merchantStatus: 'unset' | 'active' | 'draft'
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic: string
  version: number
  updatedBy?: string
  updatedAt?: string
}
interface AuditItem {
  id: number
  module: string
  capabilityId: number
  action: string
  operator: string
  reasonInternal: string
  reasonPublic: string
  createdAt: string
}

const prefix = '/admin/identity/merchant-governance'

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

function setSelectOptions(
  control: HTMLSelectElement | undefined,
  values: Array<{ value: string; label: string }>,
  allLabel: string,
  emptyLabel: string,
): void {
  if (!control) return
  control.replaceChildren()
  const all = document.createElement('option')
  all.value = ''
  all.textContent = values.length ? allLabel : emptyLabel
  control.append(all)
  control.disabled = !values.length
  for (const value of values) {
    const option = document.createElement('option')
    option.value = value.value
    option.textContent = value.label
    control.append(option)
  }
  ;(control as HTMLSelectElement & { refreshSearchSelect?: () => void }).refreshSearchSelect?.()
}

function merchantStatusLabel(value: Capability['merchantStatus']): string {
  if (value === 'active') return '商户启用'
  if (value === 'draft') return '商户草稿'
  return '未配置'
}

function platformStatusLabel(value: Capability['platformStatus']): string {
  if (value === 'restricted') return '限制'
  if (value === 'suspended') return '暂停'
  return '正常'
}

function displayTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(date)
}

export async function renderSettingsGovernance(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const capabilityTable = table({
    columns: ['能力', '租户', '商户状态', '平台状态', '公开原因', '版本', '操作'],
    empty: '请先选择商户和店铺。新设置由商户首次保存后产生商户状态；平台可先叠加干预。',
  })
  const auditTable = table({
    columns: ['能力', '操作人', '内部原因', '对外原因', '时间'],
    empty: '暂无干预审计',
  })
  const auditWrap = create('div')
  auditWrap.style.padding = '0 0 16px'
  auditWrap.append(create('h3', ui.cardTitle, '最近干预审计'), auditTable.element)
  const body = create('div')
  body.append(capabilityTable.element, auditWrap)

  let merchants: Merchant[] = []
  let shops: Shop[] = []
  let lastMerchantId = ''
  let modules: Module[] = []
  let rows: Capability[] = []
  let audits: AuditItem[] = []

  const filter = searchForm({
    fields: [
      { name: 'merchantId', label: '商户', kind: 'select', options: [{ value: '', label: '全部商户' }] },
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '全部店铺' }] },
      { name: 'module', label: '能力模块', kind: 'select', options: [{ value: '', label: '全部模块' }] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      const merchantId = filter.values().merchantId
      if (merchantId !== lastMerchantId) {
        lastMerchantId = merchantId
        filter.set({ shopId: '' })
        await loadShops(Number(merchantId) || 0, false)
      }
      await load()
    },
    onReset: () => {
      lastMerchantId = ''
      filter.set({ merchantId: '', shopId: '', module: '' })
      void loadShops(0, true)
    },
  })
  const merchantSelect = filter.control('merchantId') as HTMLSelectElement | undefined
  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined
  const moduleSelect = filter.control('module') as HTMLSelectElement | undefined

  function selectedScope(): { merchantId: number; shopId: number; shop?: Shop } {
    const values = filter.values()
    const merchantId = Number(values.merchantId)
    const shopId = Number(values.shopId)
    return { merchantId, shopId, shop: shops.find(item => item.shopId === shopId) }
  }

  function renderRows(): void {
    capabilityTable.setRows(rows.map(item => [
      `${item.moduleLabel} · ${item.module}`,
      `merchant_id ${item.merchantId} / shop_id ${item.shopId}`,
      badge({ label: merchantStatusLabel(item.merchantStatus), tone: item.merchantStatus === 'active' ? 'success' : 'neutral' }),
      badge({
        label: platformStatusLabel(item.platformStatus),
        tone: item.platformStatus === 'active' ? 'success' : 'danger',
      }),
      item.platformReasonPublic || '—',
      item.version || '未叠加',
      actions(button({ label: '干预', size: 'sm', variant: 'secondary', onClick: () => openIntervene(item) })),
    ]))
    auditTable.setRows(audits.map(item => [
      `${item.module} #${item.capabilityId}`,
      item.operator,
      item.reasonInternal,
      item.reasonPublic || '—',
      displayTime(item.createdAt),
    ]))
  }

  async function loadMerchants(): Promise<void> {
    state.set('正在加载商户、店铺和能力目录…')
    filter.setBusy(true)
    try {
      const [merchantRows, catalog] = await Promise.all([
        api.request<Merchant[]>(`${prefix}/merchants`),
        api.request<Module[]>(`${prefix}/catalog`),
      ])
      merchants = merchantRows
      modules = catalog || []
      lastMerchantId = ''
      setSelectOptions(merchantSelect, merchants.map(item => ({
        value: String(item.merchantId),
        label: `${item.name} · merchant_id ${item.merchantId}${item.status === 'DISABLED' ? ' · 已停用' : ''}`,
      })), '全部商户', '暂无商户')
      setSelectOptions(moduleSelect, modules.map(item => ({ value: item.key, label: item.label })), '全部模块', '全部模块')
      filter.set({ merchantId: '', shopId: '', module: '' })
      await loadShops(0, true)
    } catch (error) {
      merchants = []
      shops = []
      lastMerchantId = ''
      rows = []
      audits = []
      setSelectOptions(merchantSelect, [], '全部商户', '暂无商户')
      setSelectOptions(shopSelect, [], '全部店铺', '请先选择商户')
      renderRows()
      state.set(`商户能力治理范围加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function loadShops(merchantId: number, loadRows: boolean): Promise<void> {
    const scopedMerchantId = Number.isSafeInteger(merchantId) && merchantId > 0 ? merchantId : 0
    if (!scopedMerchantId) {
      shops = []
      setSelectOptions(shopSelect, [], '全部店铺', '请先选择商户')
      filter.set({ shopId: '' })
      if (loadRows) await load()
      return
    }
    filter.setBusy(true)
    state.set('正在加载商户店铺…')
    try {
      shops = await api.request<Shop[]>(`${prefix}/shops?merchantId=${encodeURIComponent(scopedMerchantId)}`)
      setSelectOptions(shopSelect, shops.map(item => ({
        value: String(item.shopId),
        label: `${item.name || item.code} · shop_id ${item.shopId}${item.status === 'DISABLED' ? ' · 已停用' : ''}`,
      })), '全部店铺', '暂无店铺')
      const currentShopId = filter.values().shopId
      const shopId = shops.some(item => String(item.shopId) === currentShopId) ? currentShopId : ''
      filter.set({ shopId })
      if (loadRows) await load()
    } catch (error) {
      shops = []
      rows = []
      audits = []
      setSelectOptions(shopSelect, [], '全部店铺', '暂无店铺')
      renderRows()
      state.set(`店铺范围加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  async function load(): Promise<void> {
    const { merchantId, shopId } = selectedScope()
    const module = filter.values().module
    if (!Number.isSafeInteger(merchantId) || merchantId <= 0 || !Number.isSafeInteger(shopId) || shopId <= 0) {
      rows = []
      audits = []
      renderRows()
      state.set('请先选择商户和店铺。')
      return
    }
    filter.setBusy(true)
    state.set('正在加载商户能力…')
    try {
      const query = new URLSearchParams({ merchantId: String(merchantId), shopId: String(shopId) })
      if (module) query.set('module', module)
      const [capabilities, auditRows] = await Promise.all([
        api.request<Capability[]>(`${prefix}?${query}`),
        api.request<AuditItem[]>(`${prefix}/audit?${query}`),
      ])
      rows = capabilities || []
      audits = auditRows || []
      renderRows()
      state.clear()
    } catch (error) {
      rows = []
      audits = []
      renderRows()
      state.set(`加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  function openIntervene(current: Capability): void {
    const scope = selectedScope()
    if (!scope.shop) {
      state.set('请先选择商户和店铺。', 'danger')
      return
    }
    const nextStatus = current.platformStatus === 'active' ? 'restricted' : 'active'
    const modal = hostFormModal({
      title: `干预 ${current.moduleLabel}`,
      submitLabel: '确认干预',
      fields: [
        { name: 'scope', label: '范围', disabled: true, wide: true },
        { name: 'platformStatus', label: '平台状态', kind: 'select', required: true, options: [
          { value: 'active', label: '恢复正常' },
          { value: 'restricted', label: '限制' },
          { value: 'suspended', label: '暂停' },
        ] },
        { name: 'reasonInternal', label: '内部原因（审计必填）', required: true, wide: true, maxLength: 1000 },
        { name: 'reasonPublic', label: '商户可见原因（限制/暂停时必填）', wide: true, maxLength: 500 },
      ],
      onSubmit: (values, editor) => {
        const platformStatus = values.platformStatus
        const reasonInternal = values.reasonInternal.trim()
        const reasonPublic = values.reasonPublic.trim()
        if (!reasonInternal) {
          editor.setError('内部原因必填。')
          return
        }
        if (platformStatus !== 'active' && !reasonPublic) {
          editor.setError('限制或暂停时必须填写商户可见原因。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/intervene`, {
          method: 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: current.version || 0,
            merchantId: scope.merchantId,
            shopId: scope.shopId,
            module: current.module,
            platformStatus,
            reasonInternal,
            reasonPublic: platformStatus === 'active' ? '' : reasonPublic,
          }),
        })
          .then(() => { editor.close(); return load() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      scope: `${scope.shop.name || scope.shop.code} · shop_id ${scope.shopId} · ${current.moduleLabel}`,
      platformStatus: nextStatus,
      reasonInternal: '',
      reasonPublic: current.platformReasonPublic || '',
    })
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '商户能力',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void load() })],
        status: state.element,
        body,
      }),
    ],
  }))
  await loadMerchants()
}
