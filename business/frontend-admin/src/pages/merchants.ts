import type { HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshops/design-tokens'

interface Merchant {
  merchantId: number
  name: string
  externalId: string
  account: string
  contactName: string
  contactPhone: string
  status: 'ACTIVE' | 'DISABLED' | 'CLOSED'
  version: number
  shopId: number
  shopCode: string
}
interface MerchantPage { items: Merchant[]; page: number; pageSize: number; total: number }
interface Shop { shopId: number; merchantId: number; name: string; code: string; status: string }
interface Plan { id: number; code: string; name: string; durationDays: number; status: string; default: boolean }
interface Assignment { merchantId: number; planId: number; planCode: string; planName: string; expiresAt: string; version: number }
interface PaymentGrant { channelCode: string; name: string; enabled: boolean; priority: number }
interface PaymentChannels { merchantId: number; shopId: number; channels: PaymentGrant[]; version: number }
interface SMSRegionOption { dialCode: string; name: string; iso2: string; emoji: string; enabled: boolean }
interface SMSRegions { merchantId: number; shopId: number; dialCodes: string[]; unrestricted: boolean; regions: SMSRegionOption[]; version: number }
interface LiveAssignment { providerCode: string; name: string; enabled: boolean; default: boolean }
interface LiveProviders { merchantId: number; providers: LiveAssignment[]; version: number }

const prefix = '/admin/identity/merchants'
const usernamePattern = /^[a-z][a-z0-9._-]{2,63}$/
const emailPattern = /^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,24}$/

function validMerchantAccount(account: string): boolean {
  return usernamePattern.test(account) || (account.length <= 191 && emailPattern.test(account))
}

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

