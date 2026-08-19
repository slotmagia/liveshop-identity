import type { HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, definitionList, page, statusLine } from '@liveshops/design-tokens'

interface Credential {
  id: number
  version: number
  kind: string
  identifier: string
  status: string
}

interface AccountSecurity {
  subject: string
  displayName: string
  account: string
  principalType: string
  owner: boolean
  status: string
  credential: Credential
  activeSessions: number
}

interface ChangeOwnCredentialResult {
  credential: Credential
  revokedSessions: number
  currentRetained: boolean
  replayed: boolean
}

const securityPath = '/merch/identity/account/security'
const credentialsPath = '/merch/identity/account/credentials'

const principalLabels: Record<string, string> = {
  MERCHANT_OWNER: '商户所有者',
  MERCHANT_STAFF: '员工',
  SHOP_ANCHOR: '主播',
}

function statusBadge(status: string): HTMLElement {
  if (status === 'ACTIVE') return badge({ label: '启用', tone: 'success' })
  if (status === 'DISABLED') return badge({ label: '停用', tone: 'warning' })
  return badge({ label: status || '—', tone: 'neutral' })
}

function canChangePassword(value?: AccountSecurity): boolean {
  return Boolean(value && value.credential?.id && value.credential.status === 'ACTIVE')
}

export async function renderSecurity(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const body = create('div', 'identity-account-body')
  let current: AccountSecurity | undefined
  const changeButton = button({ label: '修改密码', onClick: () => openChangePassword(), disabled: true })

  function render(value: AccountSecurity): void {
    const heading = create('div', 'identity-account-heading')
    heading.append(create('h3', 'identity-plan-name', value.displayName || value.account || '当前账号'))
    const marks = create('div', 'identity-plan-meta')
    marks.append(badge({
      label: principalLabels[value.principalType] || value.principalType || '未知身份',
      tone: 'info',
    }))
    marks.append(statusBadge(value.status))
    if (value.owner) marks.append(badge({ label: '所有者', tone: 'success' }))
    heading.append(marks)
    body.replaceChildren(
      heading,
      definitionList([
        { label: '登录账号', value: value.account || '—' },
        { label: '凭据版本', value: value.credential?.id ? String(value.credential.version) : '—' },
        { label: '凭据状态', value: value.credential?.id ? statusBadge(value.credential.status) : '无密码凭据' },
        { label: '活动会话', value: String(value.activeSessions ?? 0) },
        { label: '主体', value: value.subject || '—' },
      ]),
    )
  }

  async function load(): Promise<void> {
    state.set('正在加载账号安全…')
    try {
      current = await api.request<AccountSecurity>(securityPath)
      current.credential ||= { id: 0, version: 0, kind: '', identifier: '', status: '' }
      render(current)
      changeButton.disabled = !canChangePassword(current)
      const role = principalLabels[current.principalType] || current.principalType || '当前账号'
      const credential = canChangePassword(current) ? `凭据版本 ${current.credential.version}` : '没有可修改的密码凭据'
      state.set(`${current.displayName || current.account || '当前账号'} · ${role} · ${credential}`)
    } catch (error) {
      current = undefined
      changeButton.disabled = true
      body.replaceChildren()
      state.set(`账号安全加载失败：${String(error)}`, 'danger')
    }
  }

  function openChangePassword(): void {
    if (!current || !canChangePassword(current)) {
      state.set('当前账号没有可修改的密码凭据。', 'warning')
      return
    }
    const snapshot = current
    const modal = hostFormModal({
      title: '修改密码',
      fields: [
        { name: 'oldPassword', label: '当前密码', kind: 'input', type: 'password', required: true, autocomplete: 'current-password' },
        { name: 'password', label: '新密码', kind: 'input', type: 'password', required: true, minLength: 8, autocomplete: 'new-password' },
        { name: 'confirmPassword', label: '确认新密码', kind: 'input', type: 'password', required: true, minLength: 8, autocomplete: 'new-password' },
      ],
      submitLabel: '保存',
      onSubmit: (values, editor) => {
        if ([...values.password].length < 8) {
          editor.setError('新密码至少 8 个字符。')
          return
        }
        if (values.password !== values.confirmPassword) {
          editor.setError('两次输入的新密码不一致。')
          return
        }
        if (values.password === values.oldPassword) {
          editor.setError('新密码不能与当前密码相同。')
          return
        }
        editor.setBusy(true)
        api.request<ChangeOwnCredentialResult>(credentialsPath, {
          method: 'PUT',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: snapshot.credential.version,
            oldPassword: values.oldPassword,
            password: values.password,
          }),
        })
          .then(async result => {
            editor.close()
            await load()
            const revoked = result.revokedSessions ? `已撤销 ${result.revokedSessions} 个其它会话` : '没有其它活动会话'
            const retained = result.currentRetained ? '当前登录已保留' : '当前登录未能保留'
            state.set(`密码已更新 · ${revoked} · ${retained}`, result.currentRetained ? 'success' : 'warning')
          })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open()
  }

  root.replaceChildren(page({
    showSummary: false,
    children: dataCard({
      title: '账号安全',
      actions: [
        button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
        changeButton,
      ],
      status: state.element,
      body,
    }),
  }))
  await load()
}
