import type { HostHttpClient } from '@liveshops/host-sdk'
import { hostFormModal } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, page, statusLine, table, ui } from '@liveshops/design-tokens'

interface ShopCategory {
  id: number
  code: string
  name: string
  icon: string
  sort: number
  status: 'ACTIVE' | 'DISABLED'
  version: number
  usedShopCount: number
}

const prefix = '/admin/identity/shop-categories'
const codePattern = /^[a-z][a-z0-9_]{1,31}$/

function actions(...children: Node[]): HTMLElement {
  const node = create('div', ui.actions)
  node.append(...children)
  return node
}

export async function renderShopCategories(root: HTMLElement, api: HostHttpClient): Promise<void> {
  const state = statusLine()
  const categoryTable = table({
    columns: ['排序', '图标', '品类代码', '品类名称', '状态', '引用店铺', '版本', '操作'],
    empty: '暂无店铺品类',
  })
  let categories: ShopCategory[] = []

  async function load(): Promise<void> {
    state.set('正在加载店铺品类目录…')
    try {
      categories = await api.request<ShopCategory[]>(prefix)
      categoryTable.setRows(categories.map(category => [
        category.sort,
        category.icon || '—',
        category.code,
        category.name,
        badge({
          label: category.status === 'ACTIVE' ? '启用' : '停用',
          tone: category.status === 'ACTIVE' ? 'success' : 'warning',
        }),
        category.usedShopCount,
        category.version,
        actions(
          button({
            label: category.status === 'ACTIVE' ? '停用' : '启用',
            size: 'sm',
            variant: 'secondary',
            onClick: () => void setEnabled(category, category.status !== 'ACTIVE'),
          }),
          button({ label: '编辑', size: 'sm', variant: 'secondary', onClick: () => openEditor(category) }),
          button({ label: '退役', size: 'sm', variant: 'danger', onClick: () => openRetire(category) }),
        ),
      ]))
      const active = categories.filter(category => category.status === 'ACTIVE').length
      const referenced = categories.filter(category => category.usedShopCount > 0).length
      state.set(`品类 ${categories.length} 个 · 启用 ${active} 个 · 已被店铺引用 ${referenced} 个`)
    } catch (error) {
      categories = []
      categoryTable.setRows([])
      state.set(`店铺品类加载失败：${String(error)}`, 'danger')
    }
  }

  function openEditor(current?: ShopCategory): void {
    const modal = hostFormModal({
      title: current ? `编辑店铺品类 · ${current.name}` : '新增店铺品类',
      fields: [
        { name: 'code', label: '稳定品类代码', required: true, disabled: Boolean(current), mono: true, placeholder: 'apparel' },
        { name: 'name', label: '品类名称', required: true, placeholder: '服装服饰' },
        { name: 'icon', label: '图标（Emoji，可选）', placeholder: '👗' },
        { name: 'sort', label: '排序', type: 'number', min: 0, required: true },
        {
          name: 'status', label: '状态', kind: 'select', required: true,
          options: [{ value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' }],
        },
      ],
      onSubmit: (values, editor) => {
        const code = values.code.trim()
        const name = values.name.trim()
        const sort = Number(values.sort)
        if (!codePattern.test(code)) {
          editor.setError('品类代码必须以小写字母开头，只能包含小写字母、数字和下划线，长度为 2–32。')
          return
        }
        if (!name) {
          editor.setError('请输入品类名称。')
          return
        }
        if (!Number.isSafeInteger(sort) || sort < 0 || sort > 1_000_000) {
          editor.setError('排序必须是 0–1000000 的整数。')
          return
        }
        editor.setBusy(true)
        api.request(current ? `${prefix}/${current.id}` : prefix, {
          method: current ? 'PUT' : 'POST',
          body: JSON.stringify({
            commandKey: crypto.randomUUID(),
            expectedVersion: current?.version ?? 0,
            code,
            name,
            icon: values.icon.trim(),
            sort,
            status: values.status,
          }),
        })
          .then(() => { editor.close(); return load() })
          .catch(error => editor.setError(String(error)))
          .finally(() => editor.setBusy(false))
      },
    })
    modal.open({
      code: current?.code ?? '',
      name: current?.name ?? '',
      icon: current?.icon ?? '',
      sort: current?.sort ?? categories.length + 1,
      status: current?.status ?? 'ACTIVE',
    })
  }

  async function setEnabled(category: ShopCategory, enabled: boolean): Promise<void> {
    state.set(`正在${enabled ? '启用' : '停用'} ${category.name}…`)
    try {
      await api.request(`${prefix}/${category.id}/${enabled ? 'enable' : 'disable'}`, {
        method: 'POST',
        body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: category.version }),
      })
      await load()
    } catch (error) {
      state.set(`${enabled ? '启用' : '停用'}失败：${String(error)}`, 'danger')
    }
  }

  function openRetire(category: ShopCategory): void {
    const referenced = category.usedShopCount > 0
      ? `当前仍有 ${category.usedShopCount} 个店铺引用；退役只阻止新选择，不会删除历史引用。`
      : '退役后不再出现在管理列表和新建店铺可选目录中。'
    const modal = hostFormModal({
      title: `退役店铺品类 · ${category.name}`,
      fields: [{
        name: 'confirm',
        label: referenced,
        kind: 'select',
        required: true,
        options: [{ value: '', label: '请选择' }, { value: category.code, label: `确认退役 ${category.code}` }],
      }],
      submitLabel: '退役',
      onSubmit: (values, retireModal) => {
        if (values.confirm !== category.code) {
          retireModal.setError('请选择确认项。')
          return
        }
        retireModal.setBusy(true)
        api.request(`${prefix}/${category.id}/retire`, {
          method: 'POST',
          body: JSON.stringify({ commandKey: crypto.randomUUID(), expectedVersion: category.version }),
        })
          .then(() => { retireModal.close(); return load() })
          .catch(error => retireModal.setError(String(error)))
          .finally(() => retireModal.setBusy(false))
      },
    })
    modal.open()
  }

  root.replaceChildren(page({
    showSummary: false,
    children: [dataCard({
      title: '店铺经营品类目录',
      actions: [
        button({ label: '刷新', variant: 'secondary', onClick: () => void load() }),
        button({ label: '新增品类', onClick: () => openEditor() }),
      ],
      status: state.element,
      body: categoryTable.element,
    })],
  }))
  await load()
}