export async function renderMerchants(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const merchantTable = table({
    columns: ['商户 ID', '名称', '登录账号', '状态', '版本', '操作'],
    empty: '暂无商户',
  })
  let rows: Merchant[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void load() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void load() },
  })

  const filter = searchForm({
    fields: [
      { name: 'keyword', label: '关键字', placeholder: '名称 / 账号 / 外部标识' },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' }, { value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' },
      ] },
    ],
    searchLabel: '查询',
    onSearch: () => { currentPage = 1; void load() },
    onReset: () => { currentPage = 1; filter.set({ keyword: '', status: '' }); void load() },
  })

  function renderRows(): void {
    merchantTable.setRows(rows.map(item => [
      item.merchantId,
      item.name,
      item.account || '—',
      badge({ label: item.status === 'ACTIVE' ? '启用' : '停用', tone: item.status === 'ACTIVE' ? 'success' : 'warning' }),
      item.version,
      actions(
        button({ label: '编辑', size: 'sm', variant: 'secondary', onClick: () => openEditor(item) }),
        button({ label: '支付', size: 'sm', variant: 'secondary', onClick: () => void openPayment(item) }),
        button({ label: '短信', size: 'sm', variant: 'secondary', onClick: () => void openSMS(item) }),
        button({ label: '流媒体', size: 'sm', variant: 'secondary', onClick: () => void openLive(item) }),
        button({ label: '订阅', size: 'sm', variant: 'secondary', onClick: () => void openSubscription(item) }),
        button({ label: '重置密码', size: 'sm', variant: 'secondary', onClick: () => openReset(item) }),
        button({ label: '关闭', size: 'sm', variant: 'danger', onClick: () => openClose(item) }),
      ),
    ]))
  }

  async function load(): Promise<void> {
    const values = filter.values()
    const query = new URLSearchParams({
      page: String(currentPage),
      pageSize: String(currentPageSize),
    })
    if (values.keyword) query.set('keyword', values.keyword)
    if (values.status) query.set('status', values.status)
    state.set('正在加载商户目录…')
    filter.setBusy(true)
    try {
      const page = await api.request<MerchantPage>(`${prefix}?${query}`)
      rows = page.items ?? []
      currentPage = page.page
      currentPageSize = page.pageSize
      renderRows()
      pager.set({ page: page.page, pageSize: page.pageSize, total: page.total, itemCount: rows.length })
      state.set('')
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0, itemCount: 0 })
      state.set(`商户目录加载失败：${String(error)}`, 'danger')
    } finally {
      filter.setBusy(false)
    }
  }

  function openCreate(): void {
    const modal = hostFormModal({
      title: '新建商户',
      fields: [
        { name: 'account', label: '登录账号', required: true, placeholder: '用户名或邮箱' },
        { name: 'password', label: '登录密码', type: 'password', required: true, minLength: 8 },
        { name: 'name', label: '商户名称', required: true },
      ],
      onSubmit: (values, editor) => {
        const account = values.account.trim().toLowerCase()
        if (!validMerchantAccount(account) || !values.name.trim() || values.password.length < 8) {
          editor.setError('请填写用户名或邮箱、至少 8 位密码和商户名称。用户名须字母开头，3–64 位。')
          return
        }
        editor.setBusy(true)
        api.request<{ merchant: Merchant; shopId: number; shopCode: string; account: string }>(prefix, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), account, password: values.password, name: values.name.trim() }),
        })
          .then(result => {
            editor.close()
            hostFormModal({
              title: '商户已创建',
              fields: [
                { name: 'merchantId', label: '商户 ID', disabled: true },
                { name: 'account', label: '登录账号', disabled: true },
                { name: 'shopId', label: '默认店铺 ID', disabled: true },
                { name: 'shopCode', label: '默认店铺短码', disabled: true },
              ],
              submitLabel: '完成',
              onSubmit: (_next, done) => done.close(),
            }).open({
              merchantId: result.merchant.merchantId,
              account: result.account,
              shopId: result.shopId,
              shopCode: result.shopCode,
            })
            return load()
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openEditor(current: Merchant): void {
    const modal = hostFormModal({
      title: `编辑商户 · ${current.name}`,
      fields: [
        { name: 'account', label: '登录账号', disabled: true },
        { name: 'name', label: '商户名称', required: true },
        { name: 'status', label: '状态', kind: 'select', required: true, options: [
          { value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' },
        ] },
      ],
      onSubmit: (values, editor) => {
        if (!values.name.trim()) {
          editor.setError('请输入商户名称。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${current.merchantId}`, {
          method: 'PUT',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: current.version,
            name: values.name.trim(),
            status: values.status,
            contactName: current.contactName,
            contactPhone: current.contactPhone,
          }),
        })
          .then(() => { editor.close(); return load() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({ account: current.account, name: current.name, status: current.status })
  }

  function openReset(current: Merchant): void {
    const modal = hostFormModal({
      title: `重置密码 · ${current.name}`,
      fields: [{ name: 'password', label: '新密码', type: 'password', required: true, minLength: 8 }],
      submitLabel: '重置',
      onSubmit: (values, editor) => {
        if (values.password.length < 8) {
          editor.setError('密码至少 8 位。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${current.merchantId}/credentials/reset`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), password: values.password }),
        })
          .then(() => { editor.close(); state.set(`已重置 ${current.name} 的登录密码。`, 'success'); return load() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openClose(current: Merchant): void {
    const modal = hostFormModal({
      title: `关闭商户 · ${current.name}`,
      fields: [{
        name: 'confirm',
        label: '关闭后商户、店铺和所有者凭据进入终态，不能再登录。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: String(current.merchantId), label: `确认关闭 merchant_id ${current.merchantId}` }],
      }],
      submitLabel: '关闭',
      onSubmit: (values, editor) => {
        if (values.confirm !== String(current.merchantId)) {
          editor.setError('请选择确认项。')
          return
        }
        editor.setBusy(true)
        api.request(`${prefix}/${current.merchantId}/close`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: current.version }),
        })
          .then(() => { editor.close(); return load() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  async function openSubscription(current: Merchant): Promise<void> {
    try {
      const [plans, assignment] = await Promise.all([
        api.request<Plan[]>(`${prefix}/subscription-plans`),
        api.request<Assignment>(`${prefix}/${current.merchantId}/subscription`),
      ])
      const modal = hostFormModal({
        title: `订阅指派 · ${current.name}`,
        fields: [
          { name: 'planId', label: '套餐', kind: 'select', required: true, options: plans.map(plan => ({
            value: String(plan.id),
            label: `${plan.name} · ${plan.code}${plan.default ? ' · 默认' : ''}${plan.durationDays ? ` · ${plan.durationDays} 天` : ' · 永久'}`,
          })) },
          { name: 'expiresAt', label: '当前到期', disabled: true },
        ],
        onSubmit: (values, editor) => {
          editor.setBusy(true)
          api.request(`${prefix}/${current.merchantId}/subscription`, {
            method: 'PUT',
            body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: assignment.version ?? 0, planId: Number(values.planId) }),
          })
            .then(() => { editor.close(); return load() })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
      })
      modal.open({ planId: assignment.planId || plans.find(plan => plan.default)?.id || '', expiresAt: assignment.expiresAt || '未指派' })
    } catch (error) {
      state.set(`订阅指派加载失败：${String(error)}`, 'danger')
    }
  }

  async function openPayment(current: Merchant): Promise<void> {
    try {
      const shops = await api.request<Shop[]>(`${prefix}/${current.merchantId}/shops`)
      if (!shops.length) {
        state.set('该商户暂无店铺，无法配置支付通道。', 'danger')
        return
      }
      let grants = await api.request<PaymentChannels>(`${prefix}/${current.merchantId}/payment-channels?shopId=${shops[0].shopId}`)
      const shopOptions = shops.map(shop => ({ value: String(shop.shopId), label: `${shop.name || shop.code} · shop_id ${shop.shopId}` }))
      const fields = (value: PaymentChannels) => [
        { name: 'shopId', label: '店铺', kind: 'select' as const, required: true, options: shopOptions },
        { name: 'channels', label: '开通通道', kind: 'checkbox-tree' as const, wide: true, empty: '当前没有可开通的支付通道', tree: (value.channels ?? []).map(item => ({
          id: item.channelCode, label: item.name || item.channelCode, value: item.channelCode,
        })) },
      ]
      const modal = hostFormModal({
        title: `支付通道 · ${current.name}`,
        fields: fields(grants),
        onChange: (values, field, editor) => {
          if (field !== 'shopId' || !values.shopId) return
          editor.setBusy(true)
          api.request<PaymentChannels>(`${prefix}/${current.merchantId}/payment-channels?shopId=${values.shopId}`)
            .then(next => {
              grants = next
              editor.setFields(fields(next), {
                shopId: next.shopId,
                channels: (next.channels ?? []).filter(item => item.enabled).map(item => item.channelCode).join(','),
              })
            })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
        onSubmit: (values, editor) => {
          const selected = new Set(String(values.channels || '').split(',').filter(Boolean))
          editor.setBusy(true)
          api.request(`${prefix}/${current.merchantId}/payment-channels`, {
            method: 'PUT',
            body: JSON.stringify({
              commandKey: crypto.randomUUID(),
              expectedVersion: grants.version ?? 0,
              shopId: Number(values.shopId),
              channels: (grants.channels ?? []).map(item => ({
                channelCode: item.channelCode, name: item.name, enabled: selected.has(item.channelCode), priority: item.priority,
              })),
            }),
          })
            .then(() => { editor.close(); state.set('支付通道已保存。', 'success') })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
      })
      modal.open({
        shopId: shops[0].shopId,
        channels: (grants.channels ?? []).filter(item => item.enabled).map(item => item.channelCode).join(','),
      })
    } catch (error) {
      state.set(`支付通道加载失败：${String(error)}`, 'danger')
    }
  }

  async function openSMS(current: Merchant): Promise<void> {
    try {
      const shops = await api.request<Shop[]>(`${prefix}/${current.merchantId}/shops`)
      if (!shops.length) {
        state.set('该商户暂无店铺，无法配置短信区域。', 'danger')
        return
      }
      let grant = await api.request<SMSRegions>(`${prefix}/${current.merchantId}/sms-regions?shopId=${shops[0].shopId}`)
      const shopOptions = shops.map(shop => ({ value: String(shop.shopId), label: `${shop.name || shop.code} · shop_id ${shop.shopId}` }))
      const selectedRegions = (value: SMSRegions) => (value.unrestricted ? [] : (value.regions ?? []).filter(item => item.enabled).map(item => item.dialCode)).join(',')
      const fields = (value: SMSRegions) => [
        { name: 'shopId', label: '店铺', kind: 'select' as const, required: true, wide: true, options: shopOptions },
        {
          name: 'dialCodes',
          label: '区域（不选 = 全部）',
          kind: 'checkbox-tree' as const,
          wide: true,
          columns: 3 as const,
          empty: '区域管理中还没有可开通的区域',
          tree: (value.regions ?? []).map(item => ({
            id: item.dialCode,
            label: [item.emoji, item.name || item.iso2].filter(Boolean).join(' ') || item.dialCode,
            description: item.dialCode,
            value: item.dialCode,
          })),
        },
      ]
      const modal = hostFormModal({
        title: `短信区域 · ${current.name}`,
        fields: fields(grant),
        onChange: (values, field, editor) => {
          if (field !== 'shopId' || !values.shopId) return
          editor.setBusy(true)
          api.request<SMSRegions>(`${prefix}/${current.merchantId}/sms-regions?shopId=${values.shopId}`)
            .then(next => {
              grant = next
              editor.setFields(fields(next), { shopId: next.shopId, dialCodes: selectedRegions(next) })
            })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
        onSubmit: (values, editor) => {
          editor.setBusy(true)
          api.request(`${prefix}/${current.merchantId}/sms-regions`, {
            method: 'PUT',
            body: JSON.stringify({
              commandKey: crypto.randomUUID(),
              expectedVersion: grant.version ?? 0,
              shopId: Number(values.shopId),
              dialCodes: String(values.dialCodes || '').split(/[\s,]+/).map(item => item.trim()).filter(Boolean),
            }),
          })
            .then(() => { editor.close(); state.set('短信区域已保存。', 'success') })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
      })
      modal.open({ shopId: shops[0].shopId, dialCodes: selectedRegions(grant) })
    } catch (error) {
      state.set(`短信区域加载失败：${String(error)}`, 'danger')
    }
  }

  async function openLive(current: Merchant): Promise<void> {
    try {
      const grants = await api.request<LiveProviders>(`${prefix}/${current.merchantId}/live-providers`)
      const modal = hostFormModal({
        title: `流媒体方式 · ${current.name}`,
        fields: [
          { name: 'providers', label: '开通方式', kind: 'checkbox-tree', wide: true, empty: '当前没有可开通的流媒体方式', tree: (grants.providers ?? []).map(item => ({
            id: item.providerCode, label: item.name || item.providerCode, value: item.providerCode,
          })) },
          { name: 'defaultCode', label: '默认方式', kind: 'select', options: [
            { value: '', label: '不指定' },
            ...(grants.providers ?? []).map(item => ({ value: item.providerCode, label: item.name || item.providerCode })),
          ] },
        ],
        onSubmit: (values, editor) => {
          const selected = new Set(String(values.providers || '').split(',').filter(Boolean))
          editor.setBusy(true)
          api.request(`${prefix}/${current.merchantId}/live-providers`, {
            method: 'PUT',
            body: JSON.stringify({
              commandKey: crypto.randomUUID(),
              expectedVersion: grants.version ?? 0,
              providers: (grants.providers ?? []).map(item => ({
                providerCode: item.providerCode,
                name: item.name,
                enabled: selected.has(item.providerCode),
                default: values.defaultCode === item.providerCode,
              })),
            }),
          })
            .then(() => { editor.close(); state.set('流媒体授权已保存。', 'success') })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
      })
      modal.open({
        providers: (grants.providers ?? []).filter(item => item.enabled).map(item => item.providerCode).join(','),
        defaultCode: (grants.providers ?? []).find(item => item.default)?.providerCode ?? '',
      })
    } catch (error) {
      state.set(`流媒体授权加载失败：${String(error)}`, 'danger')
    }
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '商户目录',
        actions: [
          button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
          button({ label: '新建', onClick: () => openCreate() }),
        ],
        status: state.element,
        body: merchantTable.element,
        footer: pager.element,
      }),
    ],
  }))
  await load()
}
