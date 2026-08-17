import type { HostContext, HostHttpClient } from '@liveshop/host-sdk'
import { hostFormModal } from '@liveshop/host-sdk'
import { badge, button, dataCard, page, pagination, searchCard, searchForm, statusLine, table, ui } from '@liveshop/design-tokens'

interface Shop { id: number; merchantId: number; name: string; code: string; status: string }
interface Role { id: number; code: string; name: string; status: string; systemRole: boolean }
interface Unit { id: number; parentId: number; name: string; status: string }
interface Credential { id: number; version: number; kind: string; identifier: string; status: string }
interface Member {
  id: number
  subject: string
  displayName: string
  type: 'STAFF' | 'ANCHOR'
  status: 'ACTIVE' | 'DISABLED'
  memberStatus: string
  principalType: string
  accessVersion: number
  subjectVersion: number
  credential: Credential
  roleIds: number[]
  unitIds: number[]
  shopIds: number[]
  activeSessions: number
}
interface MemberPage { items: Member[]; page: number; pageSize: number; total: number }
interface MemberOptions { shops: Shop[]; roles: Role[]; units: Unit[] }
interface Session { id: string; deviceName: string; ipAddress: string; status: string }

const prefix = '/merch/identity/members'
const typeLabels: Record<Member['type'], string> = { STAFF: '员工', ANCHOR: '主播' }

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

function ids(value: string): number[] {
  return [...new Set(value.split(/[\s,]+/).map(item => Number(item.trim())).filter(item => Number.isSafeInteger(item) && item > 0))]
}

function namedList(ids: number[], labels: Map<number, string>): string {
  if (!ids.length) return '—'
  return ids.map(id => labels.get(id) ?? `#${id}`).join('、')
}

function statusBadge(status: Member['status']): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  return badge({ label: '停用', tone: 'warning' })
}

function optionTree(
  items: Array<{ id: number; name: string; code?: string }>,
  groupId: string,
  groupLabel: string,
): Array<{ id: string; label: string; children: Array<{ id: string; value: string; label: string }> }> {
  return [{
    id: groupId,
    label: groupLabel,
    children: items.map(item => ({
      id: `${groupId}:${item.id}`,
      value: String(item.id),
      label: item.code ? `${item.name} · ${item.code}` : item.name,
    })),
  }]
}

