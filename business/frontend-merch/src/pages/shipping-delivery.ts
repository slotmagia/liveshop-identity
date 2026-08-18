import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, notify, page, pagination, searchCard, searchForm, table, ui } from '@liveshop/design-tokens'

interface Shop { shopId: number; merchantId: number; name: string; code: string; status: 'ACTIVE' | 'DISABLED' }
interface Overlay {
  platformStatus: 'active' | 'restricted' | 'suspended'
  platformReasonPublic?: string
  editable?: boolean
}
interface ShippingRule extends Overlay {
  id: number
  merchantId: number
  shopId: number
  name: string
  regions: string
  feeFen: number
  freeOverFen: number
  minDays: number
  maxDays: number
  sortOrder: number
  status: 'ACTIVE' | 'DISABLED' | 'RETIRED' | string
  version: number
  createdAt: string
  updatedAt: string
}
interface ShippingRegion {
  regionCode: string
  regionName: string
  countryCode: string
  countryName: string
  subdivisionCode?: string
  subdivisionName?: string
}
interface ShippingRate {
  id: number
  name: string
  isFree: boolean
  priceFen: number
  transitType: 'STANDARD' | 'EXPRESS' | 'ECONOMY' | string
  minDays: number
  maxDays: number
  sortOrder: number
  status: string
}
interface ShippingZone {
  id: number
  name: string
  sortOrder: number
  regions: ShippingRegion[]
  rates: ShippingRate[]
}
interface ShippingPreset extends Overlay {
  id: number
  merchantId: number
  shopId: number
  name: string
  isDefault: boolean
  productScope: 'ALL' | 'SELECTED' | string
  productIds: number[]
  originName: string
  originRegionCode: string
  originRegionName: string
  originCountryCode: string
  originCountryName: string
  originSubdivisionCode?: string
  originSubdivisionName?: string
  status: 'ACTIVE' | 'DISABLED' | 'RETIRED' | string
  zones: ShippingZone[]
  version: number
  createdAt: string
  updatedAt: string
}
interface ShippingRulePage extends Overlay {
  items: ShippingRule[]
  page: number
  pageSize: number
  total: number
}
interface ShippingPresetPage extends Overlay {
  items: ShippingPreset[]
  page: number
  pageSize: number
  total: number
}
interface ShippingRuleResult { rule: ShippingRule; replayed?: boolean }
interface ShippingPresetResult { preset: ShippingPreset; replayed?: boolean }

const prefix = '/merch/identity/shipping-delivery'
const defaultZones = JSON.stringify([
  {
    name: '默认分区',
    sortOrder: 0,
    regions: [{ regionCode: 'US', regionName: 'United States', countryCode: 'US', countryName: 'United States' }],
    rates: [{ name: '标准', isFree: false, priceFen: 0, transitType: 'STANDARD', minDays: 3, maxDays: 7, sortOrder: 0, status: 'ACTIVE' }],
  },
], null, 2)

function actions(...children: Node[]): HTMLElement {
  const node = document.createElement('div')
  node.className = ui.actions
  node.append(...children)
  return node
}

function setSelectOptions(
  control: HTMLSelectElement | undefined,
  values: Array<{ value: string; label: string }>,
  emptyLabel: string,
): void {
  if (!control) return
  control.replaceChildren()
  const empty = document.createElement('option')
  empty.value = ''
  empty.textContent = values.length ? emptyLabel : '暂无店铺'
  control.append(empty)
  control.disabled = !values.length
  for (const value of values) {
    const option = document.createElement('option')
    option.value = value.value
    option.textContent = value.label
    control.append(option)
  }
  ;(control as HTMLSelectElement & { refreshSearchSelect?: () => void }).refreshSearchSelect?.()
}

