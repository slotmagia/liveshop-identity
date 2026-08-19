import type { HostContext, HostHttpClient } from '@liveshops/host-sdk'
import { badge, button, create, dataCard, definitionList, page, statusLine } from '@liveshops/design-tokens'

// The page is transport-neutral: it receives an already-authorized client and
// never builds a URL or a token itself. A test can drive it with a stub client.

interface Health {
  status: string
}

// Paths are absolute and go through the Gateway, which routes /admin/identity
// to this module. They must match module.json, or the session will not cover them.
const HEALTH_PATH = '/admin/identity/health'

export async function renderHealth(
  container: HTMLElement,
  api: HostHttpClient,
  context: HostContext,
): Promise<void> {
  const status = statusLine()
  const details = create('div')

  // Every node comes from the shared kit, so this page inherits the console
  // look without declaring a single colour. Replace the body with the real
  // capability; keep building it the same way.
  async function load(): Promise<void> {
    status.set('加载中…')
    try {
      const health = await api.request<Health>(HEALTH_PATH)
      details.replaceChildren(definitionList([
        { label: '状态', value: badge({ label: health.status, tone: health.status === 'ok' ? 'success' : 'warning' }) },
        { label: 'Surface', value: context.surface },
        { label: '模块版本', value: context.moduleVersion },
      ]))
      status.set('已刷新')
    } catch (error) {
      // Show the failure rather than an empty page: a contribution that
      // silently renders nothing is indistinguishable from one that was never
      // granted permission.
      details.replaceChildren()
      status.set(error instanceof Error ? error.message : String(error), 'danger')
    }
  }

  container.replaceChildren(page({
    showSummary: false,
    children: dataCard({ title: '运行状态', actions: button({ label: '刷新', variant: 'secondary', onClick: () => void load() }), status: status.element, body: details }),
  }))
  await load()
}