export async function renderUsers(root: HTMLElement, api: HostHttpClient, context: HostContext): Promise<void> {
  const canManage = context.permissions.includes('identity.staff.manage')
  const canSessions = context.permissions.includes('identity.session.manage')
  const state = statusLine()
  const memberTable = table({
    columns: ['姓名', '登录账号', '类型', '角色', '店铺范围', '状态', '活动会话', '操作'],
    empty: '暂无员工或主播',
  })
  let options: MemberOptions = { shops: [], roles: [], units: [] }
  let rows: Member[] = []
  let currentPage = 1
  let currentPageSize = 20

  const pager = pagination({
    pageSize: currentPageSize,
    onPageChange: value => { currentPage = value; void loadMembers() },
    onPageSizeChange: value => { currentPage = 1; currentPageSize = value; void loadMembers() },
  })

  const filter = searchForm({
    fields: [
      { name: 'keyword', label: '关键字', placeholder: '姓名或登录账号' },
      { name: 'type', label: '类型', kind: 'select', options: [
        { value: '', label: '全部类型' },
        { value: 'STAFF', label: '员工' },
        { value: 'ANCHOR', label: '主播' },
      ] },
      { name: 'status', label: '状态', kind: 'select', options: [
        { value: '', label: '全部状态' },
        { value: 'ACTIVE', label: '启用' },
        { value: 'DISABLED', label: '停用' },
      ] },
      { name: 'shopId', label: '店铺', kind: 'select', options: [{ value: '', label: '全部店铺' }] },
    ],
    searchLabel: '查询',
    onSearch: async () => {
      currentPage = 1
      await loadMembers()
    },
    onReset: () => {
      currentPage = 1
      currentPageSize = 20
      filter.set({ keyword: '', type: '', status: '', shopId: '' })
      void loadMembers()
    },
  })
  const shopSelect = filter.control('shopId') as HTMLSelectElement | undefined

  function shopLabels(): Map<number, string> {
    return new Map(options.shops.map(item => [item.id, item.name]))
  }

  function roleLabels(): Map<number, string> {
    return new Map(options.roles.map(item => [item.id, item.name]))
  }

  function renderRows(): void {
    memberTable.setRows(rows.map(item => {
      const buttons = []
      if (canManage) {
        buttons.push(button({ label: '编辑', size: 'sm', onClick: () => openEditor(item) }))
        buttons.push(button({
          label: item.status === 'ACTIVE' ? '停用' : '启用',
          size: 'sm',
          variant: 'secondary',
          onClick: () => openStatus(item),
        }))
        buttons.push(button({ label: '重置密码', size: 'sm', variant: 'secondary', onClick: () => openReset(item) }))
      }
      if (canSessions) {
        buttons.push(button({ label: '会话', size: 'sm', variant: 'secondary', onClick: () => void openSessions(item) }))
      }
      return [
        item.displayName,
        item.credential.identifier || '—',
        typeLabels[item.type] ?? item.type,
        namedList(item.roleIds ?? [], roleLabels()),
        namedList(item.shopIds ?? [], shopLabels()),
        statusBadge(item.status),
        String(item.activeSessions ?? 0),
        actions(...buttons),
      ]
    }))
  }

  async function loadOptions(): Promise<void> {
    options = await api.request<MemberOptions>(`${prefix}/options`)
    setSelectOptions(shopSelect, options.shops.map(item => ({
      value: String(item.id),
      label: `${item.name} · ${item.code} · shop_id ${item.id}`,
    })), '全部店铺')
  }

  async function loadMembers(): Promise<void> {
    state.set('正在加载员工…')
    try {
      const values = filter.values()
      const query = new URLSearchParams({
        page: String(currentPage),
        pageSize: String(currentPageSize),
      })
      if (values.keyword) query.set('keyword', values.keyword)
      if (values.type) query.set('type', values.type)
      if (values.status) query.set('status', values.status)
      if (values.shopId) query.set('shopId', values.shopId)
      const pageResult = await api.request<MemberPage>(`${prefix}?${query}`)
      rows = pageResult.items ?? []
      currentPage = pageResult.page
      currentPageSize = pageResult.pageSize
      renderRows()
      pager.set({ page: pageResult.page, pageSize: pageResult.pageSize, total: pageResult.total })
      state.set(`共 ${pageResult.total} 名员工 / 主播`)
    } catch (error) {
      rows = []
      renderRows()
      pager.set({ page: 1, pageSize: currentPageSize, total: 0 })
      state.set(`员工加载失败：${String(error)}`, 'danger')
    }
  }

  function memberFields(item?: Member) {
    const creating = !item
    return [
      { name: 'displayName', label: '姓名', required: true, value: item?.displayName ?? '' },
      { name: 'username', label: '登录账号', required: creating, disabled: !creating, value: item?.credential.identifier ?? '' },
      { name: 'password', label: creating ? '初始密码' : '密码', kind: 'input' as const, type: 'password', required: creating, disabled: !creating, minLength: 8 },
      {
        name: 'memberType',
        label: '类型',
        kind: 'select' as const,
        required: true,
        disabled: !creating,
        options: [
          { value: 'STAFF', label: '员工' },
          { value: 'ANCHOR', label: '主播' },
        ],
        value: item?.type ?? 'STAFF',
      },
      {
        name: 'shopIds',
        label: '店铺范围',
        kind: 'checkbox-tree' as const,
        tree: optionTree(options.shops, 'shops', '店铺'),
        wide: true,
        columns: 2 as const,
        empty: '当前商户没有可分配店铺',
        value: (item?.shopIds ?? []).join(','),
      },
      {
        name: 'roleIds',
        label: '角色',
        kind: 'checkbox-tree' as const,
        tree: optionTree(options.roles, 'roles', '角色'),
        wide: true,
        columns: 2 as const,
        empty: '没有可分配的活动角色，请先在角色管理中创建',
        value: (item?.roleIds ?? []).join(','),
      },
      {
        name: 'unitIds',
        label: '组织单元（可选）',
        kind: 'checkbox-tree' as const,
        tree: optionTree(options.units, 'units', '组织单元'),
        wide: true,
        empty: '暂无组织单元',
        value: (item?.unitIds ?? []).join(','),
      },
    ]
  }

  function validateAccess(memberType: string, shopIds: number[], roleIds: number[]): string {
    if (!roleIds.length) return '至少选择一个角色。'
    if (memberType === 'ANCHOR' && shopIds.length !== 1) return '主播必须且只能分配 1 家店铺。'
    if (memberType === 'STAFF' && shopIds.length < 1) return '员工至少分配 1 家店铺。'
    return ''
  }

  function openEditor(item?: Member): void {
    if (!options.roles.length) {
      state.set('没有可分配的活动角色，请先在角色管理中创建。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: item ? `编辑 ${item.displayName}` : '新建员工 / 主播',
      fields: memberFields(item),
      onSubmit: (values, editor) => {
        const displayName = values.displayName.trim()
        const memberType = values.memberType === 'ANCHOR' ? 'ANCHOR' : 'STAFF'
        const shopIds = ids(values.shopIds ?? '')
        const roleIds = ids(values.roleIds ?? '')
        const unitIds = ids(values.unitIds ?? '')
        if (!displayName) {
          editor.setError('请填写姓名。')
          return
        }
        const accessError = validateAccess(memberType, shopIds, roleIds)
        if (accessError) {
          editor.setError(accessError)
          return
        }
        if (!item) {
          const username = values.username.trim()
          const password = values.password
          if (!username) {
            editor.setError('请填写登录账号。')
            return
          }
          if ([...password].length < 8) {
            editor.setError('密码至少 8 个字符。')
            return
          }
          const id = crypto.randomUUID()
          editor.setBusy(true)
          api.request(prefix, {
            method: 'POST',
            body: JSON.stringify({
              idempotencyKey: id,
              operationId: id,
              displayName,
              username,
              password,
              memberType,
              shopIds,
              roleIds,
              unitIds,
            }),
          })
            .then(() => { editor.close(); return loadMembers() })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
          return
        }
        const id = crypto.randomUUID()
        editor.setBusy(true)
        api.request(`${prefix}/${encodeURIComponent(item.subject)}`, {
          method: 'PUT',
          body: JSON.stringify({
            idempotencyKey: id,
            operationId: id,
            displayName,
            memberType,
            expectedIdentityVersion: item.subjectVersion,
            expectedAccessVersion: item.accessVersion,
            shopIds,
            roleIds,
            unitIds,
          }),
        })
          .then(() => { editor.close(); return loadMembers() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    if (item) {
      modal.open({
        displayName: item.displayName,
        username: item.credential.identifier,
        memberType: item.type,
        shopIds: (item.shopIds ?? []).join(','),
        roleIds: (item.roleIds ?? []).join(','),
        unitIds: (item.unitIds ?? []).join(','),
      })
      return
    }
    modal.open({ memberType: 'STAFF' })
  }

  function openStatus(item: Member): void {
    const enable = item.status !== 'ACTIVE'
    const modal = hostFormModal({
      title: `${enable ? '启用' : '停用'} ${item.displayName}`,
      fields: [{
        name: 'confirm',
        label: enable ? '启用后可重新登录。' : '停用后立即撤销活动会话。',
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: item.subject, label: `确认${enable ? '启用' : '停用'}` }],
      }],
      submitLabel: enable ? '启用' : '停用',
      onSubmit: (values, editor) => {
        if (values.confirm !== item.subject) {
          editor.setError('请选择确认项。')
          return
        }
        const id = crypto.randomUUID()
        editor.setBusy(true)
        api.request(`${prefix}/${encodeURIComponent(item.subject)}/${enable ? 'enable' : 'disable'}`, {
          method: 'POST',
          body: JSON.stringify({
            idempotencyKey: id,
            operationId: id,
            expectedIdentityVersion: item.subjectVersion,
            expectedAccessVersion: item.accessVersion,
          }),
        })
          .then(() => { editor.close(); return loadMembers() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  function openReset(item: Member): void {
    if (!item.credential.id) {
      state.set('该成员没有可重置的登录凭据。', 'warning')
      return
    }
    const modal = hostFormModal({
      title: `重置 ${item.displayName} 的密码`,
      fields: [{ name: 'password', label: '新密码', kind: 'input', type: 'password', required: true, minLength: 8 }],
      submitLabel: '重置',
      onSubmit: (values, editor) => {
        if ([...values.password].length < 8) {
          editor.setError('密码至少 8 个字符。')
          return
        }
        const id = crypto.randomUUID()
        editor.setBusy(true)
        api.request(`${prefix}/${encodeURIComponent(item.subject)}/credentials/${item.credential.id}/reset`, {
          method: 'POST',
          body: JSON.stringify({
            idempotencyKey: id,
            operationId: id,
            password: values.password,
            expectedCredentialVersion: item.credential.version,
          }),
        })
          .then(() => { editor.close(); return loadMembers() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  async function openSessions(item: Member): Promise<void> {
    try {
      const values = await api.request<Session[]>(`${prefix}/${encodeURIComponent(item.subject)}/sessions`)
      const optionsList = ['全部活动会话', ...values.map(session => `${session.id} · ${session.status} · ${session.deviceName || '未知设备'} · ${session.ipAddress || '-'}`)]
      const modal = hostFormModal({
        title: `${item.displayName} · 会话`,
        fields: [{ name: 'session', label: '选择要撤销的会话', kind: 'select', options: optionsList, required: true }],
        submitLabel: '撤销',
        onSubmit: (form, editor) => {
          const selected = form.session === '全部活动会话' ? '' : form.session.split(' · ')[0]
          const id = crypto.randomUUID()
          const path = selected
            ? `${prefix}/${encodeURIComponent(item.subject)}/sessions/${encodeURIComponent(selected)}/revoke`
            : `${prefix}/${encodeURIComponent(item.subject)}/sessions/revoke-all`
          editor.setBusy(true)
          api.request(path, { method: 'POST', body: JSON.stringify({ idempotencyKey: id, operationId: id }) })
            .then(() => { editor.close(); return loadMembers() })
            .catch(error => editor.setError(String(error)))
            .finally(() => editor.setBusy(false))
        },
      })
      modal.open()
    } catch (error) {
      state.set(String(error), 'danger')
    }
  }

  const toolbar = [button({ label: '刷新', variant: 'secondary', onClick: () => void loadMembers() })]
  if (canManage) toolbar.push(button({ label: '新建', onClick: () => openEditor() }))

  root.replaceChildren(page({
    showSummary: false,
    children: [
      searchCard(filter.element),
      dataCard({
        title: '员工与主播',
        actions: toolbar,
        status: state.element,
        body: memberTable.element,
        footer: pager.element,
      }),
    ],
  }))

  try {
    await loadOptions()
    await loadMembers()
  } catch (error) {
    options = { shops: [], roles: [], units: [] }
    rows = []
    renderRows()
    state.set(`用户管理页面加载失败：${String(error)}`, 'danger')
  }
}