function statusBadge(status: string): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  if (status === 'DISABLED') return badge({ label: '停用', tone: 'warning' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function platformBadge(status: Overlay['platformStatus']): HTMLElement {
  if (status === 'restricted') return badge({ label: '平台限制', tone: 'warning' })
  if (status === 'suspended') return badge({ label: '平台暂停', tone: 'danger' })
  return badge({ label: '平台正常', tone: 'success' })
}

function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function parseProductIds(value: string): number[] {
  return [...new Set(value.split(/[\s,]+/).map(item => Number(item.trim())).filter(item => Number.isInteger(item) && item > 0))]
}

function parseZones(value: string): ShippingZone[] | string {
  try {
    const parsed = JSON.parse(value) as unknown
    if (!Array.isArray(parsed) || parsed.length === 0) return '分区 JSON 必须是非空数组。'
    return parsed as ShippingZone[]
  } catch {
    return '分区 JSON 无法解析。'
  }
}

export async function renderShippingDelivery(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.shipping.manage')
  const ruleTable = table({
    columns: ['名称', '覆盖区域', '运费(分)', '满额包邮(分)', '时效(天)', '状态', '平台叠加', '更新时间', '操作'],
    empty: '暂无配送规则',
  })
  const presetTable = table({
    columns: ['名称', '默认', '商品范围', '发货地', '分区', '状态', '平台叠加', '更新时间', '操作'],
    empty: '暂无发货预设',
  })
  let shops: Shop[] = []
  let rules: ShippingRule[] = []
  let presets: ShippingPreset[] = []
  let overlayStatus: Overlay['platformStatus'] = 'active'
  let overlayReason = ''
  let rulePage = 1
  let rulePageSize = 20
  let presetPage = 1
  let presetPageSize = 20

  const rulePager = pagination({
    pageSize: rulePageSize,
    onPageChange: value => { rulePage = value; void loadRules() },
    onPageSizeChange: value => { rulePage = 1; rulePageSize = value; void loadRules() },
  })
  const presetPager = pagination({
    pageSize: presetPageSize,
    onPageChange: value => { presetPage = value; void loadPresets() },
    onPageSizeChange: value => { presetPage = 1; presetPageSize = value; void loadPresets() },
  })

  const filter = searchForm({
    fields: [
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '请选择店铺' }] },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'ACTIVE', label: '启用' },
        { value: 'DISABLED', label: '停用' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      rulePage = 1
      presetPage = 1
      await Promise.all([loadRules(), loadPresets()])
    },
    onReset: () => {
      rulePage = 1
      presetPage = 1
      rulePageSize = 20
      presetPageSize = 20
      filter.set({ shopId: defaultShopId(), status: '' })
      void Promise.all([loadRules(), loadPresets()])
    },
  })
  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined

  function defaultShopId(): string {
    const tenantShop = context.tenant?.shopId
    if (tenantShop && shops.some(item => item.shopId === tenantShop)) return String(tenantShop)
    return shops.length === 1 ? String(shops[0].shopId) : ''
  }

  function selectedShopId(): number {
    return Number(filter.values().shopId) || 0
  }

  function overlayActive(): boolean {
    return overlayStatus === 'active'
  }

  function applyOverlay(pageResult: Overlay): void {
    overlayStatus = pageResult.platformStatus || 'active'
    overlayReason = pageResult.platformReasonPublic || ''
    if (!overlayActive() && overlayReason) notify(overlayReason, 'warning')
  }

  function renderRules(): void {
    ruleTable.setRows(rules.map(item => {
      const buttons = []
      if (canManage && item.editable) {
        buttons.push(button({ label: '编辑', size: 'sm', onClick: () => openRule(item) }))
        buttons.push(button({ label: '退役', size: 'sm', variant: 'secondary', onClick: () => openRetireRule(item) }))
      }
      return [
        item.name,
        item.regions,
        String(item.feeFen),
        String(item.freeOverFen),
        `${item.minDays}-${item.maxDays}`,
        statusBadge(item.status),
        platformBadge(item.platformStatus),
        formatTime(item.updatedAt),
        actions(...buttons),
      ]
    }))
  }

  function renderPresets(): void {
    presetTable.setRows(presets.map(item => {
      const buttons = []
      if (canManage && item.editable) {
        buttons.push(button({ label: '编辑', size: 'sm', onClick: () => void openPreset(item) }))
        if (item.status === 'ACTIVE') {
          buttons.push(button({ label: '停用', size: 'sm', variant: 'secondary', onClick: () => openTogglePreset(item, false) }))
        } else {
          buttons.push(button({ label: '启用', size: 'sm', onClick: () => openTogglePreset(item, true) }))
        }
        buttons.push(button({ label: '退役', size: 'sm', variant: 'secondary', onClick: () => openRetirePreset(item) }))
      }
      return [
        item.name,
        item.isDefault ? badge({ label: '默认', tone: 'success' }) : '—',
        item.productScope === 'SELECTED' ? `指定 ${item.productIds?.length || 0} 件` : '全部商品',
        `${item.originName} · ${item.originCountryCode}`,
        String(item.zones?.length || 0),
        statusBadge(item.status),
        platformBadge(item.platformStatus),
        formatTime(item.updatedAt),
        actions(...buttons),
      ]
    }))
  }

  async function loadShops(): Promise<void> {
    shops = await api.request<Shop[]>(`${prefix}/shops`)
    setSelectOptions(shopSelect, shops.map(item => ({
      value: String(item.shopId),
      label: `${item.name} · ${item.code} · shop_id ${item.shopId}`,
    })), '请选择店铺')
    const shopId = defaultShopId()
    if (shopId) filter.set({ shopId })
  }

  async function loadRules(): Promise<void> {
    const shopId = selectedShopId()
    if (!shopId) {
      rules = []
      overlayStatus = 'active'
      overlayReason = ''
      renderRules()
      rulePager.set({ page: 1, pageSize: rulePageSize, total: 0 })
      notify(shops.length ? '请选择店铺后查看发货/配送。' : '当前商户没有可管理的店铺。', shops.length ? 'warning' : 'danger')
      return
    }
    try {
      const query = new URLSearchParams({
        shopId: String(shopId),
        page: String(rulePage),
        pageSize: String(rulePageSize),
      })
      const values = filter.values()
      if (values.status) query.set('status', values.status)
      const pageResult = await api.request<ShippingRulePage>(`${prefix}/rules?${query}`)
      rules = pageResult.items
      rulePage = pageResult.page
      rulePageSize = pageResult.pageSize
      applyOverlay(pageResult)
      renderRules()
      rulePager.set({ page: pageResult.page, pageSize: pageResult.pageSize, total: pageResult.total })
    } catch (error) {
      rules = []
      renderRules()
      rulePager.set({ page: 1, pageSize: rulePageSize, total: 0 })
      notify(`配送规则加载失败：${String(error)}`, 'danger')
    }
  }

  async function loadPresets(): Promise<void> {
    const shopId = selectedShopId()
    if (!shopId) {
      presets = []
      renderPresets()
      presetPager.set({ page: 1, pageSize: presetPageSize, total: 0 })
      return
    }
    try {
      const query = new URLSearchParams({
        shopId: String(shopId),
        page: String(presetPage),
        pageSize: String(presetPageSize),
      })
      const values = filter.values()
      if (values.status) query.set('status', values.status)
      const pageResult = await api.request<ShippingPresetPage>(`${prefix}/presets?${query}`)
      presets = pageResult.items
      presetPage = pageResult.page
      presetPageSize = pageResult.pageSize
      applyOverlay(pageResult)
      renderPresets()
      presetPager.set({ page: pageResult.page, pageSize: pageResult.pageSize, total: pageResult.total })
    } catch (error) {
      presets = []
      renderPresets()
      presetPager.set({ page: 1, pageSize: presetPageSize, total: 0 })
      notify(`发货预设加载失败：${String(error)}`, 'danger')
    }
  }

  function requireWritable(): boolean {
    if (!selectedShopId()) {
      notify('请先选择店铺。', 'warning')
      return false
    }
    if (!overlayActive()) {
      notify('平台已限制该店铺配送设置，当前不能写入。', 'warning')
      return false
    }
    return true
  }

  function openRule(item?: ShippingRule): void {
    if (!requireWritable()) return
    const shopId = selectedShopId()
    const modal = hostFormModal({
      title: item ? `编辑配送规则 · ${item.name}` : '添加配送规则',
      fields: [
        { name: 'name', label: '名称', required: true, placeholder: '美国标准' },
        { name: 'regions', label: '覆盖区域', required: true, placeholder: 'US, CA' },
        { name: 'feeFen', label: '运费（分）', required: true },
        { name: 'freeOverFen', label: '满额包邮（分，0 表示不启用）' },
        { name: 'minDays', label: '最短天数', required: true },
        { name: 'maxDays', label: '最长天数', required: true },
        { name: 'sortOrder', label: '排序' },
        { name: 'status', label: '状态', kind: 'select', required: true, options: [
          { value: 'ACTIVE', label: '启用' },
          { value: 'DISABLED', label: '停用' },
        ] },
      ],
      onSubmit: (values, editor) => {
        const name = values.name.trim()
        const regions = values.regions.trim()
        const feeFen = Number(values.feeFen)
        const freeOverFen = Number(values.freeOverFen || '0')
        const minDays = Number(values.minDays)
        const maxDays = Number(values.maxDays)
        const sortOrder = Number(values.sortOrder || '0')
        if (!name || !regions || !Number.isFinite(feeFen) || feeFen < 0 || !Number.isFinite(freeOverFen) || freeOverFen < 0) {
          editor.setError('请填写名称、覆盖区域和非负运费。')
          return
        }
        if (!Number.isInteger(minDays) || !Number.isInteger(maxDays) || minDays < 0 || maxDays < minDays || maxDays > 365) {
          editor.setError('时效必须满足 0 ≤ 最短天数 ≤ 最长天数 ≤ 365。')
          return
        }
        editor.setBusy(true)
        const body = JSON.stringify({
          commandKey: crypto.randomUUID(),
          shopId,
          expectedVersion: item?.version,
          name,
          regions,
          feeFen,
          freeOverFen,
          minDays,
          maxDays,
          sortOrder,
          status: values.status || 'ACTIVE',
        })
        const request = item
          ? api.request<ShippingRuleResult>(`${prefix}/rules/${item.id}`, { method: 'PUT', body })
          : api.request<ShippingRuleResult>(`${prefix}/rules`, { method: 'POST', body })
        request
          .then(async () => {
            editor.close()
            notify(item ? `已更新 ${name}` : `已添加 ${name}`, 'success')
            await loadRules()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      name: item?.name || '',
      regions: item?.regions || '',
      feeFen: item ? String(item.feeFen) : '0',
      freeOverFen: item ? String(item.freeOverFen) : '0',
      minDays: item ? String(item.minDays) : '3',
      maxDays: item ? String(item.maxDays) : '7',
      sortOrder: item ? String(item.sortOrder) : '0',
      status: item?.status || 'ACTIVE',
    })
  }

  function openRetireRule(item: ShippingRule): void {
    if (!requireWritable()) return
    const modal = hostFormModal({
      title: `退役 ${item.name}`,
      fields: [{
        name: 'confirm',
        label: '退役后不再出现在列表中，不能恢复同一行。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认退役 ${item.name}` }],
      }],
      submitLabel: '退役',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<ShippingRuleResult>(`${prefix}/rules/${item.id}/retire`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), shopId: item.shopId, expectedVersion: item.version }),
        })
          .then(async () => {
            editor.close()
            notify(`已退役 ${item.name}`, 'success')
            await loadRules()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  async function openPreset(item?: ShippingPreset): Promise<void> {
    if (!requireWritable()) return
    const shopId = selectedShopId()
    let current = item
    if (item) {
      try {
        const detail = await api.request<{ preset: ShippingPreset }>(`${prefix}/presets/${item.id}?shopId=${shopId}`)
        current = detail.preset
      } catch (error) {
        notify(`发货预设加载失败：${String(error)}`, 'danger')
        return
      }
    }
    const modal = hostFormModal({
      title: current ? `编辑发货预设 · ${current.name}` : '添加发货预设',
      fields: [
        { name: 'name', label: '名称', required: true, placeholder: '默认发货' },
        { name: 'isDefault', label: '设为默认', kind: 'select', required: true, options: [
          { value: '0', label: '否' },
          { value: '1', label: '是' },
        ] },
        { name: 'productScope', label: '商品范围', kind: 'select', required: true, options: [
          { value: 'ALL', label: '全部商品' },
          { value: 'SELECTED', label: '指定商品' },
        ] },
        { name: 'productIds', label: '商品 ID（逗号分隔，指定商品时必填）' },
        { name: 'originName', label: '发货地名称', required: true },
        { name: 'originRegionCode', label: '发货地区代码', required: true, placeholder: 'US-CA' },
        { name: 'originRegionName', label: '发货地区名称', required: true },
        { name: 'originCountryCode', label: '国家码', required: true, placeholder: 'US' },
        { name: 'originCountryName', label: '国家名称', required: true },
        { name: 'originSubdivisionCode', label: '下级区划代码' },
        { name: 'originSubdivisionName', label: '下级区划名称' },
        { name: 'status', label: '状态', kind: 'select', required: true, options: [
          { value: 'ACTIVE', label: '启用' },
          { value: 'DISABLED', label: '停用' },
        ] },
        { name: 'zones', label: '分区 JSON', kind: 'textarea', required: true, wide: true, mono: true, rows: 10 },
      ],
      onSubmit: (values, editor) => {
        const name = values.name.trim()
        const productScope = values.productScope || 'ALL'
        const productIds = parseProductIds(values.productIds || '')
        const zones = parseZones(values.zones)
        if (!name) {
          editor.setError('请填写名称。')
          return
        }
        if (productScope === 'ALL' && productIds.length) {
          editor.setError('全部商品时不要填写商品 ID。')
          return
        }
        if (productScope === 'SELECTED' && productIds.length === 0) {
          editor.setError('指定商品时至少填写一个商品 ID。')
          return
        }
        if (typeof zones === 'string') {
          editor.setError(zones)
          return
        }
        editor.setBusy(true)
        const body = JSON.stringify({
          commandKey: crypto.randomUUID(),
          shopId,
          expectedVersion: current?.version,
          name,
          isDefault: values.isDefault === '1',
          productScope,
          productIds,
          originName: values.originName.trim(),
          originRegionCode: values.originRegionCode.trim(),
          originRegionName: values.originRegionName.trim(),
          originCountryCode: values.originCountryCode.trim().toUpperCase(),
          originCountryName: values.originCountryName.trim(),
          originSubdivisionCode: values.originSubdivisionCode.trim(),
          originSubdivisionName: values.originSubdivisionName.trim(),
          status: values.status || 'ACTIVE',
          zones,
        })
        const request = current
          ? api.request<ShippingPresetResult>(`${prefix}/presets/${current.id}`, { method: 'PUT', body })
          : api.request<ShippingPresetResult>(`${prefix}/presets`, { method: 'POST', body })
        request
          .then(async () => {
            editor.close()
            notify(current ? `已更新 ${name}` : `已添加 ${name}`, 'success')
            await loadPresets()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      name: current?.name || '',
      isDefault: current?.isDefault ? '1' : '0',
      productScope: current?.productScope || 'ALL',
      productIds: current?.productIds?.join(',') || '',
      originName: current?.originName || '',
      originRegionCode: current?.originRegionCode || '',
      originRegionName: current?.originRegionName || '',
      originCountryCode: current?.originCountryCode || 'US',
      originCountryName: current?.originCountryName || '',
      originSubdivisionCode: current?.originSubdivisionCode || '',
      originSubdivisionName: current?.originSubdivisionName || '',
      status: current?.status || 'ACTIVE',
      zones: current ? JSON.stringify(current.zones, null, 2) : defaultZones,
    })
  }

  function openTogglePreset(item: ShippingPreset, enabled: boolean): void {
    if (!requireWritable()) return
    const action = enabled ? 'enable' : 'disable'
    const label = enabled ? '启用' : '停用'
    const modal = hostFormModal({
      title: `${label} ${item.name}`,
      fields: [{
        name: 'confirm',
        label: `${label}后该预设对报价可见性会立即变化。`,
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认${label} ${item.name}` }],
      }],
      submitLabel: label,
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<ShippingPresetResult>(`${prefix}/presets/${item.id}/${action}`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), shopId: item.shopId, expectedVersion: item.version }),
        })
          .then(async () => {
            editor.close()
            notify(`已${label} ${item.name}`, 'success')
            await loadPresets()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openRetirePreset(item: ShippingPreset): void {
    if (!requireWritable()) return
    const modal = hostFormModal({
      title: `退役 ${item.name}`,
      fields: [{
        name: 'confirm',
        label: '退役后不再出现在列表中，并清除默认标记。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(item.id), label: `确认退役 ${item.name}` }],
      }],
      submitLabel: '退役',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(item.id)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request<ShippingPresetResult>(`${prefix}/presets/${item.id}/retire`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), shopId: item.shopId, expectedVersion: item.version }),
        })
          .then(async () => {
            editor.close()
            notify(`已退役 ${item.name}`, 'success')
            await loadPresets()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  const ruleToolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadRules() })]
  const presetToolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadPresets() })]
  if (canManage) {
    ruleToolbar.push(button({ label: '添加规则', onClick: () => openRule() }))
    presetToolbar.push(button({ label: '添加预设', onClick: () => void openPreset() }))
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '配送规则',
        actions: ruleToolbar,
        body: ruleTable.element,
        footer: rulePager.element,
      }),
      dataCard({
        title: '发货预设',
        actions: presetToolbar,
        body: presetTable.element,
        footer: presetPager.element,
      }),
    ],
  }))

  try {
    await loadShops()
    await Promise.all([loadRules(), loadPresets()])
  } catch (error) {
    shops = []
    rules = []
    presets = []
    renderRules()
    renderPresets()
    notify(`发货/配送页面加载失败：${String(error)}`, 'danger')
  }
}
