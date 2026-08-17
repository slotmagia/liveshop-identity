import type { HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, create, dataCard, page, statusLine, table, ui } from '@liveshop/design-tokens'
import { permissionTree } from '../../../ui/permission-tree'

interface Plan {
  id: number
  code: string
  name: string
  level: number
  priceMinor: number
  durationDays: number
  description: string
  default: boolean
  sort: number
  status: 'ACTIVE' | 'DISABLED'
  version: number
}

interface PlanPermissionPolicy {
  planID: number
  planCode: string
  planName: string
  permissionCodes: string[]
  productLimit: number | null
  revision: number
}

interface Permission {
  moduleId: string
  code: string
  name: string
  resource: string
  action: string
  description: string
  registryRevision: number
}

const prefix = '/admin/identity/subscription'

function permissionCodes(value: string): string[] {
  return [...new Set(value.split(/[\s,]+/).map(code => code.trim()).filter(Boolean))].sort()
}

function yuanToMinor(value: string): number | null {
  const normalized = value.trim()
  if (!/^\d+(?:\.\d{1,2})?$/.test(normalized)) return null
  return Math.round(Number(normalized) * 100)
}

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

export async function renderSubscription(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const planTable = table({ columns: ['等级', '套餐码', '名称', '价格', '周期', '状态', '版本', '操作'] })
  const permissionTable = table({ columns: ['模块', '权限码', '名称', '资源', '动作', '说明'] })
  let plans: Plan[] = []
  let permissions: Permission[] = []

  async function load(): Promise<void> {
    try {
      [plans, permissions] = await Promise.all([
        api.request<Plan[]>(`${prefix}/plans`),
        api.request<Permission[]>(`${prefix}/permissions`),
      ])
      planTable.setRows(plans.map(plan => [
        plan.level,
        plan.code,
        plan.name,
        `¥${(plan.priceMinor / 100).toFixed(2)}`,
        plan.durationDays === 0 ? '永久' : `${plan.durationDays} 天`,
        actions(
          badge({ label: plan.status === 'ACTIVE' ? '启用' : '停用', tone: plan.status === 'ACTIVE' ? 'success' : 'warning' }),
          ...(plan.default ? [badge({ label: '默认', tone: 'info' })] : []),
        ),
        plan.version,
        actions(
          button({ label: '权限配置', size: 'sm', variant: 'secondary', onClick: () => void openPermissions(plan) }),
          button({ label: '编辑', size: 'sm', variant: 'secondary', onClick: () => openEditor(plan) }),
          button({ label: '退役', size: 'sm', variant: 'danger', disabled: plan.default, title: plan.default ? '请先把另一套餐设为默认' : undefined, onClick: () => openRetire(plan) }),
        ),
      ]))
      permissionTable.setRows(permissions.map(permission => [
        permission.moduleId,
        permission.code,
        permission.name,
        permission.resource,
        permission.action,
        permission.description,
      ]))
      const revision = permissions.reduce((current, permission) => Math.max(current, permission.registryRevision), 0)
      state.set(`套餐 ${plans.length} 个 · 活动权限 ${permissions.length} 个 · Registry revision ${revision}`)
    } catch (error) {
      state.set(`加载失败：${String(error)}`, 'danger')
    }
  }

  function openEditor(current?: Plan): void {
    const editor = hostFormModal({
      title: current ? `编辑套餐 · ${current.name}` : '新建套餐',
      fields: [
        { name: 'code', label: '稳定套餐码', required: true, disabled: Boolean(current), mono: true, placeholder: 'standard' },
        { name: 'name', label: '套餐名称', required: true },
        { name: 'level', label: '等级', type: 'number', required: true },
        { name: 'sort', label: '排序', type: 'number', required: true },
        { name: 'priceYuan', label: '价格（元）', required: true, placeholder: '0.00' },
        { name: 'durationDays', label: '计价天数（0=永久）', type: 'number', min: 0, required: true },
        { name: 'status', label: '状态', kind: 'select', options: [{ value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' }], required: true },
        { name: 'default', label: '默认套餐', kind: 'select', options: [{ value: 'false', label: '否' }, { value: 'true', label: '是' }], required: true },
        { name: 'description', label: '套餐说明', kind: 'textarea', rows: 5, wide: true },
      ],
      onSubmit: (values, modal) => {
        const priceMinor = yuanToMinor(values.priceYuan)
        if (priceMinor === null) {
          modal.setError('价格格式不正确，最多保留两位小数。')
          return
        }
        modal.setBusy(true)
        const body = JSON.stringify({
          commandKey: crypto.randomUUID(),
          expectedVersion: current?.version ?? 0,
          code: values.code,
          name: values.name,
          level: Number(values.level),
          priceMinor,
          durationDays: Number(values.durationDays),
          description: values.description,
          default: values.default === 'true',
          sort: Number(values.sort),
          status: values.status,
        })
        api.request(current ? `${prefix}/plans/${current.id}` : `${prefix}/plans`, { method: current ? 'PUT' : 'POST', body })
          .then(() => { modal.close(); return load() })
          .catch(error => modal.setError(String(error)))
          .finally(() => modal.setBusy(false))
      },
    })
    editor.open({
      code: current?.code ?? '',
      name: current?.name ?? '',
      level: current?.level ?? 0,
      sort: current?.sort ?? plans.length + 1,
      priceYuan: current ? (current.priceMinor / 100).toFixed(2) : '0.00',
      durationDays: current?.durationDays ?? 30,
      status: current?.status ?? 'ACTIVE',
      default: String(current?.default ?? plans.length === 0),
      description: current?.description ?? '',
    })
  }

  async function openPermissions(plan: Plan): Promise<void> {
    try {
      state.set(`正在读取 ${plan.name} 的权限配置…`)
      const policy = await api.request<PlanPermissionPolicy>(`${prefix}/plans/${plan.id}/permissions`)
      const modal = hostFormModal({
        title: `权限配置 · ${plan.name}`,
        fields: [
          { name: 'permissionCodes', label: '授权权限', kind: 'checkbox-tree', tree: permissionTree(permissions), wide: true, empty: '当前没有可授权的活动权限' },
          { name: 'productLimit', label: '商品数量上限（0=显式不限额）', type: 'number', min: 0, required: true },
        ],
        onSubmit: (values, current) => {
          const productLimit = Number(values.productLimit)
          if (!Number.isInteger(productLimit) || productLimit < 0) {
            current.setError('商品数量上限必须是非负整数；0 表示显式不限额。')
            return
          }
          const selected = permissionCodes(values.permissionCodes)
          const active = new Set(permissions.map(item => item.code))
          const inactive = selected.filter(code => !active.has(code))
          if (inactive.length) {
            current.setError(`以下权限不在活动 Registry：${inactive.join(', ')}`)
            return
          }
          current.setBusy(true)
          api.request(`${prefix}/plans/${plan.id}/permissions`, {
            method: 'PUT',
            body: JSON.stringify({
              commandKey: crypto.randomUUID(),
              expectedRevision: policy.revision,
              permissionCodes: selected,
              productLimit: productLimit === 0 ? null : productLimit,
            }),
          })
            .then(() => { current.close(); return load() })
            .catch(error => current.setError(String(error)))
            .finally(() => current.setBusy(false))
        },
      })
      modal.open({ permissionCodes: policy.permissionCodes.join('\n'), productLimit: policy.productLimit ?? 0 })
      state.set(`已打开 ${plan.name} 的权限配置 · revision ${policy.revision}`)
    } catch (error) {
      state.set(`权限配置加载失败：${String(error)}`, 'danger')
    }
  }

  function openRetire(plan: Plan): void {
    const modal = hostFormModal({
      title: `退役套餐 · ${plan.name}`,
      fields: [{ name: 'confirm', label: '此操作不可恢复', kind: 'select', options: [{ value: '', label: '请选择' }, { value: plan.code, label: `确认退役 ${plan.code}` }], required: true }],
      submitLabel: '退役',
      onSubmit: (values, current) => {
        if (values.confirm !== plan.code) return
        current.setBusy(true)
        api.request(`${prefix}/plans/${plan.id}/retire`, { method: 'POST', body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: plan.version }) })
          .then(() => { current.close(); return load() })
          .catch(error => current.setError(String(error)))
          .finally(() => current.setBusy(false))
      },
    })
    modal.open()
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [
      dataCard({
        title: '套餐管理',
        actions: [button({ label: '刷新', variant: 'secondary', onClick: () => void load() }), button({ label: '新建套餐', onClick: () => openEditor() })],
        status: state.element,
        body: planTable.element,
      }),
      dataCard({ title: 'Platform Registry 活动权限目录（只读）', body: permissionTable.element }),
    ],
  }))
  await load()
}
